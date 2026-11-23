// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/shared/httpx"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// ListRequestWhitelistHandler lists the global WAF request whitelist.
// @Summary 列出全局防护白名单
// @Tags openflare-waf
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]model.OpenFlareWAFWhitelistEntry}
// @Failure 401 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /api/v1/d/waf/request-whitelist [get]
func ListRequestWhitelistHandler(c *gin.Context) {
	entries, err := repository.ListOpenFlareWAFWhitelist(c.Request.Context())
	if !handleRuleError(c, err) {
		c.JSON(http.StatusOK, response.OK(entries))
	}
}

// CreateRequestWhitelistHandler creates a host and path exemption.
// @Summary 添加全局防护白名单
// @Description 域名与路径均匹配时，跳过全局和站点绑定的 WAF 流程。
// @Tags openflare-waf
// @Security SessionCookie
// @Param request body whitelistEntryInput true "白名单条目"
// @Success 200 {object} response.Any{data=model.OpenFlareWAFWhitelistEntry}
// @Failure 400 {object} response.Any
// @Failure 401 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /api/v1/d/waf/request-whitelist [post]
func CreateRequestWhitelistHandler(c *gin.Context) { saveRequestWhitelistHandler(c, 0) }

// UpdateRequestWhitelistHandler replaces the matching conditions for a whitelist entry.
// @Summary 更新全局防护白名单
// @Tags openflare-waf
// @Security SessionCookie
// @Param id path int true "条目 ID"
// @Param request body whitelistEntryInput true "白名单条目"
// @Success 200 {object} response.Any{data=model.OpenFlareWAFWhitelistEntry}
// @Failure 400 {object} response.Any
// @Failure 401 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /api/v1/d/waf/request-whitelist/{id}/update [post]
func UpdateRequestWhitelistHandler(c *gin.Context) {
	if id, ok := httpx.IDParam(c); ok {
		saveRequestWhitelistHandler(c, id)
	}
}

func saveRequestWhitelistHandler(c *gin.Context, id uint) {
	var input whitelistEntryInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	entry, err := saveWhitelistEntry(c.Request.Context(), id, input)
	if !handleRuleError(c, err) {
		c.JSON(http.StatusOK, response.OK(entry))
	}
}

// DeleteRequestWhitelistHandler removes a global whitelist entry.
// @Summary 删除全局防护白名单
// @Tags openflare-waf
// @Security SessionCookie
// @Param id path int true "条目 ID"
// @Success 200 {object} response.Any
// @Failure 400 {object} response.Any
// @Failure 401 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /api/v1/d/waf/request-whitelist/{id}/delete [post]
func DeleteRequestWhitelistHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	if !handleRuleError(c, repository.DeleteOpenFlareWAFWhitelistEntry(c.Request.Context(), id)) {
		c.JSON(http.StatusOK, response.OKNil())
	}
}
