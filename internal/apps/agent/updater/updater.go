// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package updater provides agent self-update integration with the edge updater.
package updater

import (
	"github.com/Rain-kl/Wavelet/internal/apps/agent/config"
	edgeupdater "github.com/Rain-kl/Wavelet/internal/apps/edge/updater"
)

// Service is an alias for the edge updater service type used by the agent.
type Service = edgeupdater.Service

// UpdateOptions is an alias for the edge updater options type.
type UpdateOptions = edgeupdater.UpdateOptions

// New creates and returns a new agent updater Service with the agent-specific configuration.
func New() *Service {
	return edgeupdater.New(edgeupdater.Config{
		LocalVersion: config.Version,
		AssetPrefix:  "openflare-agent",
		LogLabel:     "agent",
	})
}
