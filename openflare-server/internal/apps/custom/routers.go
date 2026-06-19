// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package custom provides custom business handlers
package custom

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

// Hello is a sample handler for custom business logic
// @Summary Sample Hello API
// @Description A sample business API for customization
// @Tags custom
// @Produce json
// @Success 200 {object} response.Any{data=string} "成功"
// @Router /api/v1/custom/hello [get]
func Hello(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK("Hello from custom business module!"))
}
