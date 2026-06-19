// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package origin

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/gin-gonic/gin"
)


func handleLogicError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	return apiutil.AbortNotFoundIfMissing(c, err, errOriginNotFound)
}

// GetOrigins 列出全部源站。
// @Summary 获取源站列表
// @Description 返回所有源站及关联代理规则数量，需要管理员权限
// @Tags openflare-origin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]origin.View} "源站列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Router /api/v1/d/origins [get]
func GetOrigins(c *gin.Context) {
	origins, err := ListOrigins(c.Request.Context())
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(origins))
}

// GetOrigin 获取源站详情。
// @Summary 获取源站详情
// @Description 返回指定源站信息及关联代理规则摘要，需要管理员权限
// @Tags openflare-origin
// @Produce json
// @Security SessionCookie
// @Param id path int true "源站 ID"
// @Success 200 {object} response.Any{data=origin.DetailView} "源站详情"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或源站不存在"
// @Router /api/v1/d/origins/{id} [get]
func GetOrigin(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	detail, err := GetOriginDetail(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(detail))
}

// CreateOriginHandler 创建源站。
// @Summary 创建源站
// @Description 创建新的上游源站记录，需要管理员权限
// @Tags openflare-origin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body origin.Input true "源站参数"
// @Success 200 {object} response.Any{data=origin.View} "创建成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Router /api/v1/d/origins [post]
func CreateOriginHandler(c *gin.Context) {
	var input Input
	if !apiutil.BindJSON(c, &input) {
		return
	}
	origin, err := CreateOrigin(c.Request.Context(), input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(origin))
}

// UpdateOriginHandler 更新源站。
// @Summary 更新源站
// @Description 更新指定源站的配置信息，需要管理员权限
// @Tags openflare-origin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "源站 ID"
// @Param body body origin.Input true "源站参数"
// @Success 200 {object} response.Any{data=origin.View} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或源站不存在"
// @Router /api/v1/d/origins/{id}/update [post]
func UpdateOriginHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input Input
	if !apiutil.BindJSON(c, &input) {
		return
	}
	origin, err := UpdateOrigin(c.Request.Context(), id, input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(origin))
}

// DeleteOriginHandler 删除源站。
// @Summary 删除源站
// @Description 删除指定源站记录，需要管理员权限
// @Tags openflare-origin
// @Produce json
// @Security SessionCookie
// @Param id path int true "源站 ID"
// @Success 200 {object} response.Any{data=string} "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或源站不存在"
// @Router /api/v1/d/origins/{id}/delete [post]
func DeleteOriginHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	if err := DeleteOrigin(c.Request.Context(), id); handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}