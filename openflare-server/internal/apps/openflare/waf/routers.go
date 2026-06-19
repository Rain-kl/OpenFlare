// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"net/http"
	"strconv"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/gin-gonic/gin"
)


func handleLogicError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	return apiutil.AbortNotFoundIfMissing(c, err, "记录不存在")
}

func routeIDParam(c *gin.Context) (uint, bool) {
	raw := c.Param("route_id")
	if raw == "" {
		response.AbortBadRequest(c, "invalid id")
		return 0, false
	}
	id64, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id64 == 0 {
		response.AbortBadRequest(c, "invalid id")
		return 0, false
	}
	return uint(id64), true
}

// ListRuleGroupsHandler 列出全部 WAF 规则组。
// @Summary 列出 WAF 规则组
// @Description 返回全部 WAF 规则组，需要管理员权限
// @Tags openflare-waf
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]waf.RuleGroupView} "规则组列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/rule-groups [get]
func ListRuleGroupsHandler(c *gin.Context) {
	groups, err := ListRuleGroups(c.Request.Context())
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(groups))
}

// GetRuleGroupHandler 获取 WAF 规则组详情。
// @Summary 获取 WAF 规则组详情
// @Description 按 ID 返回 WAF 规则组详情，需要管理员权限
// @Tags openflare-waf
// @Produce json
// @Security SessionCookie
// @Param id path int true "规则组 ID"
// @Success 200 {object} response.Any{data=waf.RuleGroupView} "规则组详情"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/rule-groups/{id} [get]
func GetRuleGroupHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	group, err := GetRuleGroup(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(group))
}

// CreateRuleGroupHandler 创建 WAF 规则组。
// @Summary 创建 WAF 规则组
// @Description 创建新的 WAF 规则组，需要管理员权限
// @Tags openflare-waf
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body waf.RuleGroupInput true "规则组参数"
// @Success 200 {object} response.Any{data=waf.RuleGroupView} "创建成功的规则组"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/rule-groups [post]
func CreateRuleGroupHandler(c *gin.Context) {
	var input RuleGroupInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	group, err := CreateRuleGroup(c.Request.Context(), input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(group))
}

// UpdateRuleGroupHandler 更新 WAF 规则组。
// @Summary 更新 WAF 规则组
// @Description 按 ID 更新 WAF 规则组，需要管理员权限
// @Tags openflare-waf
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "规则组 ID"
// @Param request body waf.RuleGroupInput true "规则组参数"
// @Success 200 {object} response.Any{data=waf.RuleGroupView} "更新后的规则组"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/rule-groups/{id}/update [post]
func UpdateRuleGroupHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input RuleGroupInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	group, err := UpdateRuleGroup(c.Request.Context(), id, input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(group))
}

// DeleteRuleGroupHandler 删除 WAF 规则组。
// @Summary 删除 WAF 规则组
// @Description 按 ID 删除 WAF 规则组，需要管理员权限
// @Tags openflare-waf
// @Produce json
// @Security SessionCookie
// @Param id path int true "规则组 ID"
// @Success 200 {object} response.Any "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/rule-groups/{id}/delete [post]
func DeleteRuleGroupHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	if err := DeleteRuleGroup(c.Request.Context(), id); handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// ReplaceRuleGroupSitesHandler 替换规则组绑定的站点。
// @Summary 替换规则组站点绑定
// @Description 替换 WAF 规则组关联的代理站点列表，需要管理员权限
// @Tags openflare-waf
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "规则组 ID"
// @Param request body waf.IDsRequest true "站点 ID 列表"
// @Success 200 {object} response.Any{data=waf.RuleGroupView} "更新后的规则组"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/rule-groups/{id}/sites [post]
func ReplaceRuleGroupSitesHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var request IDsRequest
	if !apiutil.BindJSON(c, &request) {
		return
	}
	group, err := ReplaceRuleGroupSites(c.Request.Context(), id, request.IDs)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(group))
}

// GetSiteRuleGroupsHandler 获取站点的 WAF 规则组绑定。
// @Summary 获取站点 WAF 规则组
// @Description 返回代理站点关联的 WAF 规则组绑定，需要管理员权限
// @Tags openflare-waf
// @Produce json
// @Security SessionCookie
// @Param route_id path int true "代理路由 ID"
// @Success 200 {object} response.Any{data=waf.SiteRuleGroupsView} "站点规则组绑定"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/sites/{route_id}/rule-groups [get]
func GetSiteRuleGroupsHandler(c *gin.Context) {
	routeID, ok := routeIDParam(c)
	if !ok {
		return
	}
	view, err := GetSiteRuleGroups(c.Request.Context(), routeID)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// ReplaceSiteRuleGroupsHandler 替换站点的 WAF 规则组绑定。
// @Summary 替换站点 WAF 规则组
// @Description 替换代理站点关联的 WAF 规则组列表，需要管理员权限
// @Tags openflare-waf
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param route_id path int true "代理路由 ID"
// @Param request body waf.IDsRequest true "规则组 ID 列表"
// @Success 200 {object} response.Any{data=waf.SiteRuleGroupsView} "更新后的站点规则组绑定"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/sites/{route_id}/rule-groups [post]
func ReplaceSiteRuleGroupsHandler(c *gin.Context) {
	routeID, ok := routeIDParam(c)
	if !ok {
		return
	}
	var request IDsRequest
	if !apiutil.BindJSON(c, &request) {
		return
	}
	view, err := ReplaceSiteRuleGroups(c.Request.Context(), routeID, request.IDs)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// ListIPGroupsHandler 列出全部 WAF IP 组。
// @Summary 列出 WAF IP 组
// @Description 返回全部 WAF IP 组，需要管理员权限
// @Tags openflare-waf
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]waf.IPGroupView} "IP 组列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/ip-groups [get]
func ListIPGroupsHandler(c *gin.Context) {
	groups, err := ListIPGroups(c.Request.Context())
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(groups))
}

// GetIPGroupHandler 获取 WAF IP 组详情。
// @Summary 获取 WAF IP 组详情
// @Description 按 ID 返回 WAF IP 组详情，需要管理员权限
// @Tags openflare-waf
// @Produce json
// @Security SessionCookie
// @Param id path int true "IP 组 ID"
// @Success 200 {object} response.Any{data=waf.IPGroupView} "IP 组详情"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/ip-groups/{id} [get]
func GetIPGroupHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	group, err := GetIPGroup(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(group))
}

// CreateIPGroupHandler 创建 WAF IP 组。
// @Summary 创建 WAF IP 组
// @Description 创建新的 WAF IP 组，需要管理员权限
// @Tags openflare-waf
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body waf.IPGroupInput true "IP 组参数"
// @Success 200 {object} response.Any{data=waf.IPGroupView} "创建成功的 IP 组"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/ip-groups [post]
func CreateIPGroupHandler(c *gin.Context) {
	var input IPGroupInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	group, err := CreateIPGroup(c.Request.Context(), input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(group))
}

// UpdateIPGroupHandler 更新 WAF IP 组。
// @Summary 更新 WAF IP 组
// @Description 按 ID 更新 WAF IP 组，需要管理员权限
// @Tags openflare-waf
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "IP 组 ID"
// @Param request body waf.IPGroupInput true "IP 组参数"
// @Success 200 {object} response.Any{data=waf.IPGroupView} "更新后的 IP 组"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/ip-groups/{id}/update [post]
func UpdateIPGroupHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input IPGroupInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	group, err := UpdateIPGroup(c.Request.Context(), id, input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(group))
}

// DeleteIPGroupHandler 删除 WAF IP 组。
// @Summary 删除 WAF IP 组
// @Description 按 ID 删除 WAF IP 组，需要管理员权限
// @Tags openflare-waf
// @Produce json
// @Security SessionCookie
// @Param id path int true "IP 组 ID"
// @Success 200 {object} response.Any "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/ip-groups/{id}/delete [post]
func DeleteIPGroupHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	if err := DeleteIPGroup(c.Request.Context(), id); handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// SyncIPGroupHandler 触发 WAF IP 组同步。
// @Summary 同步 WAF IP 组
// @Description 手动触发 WAF IP 组外部 IP 同步，需要管理员权限
// @Tags openflare-waf
// @Produce json
// @Security SessionCookie
// @Param id path int true "IP 组 ID"
// @Success 200 {object} response.Any{data=waf.IPGroupSyncResult} "同步结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/ip-groups/{id}/sync [post]
func SyncIPGroupHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	result, err := SyncIPGroup(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

// TestIPGroupAutoConfigHandler 测试 WAF IP 组自动配置。
// @Summary 测试 WAF IP 组自动配置
// @Description 根据自动配置规则测试 IP 匹配结果（桩实现），需要管理员权限
// @Tags openflare-waf
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body waf.IPGroupAutoTestInput true "自动配置参数"
// @Success 200 {object} response.Any{data=waf.IPGroupAutoTestResult} "测试结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/ip-groups/test [post]
func TestIPGroupAutoConfigHandler(c *gin.Context) {
	var input IPGroupAutoTestInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	result, err := TestIPGroupAutoConfig(c.Request.Context(), input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}