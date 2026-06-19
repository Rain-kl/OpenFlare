// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package oauth provides authentication and OAuth integration.
package oauth

import (
	"net/http"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/gin-contrib/sessions"
)

// GetSessionOptions 根据配置构建 Session 选项
func GetSessionOptions(maxAge int) sessions.Options {
	return sessions.Options{
		Path:     "/",
		Domain:   config.Config.App.SessionDomain,
		MaxAge:   maxAge,
		HttpOnly: config.Config.App.SessionHTTPOnly,
		Secure:   config.Config.App.SessionSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

// StripCookieMaxAgeAndExpires 从 Set-Cookie 响应头中移除 Max-Age 和 Expires，从而使其成为浏览器会话 Cookie
func StripCookieMaxAgeAndExpires(header http.Header, cookieName string) {
	headers := header["Set-Cookie"]
	if len(headers) == 0 {
		return
	}

	newHeaders := make([]string, 0, len(headers))
	for _, h := range headers {
		if strings.HasPrefix(h, cookieName+"=") {
			parts := strings.Split(h, ";")
			newParts := make([]string, 0, len(parts))
			for _, p := range parts {
				trimmed := strings.TrimSpace(p)
				lower := strings.ToLower(trimmed)
				if strings.HasPrefix(lower, "max-age=") || strings.HasPrefix(lower, "expires=") {
					continue
				}
				newParts = append(newParts, p)
			}
			newHeaders = append(newHeaders, strings.Join(newParts, ";"))
		} else {
			newHeaders = append(newHeaders, h)
		}
	}
	header["Set-Cookie"] = newHeaders
}
