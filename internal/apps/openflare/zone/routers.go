// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package zone

import (
	"errors"
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func abort(c *gin.Context, err error, missing string) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.AbortNotFound(c, missing)
	case err.Error() == errDomainExists:
		response.AbortConflict(c, err.Error())
	default:
		response.AbortBadRequest(c, err.Error())
	}
	return true
}

// ListHandler lists registered Zones.
// @Summary 获取 Zone 列表
// @Tags openflare-zone
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]zone.ListItem}
// @Router /api/v1/d/zones [get]
func ListHandler(c *gin.Context) {
	items, err := List(c.Request.Context())
	if abort(c, err, errZoneNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(items))
}

// CreateHandler creates a registered root domain.
// @Summary 创建 Zone
// @Tags openflare-zone
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body zone.Input true "Zone 参数"
// @Success 200 {object} response.Any{data=model.Zone}
// @Failure 400 {object} response.Any
// @Failure 409 {object} response.Any
// @Router /api/v1/d/zones [post]
func CreateHandler(c *gin.Context) {
	var input Input
	if !apiutil.BindJSON(c, &input) {
		return
	}
	item, err := Create(c.Request.Context(), input)
	if abort(c, err, errZoneNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item))
}

// GetOverviewHandler returns a Zone and its explicit domains.
// @Summary 获取 Zone 概览
// @Tags openflare-zone
// @Produce json
// @Security SessionCookie
// @Param id path int true "Zone ID"
// @Success 200 {object} response.Any{data=zone.Overview}
// @Failure 404 {object} response.Any
// @Router /api/v1/d/zones/{id}/overview [get]
func GetOverviewHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	item, err := GetOverview(c.Request.Context(), id)
	if abort(c, err, errZoneNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item))
}

// GetStatsHandler returns Zone traffic metrics for a time range.
// @Summary 获取 Zone 流量统计
// @Description 按 Zone 下全部域名聚合访问日志：唯一访问者、请求总数、已提供数据（字节）。range 支持 24h/7d/30d。
// @Tags openflare-zone
// @Produce json
// @Security SessionCookie
// @Param id path int true "Zone ID"
// @Param range query string false "时间范围：24h（默认）、7d、30d"
// @Success 200 {object} response.Any{data=zone.Stats}
// @Failure 400 {object} response.Any
// @Failure 404 {object} response.Any
// @Router /api/v1/d/zones/{id}/stats [get]
func GetStatsHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	item, err := GetStats(c.Request.Context(), id, c.Query("range"))
	if abort(c, err, errZoneNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item))
}

// UpdateHandler updates a Zone.
// @Summary 更新 Zone
// @Tags openflare-zone
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "Zone ID"
// @Param body body zone.Input true "Zone 参数"
// @Success 200 {object} response.Any{data=model.Zone}
// @Failure 400 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 409 {object} response.Any
// @Router /api/v1/d/zones/{id}/update [post]
func UpdateHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input Input
	if !apiutil.BindJSON(c, &input) {
		return
	}
	item, err := Update(c.Request.Context(), id, input)
	if abort(c, err, errZoneNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item))
}

// DeleteHandler deletes a Zone with no remaining domains.
// @Summary 删除 Zone
// @Tags openflare-zone
// @Produce json
// @Security SessionCookie
// @Param id path int true "Zone ID"
// @Success 200 {object} response.Any
// @Failure 400 {object} response.Any
// @Failure 404 {object} response.Any
// @Router /api/v1/d/zones/{id}/delete [post]
func DeleteHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	if err := Delete(c.Request.Context(), id); abort(c, err, errZoneNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// CreateDomainHandler creates an explicit FQDN under a Zone.
// @Summary 创建 Zone 域名
// @Tags openflare-zone
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "Zone ID"
// @Param body body zone.DomainInput true "域名参数"
// @Success 200 {object} response.Any{data=model.ZoneDomain}
// @Failure 400 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 409 {object} response.Any
// @Router /api/v1/d/zones/{id}/domains [post]
func CreateDomainHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input DomainInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	item, err := CreateDomain(c.Request.Context(), id, input)
	if abort(c, err, errZoneNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item))
}

// UpdateDomainHandler updates a Zone domain.
// @Summary 更新 Zone 域名
// @Tags openflare-zone
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "Zone ID"
// @Param domainId path int true "域名 ID"
// @Param body body zone.DomainInput true "域名参数"
// @Success 200 {object} response.Any{data=model.ZoneDomain}
// @Failure 400 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 409 {object} response.Any
// @Router /api/v1/d/zones/{id}/domains/{domainId}/update [post]
func UpdateDomainHandler(c *gin.Context) {
	zoneID, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	domainID, ok := apiutil.NamedIDParam(c, "domainId")
	if !ok {
		return
	}
	var input DomainInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	item, err := UpdateDomain(c.Request.Context(), zoneID, domainID, input)
	if abort(c, err, errDomainNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item))
}

// DeleteDomainHandler deletes a Zone domain not bound to a proxy route.
// @Summary 删除 Zone 域名
// @Tags openflare-zone
// @Produce json
// @Security SessionCookie
// @Param id path int true "Zone ID"
// @Param domainId path int true "域名 ID"
// @Success 200 {object} response.Any
// @Failure 400 {object} response.Any
// @Failure 404 {object} response.Any
// @Router /api/v1/d/zones/{id}/domains/{domainId}/delete [post]
func DeleteDomainHandler(c *gin.Context) {
	zoneID, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	domainID, ok := apiutil.NamedIDParam(c, "domainId")
	if !ok {
		return
	}
	if err := DeleteDomain(c.Request.Context(), zoneID, domainID); abort(c, err, errDomainNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
