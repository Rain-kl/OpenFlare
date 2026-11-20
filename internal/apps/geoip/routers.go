// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package geoip

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// GetGeoIPHandler returns the requester's IP address observed by the control plane.
// @Summary 获取请求方 IP 地址
// @Description 返回主控端识别到的请求者 IP 地址，供边缘节点在外部探测接口不可用时回退获取自身 IP
// @Tags geoip
// @Produce json
// @Success 200 {object} response.Any{data=geoip.ClientIPView} "请求方 IP 地址"
// @Router /api/geoip [get]
func GetGeoIPHandler(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, response.OK(ResolveRequesterIP(c.ClientIP(), c.Request.RemoteAddr)))
}
