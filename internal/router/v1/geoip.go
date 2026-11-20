// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"github.com/Rain-kl/Wavelet/internal/apps/geoip"
	"github.com/gin-gonic/gin"
)

func registerGeoIPRoutes(apiGroup *gin.RouterGroup) {
	apiGroup.GET("/geoip", geoip.GetGeoIPHandler)
}
