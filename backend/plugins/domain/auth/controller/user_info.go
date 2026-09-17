// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package controller provides HTTP handlers and middlewares for the auth plugin.
package controller

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/ginutil"
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/auth/model/dto"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// UserInfoHandler handles current user info queries.
type UserInfoHandler struct{}

// NewUserInfoHandler creates a new UserInfoHandler.
func NewUserInfoHandler() *UserInfoHandler {
	return &UserInfoHandler{}
}

// UserInfo 获取当前登录用户信息
// @Summary 获取当前登录用户信息
// @Description 返回当前登录用户的基本信息，需要登录。
// @Tags oauth
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=dto.BasicUserInfo} "用户信息"
// @Failure 401 {object} response.Any "未登录"
// @Router /api/v1/oauth/user-info [get]
// @Router /api/v1/user-info [get]
func (h *UserInfoHandler) UserInfo(c *gin.Context) {
	user, _ := ginutil.GetFromContext[*contracts.UserDTO](c, contracts.AuthUserObjKey)
	session := sessions.Default(c)
	needChange := session.Get("need_change_password") == true || (user != nil && user.NeedChangePassword)

	c.JSON(
		http.StatusOK,
		response.OK(dto.BuildBasicUserInfo(user, needChange)),
	)
}
