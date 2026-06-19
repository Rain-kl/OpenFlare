// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/common"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/db/idgen"
	"github.com/Rain-kl/Wavelet/internal/listener"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Code     string `json:"code"`
}

type registerRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Nickname    string `json:"nickname"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Code        string `json:"code"`
}

type sendEmailCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
	Scene string `json:"scene" binding:"required"`
}

type updateProfileRequest struct {
	Nickname  string `json:"nickname"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio"`
	Phone     string `json:"phone"`
	Gender    string `json:"gender"`
	Website   string `json:"website"`
	Location  string `json:"location"`
}

func setLoginSession(ctx context.Context, c *gin.Context, user *model.User) error {
	session := sessions.Default(c)
	session.Set(oauth.UserIDKey, user.ID)
	session.Set(oauth.UserNameKey, user.Username)
	session.Set(oauth.PasswordHashKey, user.Password)

	// 根据系统配置动态设置 Session 过期时间
	maxAge := config.Config.App.SessionAge
	isSessionCookie := false

	ttlHours, err := repository.GetIntByKey(ctx, model.ConfigKeyLoginSessionTTLHours)
	if err == nil {
		switch {
		case ttlHours == -1:
			// 永不过期，设置为 10 年
			maxAge = 10 * 365 * 24 * 3600
		case ttlHours > 0:
			maxAge = ttlHours * 3600
		case ttlHours == 0:
			isSessionCookie = true
		}
	}
	session.Options(oauth.GetSessionOptions(maxAge))

	if err := session.Save(); err != nil {
		return err
	}

	if isSessionCookie {
		oauth.StripCookieMaxAgeAndExpires(c.Writer.Header(), config.Config.App.SessionCookieName)
	}

	return nil
}

// Login 用户密码登录
// @Summary 用户密码登录
// @Description 使用用户名和密码登录，登录成功后建立 Session。若管理员已关闭密码登录功能则返回错误。
// @Tags user
// @Accept json
// @Produce json
// @Param request body user.loginRequest true "登录请求参数"
// @Success 200 {object} response.Any{data=oauth.BasicUserInfo} "登录成功，返回用户信息"
// @Failure 400 {object} response.Any "用户名或密码错误、帐号已禁用等"
// @Failure 500 {object} response.Any "服务内部错误"
// @Router /api/v1/user/login [post]
func Login(c *gin.Context) {
	ctx := c.Request.Context()
	if !isPasswordLoginEnabled(ctx) {
		response.AbortBadRequest(c, errPasswordLoginDisabled)
		return
	}
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}

	var user model.User
	if err := db.DB(ctx).Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error; err != nil {
		logger.WarnF(ctx, "[LoginAudit] failed login attempt (username not found) for input: %s, IP: %s", req.Username, c.ClientIP())
		response.AbortBadRequest(c, errUsernameOrPasswordWrong)
		return
	}
	if !user.IsActive {
		logger.WarnF(ctx, "[LoginAudit] banned user login attempt for username: %s, ID: %d, IP: %s", user.Username, user.ID, c.ClientIP())
		response.AbortBadRequest(c, common.BannedAccount)
		return
	}

	// 判定是否是明文密码存储
	isPlaintext := !user.IsPasswordEncrypted()

	if !user.CheckPassword(req.Password) {
		logger.WarnF(ctx, "[LoginAudit] failed login attempt (incorrect password) for username: %s, ID: %d, IP: %s", user.Username, user.ID, c.ClientIP())
		response.AbortBadRequest(c, errUsernameOrPasswordWrong)
		return
	}

	if isEmailLoginVerificationEnabled(ctx) {
		result, err := processLoginEmailVerification(ctx, req.Code, &user)
		if err != nil {
			response.AbortBadRequest(c, err.Error())
			return
		}
		if result.Status != LoginEmailVerificationPassed {
			response.AbortBadRequest(c, result.Message)
			return
		}
	}

	session := sessions.Default(c)
	needChangePassword := isPlaintext

	if isPlaintext {
		session.Set("need_change_password", true)
	} else {
		session.Delete("need_change_password")
	}

	user.LastLoginAt = time.Now()
	if err := db.DB(ctx).Model(&user).Update("last_login_at", user.LastLoginAt).Error; err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	if err := setLoginSession(ctx, c, &user); err != nil {
		response.AbortBadRequest(c, errSaveSessionFailed)
		return
	}

	logger.InfoF(ctx, "[LoginAudit] successful login for user: %s, ID: %d, IP: %s", user.Username, user.ID, c.ClientIP())

	listener.EmitAdminLoggedIn(ctx, &user, c.ClientIP())

	c.JSON(http.StatusOK, response.OK(oauth.BuildBasicUserInfo(&user, needChangePassword)))
}

// Register 用户注册
// @Summary 用户注册
// @Description 使用用户名和密码注册新账号，注册成功后自动登录并建立 Session。密码长度不能少于 8 位。
// @Tags user
// @Accept json
// @Produce json
// @Param request body user.registerRequest true "注册请求参数"
// @Success 200 {object} response.Any{data=oauth.BasicUserInfo} "注册并登录成功，返回用户信息"
// @Failure 400 {object} response.Any "参数错误、用户名已存在或注册已关闭"
// @Failure 500 {object} response.Any "服务内部错误"
// @Router /api/v1/user/register [post]
func Register(c *gin.Context) {
	ctx := c.Request.Context()
	if !isRegistrationEnabled(ctx) || !isPasswordRegisterEnabled(ctx) {
		response.AbortBadRequest(c, errRegistrationDisabled)
		return
	}

	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Email = strings.TrimSpace(req.Email)
	req.Code = strings.TrimSpace(req.Code)

	if req.Username == "" || req.Password == "" {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}
	if req.Email == "" {
		response.AbortBadRequest(c, errEmailRequired)
		return
	}
	if len(req.Password) < minPasswordLength {
		response.AbortBadRequest(c, errPasswordTooShort)
		return
	}

	// 邮箱注册验证校验
	if err := validateRegisterEmailVerification(ctx, req.Email, req.Code); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	user := model.User{
		ID:          idgen.NextUint64ID(),
		Username:    req.Username,
		Nickname:    req.Nickname,
		Email:       req.Email,
		AvatarURL:   "",
		IsActive:    true,
		IsAdmin:     false,
		LastLoginAt: time.Now(),
	}
	if user.Nickname == "" {
		user.Nickname = req.DisplayName
	}
	if user.Nickname == "" {
		user.Nickname = req.Username
	}
	if err := user.SetEncryptedPassword(req.Password); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if err := user.RegisterUser(ctx, db.DB(ctx)); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if err := setLoginSession(ctx, c, &user); err != nil {
		response.AbortBadRequest(c, errSaveSessionFailed)
		return
	}

	c.JSON(http.StatusOK, response.OK(oauth.BuildBasicUserInfo(&user, false)))
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
	session := sessions.Default(c)
	userID := session.Get(oauth.UserIDKey)
	username := session.Get(oauth.UserNameKey)
	if userID != nil {
		logger.InfoF(c.Request.Context(), "[LoginAudit] user logged out: %v, ID: %v, IP: %s", username, userID, c.ClientIP())
	}
	session.Options(oauth.GetSessionOptions(-1))
	session.Clear()
	if err := session.Save(); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(""))
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword 修改用户密码
// @Summary 修改用户密码
// @Description 修改当前登录用户的密码。修改成功后，如果是首次明文登录的升级提示，则清除修改密码的提示状态。
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
		response.AbortBadRequest(c, err.Error())
		return
	}

	req.OldPassword = strings.TrimSpace(req.OldPassword)
	req.NewPassword = strings.TrimSpace(req.NewPassword)

	if req.OldPassword == "" || req.NewPassword == "" {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}
	if len(req.NewPassword) < minPasswordLength {
		response.AbortBadRequest(c, errNewPasswordTooShort)
		return
	}

	userObj, _ := oauth.GetFromContext[*model.User](c, oauth.UserObjKey)
	if userObj == nil {
		response.AbortUnauthorized(c, errLoginRequired)
		return
	}

	ctx := c.Request.Context()
	var dbUser model.User
	if err := db.DB(ctx).Where("id = ?", userObj.ID).First(&dbUser).Error; err != nil {
		response.AbortBadRequest(c, errUserNotFound)
		return
	}

	// 校验旧密码
	if !dbUser.CheckPassword(req.OldPassword) {
		response.AbortBadRequest(c, errOldPasswordIncorrect)
		return
	}

	// 加密并更新为新密码
	if err := dbUser.SetEncryptedPassword(req.NewPassword); err != nil {
		response.AbortBadRequest(c, errPasswordEncryptFailed)
		return
	}

	if err := db.DB(ctx).Model(&dbUser).Update("password", dbUser.Password).Error; err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	// 吊销该用户所有的 Access Token
	if err := db.DB(ctx).Where("user_id = ?", dbUser.ID).Delete(&model.AccessToken{}).Error; err != nil {
		response.AbortBadRequest(c, "吊销 Access Token 失败: "+err.Error())
		return
	}

	// 销毁当前活跃会话以强制重新登录
	session := sessions.Default(c)
	session.Clear()
	_ = session.Save()

	c.JSON(http.StatusOK, response.OK("密码修改成功"))
}

// SendEmailCode 发送邮箱验证码
// @Summary 发送邮箱验证码
// @Description 向指定邮箱发送验证码（用于注册场景）
// @Tags user
// @Accept json
// @Produce json
// @Param request body user.sendEmailCodeRequest true "发送验证码请求参数"
// @Success 200 {object} response.Any "发送成功"
// @Failure 400 {object} response.Any "参数错误"
// @Router /api/v1/user/send-email-code [post]
func SendEmailCode(c *gin.Context) {
	var req sendEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		response.AbortBadRequest(c, errEmailRequired)
		return
	}

	if req.Scene != "register" {
		response.AbortBadRequest(c, errUnsupportedEmailScene)
		return
	}

	ctx := c.Request.Context()
	if err := sendRegisterEmailCode(ctx, req.Email); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

// UpdateProfile 修改当前登录用户的个人资料
// @Summary 修改当前登录用户的个人资料
// @Description 修改当前登录用户的昵称、邮箱、头像、简介、电话、性别、个人网站和所在地。
// @Tags user
// @Accept json
// @Produce json
// @Param request body user.updateProfileRequest true "更新请求参数"
// @Success 200 {object} response.Any{data=oauth.BasicUserInfo} "修改成功，返回更新后的用户信息"
// @Failure 400 {object} response.Any "邮箱已被占用或参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Router /api/v1/user/profile [put]
func UpdateProfile(c *gin.Context) {
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	userObj, _ := oauth.GetFromContext[*model.User](c, oauth.UserObjKey)
	if userObj == nil {
		response.AbortUnauthorized(c, errLoginRequired)
		return
	}

	ctx := c.Request.Context()
	dbUser, err := updateUserProfile(ctx, userObj.ID, updateProfileInput(req))
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	session := sessions.Default(c)
	needChange := session.Get("need_change_password") == true

	c.JSON(http.StatusOK, response.OK(oauth.BuildBasicUserInfo(dbUser, needChange)))
}
