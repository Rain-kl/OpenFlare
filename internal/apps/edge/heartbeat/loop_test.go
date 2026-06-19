package heartbeat

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunLoopImmediateAndTicker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls atomic.Int32
	interval := 20 * time.Millisecond

	done := make(chan struct{})
	go func() {
		RunLoop(ctx, interval, func(context.Context) {
			calls.Add(1)
		})
		close(done)
	}()

	time.Sleep(5 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 immediate call, got %d", got)
	}

	time.Sleep(35 * time.Millisecond)
	if got := calls.Load(); got < 2 {
		t.Fatalf("expected at least 2 calls after ticker, got %d", got)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RunLoop did not exit after context cancellation")
	}
}