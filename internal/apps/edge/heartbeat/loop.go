package heartbeat

import (
	"context"
	"time"
)

// RunLoop invokes fn immediately, then on each interval tick until ctx is cancelled.
func RunLoop(ctx context.Context, interval time.Duration, fn func(context.Context)) {
	fn(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fn(ctx)
		}
	}
}
