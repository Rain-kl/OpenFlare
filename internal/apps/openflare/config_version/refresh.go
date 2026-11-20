// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package config_version assembles and synchronizes the live edge snapshot.
package config_version

import (
	"context"
	"net/http"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/platform/lifecycle"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
)

const snapshotPollInterval = 2 * time.Second

var refreshRequested = make(chan struct{}, 1)

// WakeAfterWrite schedules snapshot assembly after a successful control-plane write.
func WakeAfterWrite(c *gin.Context) {
	c.Next()
	if c.IsAborted() {
		return
	}
	switch c.Request.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		if c.Writer.Status() >= http.StatusOK && c.Writer.Status() < http.StatusMultipleChoices {
			RequestRefresh()
		}
	}
}

// RequestRefresh wakes the snapshot compiler after a committed write.
func RequestRefresh() {
	select {
	case refreshRequested <- struct{}{}:
	default:
	}
}

// StartRefreshLoop assembles changes immediately after API writes and polls for
// committed writes from workers and other controller instances.
func StartRefreshLoop(ctx context.Context) lifecycle.ShutdownFunc {
	loopCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(snapshotPollInterval)
		defer ticker.Stop()
		var lastError string
		for {
			err := RefreshSiteSnapshot(loopCtx)
			if err == nil {
				lastError = ""
			} else if loopCtx.Err() == nil && err.Error() != lastError {
				lastError = err.Error()
				logger.ErrorF(loopCtx, "refresh live edge snapshot: %v", err)
			}
			select {
			case <-loopCtx.Done():
				return
			case <-refreshRequested:
			case <-ticker.C:
			}
		}
	}()
	return func(ctx context.Context) error {
		cancel()
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// RefreshSiteSnapshot compiles committed configuration changes and notifies nodes.
func RefreshSiteSnapshot(ctx context.Context) error {
	ctx = db.WithPrimaryReads(ctx)
	state, err := repository.GetEdgeSnapshotState(ctx)
	if err != nil || state.SourceRevision == state.CompiledRevision {
		return err
	}
	bundle, err := buildCurrentConfigBundle(ctx)
	if err != nil {
		return err
	}
	changed, err := repository.StoreEdgeSnapshot(ctx, state.SourceRevision, &model.ConfigVersion{
		SnapshotJSON:     bundle.SnapshotJSON,
		SupportFilesJSON: bundle.SupportFilesJSON,
		Checksum:         bundle.Checksum,
		PayloadChecksum:  bundle.Checksum,
	})
	if changed {
		websocket.NotifyAgentConfigChanged(ctx)
	}
	return err
}
