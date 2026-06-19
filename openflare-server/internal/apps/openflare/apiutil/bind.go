// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package apiutil provides HTTP helpers for OpenFlare v1 custom API handlers.
package apiutil

import (
	"strconv"

	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/gin-gonic/gin"
)

const errInvalidParams = "参数错误"
const errInvalidID = "无效的 ID"

// BindJSON binds JSON body; returns false after aborting with 400.
func BindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return false
	}
	return true
}

// IDParam parses :id from the URL path.
func IDParam(c *gin.Context) (uint, bool) {
	raw := c.Param("id")
	if raw == "" {
		response.AbortBadRequest(c, errInvalidID)
		return 0, false
	}
	id64, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id64 == 0 {
		response.AbortBadRequest(c, errInvalidID)
		return 0, false
	}
	return uint(id64), true
}