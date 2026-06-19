// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package config 提供公开配置查询接口
package config

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/gin-gonic/gin"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

// GetPublicConfig 获取公共配置
// @Summary 获取公共配置
// @Description 返回系统配置表中 visibility 为 1 的配置键值集合
// @Tags config
// @Accept json
// @Produce json
// @Success 200 {object} response.Any
// @Router /api/v1/config/public [get]
func GetPublicConfig(c *gin.Context) {
	ctx := c.Request.Context()
	configs, err := repository.ListVisibleSystemConfigs(ctx)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	resp := make(map[string]string, len(configs))
	for _, config := range configs {
		resp[config.Key] = config.Value
	}

	c.JSON(http.StatusOK, response.OK(resp))
}

// GetRobotsTXT 动态生成 robots.txt
// @Summary 获取 robots.txt
// @Description 根据系统配置决定是否允许搜索引擎检索，并返回相应的 robots.txt 文件内容
// @Tags config
// @Produce text/plain
// @Success 200 {string} string "robots.txt 内容"
// @Router /robots.txt [get]
func GetRobotsTXT(c *gin.Context) {
	ctx := c.Request.Context()
	enabled, err := repository.GetBoolByKey(ctx, model.ConfigKeySearchEngineIndexingEnabled)
	content := "User-Agent: *\nDisallow: /\n"
	if err == nil && enabled {
		content = "User-Agent: *\nAllow: /\n"
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(content))
}
