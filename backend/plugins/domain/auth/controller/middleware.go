// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package controller provides HTTP handlers and middlewares for the auth plugin.
package controller

import (
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/pkg/ginutil"
	"Wavelet/pkg/response"
	"Wavelet/pkg/trace"
	"Wavelet/plugins/domain/auth/consts"
	"Wavelet/plugins/domain/auth/dao"
	"Wavelet/plugins/domain/auth/model/do"
	"Wavelet/plugins/domain/auth/model/dto"
	"Wavelet/plugins/domain/auth/service"
	"context"
	"errors"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// GetUserIDFromSession 从 Session 中提取用户 ID
func GetUserIDFromSession(s sessions.Session) uint64 {
	val := s.Get(consts.UserIDKey)
	return dto.ParseUserID(val)
}

// GetUserIDFromContext 从 Gin Context 的 Session 中提取用户 ID
func GetUserIDFromContext(c *gin.Context) (uid uint64) {
	defer func() {
		_ = recover()
	}()
	session := sessions.Default(c)
	return GetUserIDFromSession(session)
}

// CurrentUserIDFromRequestContext 是接入层向 Service 层暴露的登录态桥接。
func CurrentUserIDFromRequestContext(ctx context.Context) (uint64, bool) {
	ginCtx, ok := ctx.(*gin.Context)
	if !ok {
		return 0, false
	}
	return GetUserIDFromContext(ginCtx), true
}

func getUserByToken(ctx context.Context, d *dao.DAO, tokenStr string) (*contracts.UserDTO, *do.CachedToken, error) {
	tokenHash := service.HashToken(tokenStr)
	tokenRecord, err := d.GetCachedToken(ctx, tokenHash)
	if err != nil || tokenRecord == nil {
		tokenRecord, err = d.GetAccessTokenByHash(ctx, tokenHash)
		if err != nil {
			return nil, nil, err
		}
		d.SetCachedToken(ctx, tokenHash, tokenRecord)
	}

	user, err := d.GetCachedUser(ctx, tokenRecord.UserID)
	if err != nil || user == nil || !user.IsActive {
		user, err = d.GetActiveUserByID(ctx, tokenRecord.UserID)
		if err != nil {
			return nil, nil, err
		}
		d.SetCachedUser(ctx, tokenRecord.UserID, user)
	}

	return user, tokenRecord, nil
}

// GetUserFromRequest 从请求中获取当前用户（优先 Access Token，其次 Session）
func GetUserFromRequest(c *gin.Context, d *dao.DAO) (*contracts.UserDTO, error) {
	ctx := c.Request.Context()
	var tokenStr string

	tokenFromQuery := c.Query("token")
	if tokenFromQuery != "" {
		tokenStr = tokenFromQuery
	} else {
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenStr = authHeader[7:]
		}
	}

	// 优先使用 Access Token 鉴权
	if tokenStr != "" {
		if user, tokenRecord, err := getUserByToken(ctx, d, tokenStr); err == nil {
			if user.Username == consts.SystemUsername {
				return nil, errors.New(consts.ErrSystemUserLoginNotAllowed)
			}
			ginutil.SetToContext(c, contracts.AuthTokenAuthKey, true)
			ginutil.SetToContext(c, contracts.AuthTokenAdminKey, tokenRecord.IsAdmin)
			return user, nil
		}
	}

	// 降级使用 Session 鉴权
	userID := GetUserIDFromContext(c)
	if userID <= 0 {
		return nil, errors.New(consts.ErrUnauthorizedInternal)
	}

	user, err := d.GetCachedUser(ctx, userID)
	if err != nil || user == nil || !user.IsActive {
		user, err = d.GetActiveUserByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		d.SetCachedUser(ctx, userID, user)
	}

	ginutil.SetToContext(c, contracts.AuthTokenAuthKey, false)
	ginutil.SetToContext(c, contracts.AuthTokenAdminKey, false)

	if user.Username == consts.SystemUsername {
		return nil, errors.New(consts.ErrSystemUserLoginNotAllowed)
	}

	return user, nil
}

// LoginRequiredMiddleware returns a Gin handler function for authentication check.
func LoginRequiredMiddleware(whitelist *extpoints.PathWhitelist, d *dao.DAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		if whitelist != nil && whitelist.Match(c.Request.URL.Path) {
			c.Next()
			return
		}

		_, span := trace.Start(c.Request.Context(), "LoginRequired")
		defer span.End()

		user, err := GetUserFromRequest(c, d)
		if err != nil {
			response.AbortUnauthorized(c, consts.ErrUnAuthorized)
			return
		}

		LogForAudit(c.Request.Context(), user, c)
		ginutil.SetToContext(c, contracts.AuthUserObjKey, user)
		c.Next()
	}
}

// AdminRequiredMiddleware returns a Gin handler function for admin authorization check.
func AdminRequiredMiddleware(d *dao.DAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := trace.Start(c.Request.Context(), "AdminRequired")
		defer span.End()

		user, err := GetUserFromRequest(c, d)
		if err != nil {
			response.AbortUnauthorized(c, consts.ErrUnAuthorized)
			return
		}

		isTokenAuth, _ := ginutil.GetFromContext[bool](c, contracts.AuthTokenAuthKey)
		isTokenAdmin, _ := ginutil.GetFromContext[bool](c, contracts.AuthTokenAdminKey)

		// Logged-in but lacking admin permission is 403, not 401/404.
		if isTokenAuth && !isTokenAdmin && !user.IsAdmin {
			response.AbortForbidden(c, consts.ErrInsufficientPermission)
			return
		}
		if !isTokenAuth && !user.IsAdmin {
			response.AbortForbidden(c, consts.ErrInsufficientPermission)
			return
		}

		LogForAudit(c.Request.Context(), user, c)
		ginutil.SetToContext(c, contracts.AuthUserObjKey, user)
		c.Next()
	}
}

// DisallowTokenAuth returns a middleware that rejects requests authenticated via access token.
func DisallowTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if tokenAuth, _ := ginutil.GetFromContext[bool](c, contracts.AuthTokenAuthKey); tokenAuth {
			response.AbortForbidden(c, consts.ErrTokenAuthNotAllowed)
			return
		}
		c.Next()
	}
}
