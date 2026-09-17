// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
)

const maxHandlerParams = 2

var (
	ctxInterfaceType = reflect.TypeFor[context.Context]()
	errInterfaceType = reflect.TypeFor[error]()
)

type eventListener struct {
	id         uint64
	fnVal      reflect.Value
	numIn      int
	numOut     int
	hasCtx     bool
	hasPayload bool
	argType    reflect.Type
	returnsErr bool
	returnsVal bool
}

// EventBus is a thread-safe, strongly-typed in-process domain event bus supporting
// Emit, Waterfall, Parallel, and Serial dispatch semantics.
type EventBus struct {
	mu       sync.RWMutex
	nextID   atomic.Uint64
	handlers map[string][]eventListener
}

// NewEventBus creates a new EventBus instance.
func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[string][]eventListener),
	}
}

// On registers an event handler for the given topic.
//
// Supported handler signatures:
//   - func(ctx context.Context, event T) (T, error)
//   - func(ctx context.Context, event T) T
//   - func(event T) (T, error)
//   - func(event T) T
//   - func(ctx context.Context, event T) error
//   - func(ctx context.Context, event T)
//   - func(event T) error
//   - func(event T)
//   - func(ctx context.Context) error
//   - func(ctx context.Context)
//   - func() error
//   - func()
//
// Returns a Disposer function that unregisters the handler when called.
func (b *EventBus) On(topic string, handler any) Disposer {
	if handler == nil {
		panic("core/events: handler cannot be nil")
	}

	fnVal := reflect.ValueOf(handler)
	fnType := fnVal.Type()

	if fnType.Kind() != reflect.Func {
		panic(fmt.Sprintf("core/events: expected func, got %s", fnType.Kind()))
	}

	numIn := fnType.NumIn()
	if numIn > maxHandlerParams {
		panic(fmt.Sprintf("core/events: handler has %d parameters, maximum 2 supported (ctx, event)", numIn))
	}

	const maxHandlerReturnValues = 2
	numOut := fnType.NumOut()
	if numOut > maxHandlerReturnValues {
		panic(fmt.Sprintf("core/events: handler has %d return values, maximum 2 supported (value, error)", numOut))
	}

	returnsErr := false
	returnsVal := false

	switch numOut {
	case 1:
		out0 := fnType.Out(0)
		if out0.Implements(errInterfaceType) {
			returnsErr = true
		} else {
			returnsVal = true
		}
	case 2:
		out1 := fnType.Out(1)
		if !out1.Implements(errInterfaceType) {
			panic(fmt.Sprintf("core/events: second return value must be error, got %v", out1))
		}
		returnsVal = true
		returnsErr = true
	}

	listener := eventListener{
		id:         b.nextID.Add(1),
		fnVal:      fnVal,
		numIn:      numIn,
		numOut:     numOut,
		returnsErr: returnsErr,
		returnsVal: returnsVal,
	}

	switch numIn {
	case 0:
		// func() or func() error
	case 1:
		in0 := fnType.In(0)
		if in0.Implements(ctxInterfaceType) {
			listener.hasCtx = true
		} else {
			listener.hasPayload = true
			listener.argType = in0
		}
	case 2:
		in0 := fnType.In(0)
		if !in0.Implements(ctxInterfaceType) {
			panic(fmt.Sprintf("core/events: first parameter must implement context.Context, got %v", in0))
		}
		listener.hasCtx = true
		listener.hasPayload = true
		listener.argType = fnType.In(1)
	}

	b.mu.Lock()
	b.handlers[topic] = append(b.handlers[topic], listener)
	b.mu.Unlock()

	listenerID := listener.id
	var disposed atomic.Bool

	return func() error {
		if disposed.Swap(true) {
			return nil
		}

		b.mu.Lock()
		defer b.mu.Unlock()

		list := b.handlers[topic]
		for i, l := range list {
			if l.id == listenerID {
				b.handlers[topic] = append(list[:i], list[i+1:]...)
				break
			}
		}

		if len(b.handlers[topic]) == 0 {
			delete(b.handlers, topic)
		}

		return nil
	}
}

// Subscribe registers a strongly-typed generic event listener on the given EventBus.
func Subscribe[T any](bus *EventBus, topic string, handler func(ctx context.Context, event T) error) Disposer {
	if bus == nil {
		panic("core/events: nil EventBus provided to Subscribe")
	}
	return bus.On(topic, handler)
}

// Emit publishes an event to all subscribers of the specified topic.
// Handlers are executed synchronously. If any handler panics or returns an error,
// the error is collected and returned via errors.Join.
//
//nolint:contextcheck
func (b *EventBus) Emit(ctx context.Context, topic string, payload any) error {
	if ctx == nil {
		ctx = context.Background()
	}

	listeners := b.getListeners(topic)
	if len(listeners) == 0 {
		return nil
	}

	var payloadVal reflect.Value
	if payload != nil {
		payloadVal = reflect.ValueOf(payload)
	}

	var errs []error
	for _, l := range listeners {
		args := b.buildArgs(ctx, l, payloadVal)

		err := func() (resErr error) {
			defer func() {
				if r := recover(); r != nil {
					resErr = fmt.Errorf("core/events: panic in handler for topic %q: %v", topic, r)
				}
			}()

			results := l.fnVal.Call(args)
			if l.returnsErr {
				errIdx := l.numOut - 1
				if len(results) > errIdx && !results[errIdx].IsNil() {
					resErr = results[errIdx].Interface().(error)
				}
			}
			return resErr
		}()
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// Waterfall runs handlers sequentially as a transformation pipeline.
// The returned value of each handler becomes the payload input for the next handler.
// If any handler returns an error or panics, execution aborts immediately.
//
//nolint:contextcheck
func (b *EventBus) Waterfall(ctx context.Context, topic string, initialPayload any) (any, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	listeners := b.getListeners(topic)
	if len(listeners) == 0 {
		return initialPayload, nil
	}

	currentPayload := initialPayload

	for _, l := range listeners {
		var payloadVal reflect.Value
		if currentPayload != nil {
			payloadVal = reflect.ValueOf(currentPayload)
		}

		args := b.buildArgs(ctx, l, payloadVal)

		var stepVal any
		var stepErr error

		func() {
			defer func() {
				if r := recover(); r != nil {
					stepErr = fmt.Errorf("core/events: panic in waterfall handler for topic %q: %v", topic, r)
				}
			}()

			results := l.fnVal.Call(args)
			if l.returnsErr {
				errIdx := l.numOut - 1
				if len(results) > errIdx && !results[errIdx].IsNil() {
					stepErr = results[errIdx].Interface().(error)
				}
			}
			if stepErr == nil && l.returnsVal && len(results) > 0 {
				stepVal = results[0].Interface()
			}
		}()

		if stepErr != nil {
			return nil, stepErr
		}

		if l.returnsVal {
			currentPayload = stepVal
		}
	}

	return currentPayload, nil
}

// Parallel executes all subscribers of the topic concurrently in separate goroutines.
// It waits for all handlers to complete or returns immediately if ctx is cancelled/timed out,
// collecting any handler errors via errors.Join.
//
//nolint:contextcheck
func (b *EventBus) Parallel(ctx context.Context, topic string, payload any) error {
	if ctx == nil {
		ctx = context.Background()
	}

	listeners := b.getListeners(topic)
	if len(listeners) == 0 {
		return nil
	}

	var payloadVal reflect.Value
	if payload != nil {
		payloadVal = reflect.ValueOf(payload)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(listeners))

	for _, l := range listeners {
		wg.Add(1)
		go func(listener eventListener) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errCh <- fmt.Errorf("core/events: panic in parallel handler for topic %q: %v", topic, r)
				}
			}()

			args := b.buildArgs(ctx, listener, payloadVal)
			results := listener.fnVal.Call(args)
			if listener.returnsErr {
				errIdx := listener.numOut - 1
				if len(results) > errIdx && !results[errIdx].IsNil() {
					errCh <- results[errIdx].Interface().(error)
				}
			}
		}(l)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		close(errCh)
		var errs []error
		for err := range errCh {
			if err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Serial executes subscribers strictly in sequence.
// If any subscriber returns an error or panics, execution stops immediately and returns that error.
//
//nolint:contextcheck
func (b *EventBus) Serial(ctx context.Context, topic string, payload any) error {
	if ctx == nil {
		ctx = context.Background()
	}

	listeners := b.getListeners(topic)
	if len(listeners) == 0 {
		return nil
	}

	var payloadVal reflect.Value
	if payload != nil {
		payloadVal = reflect.ValueOf(payload)
	}

	for _, l := range listeners {
		args := b.buildArgs(ctx, l, payloadVal)

		var stepErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					stepErr = fmt.Errorf("core/events: panic in serial handler for topic %q: %v", topic, r)
				}
			}()

			results := l.fnVal.Call(args)
			if l.returnsErr {
				errIdx := l.numOut - 1
				if len(results) > errIdx && !results[errIdx].IsNil() {
					stepErr = results[errIdx].Interface().(error)
				}
			}
		}()

		if stepErr != nil {
			return stepErr
		}
	}

	return nil
}

func (b *EventBus) getListeners(topic string) []eventListener {
	b.mu.RLock()
	raw := b.handlers[topic]
	if len(raw) == 0 {
		b.mu.RUnlock()
		return nil
	}
	listeners := make([]eventListener, len(raw))
	copy(listeners, raw)
	b.mu.RUnlock()
	return listeners
}

func (b *EventBus) buildArgs(ctx context.Context, l eventListener, payloadVal reflect.Value) []reflect.Value {
	if l.numIn == 0 {
		return nil
	}

	args := make([]reflect.Value, 0, l.numIn)
	if l.hasCtx {
		args = append(args, reflect.ValueOf(ctx))
	}

	if l.hasPayload {
		arg := b.convertPayload(payloadVal, l.argType)
		args = append(args, arg)
	}

	return args
}

func (b *EventBus) convertPayload(payloadVal reflect.Value, targetType reflect.Type) reflect.Value {
	if !payloadVal.IsValid() {
		return reflect.Zero(targetType)
	}

	valType := payloadVal.Type()

	// 1. Direct assignable
	if valType.AssignableTo(targetType) {
		return payloadVal
	}

	// 2. Direct convertible
	if valType.ConvertibleTo(targetType) {
		return payloadVal.Convert(targetType)
	}

	// 3. Payload is pointer *T, target expects T
	if valType.Kind() == reflect.Pointer && valType.Elem().AssignableTo(targetType) {
		if !payloadVal.IsNil() {
			return payloadVal.Elem()
		}
		return reflect.Zero(targetType)
	}

	// 4. Payload is value T, target expects *T
	if targetType.Kind() == reflect.Pointer && valType.AssignableTo(targetType.Elem()) {
		ptr := reflect.New(valType)
		ptr.Elem().Set(payloadVal)
		return ptr
	}

	// Fallback to zero value of targetType
	return reflect.Zero(targetType)
}

// Listeners returns the number of active listeners for a topic.
func (b *EventBus) Listeners(topic string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[topic])
}

// Topics returns all topics that have registered listeners.
func (b *EventBus) Topics() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topics := make([]string, 0, len(b.handlers))
	for t := range b.handlers {
		topics = append(topics, t)
	}
	return topics
}
