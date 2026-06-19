// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"context"
	"net/http"
	"time"

	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

// GetUpdateStatus 获取应用更新状态
// @Summary 获取应用更新状态
// @Description 从系统配置指定的 GitHub 上游仓库查询最新兼容 Release，并与当前服务版本比较
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=updater.Status} "更新状态"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "查询失败"
// @Router /api/v1/admin/update [get]
func GetUpdateStatus(c *gin.Context) {
	status, _, err := defaultManager.status(c.Request.Context())
	if err != nil {
		logger.ErrorF(c.Request.Context(), "[Updater] check release failed: %v", err)
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(status))
}

// ApplyUpdate 下载并应用应用更新
// @Summary 下载并应用应用更新
// @Description 下载当前平台对应的 GitHub Actions Release 资产，替换当前二进制并重启进程
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any "升级已准备并即将重启"
// @Failure 400 {object} response.Any "当前版本不可升级"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "升级准备失败"
// @Router /api/v1/admin/update/apply [post]
func ApplyUpdate(c *gin.Context) {
	executable, stagedBinary, err := defaultManager.prepareUpgrade(c.Request.Context())
	if err != nil {
		logger.ErrorF(c.Request.Context(), "[Updater] prepare upgrade failed: %v", err)
		response.AbortBadRequest(c, err.Error())
		return
	}

	logger.InfoF(c.Request.Context(), "[Updater] upgrade prepared; restarting with %s", stagedBinary)
	c.JSON(http.StatusOK, response.OKNil())

	go func() {
		time.Sleep(time.Second)
		if err := replaceAndRestart(executable, stagedBinary); err != nil {
			defaultManager.finishUpgrade()
			logger.ErrorF(context.Background(), "[Updater] replace and restart failed: %v", err)
		}
	}()
}
