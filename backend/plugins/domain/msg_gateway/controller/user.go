// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/msg_gateway/consts"
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"Wavelet/plugins/domain/msg_gateway/service"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ListChannels lists enabled channels a user can bind.
// @Summary List enabled messaging channels
// @Description Returns enabled system bots the current user can pair with
// @Tags message-gateway
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]do.PublicChannelDTO}
// @Failure 401 {object} response.Any
// @Router /api/v1/message-gateway/channels [get]
func ListChannels(c *gin.Context) {
	if user, ok := currentUser(c); !ok || user == nil {
		response.AbortUnauthorized(c, consts.ErrLoginRequired)
		return
	}
	rows, err := service.ListEnabledPublicChannels(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(rows))
}

// ListBindings lists the current user's bot bindings.
// @Summary List message gateway bindings
// @Description Returns the current user's bound messaging channels
// @Tags message-gateway
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]do.BindingDTO}
// @Failure 401 {object} response.Any
// @Router /api/v1/message-gateway/bindings [get]
func ListBindings(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok || user == nil {
		response.AbortUnauthorized(c, consts.ErrLoginRequired)
		return
	}
	rows, err := service.ListUserBindings(c.Request.Context(), user.ID)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(rows))
}

// BindBinding consumes a pairing code and binds the platform identity.
// @Summary Bind a messaging channel
// @Description Binds the current user to a platform identity using a one-time pairing code
// @Tags message-gateway
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body do.BindRequest true "bind body"
// @Success 200 {object} response.Any{data=do.BindingDTO}
// @Failure 400 {object} response.Any
// @Failure 409 {object} response.Any
// @Router /api/v1/message-gateway/bindings [post]
func BindBinding(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok || user == nil {
		response.AbortUnauthorized(c, consts.ErrLoginRequired)
		return
	}
	var req do.BindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	dto, err := service.BindChannel(c.Request.Context(), user.ID, req)
	if err != nil {
		if errors.Is(err, consts.ErrPlatformAlreadyBound) {
			response.AbortConflict(c, err.Error())
			return
		}
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(dto))
}

// UnbindBinding removes the current user's binding.
// @Summary Unbind a messaging channel
// @Description Removes a binding owned by the current user
// @Tags message-gateway
// @Produce json
// @Security SessionCookie
// @Param id path int true "binding id"
// @Success 200 {object} response.Any
// @Failure 403 {object} response.Any
// @Failure 404 {object} response.Any
// @Router /api/v1/message-gateway/bindings/{id} [delete]
func UnbindBinding(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok || user == nil {
		response.AbortUnauthorized(c, consts.ErrLoginRequired)
		return
	}
	id, ok := parseUint64Param(c, "id", consts.ErrInvalidBindingID)
	if !ok {
		return
	}
	if err := service.UnbindChannel(c.Request.Context(), user.ID, id); err != nil {
		if errors.Is(err, consts.ErrBindingNotFound) {
			response.AbortNotFound(c, err.Error())
			return
		}
		if errors.Is(err, consts.ErrBindingForbidden) {
			response.AbortForbidden(c, err.Error())
			return
		}
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
