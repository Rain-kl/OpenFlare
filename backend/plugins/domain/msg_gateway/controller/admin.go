// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/msg_gateway/consts"
	"Wavelet/plugins/domain/msg_gateway/service"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ListAdminChannelDefinitions returns form schemas for supported channel types.
// @Summary List message gateway channel definitions
// @Description Returns form field definitions for Telegram and QQ channels
// @Tags admin-message-gateway
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]do.Definition}
// @Router /api/v1/admin/message-gateway/channels/definitions [get]
func ListAdminChannelDefinitions(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(service.ListDefinitions()))
}

// ListAdminChannels lists configured messaging channels with secrets masked.
// @Summary List message gateway channels
// @Description Returns all messaging channels; secrets are masked
// @Tags admin-message-gateway
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]do.ChannelDTO}
// @Router /api/v1/admin/message-gateway/channels [get]
func ListAdminChannels(c *gin.Context) {
	rows, err := service.ListChannels(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(rows))
}

func parseAdminChannelID(c *gin.Context) (uint64, bool) {
	return parseUint64Param(c, "id", consts.ErrInvalidChannelID)
}

func handleAdminChannelError(c *gin.Context, err error, fallback func(c *gin.Context, msg string)) {
	if errors.Is(err, consts.ErrChannelNotFound) || err.Error() == consts.ErrChannelNotFoundText {
		response.AbortNotFound(c, consts.ErrChannelNotFoundText)
		return
	}
	fallback(c, err.Error())
}

// CreateAdminChannel creates a messaging channel.
// @Summary Create message gateway channel
// @Description Creates a Telegram or QQ channel with encrypted credentials
// @Tags admin-message-gateway
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body do.CreateChannelRequest true "create body"
// @Success 200 {object} response.Any{data=do.ChannelDTO}
// @Failure 400 {object} response.Any
// @Router /api/v1/admin/message-gateway/channels [post]
func CreateAdminChannel(c *gin.Context) {
	handleJSONRequest(c, service.CreateChannel)
}

// UpdateAdminChannel patches a messaging channel. Empty secrets keep the previous values.
// @Summary Update message gateway channel
// @Description Updates a channel; empty secrets keep the current ciphertext
// @Tags admin-message-gateway
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "channel id"
// @Param request body do.UpdateChannelRequest true "update body"
// @Success 200 {object} response.Any{data=do.ChannelDTO}
// @Failure 400 {object} response.Any
// @Failure 404 {object} response.Any
// @Router /api/v1/admin/message-gateway/channels/{id} [patch]
func UpdateAdminChannel(c *gin.Context) {
	handleEntityUpdate(c, parseAdminChannelID, service.UpdateChannel, func(c *gin.Context, err error) {
		handleAdminChannelError(c, err, response.AbortBadRequest)
	})
}

// DeleteAdminChannel removes a channel and its bindings/pairing codes.
// @Summary Delete message gateway channel
// @Description Deletes a channel and cascaded bindings and pairing codes
// @Tags admin-message-gateway
// @Produce json
// @Security SessionCookie
// @Param id path int true "channel id"
// @Success 200 {object} response.Any
// @Failure 404 {object} response.Any
// @Router /api/v1/admin/message-gateway/channels/{id} [delete]
func DeleteAdminChannel(c *gin.Context) {
	id, ok := parseAdminChannelID(c)
	if !ok {
		return
	}
	if err := service.DeleteChannel(c.Request.Context(), id); err != nil {
		handleAdminChannelError(c, err, response.AbortInternal)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// TestAdminChannel probes stored credentials (Telegram getMe or QQ token).
// @Summary Test message gateway channel
// @Description Probes stored credentials without returning secrets
// @Tags admin-message-gateway
// @Produce json
// @Security SessionCookie
// @Param id path int true "channel id"
// @Success 200 {object} response.Any
// @Failure 400 {object} response.Any
// @Failure 404 {object} response.Any
// @Router /api/v1/admin/message-gateway/channels/{id}/test [post]
func TestAdminChannel(c *gin.Context) {
	id, ok := parseAdminChannelID(c)
	if !ok {
		return
	}
	if err := service.ProbeChannel(c.Request.Context(), id); err != nil {
		handleAdminChannelError(c, err, response.AbortBadRequest)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
