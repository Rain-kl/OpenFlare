// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package updater provides update service capabilities for flared.
package updater

import (
	edgeupdater "github.com/Rain-kl/Wavelet/internal/apps/edge/updater"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/config"
)

// Service is an alias for the edge updater Service.
type Service = edgeupdater.Service

// UpdateOptions is an alias for the edge updater UpdateOptions.
type UpdateOptions = edgeupdater.UpdateOptions

// New creates a new updater Service instance.
func New() *Service {
	return edgeupdater.New(edgeupdater.Config{
		LocalVersion: config.Version,
		AssetPrefix:  "openflared",
		LogLabel:     "flared",
	})
}
