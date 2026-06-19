// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Rain-kl/Wavelet/internal/common"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/listener"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Callback OAuth 回调处理
// @Summary OAuth 回调处理
// @Description 接收前端传回的 state 和 code，完成 OAuth/OIDC 认证并建立会话。支持登录（login）和账号绑定（bind）两种场景。
// @Tags oauth
// @Accept json
// @Produce json
// @Param request body oauth.CallbackRequest true "回调请求参数"
// @Success 200 {object} response.Any{data=oauth.OAuthCallbackResult} "登录或绑定成功"
// @Failure 400 {object} response.Any "state 无效、参数错误或认证源错误"
// @Failure 401 {object} response.Any "绑定场景未登录"
// @Failure 500 {object} response.Any "OAuth 认证失败或内部错误"
// @Router /api/v1/oauth/callback [post]
func Callback(c *gin.Context) {
	var req CallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	ctx := c.Request.Context()
	stateKey := db.PrefixedKey(fmt.Sprintf(OAuthStateCacheKeyFormat, req.State))
	payloadRaw, err := db.Redis.Get(ctx, stateKey).Result()
	if err != nil {
		response.AbortBadRequest(c, errInvalidState)
		return
	}
	_ = db.Redis.Del(ctx, stateKey)

	payload, err := decodeOAuthStatePayload(payloadRaw)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	session := sessions.Default(c)
	currentUserID := GetUserIDFromSession(session)

	if payload.Purpose == OAuthPurposeBind && currentUserID == 0 {
		response.AbortUnauthorized(c, common.UnAuthorized)
		return
	}

	token, ok := session.Get(SessionTokenKey).(string)
	if !ok || token == "" {
		response.AbortBadRequest(c, "invalid session context")
		return
	}

	if hashSessionToken(token) != payload.SessionHash {
		response.AbortBadRequest(c, "session mismatch for oauth state")
		return
	}

	if payload.Purpose == OAuthPurposeBind && currentUserID != payload.UserID {
		response.AbortBadRequest(c, "user context mismatch for oauth binding")
		return
	}

	if !isOIDCLoginEnabled(ctx) {
		response.AbortBadRequest(c, errAuthSourceDisabled)
		return
	}

	source, err := resolveAuthSource(ctx, payload.SourceName)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if !source.IsActive {
		response.AbortBadRequest(c, errAuthSourceDisabled)
		return
	}

	redirectURL, err := getFrontendLoginRedirectURL(ctx)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	userInfo, err := buildOAuthUserInfo(ctx, source, req.Code, req.State, redirectURL)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	if err := normalizeOAuthUserInfo(userInfo); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	if userInfo.Sub == "" {
		userInfo.Sub = userInfo.Username
	}

	if payload.Purpose == OAuthPurposeBind {
		handleCallbackBind(ctx, c, source, userInfo)
		return
	}

	handleCallbackLogin(ctx, c, source, userInfo)
}

// handleCallbackBind 处理 OAuth 回调中的帐号绑定流程
func handleCallbackBind(ctx context.Context, c *gin.Context, source *model.AuthSource, userInfo *model.OAuthUserInfo) {
	userID := GetUserIDFromContext(c)
	if userID == 0 {
		response.AbortUnauthorized(c, common.UnAuthorized)
		return
	}
	var user model.User
	if err := db.DB(ctx).First(&user, "id = ?", userID).Error; err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	if err := model.BindExternalAccount(ctx, &model.ExternalAccount{
		AuthSourceID:     source.ID,
		UserID:           user.ID,
		ExternalID:       userInfo.Sub,
		ExternalUsername: userInfo.Username,
		Email:            userInfo.Email,
	}); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	user.LastLoginAt = time.Now()
	_ = db.DB(ctx).Model(&user).Update("last_login_at", user.LastLoginAt).Error
	c.JSON(http.StatusOK, response.OK(buildCallbackResult(&user, "bound")))
}

// handleCallbackLogin 处理 OAuth 回调中的登录流程（查找已有帐号或自动注册）
func handleCallbackLogin(ctx context.Context, c *gin.Context, source *model.AuthSource, userInfo *model.OAuthUserInfo) {
	var user model.User

	account, err := model.FindExternalAccount(ctx, source.ID, userInfo.Sub)
	switch {
	case err == nil:
		if err := db.DB(ctx).First(&user, "id = ?", account.UserID).Error; err != nil {
			response.AbortInternal(c, err.Error())
			return
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		newUser, ok := handleCallbackRegister(ctx, c, source, userInfo)
		if !ok {
			return
		}
		user = newUser
	default:
		response.AbortInternal(c, err.Error())
		return
	}

	user.LastLoginAt = time.Now()
	_ = db.DB(ctx).Model(&user).Update("last_login_at", user.LastLoginAt).Error
	if err := setLoginSession(ctx, c, &user); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	logger.InfoF(ctx, "[LoginAudit] successful OAuth login via source: %s, external ID: %s, user: %s, ID: %d, IP: %s", source.Name, userInfo.Sub, user.Username, user.ID, c.ClientIP())

	listener.EmitAdminLoggedIn(ctx, &user, c.ClientIP())

	c.JSON(http.StatusOK, response.OK(buildCallbackResult(&user, "logged_in")))
}

// handleCallbackRegister 处理 OAuth 回调中的自动注册流程
// 若注册被禁用则保存 pending 信息并返回 false；若注册成功则返回新用户；若出错则返回 false
func handleCallbackRegister(ctx context.Context, c *gin.Context, source *model.AuthSource, userInfo *model.OAuthUserInfo) (model.User, bool) {
	registrationEnabled, regErr := repository.GetBoolByKey(ctx, model.ConfigKeyRegistrationEnabled)
	if regErr != nil {
		registrationEnabled = true
	}

	if !registrationEnabled {
		c.JSON(http.StatusOK, response.OK(buildCallbackResult(nil, "need_bind")))
		return model.User{}, false
	}

	username, uniqueErr := uniqueUsername(ctx, userInfo.Username)
	if uniqueErr != nil {
		response.AbortInternal(c, uniqueErr.Error())
		return model.User{}, false
	}
	userInfo.Username = username

	var user model.User
	if err := user.CreateUser(ctx, db.DB(ctx), userInfo); err != nil {
		response.AbortInternal(c, err.Error())
		return model.User{}, false
	}
	if err := model.BindExternalAccount(ctx, &model.ExternalAccount{
		AuthSourceID:     source.ID,
		UserID:           user.ID,
		ExternalID:       userInfo.Sub,
		ExternalUsername: userInfo.Username,
		Email:            userInfo.Email,
	}); err != nil {
		response.AbortBadRequest(c, err.Error())
		return model.User{}, false
	}
	logger.InfoF(ctx, "[LoginAudit] successful OAuth registration via source: %s, external ID: %s, user: %s, ID: %d, IP: %s", source.Name, userInfo.Sub, user.Username, user.ID, c.ClientIP())

	return user, true
}
