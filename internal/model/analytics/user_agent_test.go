// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseBrowserName(t *testing.T) {
	tests := []struct {
		name string
		ua   string
		want string
	}{
		{name: "chrome", ua: "Mozilla/5.0 Chrome/120.0.0.0", want: "Chrome"},
		{name: "firefox", ua: "Mozilla/5.0 Firefox/121.0", want: "Firefox"},
		{name: "safari", ua: "Mozilla/5.0 Safari/605.1.15", want: "Safari"},
		{name: "edge", ua: "Mozilla/5.0 Edg/120.0.0.0", want: "Edge"},
		{name: "wechat", ua: "MicroMessenger/8.0", want: "WeChat"},
		{name: "postman", ua: "PostmanRuntime/7.36.0", want: "Postman"},
		{name: "cli", ua: "curl/8.0", want: "CLI"},
		{name: "other", ua: "CustomClient/1.0", want: "Other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ParseBrowserName(tt.ua))
		})
	}
}
