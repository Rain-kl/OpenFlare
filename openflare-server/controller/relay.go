package controller

import (
	"log/slog"
	"net"
	"time"

	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/controller/bind"
	"github.com/rain-kl/openflare/openflare-server/model"
	"github.com/rain-kl/openflare/openflare-server/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/websocket"
)

// RelayHeartbeat godoc
// @Summary Report relay heartbeat
// @Tags Relay
// @Accept json
// @Produce json
// @Security AccessTokenAuth
// @Param payload body service.RelayHeartbeatPayload true "Relay heartbeat payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/relay/heartbeat [post]
func RelayHeartbeat(c *gin.Context) {
	var payload service.RelayHeartbeatPayload
	if !bind.JSON(c, &payload) {
		return
	}
	payload.IP = service.ResolveReportedNodeIP(payload.IP, c.Request.RemoteAddr)
	authNode, ok := c.Get("relay_node")
	if !ok {
		response.RespondUnauthorized(c, "无权进行此操作")
		return
	}
	node := authNode.(*model.Node)
	result, err := service.HeartbeatRelay(node, payload)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, result)
}

// RelayWebSocket godoc
// @Summary Upgrade relay connection to websocket
// @Tags Relay
// @Security AccessTokenAuth
// @Router /api/relay/ws [get]
func RelayWebSocket(c *gin.Context) {
	authNode, ok := c.Get("relay_node")
	if !ok {
		response.RespondUnauthorized(c, "无权进行此操作")
		return
	}
	node := authNode.(*model.Node)
	slog.Debug("relay ws upgrade requested", "node_id", node.NodeID, "remote", c.Request.RemoteAddr)
	websocket.Handler(func(conn *websocket.Conn) {
		client := service.RegisterRelayWSClient(node.NodeID)
		defer service.UnregisterRelayWSClient(client)
		defer func() {
			_ = conn.Close()
			slog.Debug("relay ws connection closed", "node_id", node.NodeID)
		}()

		slog.Debug("relay ws upgrade succeeded", "node_id", node.NodeID, "remote", c.Request.RemoteAddr)

		go func() {
			<-client.Done()
			_ = conn.Close()
		}()

		go streamRelayWSMessages(c, conn, client)

		for {
			var message service.WSMessage
			_ = conn.SetReadDeadline(time.Now().Add(agentWSReadTimeout()))
			if err := websocket.JSON.Receive(conn, &message); err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					slog.Debug("relay ws receive timeout", "node_id", node.NodeID)
					return
				}
				slog.Debug("relay ws receive failed", "node_id", node.NodeID, "error", err)
				return
			}
			slog.Debug("relay ws message received", "node_id", node.NodeID, "type", message.Type)
			switch message.Type {
			case "ping":
				if !service.SendRelayWSPong(node.NodeID) {
					slog.Debug("relay ws pong enqueue failed", "node_id", node.NodeID)
				}
			case "pong":
				slog.Debug("relay ws pong received", "node_id", node.NodeID)
			default:
				slog.Debug("relay ws unsupported message type", "node_id", node.NodeID, "type", message.Type)
			}
		}
	}).ServeHTTP(c.Writer, c.Request)
}

func streamRelayWSMessages(c *gin.Context, conn *websocket.Conn, client *service.WSClient) {
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
				slog.Debug("relay ws send failed", "node_id", client.ID(), "error", err)
				return
			}
		}
	}
}
