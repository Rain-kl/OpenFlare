package controller

import (
	"openflare/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

type nodeAgentUpdateRequest struct {
	Channel string `json:"channel"`
	TagName string `json:"tag_name"`
}

type nodeObservabilityQuery struct {
	Hours int `form:"hours"`
	Limit int `form:"limit"`
}

// CreateNode godoc
// @Summary Create node
// @Tags Nodes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body service.NodeInput true "Node payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/ [post]
func CreateNode(c *gin.Context) {
	var input service.NodeInput
	if err := decodeJSONBody(c.Request.Body, &input); err != nil {
		respondBadRequest(c, "")
		return
	}

	node, err := service.CreateNode(input)
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, node)
}

// GetNodeBootstrapToken godoc
// @Summary Get global discovery token
// @Tags Nodes
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/nodes/bootstrap-token [get]
func GetNodeBootstrapToken(c *gin.Context) {
	bootstrap, err := service.GetNodeBootstrapView()
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, bootstrap)
}

// RotateNodeBootstrapToken godoc
// @Summary Rotate global discovery token
// @Tags Nodes
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/nodes/bootstrap-token/rotate [post]
func RotateNodeBootstrapToken(c *gin.Context) {
	bootstrap, err := service.RotateGlobalDiscoveryToken()
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, bootstrap)
}

// UpdateNode godoc
// @Summary Update node
// @Tags Nodes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Node ID"
// @Param payload body service.NodeInput true "Node payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/{id}/update [post]
func UpdateNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		respondBadRequest(c, "")
		return
	}

	var input service.NodeInput
	if err = decodeJSONBody(c.Request.Body, &input); err != nil {
		respondBadRequest(c, "")
		return
	}

	node, err := service.UpdateNode(uint(id), input)
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, node)
}

// DeleteNode godoc
// @Summary Delete node
// @Tags Nodes
// @Produce json
// @Security BearerAuth
// @Param id path int true "Node ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/{id}/delete [post]
func DeleteNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		respondBadRequest(c, "")
		return
	}

	if err = service.DeleteNode(uint(id)); err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccessMessage(c, "")
}

// RequestNodeAgentUpdate godoc
// @Summary Request agent self-update on node
// @Tags Nodes
// @Produce json
// @Security BearerAuth
// @Param id path int true "Node ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/{id}/agent-update [post]
func RequestNodeAgentUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		respondBadRequest(c, "")
		return
	}

	var request nodeAgentUpdateRequest
	if c.Request.ContentLength > 0 {
		if err = decodeOptionalJSONBody(c.Request.Body, &request); err != nil {
			respondBadRequest(c, "")
			return
		}
	}

	node, err := service.RequestNodeAgentUpdate(uint(id), service.NodeAgentUpdateInput{
		Channel: request.Channel,
		TagName: request.TagName,
	})
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, node)
}

// RequestNodeOpenrestyRestart godoc
// @Summary Request openresty restart on node
// @Tags Nodes
// @Produce json
// @Security BearerAuth
// @Param id path int true "Node ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/{id}/openresty-restart [post]
func RequestNodeOpenrestyRestart(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		respondBadRequest(c, "")
		return
	}

	node, err := service.RequestNodeOpenrestyRestart(uint(id))
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, node)
}

// GetNodeAgentRelease godoc
// @Summary Check latest agent release for node
// @Tags Nodes
// @Produce json
// @Security BearerAuth
// @Param id path int true "Node ID"
// @Param channel query string false "stable or preview"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/{id}/agent-release [get]
func GetNodeAgentRelease(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		respondBadRequest(c, "")
		return
	}

	release, err := service.GetNodeAgentRelease(c.Request.Context(), uint(id), c.Query("channel"))
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, release)
}

// GetNodeObservability godoc
// @Summary Get node observability details
// @Tags Nodes
// @Produce json
// @Security BearerAuth
// @Param id path int true "Node ID"
// @Param hours query int false "Lookback window in hours"
// @Param limit query int false "Max records per section"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/{id}/observability [get]
func GetNodeObservability(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		respondBadRequest(c, "")
		return
	}

	var query nodeObservabilityQuery
	if err = c.ShouldBindQuery(&query); err != nil {
		respondBadRequest(c, "")
		return
	}

	view, err := service.GetNodeObservability(uint(id), service.NodeObservabilityQuery{
		Hours: query.Hours,
		Limit: query.Limit,
	})
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, view)
}

// CleanupNodeHealthEvents godoc
// @Summary Cleanup node health events
// @Tags Nodes
// @Produce json
// @Security BearerAuth
// @Param id path int true "Node ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/{id}/observability/cleanup [post]
func CleanupNodeHealthEvents(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		respondBadRequest(c, "")
		return
	}

	result, err := service.CleanupNodeHealthEvents(uint(id))
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, result)
}
