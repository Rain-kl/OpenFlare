// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package apiutil

import (
	"github.com/Rain-kl/Wavelet/internal/apps/admin"
	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/common"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/gin-gonic/gin"
)

// AdminRequired ensures the caller is logged in as a Wavelet administrator.
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := oauth.GetUserFromRequest(c)
		if err != nil {
			response.AbortUnauthorized(c, common.UnAuthorized)
			return
		}
		oauth.SetToContext(c, oauth.UserObjKey, user)

		if tokenAuth, _ := oauth.GetFromContext[bool](c, oauth.TokenAuthKey); tokenAuth {
			tokenAdmin, _ := oauth.GetFromContext[bool](c, oauth.TokenAdminKey)
			if !tokenAdmin {
				response.AbortNotFound(c, admin.TokenAdminRequired)
				return
			}
		}
		if !user.IsAdmin {
			response.AbortNotFound(c, admin.AdminRequired)
			return
		}
		c.Next()
	}
}