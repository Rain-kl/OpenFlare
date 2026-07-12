// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package zone

import (
	"errors"
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/common/response"
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
// @Success 200 {object} response.Any{data=[]model.Zone}
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
