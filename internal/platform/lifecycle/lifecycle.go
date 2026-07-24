// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package lifecycle manages global application and business component shutdown hooks.
package lifecycle

import (
	"context"
	"log"
	"sync"
)

// ShutdownFunc defines the signature for a graceful shutdown callback.
type ShutdownFunc func(ctx context.Context) error

type hook struct {
	name string
	fn   ShutdownFunc
}

var (
	hooks []hook
	mu    sync.Mutex
)

// OnShutdown registers a callback to be run during graceful shutdown.
func OnShutdown(name string, fn ShutdownFunc) {
	mu.Lock()
	defer mu.Unlock()
	hooks = append(hooks, hook{name: name, fn: fn})
}

// Stop executes all registered shutdown hooks concurrently and waits for completion or context timeout.
func Stop(ctx context.Context) {
	mu.Lock()
	localHooks := make([]hook, len(hooks))
	copy(localHooks, hooks)
	mu.Unlock()

	var wg sync.WaitGroup
	for _, h := range localHooks {
		wg.Add(1)
		go func(name string, fn ShutdownFunc) {
			defer wg.Done()
			log.Printf("[Lifecycle] stopping %s...\n", name)
			if err := fn(ctx); err != nil {
				log.Printf("[Lifecycle] stop %s failed: %v\n", name, err)
			} else {
				log.Printf("[Lifecycle] %s stopped successfully\n", name)
			}
		}(h.name, h.fn)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("[Lifecycle] all services stopped gracefully")
	case <-ctx.Done():
		log.Printf("[Lifecycle] shutdown timed out: %v\n", ctx.Err())
	}
}
