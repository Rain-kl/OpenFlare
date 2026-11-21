// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"errors"
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
)

const (
	errSettingsLoad       = "Could not load automation settings"
	errSettingsPermission = "An administrator session is required"
	errRevokeGrants       = "Could not revoke automation access"
)

type mcpSettingsResponse struct {
	Enabled            bool   `json:"enabled"`
	ConfigurationError string `json:"configuration_error,omitempty"`
	ClientName         string `json:"client_name,omitempty"`
	MCPURL             string `json:"mcp_url,omitempty"`
	InstanceID         string `json:"instance_id,omitempty"`
}

func adminSettingsSession(c *gin.Context) (uint64, bool) {
	c.Header("Cache-Control", "no-store")
	user, err := browserAdmin(c)
	switch {
	case errors.Is(err, errLoginNeeded):
		response.AbortUnauthorized(c, errSettingsPermission)
	case isInvalidCredential(err):
		response.AbortForbidden(c, errSettingsPermission)
	case err != nil:
		settingsError(c, errSettingsLoad, err)
	default:
		return user.ID, true
	}
	return 0, false
}

func settingsError(c *gin.Context, msg string, err error) {
	logger.ErrorF(c.Request.Context(), "%s: %v", msg, err)
	response.AbortInternal(c, msg)
}

// @Summary Get MCP connection settings
// @Tags admin-automation
// @Produce json
// @Success 200 {object} response.Any{data=mcpSettingsResponse}
// @Failure 401 {object} response.Any
// @Failure 403 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /api/v1/admin/automation/mcp [get]
func getMCPSettings(c *gin.Context) {
	if _, ok := adminSettingsSession(c); !ok {
		return
	}
	site, err := loadInstance(c.Request.Context())
	if errors.Is(err, errDisabled) {
		c.JSON(http.StatusOK, response.OK(mcpSettingsResponse{}))
		return
	}
	if errors.Is(err, errShortSessionSecret) {
		c.JSON(http.StatusOK, response.OK(mcpSettingsResponse{ConfigurationError: "session_secret_too_short"}))
		return
	}
	if err != nil {
		settingsError(c, errSettingsLoad, err)
		return
	}
	id := site.id()
	c.JSON(http.StatusOK, response.OK(mcpSettingsResponse{
		Enabled: true, ClientName: "openflare-" + id[3:11], MCPURL: site.mcpResource(),
		InstanceID: id,
	}))
}

// @Summary Revoke the current administrator's OAuth automation grants
// @Tags admin-automation
// @Produce json
// @Success 200 {object} response.Any
// @Failure 401 {object} response.Any
// @Failure 403 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /api/v1/admin/automation/grants [delete]
func revokeBrowserGrants(c *gin.Context) {
	userID, ok := adminSettingsSession(c)
	if !ok {
		return
	}
	if err := repository.RevokeUserAutomationGrants(c.Request.Context(), userID); err != nil {
		settingsError(c, errRevokeGrants, err)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// RegisterSettings mounts administrator endpoints for the MCP settings tab.
func RegisterSettings(group *gin.RouterGroup) {
	group.GET("/mcp", getMCPSettings)
	group.DELETE("/grants", revokeBrowserGrants)
}
