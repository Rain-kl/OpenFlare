package controller

import (
	"atsflare/model"
	"atsflare/service"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AgentRegister godoc
// @Summary Register or discover agent node
// @Tags Agent
// @Accept json
// @Produce json
// @Security AgentTokenAuth
// @Param payload body service.AgentNodePayload true "Agent node payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/agent/nodes/register [post]
func AgentRegister(c *gin.Context) {
	var payload service.AgentNodePayload
	if err := json.NewDecoder(c.Request.Body).Decode(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	var (
		result *service.AgentRegistrationResponse
		err    error
	)
	if authNode, ok := c.Get("agent_node"); ok {
		result, err = service.RegisterNodeWithAgentToken(authNode.(*model.Node), payload)
	} else {
		result, err = service.RegisterNodeWithDiscovery(payload)
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}

// AgentHeartbeat godoc
// @Summary Report agent heartbeat
// @Tags Agent
// @Accept json
// @Produce json
// @Security AgentTokenAuth
// @Param payload body service.AgentNodePayload true "Agent heartbeat payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/agent/nodes/heartbeat [post]
func AgentHeartbeat(c *gin.Context) {
	var payload service.AgentNodePayload
	if err := json.NewDecoder(c.Request.Body).Decode(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	authNode, ok := c.Get("agent_node")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "无权进行此操作，Agent Token 无效",
		})
		return
	}
	node, err := service.HeartbeatNode(authNode.(*model.Node), payload)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        "",
		"data":           node.Node,
		"agent_settings": node.AgentSettings,
		"active_config":  node.ActiveConfig,
	})
}

// AgentGetActiveConfig godoc
// @Summary Get active config for agent
// @Tags Agent
// @Produce json
// @Security AgentTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/agent/config-versions/active [get]
func AgentGetActiveConfig(c *gin.Context) {
	config, err := service.GetActiveConfigForAgent()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "当前没有激活版本",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    config,
	})
}

// AgentReportApplyLog godoc
// @Summary Report agent apply result
// @Tags Agent
// @Accept json
// @Produce json
// @Security AgentTokenAuth
// @Param payload body service.ApplyLogPayload true "Apply log payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/agent/apply-logs [post]
func AgentReportApplyLog(c *gin.Context) {
	var payload service.ApplyLogPayload
	if err := json.NewDecoder(c.Request.Body).Decode(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	authNode, ok := c.Get("agent_node")
	if ok {
		payload.NodeID = authNode.(*model.Node).NodeID
	}
	log, err := service.ReportApplyLog(payload)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    log,
	})
}

// GetNodes godoc
// @Summary List nodes
// @Tags Nodes
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/nodes/ [get]
func GetNodes(c *gin.Context) {
	nodes, err := service.ListNodeViews()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    nodes,
	})
}

// GetApplyLogs godoc
// @Summary List apply logs
// @Tags ApplyLogs
// @Produce json
// @Security BearerAuth
// @Param node_id query string false "Node ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/apply-logs/ [get]
func GetApplyLogs(c *gin.Context) {
	logs, err := service.ListApplyLogs(c.Query("node_id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
}
