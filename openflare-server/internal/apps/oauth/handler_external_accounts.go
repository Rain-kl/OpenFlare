// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/common"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/gin-gonic/gin"
)

// ListExternalAccounts 获取当前用户的外部帐号绑定列表
// @Summary 获取外部帐号列表
// @Description 返回当前登录用户已绑定的所有外部 OAuth 帐号信息，需要登录
// @Tags oauth
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]model.ExternalAccountView} "外部帐号列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/oauth/external-accounts [get]
func ListExternalAccounts(c *gin.Context) {
	userID := GetUserIDFromContext(c)
	accounts, err := model.ListExternalAccountsByUserID(c.Request.Context(), userID)
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
func DeleteExternalAccount(c *gin.Context) {
	userID := GetUserIDFromContext(c)
	if userID == 0 {
		response.AbortUnauthorized(c, common.UnAuthorized)
		return
	}
	rawID := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || id == 0 {
		response.AbortBadRequest(c, errInvalidExternalAccountBindingID)
		return
	}
	if err := model.DeleteExternalAccountForUser(c.Request.Context(), id, userID); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
