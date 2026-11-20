// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package performance

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ListHandler lists Pingora policies, parameter defaults, and eligible nodes.
// @Summary List Pingora performance policies
// @Tags Performance Policies
// @Success 200 {object} response.Any{data=policyList}
// @Failure 401 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /d/performance-policies [get]
func ListHandler(c *gin.Context) {
	result, err := listPolicies(c.Request.Context())
	if err != nil {
		abortPolicyError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

// CreateHandler creates a named performance policy.
// @Summary Create a Pingora performance policy
// @Tags Performance Policies
// @Param request body policyRequest true "Policy name and parameters"
// @Success 200 {object} response.Any{data=policyView}
// @Failure 400 {object} response.Any
// @Failure 401 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /d/performance-policies [post]
func CreateHandler(c *gin.Context) {
	savePolicyHandler(c, 0)
}

// UpdateHandler updates the performance policy used by assigned nodes.
// @Summary Update a Pingora performance policy
// @Tags Performance Policies
// @Param id path int true "Policy ID"
// @Param request body policyRequest true "Policy name and parameters"
// @Success 200 {object} response.Any{data=policyView}
// @Failure 400 {object} response.Any
// @Failure 401 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /d/performance-policies/{id}/update [post]
func UpdateHandler(c *gin.Context) {
	if id := parsePolicyID(c); id != 0 {
		savePolicyHandler(c, id)
	}
}

// AssignHandler replaces the nodes that use the selected policy.
// @Summary Assign a Pingora performance policy to a set of nodes
// @Tags Performance Policies
// @Param id path int true "Policy ID"
// @Param request body assignmentRequest true "Selected Pingora node IDs"
// @Success 200 {object} response.Any
// @Failure 400 {object} response.Any
// @Failure 401 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /d/performance-policies/{id}/assign [post]
func AssignHandler(c *gin.Context) {
	id := parsePolicyID(c)
	if id == 0 {
		return
	}
	var request assignmentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}
	if err := assignPolicy(c.Request.Context(), id, request.NodeIDs); err != nil {
		abortPolicyError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// DeleteHandler deletes a policy and clears its assignments.
// @Summary Delete a Pingora performance policy
// @Tags Performance Policies
// @Param id path int true "Policy ID"
// @Success 200 {object} response.Any
// @Failure 400 {object} response.Any
// @Failure 401 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /d/performance-policies/{id}/delete [post]
func DeleteHandler(c *gin.Context) {
	id := parsePolicyID(c)
	if id == 0 {
		return
	}
	if err := repository.DeletePerformancePolicy(c.Request.Context(), id); err != nil {
		abortPolicyError(c, err)
		return
	}
	websocket.NotifyAgentConfigChanged(c.Request.Context())
	c.JSON(http.StatusOK, response.OKNil())
}

func savePolicyHandler(c *gin.Context, id uint) {
	var request policyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}
	result, err := savePolicy(c.Request.Context(), id, request)
	if err != nil {
		abortPolicyError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

func parsePolicyID(c *gin.Context) uint {
	id, err := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize)
	if err != nil || id == 0 {
		response.AbortBadRequest(c, errInvalidParams)
		return 0
	}
	return uint(id)
}

func abortPolicyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errInvalidPolicy):
		response.AbortBadRequest(c, err.Error())
	case errors.Is(err, repository.ErrInvalidPerformancePolicyNodes):
		response.AbortBadRequest(c, errInvalidNodes)
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.AbortNotFound(c, errPolicyNotFound)
	default:
		logger.ErrorF(c.Request.Context(), "performance policy operation failed: %v", err)
		response.AbortInternal(c, errPolicyOperation)
	}
}
