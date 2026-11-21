// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ory/fosite"
)

const (
	directTokenPrefix     = "ofm_"
	directTokenLifetime   = 30 * 24 * time.Hour
	errSettingsLoad       = "Could not load automation settings"
	errSettingsPermission = "An administrator session is required"
	errTokenScopes        = "Select at least one available permission"
	errTokenCreate        = "Could not create the MCP access token"
	errRevokeGrants       = "Could not revoke automation access"
)

type mcpSettingsResponse struct {
	Enabled    bool     `json:"enabled"`
	ClientName string   `json:"client_name,omitempty"`
	MCPURL     string   `json:"mcp_url,omitempty"`
	InstanceID string   `json:"instance_id,omitempty"`
	Scopes     []string `json:"scopes,omitempty"`
}

type createMCPTokenRequest struct {
	Scopes []string `json:"scopes" binding:"required,max=32,dive,required"`
}

type mcpTokenResponse struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
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
	if err != nil {
		settingsError(c, errSettingsLoad, err)
		return
	}
	id := site.id()
	c.JSON(http.StatusOK, response.OK(mcpSettingsResponse{
		Enabled: true, ClientName: "openflare-" + id[3:11], MCPURL: site.mcpResource(),
		InstanceID: id, Scopes: availableScopes(),
	}))
}

// @Summary Create a scoped MCP access token
// @Tags admin-automation
// @Accept json
// @Produce json
// @Param body body createMCPTokenRequest true "Requested permissions"
// @Success 200 {object} response.Any{data=mcpTokenResponse}
// @Failure 400 {object} response.Any
// @Failure 401 {object} response.Any
// @Failure 403 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /api/v1/admin/automation/mcp/token [post]
func createMCPToken(c *gin.Context) {
	userID, ok := adminSettingsSession(c)
	if !ok {
		return
	}
	site, err := loadInstance(c.Request.Context())
	if errors.Is(err, errDisabled) {
		response.AbortNotFound(c, "MCP is disabled")
		return
	}
	if err != nil {
		settingsError(c, errSettingsLoad, err)
		return
	}
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		response.AbortBadRequest(c, errTokenScopes)
		return
	}
	var body createMCPTokenRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxOAuthBody)
	if err := c.ShouldBindJSON(&body); err != nil {
		response.AbortBadRequest(c, errTokenScopes)
		return
	}
	scopes, ok := selectedScopes(strings.Join(body.Scopes, " "), availableScopes())
	if !ok {
		response.AbortBadRequest(c, errTokenScopes)
		return
	}
	token := randomCredential(directTokenPrefix)
	expires := time.Now().UTC().Add(directTokenLifetime)
	request := fosite.NewRequest()
	request.SetID(uuid.NewString())
	request.Session = &fosite.DefaultSession{Subject: strconv.FormatUint(userID, 10)}
	request.Session.SetExpiresAt(fosite.AccessToken, expires)
	request.GrantAudience(site.mcpResource())
	for _, scope := range scopes {
		request.GrantScope(scope)
	}
	if err := repository.PruneAutomationSessions(c.Request.Context(), time.Now().UTC()); err != nil {
		settingsError(c, errTokenCreate, err)
		return
	}
	if err := saveOAuthSession(c.Request.Context(), sessionDirect, token, request, fosite.AccessToken); err != nil {
		settingsError(c, errTokenCreate, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(mcpTokenResponse{AccessToken: token, ExpiresAt: expires}))
}

// @Summary Revoke the current administrator's automation grants and MCP access tokens
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
	group.POST("/mcp/token", createMCPToken)
	group.DELETE("/grants", revokeBrowserGrants)
}
