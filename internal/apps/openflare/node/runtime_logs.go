// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	ofws "github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/shared/httpx"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const runtimeLogWriteTimeout = 5 * time.Second

func openNodeRuntimeLogs(ctx context.Context, id uint) (*ofws.RuntimeLogStream, error) {
	node, err := repository.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if node.IsQuarantined() {
		return nil, ofws.ErrRuntimeLogsUnavailable
	}
	return ofws.OpenRuntimeLogStream(ctx, node.NodeID)
}

// RuntimeLogsHandler streams current Edge runtime logs to the authenticated administrator.
// @Summary 实时查看节点运行日志
// @Description 通过 SSE 即时转发节点运行日志，需要管理员权限。连接建立后发送 ready 事件，日志批次使用 logs 事件，流结束时发送 closed 事件。
// @Tags openflare-node
// @Produce text/event-stream
// @Security SessionCookie
// @Param id path int true "节点 ID"
// @Success 200 {string} string "SSE 日志流"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或节点不存在"
// @Failure 503 {object} response.Any "节点日志流不可用"
// @Router /api/v1/d/nodes/{id}/runtime-logs [get]
func RuntimeLogsHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	stream, err := openNodeRuntimeLogs(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.AbortNotFound(c, errNodeNotFound)
		case ctx.Err() != nil:
			return
		default:
			if !errors.Is(err, ofws.ErrRuntimeLogsUnavailable) {
				logger.WarnF(ctx, "Failed to open Edge runtime log stream for node %d: %v", id, err)
			}
			response.AbortWithError(c, http.StatusServiceUnavailable, errRuntimeLogsUnavailable)
		}
		return
	}
	defer stream.Close()

	readyCtx, cancel := context.WithTimeout(ctx, ofws.RuntimeLogHandshakeTimeout)
	err = ofws.WaitRuntimeLogReady(readyCtx, stream)
	cancel()
	if err != nil {
		if ctx.Err() == nil {
			response.AbortWithError(c, http.StatusServiceUnavailable, errRuntimeLogsUnavailable)
		}
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-store, no-transform")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	writeNodeRuntimeLogs(ctx, c.Writer, stream)
}

func writeNodeRuntimeLogs(ctx context.Context, writer http.ResponseWriter, stream *ofws.RuntimeLogStream) {
	controller := http.NewResponseController(writer)
	defer func() { _ = controller.SetWriteDeadline(time.Time{}) }()
	write := func(text string) bool {
		if ctx.Err() != nil {
			return false
		}
		if err := controller.SetWriteDeadline(time.Now().Add(runtimeLogWriteTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			return false
		}
		_, err := fmt.Fprint(writer, text)
		return err == nil && controller.Flush() == nil
	}

	if !write("event: ready\ndata: {}\n\n") {
		return
	}
	ticker := time.NewTicker(ofws.RuntimeLogKeepAlive)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stream.Done:
			write("event: closed\ndata: {}\n\n")
			return
		case frame, ok := <-stream.Frames:
			if !ok {
				write("event: closed\ndata: {}\n\n")
				return
			}
			if !writeRuntimeLogFrame(write, frame) {
				return
			}
		case <-ticker.C:
			stream.Renew()
			if !write(": keepalive\n\n") {
				return
			}
		}
	}
}

func writeRuntimeLogFrame(write func(string) bool, frame protocol.RuntimeLogFrame) bool {
	batch := struct {
		Lines   []string `json:"lines,omitempty"`
		Dropped uint64   `json:"dropped,omitempty"`
	}{Lines: frame.Lines, Dropped: frame.Dropped}
	payload, err := json.Marshal(batch)
	return err == nil && write("event: logs\ndata: "+string(payload)+"\n\n")
}
