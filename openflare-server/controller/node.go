package controller

import (
	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/controller/bind"
	"github.com/rain-kl/openflare/openflare-server/service"

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
// @Security OpenFlareTokenAuth
// @Param payload body service.NodeInput true "Node payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/ [post]
func CreateNode(c *gin.Context) {
	var input service.NodeInput
	if !bind.JSON(c, &input) {
		return
	}

	node, err := service.CreateNode(input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, node)
}

// GetNodeBootstrapToken godoc
// @Summary Get global discovery token
// @Tags Nodes
// @Produce json
// @Security OpenFlareTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/nodes/bootstrap-token [get]
func GetNodeBootstrapToken(c *gin.Context) {
	bootstrap, err := service.GetNodeBootstrapView()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, bootstrap)
}

// RotateNodeBootstrapToken godoc
// @Summary Rotate global discovery token
// @Tags Nodes
// @Produce json
// @Security OpenFlareTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/nodes/bootstrap-token/rotate [post]
func RotateNodeBootstrapToken(c *gin.Context) {
	bootstrap, err := service.RotateGlobalDiscoveryToken()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, bootstrap)
}

// UpdateNode godoc
// @Summary Update node
// @Tags Nodes
// @Accept json
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Node ID"
// @Param payload body service.NodeInput true "Node payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/{id}/update [post]
func UpdateNode(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}

	var input service.NodeInput
	if !bind.JSON(c, &input) {
		return
	}

	node, err := service.UpdateNode(id, input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, node)
}

// DeleteNode godoc
// @Summary Delete node
// @Tags Nodes
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Node ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/{id}/delete [post]
func DeleteNode(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}

	if err := service.DeleteNode(id); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccessMessage(c, "")
}

// RequestNodeAgentUpdate godoc
// @Summary Request agent self-update on node
// @Tags Nodes
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Node ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/{id}/agent-update [post]
func RequestNodeAgentUpdate(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}

	var request nodeAgentUpdateRequest
	if c.Request.ContentLength > 0 {
		if err := bind.OptionalJSON(c.Request.Body, &request); err != nil {
			response.RespondBadRequest(c, "")
			return
		}
	}

	node, err := service.RequestNodeAgentUpdate(id, service.NodeAgentUpdateInput{
		Channel: request.Channel,
		TagName: request.TagName,
	})
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, node)
}

// RequestNodeOpenrestyRestart godoc
// @Summary Request openresty restart on node
// @Tags Nodes
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Node ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/{id}/openresty-restart [post]
func RequestNodeOpenrestyRestart(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}

	node, err := service.RequestNodeOpenrestyRestart(id)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, node)
}

// RequestNodeForceSync godoc
// @Summary Request force sync config on node
// @Tags Nodes
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Node ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/{id}/force-sync [post]
func RequestNodeForceSync(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}

	node, err := service.RequestNodeForceSync(id)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, node)
}

// GetNodeAgentRelease godoc
// @Summary Check latest agent release for node
// @Tags Nodes
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Node ID"
// @Param channel query string false "stable or preview"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/{id}/agent-release [get]
func GetNodeAgentRelease(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}

	release, err := service.GetNodeAgentRelease(c.Request.Context(), id, c.Query("channel"))
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, release)
}

// GetNodeObservability godoc
// @Summary Get node observability details
// @Tags Nodes
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Node ID"
// @Param hours query int false "Lookback window in hours"
// @Param limit query int false "Max records per section"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/{id}/observability [get]
func GetNodeObservability(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}

	var query nodeObservabilityQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.RespondBadRequest(c, "")
		return
	}

	view, err := service.GetNodeObservability(id, service.NodeObservabilityQuery{
		Hours: query.Hours,
		Limit: query.Limit,
	})
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, view)
}

// CleanupNodeHealthEvents godoc
// @Summary Cleanup node health events
// @Tags Nodes
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Node ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/nodes/{id}/observability/cleanup [post]
func CleanupNodeHealthEvents(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}

	result, err := service.CleanupNodeHealthEvents(id)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, result)
}
