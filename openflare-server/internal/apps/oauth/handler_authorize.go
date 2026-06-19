// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/common"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetLoginURL 获取登录授权地址
// @Summary 获取登录授权地址
// @Description 根据指定认证源生成 OAuth 授权 URL，前端跳转到该 URL 完成 OAuth 登录授权。source 参数为空时使用第一个启用的认证源。
// @Tags oauth
// @Produce json
// @Param source query string false "认证源名称，为空使用第一个启用的认证源"
// @Success 200 {object} response.Any{data=oauth.OAuthAuthorizeResponse} "授权 URL"
// @Failure 400 {object} response.Any "认证源不存在或未配置"
// @Failure 500 {object} response.Any "Redis 异常 or 构造 URL 失败"
// @Router /api/v1/oauth/login [get]
func GetLoginURL(c *gin.Context) {
	ctx := c.Request.Context()
	if !isOIDCLoginEnabled(ctx) {
		response.AbortBadRequest(c, errAuthSourceDisabled)
		return
	}

	source, err := resolveAuthSource(ctx, c.Query("source"))
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if !source.IsActive {
		response.AbortBadRequest(c, errAuthSourceDisabled)
		return
	}

	session := sessions.Default(c)
	token, isNew := ensureSessionToken(session)
	if isNew {
		if err := session.Save(); err != nil {
			response.AbortInternal(c, err.Error())
			return
		}
	}

	userID := GetUserIDFromSession(session)
	sessionHash := hashSessionToken(token)

	state := uuid.NewString()
	payloadValue, err := encodeOAuthStatePayload(oauthStatePayload{
		SourceName:  source.Name,
		Purpose:     OAuthPurposeLogin,
		UserID:      userID,
		SessionHash: sessionHash,
	})
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	if err := db.Redis.Set(c.Request.Context(), db.PrefixedKey(fmt.Sprintf(OAuthStateCacheKeyFormat, state)), payloadValue, OAuthStateCacheKeyExpiration).Err(); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	authorizeURL, err := buildAuthorizeURL(c.Request.Context(), source, state)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(OAuthAuthorizeResponse{AuthorizeURL: authorizeURL}))
}

func buildAuthorizeURL(ctx context.Context, source *model.AuthSource, state string) (string, error) {
	redirectURL, err := getFrontendLoginRedirectURL(ctx)
	if err != nil {
		return "", err
	}
	authConfig, verifier, err := buildOAuthConfig(ctx, source, redirectURL)
	if err != nil {
		return "", err
	}
	if verifier != nil {
		return authConfig.AuthCodeURL(state, oidc.Nonce(state)), nil
	}
	return authConfig.AuthCodeURL(state), nil
}

// Authorize 发起指定认证源授权
// @Summary 发起指定认证源授权
// @Description 根据指定认证源名称发起 OAuth 授权，支持 purpose 参数用于区分登录和账号绑定场景。认证源必须已启用。
// @Tags oauth
// @Produce json
// @Param source path string true "认证源名称"
// @Param purpose query string false "授权目的：login（登录）或 bind（绑定账号），默认 login"
// @Success 200 {object} response.Any{data=oauth.OAuthAuthorizeResponse} "授权 URL"
// @Failure 400 {object} response.Any "认证源不存在或未启用"
// @Failure 500 {object} response.Any "Redis 异常或构造 URL 失败"
// @Router /api/v1/oauth/{source}/authorize [get]
func Authorize(c *gin.Context) {
	ctx := c.Request.Context()
	if !isOIDCLoginEnabled(ctx) {
		response.AbortBadRequest(c, errAuthSourceDisabled)
		return
	}

	source, err := resolveAuthSource(ctx, c.Param("source"))
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if !source.IsActive {
		response.AbortBadRequest(c, errAuthSourceDisabled)
		return
	}
	purpose := strings.ToLower(strings.TrimSpace(c.Query("purpose")))
	if purpose != OAuthPurposeBind {
		purpose = OAuthPurposeLogin
	}

	session := sessions.Default(c)
	userID := GetUserIDFromSession(session)
	if purpose == OAuthPurposeBind && userID == 0 {
		response.AbortUnauthorized(c, common.UnAuthorized)
		return
	}

	token, isNew := ensureSessionToken(session)
	if isNew {
		if err := session.Save(); err != nil {
			response.AbortInternal(c, err.Error())
			return
		}
	}

	sessionHash := hashSessionToken(token)

	state := uuid.NewString()
	payloadValue, err := encodeOAuthStatePayload(oauthStatePayload{
		SourceName:  source.Name,
		Purpose:     purpose,
		UserID:      userID,
		SessionHash: sessionHash,
	})
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	if err := db.Redis.Set(c.Request.Context(), db.PrefixedKey(fmt.Sprintf(OAuthStateCacheKeyFormat, state)), payloadValue, OAuthStateCacheKeyExpiration).Err(); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	authorizeURL, err := buildAuthorizeURL(c.Request.Context(), source, state)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(OAuthAuthorizeResponse{AuthorizeURL: authorizeURL}))
}
