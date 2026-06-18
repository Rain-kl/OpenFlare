// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package update

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgradeLogsUpgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}


// GetLatestReleaseHandler 获取最新 GitHub 发布版本。
// @Summary 获取最新服务端发布版本
// @Description 查询 OpenFlare 服务端最新 GitHub Release 及升级状态，需要管理员权限
// @Tags openflare-update
// @Produce json
// @Security SessionCookie
// @Param channel query string false "发布渠道（stable/preview）"
// @Success 200 {object} response.Any{data=update.LatestReleaseView} "最新发布信息"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/custom/openflare/update/latest-release [get]
func GetLatestReleaseHandler(c *gin.Context) {
	release, err := GetLatestRelease(c.Request.Context(), c.Query("channel"))
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(release))
}

// UpgradeServerHandler 调度自动升级任务。
// @Summary 触发服务端自动升级
// @Description 从最新 Release 调度 OpenFlare 服务端自动升级，需要管理员权限
// @Tags openflare-update
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body update.upgradeRequest false "升级参数"
// @Success 200 {object} response.Any{data=update.LatestReleaseView} "升级任务已调度"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/custom/openflare/update/upgrade [post]
func UpgradeServerHandler(c *gin.Context) {
	var request upgradeRequest
	if err := bindOptionalJSON(c.Request.Body, &request); err != nil {
		response.AbortBadRequest(c, "无效的参数")
		return
	}

	release, err := ScheduleUpgrade(c.Request.Context(), request.Channel)
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}

	c.JSON(http.StatusOK, response.OK(release))
}

// UploadManualServerBinaryHandler 上传手动升级二进制（已禁用）。
// @Summary 上传手动升级二进制
// @Description 上传服务端二进制以进行手动升级（当前功能已禁用），需要管理员权限
// @Tags openflare-update
// @Accept multipart/form-data
// @Produce json
// @Security SessionCookie
// @Param binary formData file true "服务端二进制文件"
// @Failure 400 {object} response.Any "功能已禁用或参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/custom/openflare/update/manual-upload [post]
func UploadManualServerBinaryHandler(c *gin.Context) {
	if apiutil.AbortBadRequestOnError(c, UploadManualBinary()) {
		return
	}
}

// ConfirmManualServerUpgradeHandler 确认手动升级（已禁用）。
// @Summary 确认手动服务端升级
// @Description 确认并执行手动上传的服务端升级（当前功能已禁用），需要管理员权限
// @Tags openflare-update
// @Accept json
// @Produce json
// @Security SessionCookie
// @Failure 400 {object} response.Any "功能已禁用或参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/custom/openflare/update/manual-upgrade [post]
func ConfirmManualServerUpgradeHandler(c *gin.Context) {
	if apiutil.AbortBadRequestOnError(c, ConfirmManualUpgrade()) {
		return
	}
}

// StreamServerUpgradeLogsHandler 通过 WebSocket 推送升级日志。
// @Summary 流式获取服务端升级日志
// @Description 通过 WebSocket 推送升级进度快照，需要管理员权限
// @Tags openflare-update
// @Security SessionCookie
// @Success 101 {object} update.StreamSnapshot "WebSocket 升级日志流"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/custom/openflare/update/logs/ws [get]
func StreamServerUpgradeLogsHandler(c *gin.Context) {
	conn, err := upgradeLogsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	updates, unsubscribe := SubscribeUpgradeStream()
	defer unsubscribe()

	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case snapshot, ok := <-updates:
			if !ok {
				return
			}
			if err := conn.WriteJSON(snapshot); err != nil {
				return
			}
		case <-heartbeatTicker.C:
			if err := conn.WriteJSON(StreamSnapshot{}); err != nil {
				return
			}
		case <-c.Request.Context().Done():
			return
		}
	}
}

func bindOptionalJSON(body io.Reader, target any) error {
	if err := json.NewDecoder(body).Decode(target); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}