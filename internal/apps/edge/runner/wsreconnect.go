package runner

import (
	"context"
	"log/slog"
	"time"
)

type WSConnection interface {
	Close() error
}

type WSReconnectConfig struct {
	ComponentName  string
	ConnectBackoff time.Duration
	ReconnectDelay time.Duration
	OnShutdown     func()
}

func RunWSReconnectLoop(ctx context.Context, cfg WSReconnectConfig,
	connect func(context.Context) (WSConnection, error),
	handle func(context.Context, WSConnection),
) error {
	if cfg.ConnectBackoff <= 0 {
		cfg.ConnectBackoff = 5 * time.Second
	}
	if cfg.ReconnectDelay <= 0 {
		cfg.ReconnectDelay = 2 * time.Second
	}
	label := cfg.ComponentName
	if label == "" {
		label = "edge"
	}

	for {
		select {
		case <-ctx.Done():
			if cfg.OnShutdown != nil {
				cfg.OnShutdown()
			}
			return ctx.Err()
		default:
		}

		conn, err := connect(ctx)
		if err != nil {
			slog.Error(label+" ws connect failed, will retry", "error", err)
			SleepContext(ctx, cfg.ConnectBackoff)
			continue
		}

		handle(ctx, conn)
		_ = conn.Close()
		slog.Info(label + " ws connection closed, reconnecting...")
		SleepContext(ctx, cfg.ReconnectDelay)
	}
}

func SleepContext(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}