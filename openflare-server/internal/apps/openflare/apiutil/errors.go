// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package apiutil

import (
	"errors"

	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AbortNotFoundIfMissing maps gorm.ErrRecordNotFound to 404; other errors to 400.
func AbortNotFoundIfMissing(c *gin.Context, err error, notFoundMsg string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.AbortNotFound(c, notFoundMsg)
		return true
	}
	response.AbortBadRequest(c, err.Error())
	return true
}

// AbortBadRequestOnError writes a 400 for any non-nil error.
func AbortBadRequestOnError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	response.AbortBadRequest(c, err.Error())
	return true
}