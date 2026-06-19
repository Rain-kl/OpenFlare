// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

// BasicUserInfo 用户基本信息结构体
type BasicUserInfo struct {
	ID                 uint64 `json:"id"`
	Username           string `json:"username"`
	Nickname           string `json:"nickname"`
	Email              string `json:"email"`
	AvatarURL          string `json:"avatar_url"`
	IsAdmin            bool   `json:"is_admin"`
	NeedChangePassword bool   `json:"need_change_password"`
	Bio                string `json:"bio"`
	Phone              string `json:"phone"`
	Gender             string `json:"gender"`
	Website            string `json:"website"`
	Location           string `json:"location"`
}

// BuildBasicUserInfo 将 User 模型转换为 BasicUserInfo
func BuildBasicUserInfo(user *model.User, needChange bool) BasicUserInfo {
	return BasicUserInfo{
		ID:                 user.ID,
		Username:           user.Username,
		Nickname:           user.Nickname,
		Email:              user.Email,
		AvatarURL:          user.AvatarURL,
		IsAdmin:            user.IsAdmin,
		NeedChangePassword: needChange,
		Bio:                user.Bio,
		Phone:              user.Phone,
		Gender:             user.Gender,
		Website:            user.Website,
		Location:           user.Location,
	}
}

// UserInfo 获取当前登录用户信息
// @Summary 获取当前登录用户信息
// @Description 返回当前登录用户的基本信息及余额数据，需要登录。包括用户 ID、用户名、信任等级、各类余额信息等。
// @Tags oauth
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=oauth.BasicUserInfo} "用户信息"
// @Failure 401 {object} response.Any "未登录"
// @Router /api/v1/oauth/user-info [get]
// @Router /api/v1/user-info [get]
// @Router /api/v1/user/self [get]
func UserInfo(c *gin.Context) {
	user, _ := GetFromContext[*model.User](c, UserObjKey)
	session := sessions.Default(c)
	needChange := session.Get("need_change_password") == true

	c.JSON(
		http.StatusOK,
		response.OK(BuildBasicUserInfo(user, needChange)),
	)
}

// GetLoginURL 获取登录地址
// @Summary 获取登录地址
// @Description 生成 OAuth 登录 URL，前端跳转至该地址完成授权。返回的 URL 中包含 state 参数用于 CSRF 防护。
// @Tags oauth
// @Produce json
// @Success 200 {object} response.Any{data=string} "OAuth 登录 URL"
// @Failure 500 {object} response.Any "Redis 异常或内部错误"
// @Router /api/v1/oauth/login [get]

// Logout 退出登录
// @Summary 退出登录
// @Description 清除当前用户的登录会话，完成退出。清除 Cookie 中的 Session 数据。
// @Tags oauth
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=string} "退出成功"
// @Failure 500 {object} response.Any "Session 清除失败"
// @Router /api/v1/oauth/logout [get]
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get(UserIDKey)
	username := session.Get(UserNameKey)
	if userID != nil {
		logger.InfoF(c.Request.Context(), "[LoginAudit] user logged out: %v, ID: %v, IP: %s", username, userID, c.ClientIP())
	}
	session.Options(GetSessionOptions(-1))
	session.Clear()
	if err := session.Save(); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
