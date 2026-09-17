// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/ginutil"
	"Wavelet/pkg/response"
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// currentUser extracts the authenticated UserDTO from gin.Context.
func currentUser(c *gin.Context) (*contracts.UserDTO, bool) {
	return ginutil.GetFromContext[*contracts.UserDTO](c, contracts.AuthUserObjKey)
}

// parseUint64Param parses a uint64 URL path parameter.
func parseUint64Param(c *gin.Context, paramName, errInvalid string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(paramName), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, errInvalid)
		return 0, false
	}
	return id, true
}

// handleJSONRequest binds a JSON body, executes the service handler, and writes the standard success envelope.
func handleJSONRequest[Req any, Res any](c *gin.Context, handler func(ctx context.Context, req Req) (Res, error)) {
	var req Req
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	res, err := handler(c.Request.Context(), req)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// handleEntityUpdate resolves a path identifier and JSON body, executes the updater, and handles errors with onErr.
func handleEntityUpdate[Req any, Res any](
	c *gin.Context,
	parseID func(*gin.Context) (uint64, bool),
	updater func(ctx context.Context, id uint64, req Req) (Res, error),
	onErr func(*gin.Context, error),
) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req Req
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	dto, err := updater(c.Request.Context(), id, req)
	if err != nil {
		onErr(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(dto))
}
