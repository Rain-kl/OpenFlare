// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package controller provides HTTP handlers and middlewares for the auth plugin.
package controller

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/auth/consts"
	"Wavelet/plugins/domain/auth/dao"
	"Wavelet/plugins/domain/auth/model/do"
	"Wavelet/plugins/domain/auth/model/dto"
	"Wavelet/plugins/domain/auth/model/entity"
	"Wavelet/plugins/domain/auth/service"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// OAuthHandler handles OAuth authentication endpoints.
type OAuthHandler struct {
	oauthSvc   *service.OAuthService
	sessionSvc *service.SessionService
	dao        *dao.DAO
}

// NewOAuthHandler creates a new OAuthHandler.
func NewOAuthHandler(oauthSvc *service.OAuthService, sessionSvc *service.SessionService, d *dao.DAO) *OAuthHandler {
	return &OAuthHandler{
		oauthSvc:   oauthSvc,
		sessionSvc: sessionSvc,
		dao:        d,
	}
}

// GetLoginSources 获取可用登录源列表
// @Summary 获取可用登录源
// @Description 返回当前系统已启用的所有 OAuth 登录源，前端展示登录按钮列表时调用
// @Tags oauth
// @Produce json
// @Success 200 {object} response.Any{data=[]dto.AuthSourceView} "登录源列表"
// @Router /api/v1/oauth/sources [get]
func (h *OAuthHandler) GetLoginSources(c *gin.Context) {
	sources, err := h.oauthSvc.ActiveLoginSources(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(sources))
}

// GetLoginURL 获取登录授权地址
// @Summary 获取登录授权地址
// @Description 根据指定认证源生成 OAuth 授权 URL，前端跳转到该 URL 完成 OAuth 登录授权。source 参数为空时使用第一个启用的认证源。
// @Tags oauth
// @Produce json
// @Param source query string false "认证源名称，为空使用第一个启用的认证源"
// @Success 200 {object} response.Any{data=dto.OAuthAuthorizeResponse} "授权 URL"
// @Failure 400 {object} response.Any "认证源不存在或未配置"
// @Failure 500 {object} response.Any "构造 URL 失败"
// @Router /api/v1/oauth/login [get]
func (h *OAuthHandler) GetLoginURL(c *gin.Context) {
	ctx := c.Request.Context()
	if !h.oauthSvc.IsOIDCLoginEnabled(ctx) {
		response.AbortBadRequest(c, consts.ErrAuthSourceDisabled)
		return
	}

	source, err := h.oauthSvc.ResolveAuthSource(ctx, c.Query("source"))
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if !source.IsActive {
		response.AbortBadRequest(c, consts.ErrAuthSourceDisabled)
		return
	}

	session := sessions.Default(c)
	token, isNew := h.sessionSvc.EnsureSessionToken(session)
	if isNew {
		if err := session.Save(); err != nil {
			response.AbortInternal(c, err.Error())
			return
		}
	}

	userID := GetUserIDFromSession(session)
	sessionHash := h.sessionSvc.HashSessionToken(token)
	if err := h.oauthSvc.ReserveOAuthStateSlot(ctx, sessionHash); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	state := uuid.NewString()
	payloadValue, err := (do.OAuthStatePayload{
		SourceName:  source.Name,
		Purpose:     consts.OAuthPurposeLogin,
		UserID:      userID,
		SessionHash: sessionHash,
	}).Encode()
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	stateKey := fmt.Sprintf(consts.OAuthStateCacheKeyFormat, state)
	if cache := h.dao.Cache(); cache != nil {
		if err := cache.Set(ctx, stateKey, payloadValue, consts.OAuthStateCacheKeyExpiration); err != nil {
			response.AbortInternal(c, err.Error())
			return
		}
	}

	authorizeURL, err := h.oauthSvc.BuildAuthorizeURL(ctx, source, state)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(dto.OAuthAuthorizeResponse{AuthorizeURL: authorizeURL}))
}

// Authorize 发起指定认证源授权
// @Summary 发起指定认证源授权
// @Description 根据指定认证源名称发起 OAuth 授权，支持 purpose 参数用于区分登录和账号绑定场景。认证源必须已启用。
// @Tags oauth
// @Produce json
// @Param source path string true "认证源名称"
// @Param purpose query string false "授权目的：login（登录）或 bind（绑定账号），默认 login"
// @Success 200 {object} response.Any{data=dto.OAuthAuthorizeResponse} "授权 URL"
// @Failure 400 {object} response.Any "认证源不存在或未启用"
// @Failure 500 {object} response.Any "构造 URL 失败"
// @Router /api/v1/oauth/{source}/authorize [get]
func (h *OAuthHandler) Authorize(c *gin.Context) {
	ctx := c.Request.Context()
	if !h.oauthSvc.IsOIDCLoginEnabled(ctx) {
		response.AbortBadRequest(c, consts.ErrAuthSourceDisabled)
		return
	}

	source, err := h.oauthSvc.ResolveAuthSource(ctx, c.Param("source"))
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if !source.IsActive {
		response.AbortBadRequest(c, consts.ErrAuthSourceDisabled)
		return
	}
	purpose := strings.ToLower(strings.TrimSpace(c.Query("purpose")))
	if purpose != consts.OAuthPurposeBind {
		purpose = consts.OAuthPurposeLogin
	}

	session := sessions.Default(c)
	userID := GetUserIDFromSession(session)
	if purpose == consts.OAuthPurposeBind && userID == 0 {
		response.AbortUnauthorized(c, consts.ErrUnAuthorized)
		return
	}

	token, isNew := h.sessionSvc.EnsureSessionToken(session)
	if isNew {
		if err := session.Save(); err != nil {
			response.AbortInternal(c, err.Error())
			return
		}
	}

	sessionHash := h.sessionSvc.HashSessionToken(token)
	if err := h.oauthSvc.ReserveOAuthStateSlot(ctx, sessionHash); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	state := uuid.NewString()
	payloadValue, err := (do.OAuthStatePayload{
		SourceName:  source.Name,
		Purpose:     purpose,
		UserID:      userID,
		SessionHash: sessionHash,
	}).Encode()
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	stateKey := fmt.Sprintf(consts.OAuthStateCacheKeyFormat, state)
	if cache := h.dao.Cache(); cache != nil {
		if err := cache.Set(ctx, stateKey, payloadValue, consts.OAuthStateCacheKeyExpiration); err != nil {
			response.AbortInternal(c, err.Error())
			return
		}
	}

	authorizeURL, err := h.oauthSvc.BuildAuthorizeURL(ctx, source, state)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(dto.OAuthAuthorizeResponse{AuthorizeURL: authorizeURL}))
}

// Callback OAuth 回调处理
// @Summary OAuth 回调处理
// @Description 接收前端传回的 state 和 code，完成 OAuth/OIDC 认证并建立会话。支持登录（login）和账号绑定（bind）两种场景。
// @Tags oauth
// @Accept json
// @Produce json
// @Param request body dto.CallbackRequest true "回调请求参数"
// @Success 200 {object} response.Any{data=dto.OAuthCallbackResult} "登录或绑定成功"
// @Failure 400 {object} response.Any "state 无效、参数错误或认证源错误"
// @Failure 401 {object} response.Any "绑定场景未登录"
// @Failure 500 {object} response.Any "OAuth 认证失败或内部错误"
// @Router /api/v1/oauth/callback [post]
func (h *OAuthHandler) Callback(c *gin.Context) {
	var req dto.CallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	ctx := c.Request.Context()
	stateKey := fmt.Sprintf(consts.OAuthStateCacheKeyFormat, req.State)
	var payloadRaw string
	cache := h.dao.Cache()
	if cache == nil {
		response.AbortBadRequest(c, consts.ErrInvalidState)
		return
	}
	if err := cache.Get(ctx, stateKey, &payloadRaw); err != nil {
		response.AbortBadRequest(c, consts.ErrInvalidState)
		return
	}
	_ = cache.Delete(ctx, stateKey)

	payload, err := do.DecodeOAuthStatePayload(payloadRaw)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	session := sessions.Default(c)
	currentUserID := GetUserIDFromSession(session)

	if payload.Purpose == consts.OAuthPurposeBind && currentUserID == 0 {
		response.AbortUnauthorized(c, consts.ErrUnAuthorized)
		return
	}

	token, ok := session.Get(consts.SessionTokenKey).(string)
	if !ok || token == "" {
		response.AbortBadRequest(c, consts.ErrInvalidSessionContext)
		return
	}

	if h.sessionSvc.HashSessionToken(token) != payload.SessionHash {
		response.AbortBadRequest(c, consts.ErrSessionMismatchForOAuth)
		return
	}

	if payload.Purpose == consts.OAuthPurposeBind && currentUserID != payload.UserID {
		response.AbortBadRequest(c, consts.ErrUserContextMismatch)
		return
	}

	if !h.oauthSvc.IsOIDCLoginEnabled(ctx) {
		response.AbortBadRequest(c, consts.ErrAuthSourceDisabled)
		return
	}

	source, err := h.oauthSvc.ResolveAuthSource(ctx, payload.SourceName)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if !source.IsActive {
		response.AbortBadRequest(c, consts.ErrAuthSourceDisabled)
		return
	}

	redirectURL, err := h.oauthSvc.GetFrontendLoginRedirectURL(ctx)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	userInfo, err := h.oauthSvc.BuildOAuthUserInfo(ctx, source, req.Code, req.State, redirectURL)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	if err := h.oauthSvc.NormalizeOAuthUserInfo(userInfo); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	if userInfo.Sub == "" {
		userInfo.Sub = userInfo.Username
	}

	if payload.Purpose == consts.OAuthPurposeBind {
		h.handleCallbackBind(ctx, c, source, userInfo)
		return
	}

	h.handleCallbackLogin(ctx, c, source, userInfo)
}

func (h *OAuthHandler) handleCallbackBind(ctx context.Context, c *gin.Context, source *entity.AuthSource, userInfo *contracts.OAuthUserInfoDTO) {
	userID := GetUserIDFromContext(c)
	if userID == 0 {
		response.AbortUnauthorized(c, consts.ErrUnAuthorized)
		return
	}
	user, err := h.dao.GetUserByID(ctx, userID)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	if err := h.oauthSvc.BindExternalAccount(ctx, source.ID, user.ID, userInfo); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(buildCallbackResult(user, "bound")))
}

func (h *OAuthHandler) handleCallbackLogin(ctx context.Context, c *gin.Context, source *entity.AuthSource, userInfo *contracts.OAuthUserInfoDTO) {
	user, ok, err := h.oauthSvc.AuthenticateOrRegisterUser(ctx, source, userInfo)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	if !ok || user == nil {
		c.JSON(http.StatusOK, response.OK(buildCallbackResult(nil, "need_bind")))
		return
	}

	session := sessions.Default(c)
	isSessionCookie, err := h.sessionSvc.ApplyLoginSession(ctx, session, user)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	if isSessionCookie {
		h.sessionSvc.StripCookieMaxAgeAndExpires(c.Writer.Header(), h.sessionSvc.Config().SessionCookieName)
	}

	h.dao.SetCachedUser(ctx, user.ID, user)
	logger.InfoF(ctx, "[LoginAudit] successful OAuth login via source: %s, external ID: %s, user: %s, ID: %d, IP: %s", source.Name, userInfo.Sub, user.Username, user.ID, c.ClientIP())

	c.JSON(http.StatusOK, response.OK(buildCallbackResult(user, "logged_in")))
}

func buildCallbackResult(user *contracts.UserDTO, status string) dto.OAuthCallbackResult {
	result := dto.OAuthCallbackResult{Status: status}
	if user != nil {
		info := dto.BuildBasicUserInfo(user, false)
		result.User = &info
	}
	return result
}

// Logout 退出登录
// @Summary 退出登录
// @Description 清除当前用户的登录会话，完成退出。清除 Cookie 中的 Session 数据。
// @Tags oauth
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=string} "退出成功"
// @Failure 500 {object} response.Any "Session 清除失败"
// @Router /api/v1/oauth/logout [get]
func (h *OAuthHandler) Logout(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get(consts.UserIDKey)
	username := session.Get(consts.UserNameKey)
	if userID != nil {
		logger.InfoF(c.Request.Context(), "[LoginAudit] user logged out: %v, ID: %v, IP: %s", username, userID, c.ClientIP())
		if id := dto.ParseUserID(userID); id > 0 {
			h.dao.InvalidateCachedUser(c.Request.Context(), id)
		}
	}
	session.Options(h.sessionSvc.GetSessionOptions(-1))
	session.Clear()
	if err := session.Save(); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// ListExternalAccounts 获取当前用户的外部帐号绑定列表
// @Summary 获取外部帐号列表
// @Description 返回当前登录用户已绑定的所有外部 OAuth 帐号信息，需要登录
// @Tags oauth
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any "外部帐号列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/oauth/external-accounts [get]
func (h *OAuthHandler) ListExternalAccounts(c *gin.Context) {
	userID := GetUserIDFromContext(c)
	accounts, err := h.oauthSvc.ListExternalAccounts(c.Request.Context(), userID)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(accounts))
}

// DeleteExternalAccount 解除外部帐号绑定
// @Summary 解除外部帐号绑定
// @Description 解除当前登录用户与指定外部帐号的绑定关系，需要登录
// @Tags oauth
// @Produce json
// @Security SessionCookie
// @Param id path uint64 true "外部帐号绑定记录 ID"
// @Success 200 {object} response.Any{data=string} "解除绑定成功"
// @Failure 400 {object} response.Any "ID 无效或解除失败"
// @Failure 401 {object} response.Any "未登录"
// @Router /api/v1/oauth/external-accounts/{id}/delete [post]
func (h *OAuthHandler) DeleteExternalAccount(c *gin.Context) {
	userID := GetUserIDFromContext(c)
	if userID == 0 {
		response.AbortUnauthorized(c, consts.ErrUnAuthorized)
		return
	}
	rawID := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || id == 0 {
		response.AbortBadRequest(c, consts.ErrInvalidExternalAccountBindingID)
		return
	}
	if err := h.oauthSvc.DeleteExternalAccount(c.Request.Context(), id, userID); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
