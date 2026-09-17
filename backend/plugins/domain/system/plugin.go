// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package system provides core system health probes, public config endpoints, and static frontend assets dispatch plugin for Cordis.
package system

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Plugin implements core.Plugin to provide system-level basic routes.
type Plugin struct{}

// New creates a new system domain plugin.
func New() *Plugin {
	return &Plugin{}
}

// Name returns the unique identifier for the system domain plugin.
func (p *Plugin) Name() string {
	return "system"
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "system",
		Version:     "1.0.0",
		Description: "System health check, public config, and assets domain plugin",
		Author:      "Wavelet Team",
	}
}

// Apply registers system routes.
func (p *Plugin) Apply(ctx *core.Context) error {
	// 1. Health check
	ctx.Router().GET("/api/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	ctx.Router().RegisterWhitelist("/api/healthz")

	// 2. Public config — owned data comes from PublicConfigProvider (admin).
	ctx.Router().GET("/api/v1/config/public", func(c *gin.Context) {
		provider := resolvePublicConfigProvider(c.Request.Context(), ctx)
		if provider == nil {
			c.JSON(http.StatusOK, response.OK(map[string]string{}))
			return
		}
		data, err := provider.PublicConfig(c.Request.Context())
		if err != nil {
			logger.ErrorF(c.Request.Context(), "[System] public config provider failed: %v", err)
			response.AbortInternal(c, "public config unavailable")
			return
		}
		if data == nil {
			data = map[string]string{}
		}
		c.JSON(http.StatusOK, response.OK(data))
	})
	ctx.Router().RegisterWhitelist("/api/v1/config/public")

	return nil
}

func resolvePublicConfigProvider(reqCtx context.Context, appCtx *core.Context) contracts.PublicConfigProvider {
	if p, err := core.InjectFrom[contracts.PublicConfigProvider](reqCtx); err == nil && p != nil {
		return p
	}
	if appCtx != nil {
		if p, err := core.Inject[contracts.PublicConfigProvider](appCtx); err == nil && p != nil {
			return p
		}
	}
	return nil
}
