// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package runner provides shared WebSocket reconnect helpers for edge daemons.
package runner

import (
	"context"
	"log/slog"
	"time"
)

// WSConnection defines the minimum interface required for a WebSocket connection that can be closed.
type WSConnection interface {
	Close() error
}

// WSReconnectConfig specifies configuration parameters for the WebSocket reconnect loop.
type WSReconnectConfig struct {
	ComponentName  string
	ConnectBackoff time.Duration
	ReconnectDelay time.Duration
	OnShutdown     func()
}

// RunWSReconnectLoop runs a loop that attempts to keep a WebSocket connection active, automatically reconnecting when closed or failed.
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
			// Continue reconnect loop
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

// SleepContext pauses execution for the given duration or until the context is canceled.
func SleepContext(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
