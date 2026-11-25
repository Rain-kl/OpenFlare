// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ofws "github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestRuntimeLogsStreamsAgentFramesAndCancelsSubscription(t *testing.T) {
	db := setupNodePageDB(t, 1)
	var node model.OpenFlareNode
	require.NoError(t, db.First(&node).Error)

	router := gin.New()
	router.Use(response.ErrorHandlerMiddleware())
	router.GET("/agent", func(c *gin.Context) { ofws.ServeAgent(c, node.NodeID) })
	router.GET("/nodes/:id/runtime-logs", RuntimeLogsHandler)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	agent, response, err := websocket.DefaultDialer.DialContext(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/agent", nil)
	require.NoError(t, err)
	defer func() { _ = agent.Close() }()
	defer func() { _ = response.Body.Close() }()
	deadline, _ := ctx.Deadline()
	require.NoError(t, agent.SetReadDeadline(deadline))

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/nodes/%d/runtime-logs", server.URL, node.ID), nil)
	require.NoError(t, err)
	type result struct {
		response *http.Response
		err      error
	}
	browser := make(chan result, 1)
	go func() {
		res, requestErr := server.Client().Do(request)
		browser <- result{res, requestErr}
	}()

	var command struct {
		Type    string                     `json:"type"`
		Payload protocol.RuntimeLogCommand `json:"payload"`
	}
	require.NoError(t, agent.ReadJSON(&command))
	require.Equal(t, protocol.WSMessageTypeRuntimeLogSubscribe, command.Type)
	require.NotEmpty(t, command.Payload.StreamID)
	require.NoError(t, agent.WriteJSON(ofws.Message{Type: protocol.WSMessageTypeRuntimeLogFrame, Payload: protocol.RuntimeLogFrame{
		StreamID: command.Payload.StreamID, Ready: true,
	}}))

	got := <-browser
	require.NoError(t, got.err)
	defer func() { _ = got.response.Body.Close() }()
	require.Equal(t, http.StatusOK, got.response.StatusCode)
	require.Equal(t, "text/event-stream", got.response.Header.Get("Content-Type"))
	require.Contains(t, got.response.Header.Get("Cache-Control"), "no-store")
	reader := bufio.NewReader(got.response.Body)
	require.Equal(t, "event: ready\ndata: {}\n\n", readRuntimeLogEvent(t, reader))

	require.NoError(t, agent.WriteJSON(ofws.Message{Type: protocol.WSMessageTypeRuntimeLogFrame, Payload: protocol.RuntimeLogFrame{
		StreamID: "wrong-subscription", Lines: []string{"must not be delivered"},
	}}))
	require.NoError(t, agent.WriteJSON(ofws.Message{Type: protocol.WSMessageTypeRuntimeLogFrame, Payload: protocol.RuntimeLogFrame{
		StreamID: command.Payload.StreamID, Lines: []string{"first\nline", "second"}, Dropped: 1,
	}}))
	require.Equal(t, "event: logs\ndata: {\"lines\":[\"first\\nline\",\"second\"],\"dropped\":1}\n\n", readRuntimeLogEvent(t, reader))

	require.NoError(t, got.response.Body.Close())
	var stop = command
	require.NoError(t, agent.ReadJSON(&stop))
	require.Equal(t, protocol.WSMessageTypeRuntimeLogUnsubscribe, stop.Type)
	require.Equal(t, command.Payload.StreamID, stop.Payload.StreamID)
}

func readRuntimeLogEvent(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	event, err := reader.ReadString('\n')
	require.NoError(t, err)
	payload, err := reader.ReadString('\n')
	require.NoError(t, err)
	end, err := reader.ReadString('\n')
	require.NoError(t, err)
	return event + payload + end
}
