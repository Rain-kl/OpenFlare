// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package apiutil

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// RegisterCollection registers a collection endpoint on both "" and "/" so requests
// work with or without a trailing slash.
func RegisterCollection(route *gin.RouterGroup, method string, handlers ...gin.HandlerFunc) {
	route.Handle(method, "/", handlers...)
	if !strings.HasSuffix(route.BasePath(), "/") {
		route.Handle(method, "", handlers...)
	}
}