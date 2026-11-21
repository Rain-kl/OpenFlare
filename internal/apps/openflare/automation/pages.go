// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func browserAPIAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errLoginNeeded):
		oauthError(c, http.StatusUnauthorized, "login_required")
	case isInvalidCredential(err):
		oauthError(c, http.StatusForbidden, "access_denied")
	default:
		oauthInternalError(c, err)
	}
}

// RegisterBrowser redirects existing connection links to the system settings tab.
func RegisterBrowser(group *gin.RouterGroup) {
	group.GET("/connect", func(c *gin.Context) {
		c.Redirect(http.StatusSeeOther, "/admin/settings?tab=mcp")
	})
}
