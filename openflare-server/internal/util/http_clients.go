// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Rain-kl/Wavelet/pkg/httppool"
)

// IsLocalhost 检查 URL 是否为 localhost
func IsLocalhost(urlStr string) bool {
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	hostname := u.Hostname()
	return hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1"
}

// HTTP 客户端配置常量
const (
	httpClientTimeout       = 10 // HTTP 客户端超时时间（秒）
	httpMaxIdleConns        = 100
	httpMaxIdleConnsPerHost = 20
	httpIdleConnTimeout     = 60 // 空闲连接超时（秒）
)

// 配置HTTP客户端 使用 otelhttp 自动注入 trace span
var httpClient = &http.Client{
	Timeout:   httpClientTimeout * time.Second,
	Transport: httppool.DefaultTransport(),
}

// SetHTTPClient 替换全局 HTTP 客户端实例
func SetHTTPClient(c *http.Client) {
	httpClient = c
}

// Request 发送 HTTP 请求，支持自定义 Headers 和 Cookies
func Request(ctx context.Context, method, url string, body io.Reader, headers, cookies map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf(errCreateHTTPRequestFailed, err)
	}

	for key, value := range cookies {
		req.AddCookie(&http.Cookie{Name: key, Value: value}) //nolint:gosec // client-side cookies do not require server attributes (Secure/HttpOnly)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(errHTTPRequestFailed, url, err)
	}

	return resp, nil
}
