// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
)

// PullConfigHandler fetches an exact advertised configuration resource.
// @Summary 拉取边缘配置资源
// @Tags openflare-agent
// @Produce json
// @Security AgentTokenAuth
// @Param key query string true "配置资源标识"
// @Param sha256 query string true "配置资源 SHA-256"
// @Success 200 {string} string "配置 JSON"
// @Failure 400 {object} response.Any
// @Failure 401 {object} response.Any
// @Failure 403 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 409 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /api/v1/agent/config/pull [get]
func PullConfigHandler(c *gin.Context) {
	node, ok := NodeFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errInvalidAgentToken)
		return
	}
	if node.Engine != enginePingora {
		response.AbortForbidden(c, errPingoraNodeRequired)
		return
	}
	key, hash := c.Query("key"), c.Query("sha256")
	digest, digestErr := hex.DecodeString(hash)
	if key == "" || len(key) > 128 || digestErr != nil || len(digest) != 32 {
		response.AbortBadRequest(c, "Invalid configuration resource key or SHA-256")
		return
	}
	body, err := pullConfigResource(c.Request.Context(), node, key, hash)
	switch {
	case errors.Is(err, errConfigResourceNotFound):
		response.AbortNotFound(c, errConfigResourceNotFound.Error())
	case errors.Is(err, errConfigResourceChanged):
		response.AbortConflict(c, errConfigResourceChanged.Error())
	case err != nil:
		logger.ErrorF(c.Request.Context(), "pull node configuration resource %s: %v", key, err)
		response.AbortInternal(c, "Could not load configuration resource")
	default:
		c.Header("Cache-Control", "private, no-store")
		c.Data(http.StatusOK, "application/json", []byte(body))
	}
}
