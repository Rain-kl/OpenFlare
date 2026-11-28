// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
)

const (
	decimalBase = 10
	bitSize32   = 32
	bitSize64   = 64
)

// ParseUint32 parses a base-10 unsigned 32-bit integer string without narrowing casts.
func ParseUint32(raw string) (uint32, error) {
	v, err := strconv.ParseUint(raw, decimalBase, bitSize32)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
}

// ParseUint64 parses a base-10 unsigned 64-bit integer string.
func ParseUint64(raw string) (uint64, error) {
	return strconv.ParseUint(raw, decimalBase, bitSize64)
}

// ParseUint parses a base-10 unsigned platform integer string without narrowing casts.
func ParseUint(raw string) (uint, error) {
	v, err := strconv.ParseUint(raw, decimalBase, strconv.IntSize)
	if err != nil {
		return 0, err
	}
	return uint(v), nil
}

// PositiveUint32Param parses a required non-zero uint32 URL path parameter, aborting with 400 if invalid.
func PositiveUint32Param(c *gin.Context, name, invalidMsg string) (uint32, bool) {
	v, err := ParseUint32(c.Param(name))
	if err != nil || v == 0 {
		response.AbortBadRequest(c, invalidMsg)
		return 0, false
	}
	return v, true
}

// OptionalPositiveUint32Query parses an optional positive uint32 query parameter.
// An empty query value returns 0 and true; any non-empty invalid or zero value aborts with 400.
func OptionalPositiveUint32Query(c *gin.Context, name, invalidMsg string) (uint32, bool) {
	raw := c.Query(name)
	if raw == "" {
		return 0, true
	}
	v, err := ParseUint32(strings.TrimSpace(raw))
	if err != nil || v == 0 {
		response.AbortBadRequest(c, invalidMsg)
		return 0, false
	}
	return v, true
}

// PositiveUint64Param parses a required non-zero uint64 URL path parameter, aborting with 400 if invalid.
func PositiveUint64Param(c *gin.Context, name, invalidMsg string) (uint64, bool) {
	v, err := ParseUint64(c.Param(name))
	if err != nil || v == 0 {
		response.AbortBadRequest(c, invalidMsg)
		return 0, false
	}
	return v, true
}
