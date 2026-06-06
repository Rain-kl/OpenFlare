package controller

import (
	"encoding/json"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/rain-kl/openflare/openflare-server/common"
	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/controller/bind"
	"github.com/rain-kl/openflare/openflare-server/model"
	"github.com/rain-kl/openflare/openflare-server/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/websocket"
)

// AgentRegister godoc
// @Summary Register or discover agent node
// @Tags Agent
// @Accept json
// @Produce json
// @Security AccessTokenAuth
// @Param payload body service.AgentNodePayload true "Agent node payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/agent/nodes/register [post]
func AgentRegister(c *gin.Context) {
	var payload service.AgentNodePayload
	if !bind.JSON(c, &payload) {
		return
	}
	payload.IP = service.ResolveReportedNodeIP(payload.IP, c.Request.RemoteAddr)

	var (
		result *service.AgentRegistrationResponse
		err    error
	)
	if authNode, ok := c.Get("agent_node"); ok {
		result, err = service.RegisterNodeWithAccessToken(authNode.(*model.Node), payload)
	} else {
		result, err = service.RegisterNodeWithDiscovery(payload)
	}
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, result)
}

// AgentHeartbeat godoc
// @Summary Report agent heartbeat
// @Tags Agent
// @Accept json
// @Produce json
// @Security AccessTokenAuth
// @Param payload body service.AgentNodePayload true "Agent heartbeat payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/agent/nodes/heartbeat [post]
func AgentHeartbeat(c *gin.Context) {
	var payload service.AgentNodePayload
	if !bind.JSON(c, &payload) {
		return
	}
	payload.IP = service.ResolveReportedNodeIP(payload.IP, c.Request.RemoteAddr)

	authNode, ok := c.Get("agent_node")
	if !ok {
		response.RespondUnauthorized(c, "鏃犳潈杩涜姝ゆ搷浣滐紝Agent Token 鏃犳晥")
		return
	}

	node, err := service.HeartbeatNode(authNode.(*model.Node), payload)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccessWithExtras(c, node.Node, gin.H{
		"agent_settings": node.AgentSettings,
		"active_config":  node.ActiveConfig,
		"waf_ip_groups":  node.WAFIPGroups,
	})
}

// AgentSyncWAFIPGroups godoc
// @Summary Sync WAF IP groups for agent
// @Tags Agent
// @Accept json
// @Produce json
// @Security AccessTokenAuth
// @Param payload body service.AgentWAFIPGroupSyncInput true "WAF IP group sync payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/agent/waf/ip-groups/sync [post]
func AgentSyncWAFIPGroups(c *gin.Context) {
	var input service.AgentWAFIPGroupSyncInput
	if !bind.JSON(c, &input) {
		return
	}
	result, err := service.SyncWAFIPGroupsForAgent(input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, result)
}

// AgentGetActiveConfig godoc
// @Summary Get active config for agent
// @Tags Agent
// @Produce json
// @Security AccessTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/agent/config-versions/active [get]
func AgentGetActiveConfig(c *gin.Context) {
	authNode, ok := c.Get("agent_node")
	if !ok {
		response.RespondUnauthorized(c, "Node object missing from context")
		return
	}
	node := authNode.(*model.Node)

	if node.NodeType == "tunnel_client" {
		config, err := service.GetFlaredTunnelConfig(node)
		if err != nil {
			response.RespondFailure(c, "无法生成隧道配置: "+err.Error())
			return
		}
		response.RespondSuccess(c, config)
		return
	}

	config, err := service.GetActiveConfigForAgent()
	if err != nil {
		response.RespondFailure(c, "当前没有激活版本")
		return
	}
	response.RespondSuccess(c, config)
}

// AgentReportApplyLog godoc
// @Summary Report agent apply result
// @Tags Agent
// @Accept json
// @Produce json
// @Security AccessTokenAuth
// @Param payload body service.ApplyLogPayload true "Apply log payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/agent/apply-logs [post]
func AgentReportApplyLog(c *gin.Context) {
	var payload service.ApplyLogPayload
	if !bind.JSON(c, &payload) {
		return
	}

	if authNode, ok := c.Get("agent_node"); ok {
		payload.NodeID = authNode.(*model.Node).NodeID
	}

	log, err := service.ReportApplyLog(payload)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, log)
}

// AgentWebSocket godoc
// @Summary Upgrade agent connection to websocket
// @Tags Agent
// @Security AccessTokenAuth
// @Router /api/agent/ws [get]
func AgentWebSocket(c *gin.Context) {
	authNode, ok := c.Get("agent_node")
	if !ok {
		response.RespondUnauthorized(c, "无权进行此操作，Agent Token 无效")
		return
	}
	node := authNode.(*model.Node)
	slog.Debug("agent ws upgrade requested", "node_id", node.NodeID, "remote", c.Request.RemoteAddr)
	websocket.Handler(func(conn *websocket.Conn) {
		client := service.RegisterAgentWSClient(node.NodeID)
		defer service.UnregisterAgentWSClient(client)
		defer func() {
			_ = conn.Close()
			slog.Debug("agent ws connection closed", "node_id", node.NodeID)
		}()

		slog.Debug("agent ws upgrade succeeded", "node_id", node.NodeID, "remote", c.Request.RemoteAddr)

		go func() {
			<-client.Done()
			_ = conn.Close()
		}()

		go streamAgentWSMessages(c, conn, client)

		for {
			var message service.AgentWSInboundMessage
			_ = conn.SetReadDeadline(time.Now().Add(agentWSReadTimeout()))
			if err := websocket.JSON.Receive(conn, &message); err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					slog.Debug("agent ws receive timeout waiting for status or pong", "node_id", node.NodeID, "timeout", agentWSReadTimeout())
					return
				}
				slog.Debug("agent ws receive failed", "node_id", node.NodeID, "error", err)
				return
			}
			slog.Debug("agent ws message received", "node_id", node.NodeID, "type", message.Type)
			switch message.Type {
			case service.AgentWSMessageTypeStatus:
				handleAgentWSStatus(c, node, message)
			case service.AgentWSMessageTypePing:
				if !service.SendAgentWSPong(node.NodeID) {
					slog.Debug("agent ws pong enqueue failed", "node_id", node.NodeID)
				}
			case service.AgentWSMessageTypePong:
				slog.Debug("agent ws pong received", "node_id", node.NodeID)
			default:
				slog.Debug("agent ws unsupported message type", "node_id", node.NodeID, "type", message.Type)
			}
		}
	}).ServeHTTP(c.Writer, c.Request)
}

func agentWSReadTimeout() time.Duration {
	timeout := time.Duration(common.AgentHeartbeatInterval) * time.Millisecond * 3
	if timeout < 30*time.Second {
		return 30 * time.Second
	}
	return timeout
}

func agentWSWriteTimeout() time.Duration {
	return 10 * time.Second
}

func streamAgentWSMessages(c *gin.Context, conn *websocket.Conn, client *service.WSClient) {
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-client.Done():
			return
		case message, ok := <-client.Messages():
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(agentWSWriteTimeout()))
			if err := websocket.JSON.Send(conn, message); err != nil {
				slog.Debug("agent ws send failed", "node_id", client.ID(), "error", err)
				return
			}
		}
	}
}

func handleAgentWSStatus(c *gin.Context, node *model.Node, message service.AgentWSInboundMessage) {
	var payload service.AgentNodePayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		slog.Debug("agent ws status payload decode failed", "node_id", node.NodeID, "error", err)
		return
	}
	freshNode, err := model.GetNodeByNodeID(node.NodeID)
	if err != nil {
		slog.Debug("agent ws status reload node failed", "node_id", node.NodeID, "error", err)
		return
	}
	payload.IP = service.ResolveReportedNodeIP(payload.IP, c.Request.RemoteAddr)
	res, err := service.HeartbeatNode(freshNode, payload)
	if err != nil {
		slog.Debug("agent ws status handling failed", "node_id", node.NodeID, "error", err)
		return
	}
	settingsSent := service.SendAgentWSSettings(node.NodeID, res.AgentSettings)
	activeConfigSent := false
	if res.ActiveConfig != nil {
		activeConfigSent = service.SendAgentWSActiveConfig(node.NodeID, res.ActiveConfig)
	}
	wafIPGroupsSent := false
	if len(res.WAFIPGroups) > 0 {
		wafIPGroupsSent = service.SendAgentWSWAFIPGroups(node.NodeID, res.WAFIPGroups)
	}
	slog.Debug("agent ws status processed",
		"node_id", node.NodeID,
		"current_version", payload.CurrentVersion,
		"openresty_status", payload.OpenrestyStatus,
		"settings_sent", settingsSent,
		"active_config_sent", activeConfigSent,
		"waf_ip_groups_sent", wafIPGroupsSent,
	)
}

// GetNodes godoc
// @Summary List nodes
// @Tags Nodes
// @Produce json
// @Security OpenFlareTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/nodes/ [get]
func GetNodes(c *gin.Context) {
	nodes, err := service.ListNodeViews()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, nodes)
}

// GetApplyLogs godoc
// @Summary List apply logs
// @Tags ApplyLogs
// @Produce json
// @Security OpenFlareTokenAuth
// @Param node_id query string false "Node ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/apply-logs/ [get]
func GetApplyLogs(c *gin.Context) {
	logs, err := service.ListApplyLogsPage(service.ApplyLogListQuery{
		NodeID:   c.Query("node_id"),
		PageNo:   readIntQueryFallback(c, "pageNo", "page_no"),
		PageSize: readIntQueryFallback(c, "pageSize", "page_size"),
	})
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, logs)
}

// CleanupApplyLogs godoc
// @Summary Cleanup apply logs
// @Tags ApplyLogs
// @Accept json
// @Produce json
// @Security OpenFlareTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/apply-logs/cleanup [post]
func CleanupApplyLogs(c *gin.Context) {
	var input service.ApplyLogCleanupInput
	if !bind.JSON(c, &input) {
		return
	}
	result, err := service.CleanupApplyLogs(input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, result)
}

func readIntQueryFallback(c *gin.Context, primary string, secondary string) int {
	value := c.Query(primary)
	if value == "" {
		value = c.Query(secondary)
	}
	parsed, _ := strconv.Atoi(value)
	return parsed
}
