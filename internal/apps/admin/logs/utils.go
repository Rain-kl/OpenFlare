// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

// getUpgrader 返回 WebSocket 升级器并执行 Origin 安全检查以防止 CSWSH 攻击
func getUpgrader() *websocket.Upgrader {
	return &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}

			// 1. 同源检查 (Same-origin check)
			u, err := url.Parse(origin)
			if err == nil && strings.EqualFold(u.Host, r.Host) {
				return true
			}

			// 2. 检查配置的允许跨域 Origin (Check allowed origins in system config)
			ctx := r.Context()
			if sc, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyServerAddress); err == nil && sc.Value != "" {
				originToCheck := strings.TrimRight(strings.TrimSpace(origin), "/")
				allowedOrigins := strings.SplitSeq(sc.Value, ",")
				for allowed := range allowedOrigins {
					allowed = strings.TrimRight(strings.TrimSpace(allowed), "/")
					if allowed != "" && strings.EqualFold(allowed, originToCheck) {
						return true
					}
				}
			}
			return false
		},
	}
}

// parsePositiveInt 解析非负整数字符串
func parsePositiveInt(s string, result *int) error {
	if s == "" {
		*result = 0
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return err
	}
	*result = n
	return nil
}
