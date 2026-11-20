// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import "strings"

// IsRetiredEdgeOption identifies the OpenResty tuning and engine rollout settings.
func IsRetiredEdgeOption(key string) bool {
	key = strings.TrimSpace(key)
	return strings.HasPrefix(key, "openresty_") || key == "pingora_enabled"
}
