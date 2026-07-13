// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import "strings"

// ParseBrowserName performs lightweight User-Agent browser identification.
func ParseBrowserName(ua string) string {
	uaLower := strings.ToLower(ua)
	if strings.Contains(uaLower, "micromessenger") {
		return "WeChat"
	}
	if strings.Contains(uaLower, "postman") {
		return "Postman"
	}
	if strings.Contains(uaLower, "edg/") || strings.Contains(uaLower, "edge") {
		return "Edge"
	}
	if strings.Contains(uaLower, "firefox") {
		return "Firefox"
	}
	if strings.Contains(uaLower, "chrome") {
		return "Chrome"
	}
	if strings.Contains(uaLower, "safari") {
		return "Safari"
	}
	return "Other"
}
