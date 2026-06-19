// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package proxy_route

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
	return apiutil.AbortNotFoundIfMissing(c, err, errProxyRouteNotFound)
}

// GetProxyRoutes 列出全部代理规则。
// @Summary 获取代理规则列表
// @Description 返回所有代理规则配置，需要管理员权限
// @Tags openflare-proxy-route
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]proxy_route.View} "代理规则列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Router /api/v1/d/proxy-routes [get]
func GetProxyRoutes(c *gin.Context) {
	routes, err := ListProxyRoutes(c.Request.Context())
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(routes))
}

// GetProxyRouteHandler 获取代理规则详情。
// @Summary 获取代理规则详情
// @Description 返回指定代理规则的完整配置，需要管理员权限
// @Tags openflare-proxy-route
// @Produce json
// @Security SessionCookie
// @Param id path int true "代理规则 ID"
// @Success 200 {object} response.Any{data=proxy_route.View} "代理规则详情"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或规则不存在"
// @Router /api/v1/d/proxy-routes/{id} [get]
func GetProxyRouteHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	route, err := GetProxyRoute(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(route))
}

// CreateProxyRouteHandler 创建代理规则。
// @Summary 创建代理规则
// @Description 创建新的反向代理规则，需要管理员权限
// @Tags openflare-proxy-route
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body proxy_route.Input true "代理规则参数"
// @Success 200 {object} response.Any{data=proxy_route.View} "创建成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Router /api/v1/d/proxy-routes [post]
func CreateProxyRouteHandler(c *gin.Context) {
	var input Input
	if !apiutil.BindJSON(c, &input) {
		return
	}
	route, err := CreateProxyRoute(c.Request.Context(), input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(route))
}

// UpdateProxyRouteHandler 更新代理规则。
// @Summary 更新代理规则
// @Description 更新指定代理规则的配置，需要管理员权限
// @Tags openflare-proxy-route
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "代理规则 ID"
// @Param body body proxy_route.Input true "代理规则参数"
// @Success 200 {object} response.Any{data=proxy_route.View} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或规则不存在"
// @Router /api/v1/d/proxy-routes/{id}/update [post]
func UpdateProxyRouteHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input Input
	if !apiutil.BindJSON(c, &input) {
		return
	}
	route, err := UpdateProxyRoute(c.Request.Context(), id, input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(route))
}

// DeleteProxyRouteHandler 删除代理规则。
// @Summary 删除代理规则
// @Description 删除指定代理规则，需要管理员权限
// @Tags openflare-proxy-route
// @Produce json
// @Security SessionCookie
// @Param id path int true "代理规则 ID"
// @Success 200 {object} response.Any{data=string} "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或规则不存在"
// @Router /api/v1/d/proxy-routes/{id}/delete [post]
func DeleteProxyRouteHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	if err := DeleteProxyRoute(c.Request.Context(), id); handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}