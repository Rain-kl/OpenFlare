// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"Wavelet/core"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type UserRegisteredEvent struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

type OrderCreatedEvent struct {
	OrderID string  `json:"order_id"`
	Amount  float64 `json:"amount"`
}

func TestEventBusPublishSubscribe(t *testing.T) {
	bus := core.NewEventBus()
	var receivedID string

	disposer := bus.On("user:registered", func(ctx context.Context, e UserRegisteredEvent) error {
		receivedID = e.UserID
		return nil
	})
	require.NotNil(t, disposer)

	assert.Equal(t, []string{"user:registered"}, bus.Topics())

	err := bus.Emit(context.Background(), "user:registered", UserRegisteredEvent{UserID: "u_999", Username: "alice"})
	assert.NoError(t, err)
	assert.Equal(t, "u_999", receivedID)

	// Emit to empty topic returns nil error
	err = bus.Emit(nil, "empty:topic", nil)
	assert.NoError(t, err)
}

func TestEventBusGenericSubscribe(t *testing.T) {
	bus := core.NewEventBus()
	var receivedOrder string

	assert.Panics(t, func() {
		core.Subscribe[OrderCreatedEvent](nil, "order:created", func(ctx context.Context, e OrderCreatedEvent) error {
			return nil
		})
	})

	disposer := core.Subscribe(bus, "order:created", func(ctx context.Context, e OrderCreatedEvent) error {
		receivedOrder = e.OrderID
		return nil
	})
	require.NotNil(t, disposer)

	err := bus.Emit(context.Background(), "order:created", OrderCreatedEvent{OrderID: "ord_123", Amount: 99.5})
	assert.NoError(t, err)
	assert.Equal(t, "ord_123", receivedOrder)

	// Test unsubscribe via disposer
	err = disposer()
	assert.NoError(t, err)

	receivedOrder = ""
	err = bus.Emit(context.Background(), "order:created", OrderCreatedEvent{OrderID: "ord_456", Amount: 100})
	assert.NoError(t, err)
	assert.Empty(t, receivedOrder, "handler should not be called after disposal")
}

func TestEventBusHandlerSignatures(t *testing.T) {
	bus := core.NewEventBus()

	var (
		calledWithCtxPayloadErr atomic.Bool
		calledWithCtxPayload    atomic.Bool
		calledWithPayloadErr    atomic.Bool
		calledWithPayload       atomic.Bool
		calledWithCtxErr        atomic.Bool
		calledWithCtx           atomic.Bool
		calledWithNoArgsErr     atomic.Bool
		calledWithNoArgs        atomic.Bool
	)

	bus.On("test:sig", func(ctx context.Context, e UserRegisteredEvent) error {
		calledWithCtxPayloadErr.Store(true)
		assert.Equal(t, "u_1", e.UserID)
		return nil
	})

	bus.On("test:sig", func(ctx context.Context, e UserRegisteredEvent) {
		calledWithCtxPayload.Store(true)
		assert.Equal(t, "u_1", e.UserID)
	})

	bus.On("test:sig", func(e UserRegisteredEvent) error {
		calledWithPayloadErr.Store(true)
		assert.Equal(t, "u_1", e.UserID)
		return nil
	})

	bus.On("test:sig", func(e UserRegisteredEvent) {
		calledWithPayload.Store(true)
		assert.Equal(t, "u_1", e.UserID)
	})

	bus.On("test:sig", func(ctx context.Context) error {
		calledWithCtxErr.Store(true)
		return nil
	})

	bus.On("test:sig", func(ctx context.Context) {
		calledWithCtx.Store(true)
	})

	bus.On("test:sig", func() error {
		calledWithNoArgsErr.Store(true)
		return nil
	})

	bus.On("test:sig", func() {
		calledWithNoArgs.Store(true)
	})

	err := bus.Emit(context.Background(), "test:sig", UserRegisteredEvent{UserID: "u_1", Username: "test"})
	assert.NoError(t, err)

	assert.True(t, calledWithCtxPayloadErr.Load())
	assert.True(t, calledWithCtxPayload.Load())
	assert.True(t, calledWithPayloadErr.Load())
	assert.True(t, calledWithPayload.Load())
	assert.True(t, calledWithCtxErr.Load())
	assert.True(t, calledWithCtx.Load())
	assert.True(t, calledWithNoArgsErr.Load())
	assert.True(t, calledWithNoArgs.Load())
}

func TestEventBusPointerAndValueConversion(t *testing.T) {
	bus := core.NewEventBus()

	var (
		receivedFromValueToPtr atomic.Bool
		receivedFromPtrToValue atomic.Bool
		receivedFromNilPtr     atomic.Bool
	)

	// Handler expects pointer, payload emitted as value
	bus.On("test:ptr", func(ctx context.Context, e *UserRegisteredEvent) error {
		if e != nil && e.UserID == "u_ptr" {
			receivedFromValueToPtr.Store(true)
		}
		return nil
	})

	err := bus.Emit(context.Background(), "test:ptr", UserRegisteredEvent{UserID: "u_ptr"})
	assert.NoError(t, err)
	assert.True(t, receivedFromValueToPtr.Load())

	// Handler expects value, payload emitted as pointer
	bus.On("test:val", func(ctx context.Context, e UserRegisteredEvent) error {
		if e.UserID == "u_val" {
			receivedFromPtrToValue.Store(true)
		}
		return nil
	})

	err = bus.Emit(context.Background(), "test:val", &UserRegisteredEvent{UserID: "u_val"})
	assert.NoError(t, err)
	assert.True(t, receivedFromPtrToValue.Load())

	// Handler expects value, payload is nil pointer
	var nilEvent *UserRegisteredEvent
	bus.On("test:nil_ptr", func(ctx context.Context, e UserRegisteredEvent) error {
		assert.Equal(t, "", e.UserID)
		receivedFromNilPtr.Store(true)
		return nil
	})
	err = bus.Emit(context.Background(), "test:nil_ptr", nilEvent)
	assert.NoError(t, err)
	assert.True(t, receivedFromNilPtr.Load())

	// Convertible type test (int to int64)
	var receivedConvert int64
	bus.On("test:conv", func(e int64) {
		receivedConvert = e
	})
	err = bus.Emit(context.Background(), "test:conv", int(42))
	assert.NoError(t, err)
	assert.Equal(t, int64(42), receivedConvert)
}

func TestEventBusErrorCollectionAndPanicRecovery(t *testing.T) {
	bus := core.NewEventBus()

	errHandler1 := errors.New("handler 1 failed")
	errHandler2 := errors.New("handler 2 failed")

	bus.On("test:err", func() error {
		return errHandler1
	})

	bus.On("test:err", func() {
		panic("something went horribly wrong")
	})

	bus.On("test:err", func() error {
		return errHandler2
	})

	err := bus.Emit(context.Background(), "test:err", nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errHandler1) || errors.Is(err, errHandler2))
	assert.Contains(t, err.Error(), "handler 1 failed")
	assert.Contains(t, err.Error(), "handler 2 failed")
	assert.Contains(t, err.Error(), "panic")
}

func TestEventBusInvalidHandlerPanics(t *testing.T) {
	bus := core.NewEventBus()

	assert.Panics(t, func() {
		bus.On("test:invalid", nil)
	})

	assert.Panics(t, func() {
		bus.On("test:invalid", "not-a-func")
	})

	assert.Panics(t, func() {
		// More than 2 arguments
		bus.On("test:invalid", func(a, b, c string) {})
	})

	assert.Panics(t, func() {
		// 2 args, but first is not context
		bus.On("test:invalid", func(a string, b int) {})
	})

	assert.Panics(t, func() {
		// More than 2 return values
		bus.On("test:invalid", func() (int, string, error) { return 0, "", nil })
	})

	assert.Panics(t, func() {
		// 2 return values, but second is not error
		bus.On("test:invalid", func() (int, string) { return 0, "" })
	})
}

func TestEventBusListenersCountAndDisposerIdempotence(t *testing.T) {
	bus := core.NewEventBus()

	assert.Equal(t, 0, bus.Listeners("topic1"))

	d1 := bus.On("topic1", func() {})
	d2 := bus.On("topic1", func() {})
	assert.Equal(t, 2, bus.Listeners("topic1"))

	_ = d1()
	assert.Equal(t, 1, bus.Listeners("topic1"))

	// Calling disposer again should be no-op
	_ = d1()
	assert.Equal(t, 1, bus.Listeners("topic1"))

	_ = d2()
	assert.Equal(t, 0, bus.Listeners("topic1"))
}

func TestEventBusConcurrentAccess(t *testing.T) {
	bus := core.NewEventBus()
	var wg sync.WaitGroup

	var receivedCount atomic.Int64

	// Concurrently subscribe and emit
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			topic := fmt.Sprintf("topic:%d", idx%5)
			disposer := bus.On(topic, func(ctx context.Context, e UserRegisteredEvent) error {
				receivedCount.Add(1)
				return nil
			})

			// Emit some events
			_ = bus.Emit(context.Background(), topic, UserRegisteredEvent{UserID: fmt.Sprintf("u_%d", idx)})

			// Randomly dispose
			if idx%2 == 0 {
				_ = disposer()
			}
		}(i)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			topic := fmt.Sprintf("topic:%d", idx%5)
			_ = bus.Emit(context.Background(), topic, UserRegisteredEvent{UserID: fmt.Sprintf("u_%d", idx)})
		}(i)
	}

	wg.Wait()
	assert.Greater(t, receivedCount.Load(), int64(0))
}

func TestEventBusWaterfall(t *testing.T) {
	bus := core.NewEventBus()

	// Handler 1: appends "-first"
	bus.On("pipeline:transform", func(ctx context.Context, s string) string {
		return s + "-first"
	})

	// Handler 2: appends "-second" with error return
	bus.On("pipeline:transform", func(s string) (string, error) {
		return s + "-second", nil
	})

	res, err := bus.Waterfall(context.Background(), "pipeline:transform", "init")
	assert.NoError(t, err)
	assert.Equal(t, "init-first-second", res)

	// Test short-circuit on error
	expectedErr := errors.New("waterfall step failed")
	bus.On("pipeline:error", func(s string) (string, error) {
		return s, expectedErr
	})
	bus.On("pipeline:error", func(s string) string {
		return s + "-should-not-run"
	})

	res, err = bus.Waterfall(context.Background(), "pipeline:error", "start")
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, res)
}

func TestEventBusParallel(t *testing.T) {
	bus := core.NewEventBus()

	var counter atomic.Int64
	err1 := errors.New("parallel err 1")

	bus.On("test:parallel", func(ctx context.Context, val int) error {
		counter.Add(int64(val))
		return nil
	})

	bus.On("test:parallel", func(val int) error {
		counter.Add(int64(val))
		return err1
	})

	err := bus.Parallel(context.Background(), "test:parallel", 10)
	assert.ErrorIs(t, err, err1)
	assert.Equal(t, int64(20), counter.Load())
}

func TestEventBusParallelContextTimeout(t *testing.T) {
	bus := core.NewEventBus()

	bus.On("test:timeout", func(ctx context.Context) error {
		select {
		case <-time.After(200 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := bus.Parallel(ctx, "test:timeout", nil)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestEventBusSerial(t *testing.T) {
	bus := core.NewEventBus()

	var executed []int
	errStop := errors.New("serial stop")

	bus.On("test:serial", func() error {
		executed = append(executed, 1)
		return nil
	})

	bus.On("test:serial", func() error {
		executed = append(executed, 2)
		return errStop
	})

	bus.On("test:serial", func() error {
		executed = append(executed, 3)
		return nil
	})

	err := bus.Serial(context.Background(), "test:serial", nil)
	assert.ErrorIs(t, err, errStop)
	assert.Equal(t, []int{1, 2}, executed)
}
