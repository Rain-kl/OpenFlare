// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package apiutil

import (
	"github.com/Rain-kl/Wavelet/internal/apps/admin"
	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/gin-gonic/gin"
)

// AdminMiddlewares returns Wavelet-standard middlewares for OpenFlare console routes.
// OpenFlare no longer distinguishes Admin vs Root tiers; all management endpoints share
// the same gate: user.IsAdmin for session users, token_admin for Access Token callers.
func AdminMiddlewares() []gin.HandlerFunc {
	return []gin.HandlerFunc{oauth.LoginRequired(), admin.LoginAdminRequired()}
}