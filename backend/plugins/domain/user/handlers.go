// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/idgen"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/pkg/util"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

var (
	authMu  sync.RWMutex
	authSvc contracts.AuthService
)

// SetAuthService binds the AuthService for cache synchronization.
func SetAuthService(s contracts.AuthService) {
	authMu.Lock()
	defer authMu.Unlock()
	authSvc = s
}

func getAuthService() contracts.AuthService {
	authMu.RLock()
	defer authMu.RUnlock()
	return authSvc
}

func getUserIDFromSession(c *gin.Context) uint64 {
	defer func() { _ = recover() }()
	session := sessions.Default(c)
	val := session.Get(contracts.AuthUserIDKey)
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case uint64:
		return v
	case int64:
		if v < 0 {
			return 0
		}
		return uint64(v)
	case float64:
		if v < 0 {
			return 0
		}
		return uint64(v)
	case string:
		id, _ := strconv.ParseUint(v, 10, 64)
		return id
	default:
		return 0
	}
}

func invalidateUserCache(ctx context.Context, userID uint64) {
	if s := getAuthService(); s != nil {
		s.InvalidateCachedUser(ctx, userID)
	}
}

func invalidateTokenCache(ctx context.Context, tokenHash string) {
	if s := getAuthService(); s != nil {
		s.InvalidateCachedToken(ctx, tokenHash)
	}
}

// Login 用户登录
// @Summary 用户登录
// @Description 使用用户名和密码登录系统，验证通过后建立 Session 并返回用户信息。
// @Tags user
// @Accept json
// @Produce json
// @Param request body user.loginRequest true "登录请求参数"
// @Success 200 {object} response.Any "登录成功，返回用户信息"
// @Failure 400 {object} response.Any "用户名或密码错误"
// @Failure 429 {object} response.Any "登录尝试过于频繁"
// @Failure 500 {object} response.Any "服务内部错误"
// @Router /api/v1/user/login [post]
func Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}

	ctx := c.Request.Context()
	clientIP := c.ClientIP()
	rateKey := "auth:login:ip:" + clientIP
	if clientIP == "" {
		rateKey = "auth:login:user:" + req.Username
	}

	limiter := getLimiter(ctx)
	if limiter != nil {
		res, err := limiter.Allow(ctx, rateKey, contracts.Rate{
			Limit:  5,
			Period: 1 * time.Minute,
		})
		if err == nil && !res.Allowed {
			response.AbortTooManyRequests(c, errTooManyLoginAttempts)
			return
		}
	}

	user, err := GetUserByUsername(ctx, req.Username)
	if err != nil {
		util.DummyCheckPassword(req.Password)
		response.AbortUnauthorized(c, errPasswordMismatch)
		return
	}

	if !user.CheckPassword(req.Password) {
		response.AbortUnauthorized(c, errPasswordMismatch)
		return
	}

	if limiter != nil {
		_ = limiter.Reset(ctx, rateKey)
	}

	if user.ID == 0 {
		newID := idgen.NextUint64ID()
		if err := getDB(ctx).Model(&User{}).Where("username = ?", user.Username).Update("id", newID).Error; err == nil {
			user.ID = newID
		}
	}

	sess := sessions.Default(c)
	sess.Set(contracts.AuthUserIDKey, strconv.FormatUint(user.ID, 10))
	sess.Set(contracts.AuthUserNameKey, user.Username)
	needChange := user.NeedChangePassword || user.IsPlaintextPassword()
	user.NeedChangePassword = needChange
	sess.Set("need_change_password", needChange)
	if err := sess.Save(); err != nil {
		logger.ErrorF(ctx, "save session failed on login: %v", err)
	}

	c.JSON(http.StatusOK, response.OK(user))
}

// Register 用户注册
// @Summary 用户注册
// @Description 使用用户名和密码注册新账号，注册成功后自动登录并建立 Session。
// @Tags user
// @Accept json
// @Produce json
// @Param request body user.registerRequest true "注册请求参数"
// @Success 200 {object} response.Any "注册并登录成功，返回用户信息"
// @Failure 400 {object} response.Any "参数错误、用户名已存在或注册已关闭"
// @Failure 429 {object} response.Any "注册尝试过于频繁"
// @Failure 500 {object} response.Any "服务内部错误"
// @Router /api/v1/user/register [post]
func Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}

	ctx := c.Request.Context()
	clientIP := c.ClientIP()
	if limiter := getLimiter(ctx); limiter != nil && clientIP != "" {
		res, err := limiter.Allow(ctx, "auth:register:ip:"+clientIP, contracts.Rate{
			Limit:  10,
			Period: 1 * time.Minute,
		})
		if err == nil && !res.Allowed {
			response.AbortTooManyRequests(c, errTooManyLoginAttempts)
			return
		}
	}

	newUser := &User{
		Username: req.Username,
		Email:    req.Email,
		IsActive: true,
	}
	if err := newUser.SetEncryptedPassword(req.Password); err != nil {
		response.AbortInternal(c, errPasswordEncryptFailed)
		return
	}

	if err := CreateUser(c.Request.Context(), newUser); err != nil {
		response.AbortBadRequest(c, errCreateUserFailed+err.Error())
		return
	}

	sess := sessions.Default(c)
	sess.Set(contracts.AuthUserIDKey, strconv.FormatUint(newUser.ID, 10))
	sess.Set(contracts.AuthUserNameKey, newUser.Username)
	if err := sess.Save(); err != nil {
		logger.ErrorF(c.Request.Context(), "save session failed on register: %v", err)
	}

	c.JSON(http.StatusOK, response.OK(newUser))
}

// Logout 用户退出登录
// @Summary 用户退出登录
// @Description 清除用户登录 Session，完成退出
// @Tags user
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=string} "退出成功"
// @Failure 500 {object} response.Any "Session 清除失败"
// @Router /api/v1/user/logout [get]
func Logout(c *gin.Context) {
	sess := sessions.Default(c)
	sess.Options(sessions.Options{
		Path:   "/",
		MaxAge: -1,
	})
	sess.Clear()
	if err := sess.Save(); err != nil {
		logger.ErrorF(c.Request.Context(), "clear session failed on logout: %v", err)
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// SendEmailCode 发送邮箱验证码
// @Summary 发送邮箱验证码
// @Description 向指定邮箱发送验证码（用于注册场景）
// @Tags user
// @Accept json
// @Produce json
// @Param request body user.sendEmailCodeRequest true "目标邮箱"
// @Success 200 {object} response.Any "发送成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 500 {object} response.Any "发送失败"
// @Router /api/v1/user/send-email-code [post]
func SendEmailCode(c *gin.Context) {
	var req sendEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}
	payload, err := json.Marshal(sendEmailCodePayload{Email: req.Email})
	if err != nil {
		response.AbortInternal(c, errSendEmailFailed)
		return
	}
	ctx := c.Request.Context()
	if taskSvc := getTaskService(ctx); taskSvc != nil {
		if _, err := taskSvc.Dispatch(ctx, TaskTypeSendEmailCode, payload, contracts.TaskTriggerSystem); err != nil {
			logger.ErrorF(ctx, "dispatch send_email_code failed: %v", err)
			response.AbortInternal(c, errSendEmailFailed)
			return
		}
		c.JSON(http.StatusOK, response.OK(gin.H{"sent": true}))
		return
	}
	if _, err := (&SendEmailCodeHandler{}).Execute(ctx, payload); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"sent": true}))
}

// ChangePassword 修改用户密码
// @Summary 修改用户密码
// @Description 修改当前登录用户的密码。
// @Tags user
// @Accept json
// @Produce json
// @Param request body user.changePasswordRequest true "修改密码请求参数"
// @Success 200 {object} response.Any{data=string} "修改密码成功"
// @Failure 400 {object} response.Any "原密码错误或新密码不符合要求"
// @Failure 401 {object} response.Any "请先登录"
// @Router /api/v1/user/change-password [post]
func ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}

	userID := getUserIDFromSession(c)
	user, err := GetUserByID(c.Request.Context(), userID)
	if err != nil {
		response.AbortNotFound(c, errUserNotFound)
		return
	}

	if !user.CheckPassword(req.OldPassword) {
		response.AbortBadRequest(c, errOldPasswordIncorrect)
		return
	}

	if err := user.SetEncryptedPassword(req.NewPassword); err != nil {
		response.AbortInternal(c, errPasswordUpdateFailed)
		return
	}

	if err := UpdateUser(c.Request.Context(), user); err != nil {
		logger.ErrorF(c.Request.Context(), "persist changed password failed: %v", err)
	}
	invalidateUserCache(c.Request.Context(), user.ID)

	sess := sessions.Default(c)
	sess.Set("need_change_password", false)
	if err := sess.Save(); err != nil {
		logger.ErrorF(c.Request.Context(), "save session failed on change-password: %v", err)
	}

	c.JSON(http.StatusOK, response.OKNil())
}

// Self 获取当前登录用户信息
// @Summary 获取当前登录用户信息
// @Description 返回当前登录用户的基本信息，需要登录。
// @Tags user
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any "用户信息"
// @Failure 401 {object} response.Any "未登录"
// @Router /api/v1/user/self [get]
func Self(c *gin.Context) {
	svc := getAuthService()
	if svc == nil {
		response.AbortUnauthorized(c, errUserNotFound)
		return
	}
	user, err := svc.GetCurrentUser(c)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "get current user failed: %v", err)
		response.AbortUnauthorized(c, errUserNotFound)
		return
	}
	c.JSON(http.StatusOK, response.OK(user))
}

// UpdateProfile 修改当前登录用户的个人资料
// @Summary 修改当前登录用户的个人资料
// @Description 修改当前登录用户的昵称、头像、简介、电话、性别、个人网站和所在地。
// @Tags user
// @Accept json
// @Produce json
// @Param request body user.updateProfileRequest true "更新请求参数"
// @Success 200 {object} response.Any "修改成功，返回更新后的用户信息"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Router /api/v1/user/profile [put]
func UpdateProfile(c *gin.Context) {
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}

	userID := getUserIDFromSession(c)
	user, err := GetUserByID(c.Request.Context(), userID)
	if err != nil {
		response.AbortNotFound(c, errUserNotFound)
		return
	}

	user.Nickname = req.Nickname
	user.AvatarURL = req.AvatarURL
	user.Bio = req.Bio
	user.Phone = req.Phone
	user.Gender = req.Gender
	user.Website = req.Website
	user.Location = req.Location

	if err := UpdateUser(c.Request.Context(), user); err != nil {
		logger.ErrorF(c.Request.Context(), "persist updated profile failed: %v", err)
	}
	invalidateUserCache(c.Request.Context(), user.ID)

	c.JSON(http.StatusOK, response.OK(user))
}

// ListAccessTokens 获取当前用户的 AccessToken 列表
// @Summary 获取当前用户的 AccessToken 列表
// @Description 返回当前登录用户的所有 active access tokens（脱敏后）
// @Tags user
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]user.AccessToken} "令牌列表"
// @Failure 401 {object} response.Any "未登录"
// @Router /api/v1/user/access-tokens [get]
func ListAccessTokens(c *gin.Context) {
	userID := getUserIDFromSession(c)
	tokens, err := listAccessTokensByUser(c.Request.Context(), userID)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "list access tokens failed: %v", err)
	}
	c.JSON(http.StatusOK, response.OK(tokens))
}

const (
	tokenEntropyByteLength = 24
	tokenMaskMinLength     = 8
)

// CreateAccessToken 创建一个新的 AccessToken
// @Summary 创建一个新的 AccessToken
// @Description 为当前用户新建一个 API 访问令牌，仅在此接口返回一次明文令牌值，请妥善保存。
// @Tags user
// @Accept json
// @Produce json
// @Param request body user.createAccessTokenRequest true "令牌名称"
// @Security SessionCookie
// @Success 200 {object} response.Any "新建令牌成功"
// @Failure 400 {object} response.Any "参数错误或超限"
// @Router /api/v1/user/access-tokens [post]
func CreateAccessToken(c *gin.Context) {
	var req createAccessTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}

	userID := getUserIDFromSession(c)
	rawBytes := make([]byte, tokenEntropyByteLength)
	_, _ = rand.Read(rawBytes)
	rawToken := "wvt_" + hex.EncodeToString(rawBytes)
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	masked := rawToken
	if len(rawToken) > tokenMaskMinLength {
		masked = rawToken[:4] + "..." + rawToken[len(rawToken)-4:]
	}

	token := AccessToken{
		UserID:      userID,
		Name:        req.Name,
		TokenHash:   tokenHash,
		MaskedToken: masked,
		IsAdmin:     req.IsAdmin,
	}

	if err := createAccessTokenRow(c.Request.Context(), &token); err != nil {
		response.AbortInternal(c, errCreateTokenFailed)
		return
	}

	c.JSON(http.StatusOK, response.OK(gin.H{
		"token":     token,
		"raw_token": rawToken,
	}))
}

// DeleteAccessToken 删除一个 AccessToken
// @Summary 删除一个 AccessToken
// @Description 撤销并删除一个属于当前用户的 API 访问令牌
// @Tags user
// @Produce json
// @Param id path string true "令牌ID"
// @Security SessionCookie
// @Success 200 {object} response.Any{data=string} "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Router /api/v1/user/access-tokens/{id} [delete]
func DeleteAccessToken(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}

	userID := getUserIDFromSession(c)
	token, err := getAccessTokenOfUser(c.Request.Context(), id, userID)
	if err != nil {
		response.AbortNotFound(c, errTokenNotFound)
		return
	}

	if err := deleteAccessTokenRow(c.Request.Context(), token); err != nil {
		logger.ErrorF(c.Request.Context(), "delete access token failed: %v", err)
	}
	invalidateTokenCache(c.Request.Context(), token.TokenHash)
	c.JSON(http.StatusOK, response.OKNil())
}

// RotateAccessToken 轮换一个 AccessToken
// @Summary 轮换一个 AccessToken
// @Description 轮换（重新生成）一个属于当前用户的 API 访问令牌的密钥，旧令牌将立即失效
// @Tags user
// @Produce json
// @Param id path string true "令牌ID"
// @Security SessionCookie
// @Success 200 {object} response.Any "令牌轮换成功"
// @Failure 400 {object} response.Any "参数错误"
// @Router /api/v1/user/access-tokens/{id}/rotate [post]
func RotateAccessToken(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}

	userID := getUserIDFromSession(c)
	token, err := getAccessTokenOfUser(c.Request.Context(), id, userID)
	if err != nil {
		response.AbortNotFound(c, errTokenNotFound)
		return
	}

	invalidateTokenCache(c.Request.Context(), token.TokenHash)

	rawBytes := make([]byte, tokenEntropyByteLength)
	_, _ = rand.Read(rawBytes)
	rawToken := "wvt_" + hex.EncodeToString(rawBytes)
	hash := sha256.Sum256([]byte(rawToken))
	token.TokenHash = hex.EncodeToString(hash[:])

	masked := rawToken
	if len(rawToken) > tokenMaskMinLength {
		masked = rawToken[:4] + "..." + rawToken[len(rawToken)-4:]
	}
	token.MaskedToken = masked

	if err := saveAccessTokenRow(c.Request.Context(), token); err != nil {
		logger.ErrorF(c.Request.Context(), "rotate access token failed: %v", err)
	}

	c.JSON(http.StatusOK, response.OK(gin.H{
		"token":     token,
		"raw_token": rawToken,
	}))
}
