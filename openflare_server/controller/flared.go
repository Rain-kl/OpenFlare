package controller

import (
	"log/slog"
	"net"
	"openflare/common"
	"openflare/common/response"
	"openflare/controller/bind"
	"openflare/model"
	"openflare/service"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/websocket"
)

// FlaredHeartbeat godoc
// @Summary Report OpenFlared heartbeat
// @Tags Flared
// @Accept json
// @Produce json
// @Security TunnelTokenAuth
// @Param payload body service.FlaredHeartbeatPayload true "Flared heartbeat payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/flared/heartbeat [post]
func FlaredHeartbeat(c *gin.Context) {
	var payload service.FlaredHeartbeatPayload
	if !bind.JSON(c, &payload) {
		return
	}
	authNode, ok := c.Get("flared_node")
	if !ok {
		response.RespondUnauthorized(c, "无权进行此操作，Tunnel Token 无效")
		return
	}
	node := authNode.(*model.Node)
	res, err := service.HeartbeatFlared(node, payload)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, res)
}

// FlaredGetActiveConfig godoc
// @Summary Get active tunnel config for OpenFlared
// @Tags Flared
// @Produce json
// @Security TunnelTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/flared/config/active [get]
func FlaredGetActiveConfig(c *gin.Context) {
	authNode, ok := c.Get("flared_node")
	if !ok {
		response.RespondUnauthorized(c, "无权进行此操作，Tunnel Token 无效")
		return
	}
	node := authNode.(*model.Node)
	config, err := service.GetFlaredTunnelConfig(node)
	if err != nil {
		response.RespondFailure(c, "无法生成隧道配置: "+err.Error())
		return
	}
	response.RespondSuccess(c, config)
}

// FlaredReportApplyLog godoc
// @Summary Report OpenFlared apply result
// @Tags Flared
// @Accept json
// @Produce json
// @Security TunnelTokenAuth
// @Param payload body service.ApplyLogPayload true "Apply log payload"
// @Success 200 {object} map[string]interface{}
// @Router /api/flared/apply-log [post]
func FlaredReportApplyLog(c *gin.Context) {
	var payload service.ApplyLogPayload
	if !bind.JSON(c, &payload) {
		return
	}
	if authNode, ok := c.Get("flared_node"); ok {
		payload.NodeID = authNode.(*model.Node).NodeID
	}
	log, err := service.ReportApplyLog(payload)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, log)
}

// FlaredWebSocket godoc
// @Summary Upgrade OpenFlared connection to websocket
// @Tags Flared
// @Security TunnelTokenAuth
// @Router /api/flared/ws [get]
func FlaredWebSocket(c *gin.Context) {
	authNode, ok := c.Get("flared_node")
	if !ok {
		response.RespondUnauthorized(c, "无权进行此操作，Tunnel Token 无效")
		return
	}
	node := authNode.(*model.Node)
	slog.Debug("flared ws upgrade requested", "node_id", node.NodeID, "remote", c.Request.RemoteAddr)
	websocket.Handler(func(conn *websocket.Conn) {
		client := service.RegisterFlaredWSClient(node.NodeID)
		defer service.UnregisterFlaredWSClient(client)
		defer func() {
			_ = conn.Close()
			slog.Debug("flared ws connection closed", "node_id", node.NodeID)
		}()

		slog.Debug("flared ws upgrade succeeded", "node_id", node.NodeID, "remote", c.Request.RemoteAddr)

		go func() {
			<-client.Done()
			_ = conn.Close()
		}()

		go streamFlaredWSMessages(c, conn, client)

		for {
			var message service.WSMessage
			_ = conn.SetReadDeadline(time.Now().Add(flaredWSReadTimeout()))
			if err := websocket.JSON.Receive(conn, &message); err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					slog.Debug("flared ws receive timeout", "node_id", node.NodeID)
					return
				}
				slog.Debug("flared ws receive failed", "node_id", node.NodeID, "error", err)
				return
			}
			slog.Debug("flared ws message received", "node_id", node.NodeID, "type", message.Type)
			switch message.Type {
			case "ping":
				if !service.SendFlaredWSPong(node.NodeID) {
					slog.Debug("flared ws pong enqueue failed", "node_id", node.NodeID)
				}
			case "pong":
				slog.Debug("flared ws pong received", "node_id", node.NodeID)
			default:
				slog.Debug("flared ws unsupported message type", "node_id", node.NodeID, "type", message.Type)
			}
		}
	}).ServeHTTP(c.Writer, c.Request)
}

func streamFlaredWSMessages(c *gin.Context, conn *websocket.Conn, client *service.WSClient) {
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
				slog.Debug("flared ws send failed", "node_id", client.ID(), "error", err)
				return
			}
		}
	}
}

func flaredWSReadTimeout() time.Duration {
	timeout := time.Duration(common.AgentHeartbeatInterval) * time.Millisecond * 3
	if timeout < 30*time.Second {
		return 30 * time.Second
	}
	return timeout
}
