// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package risk_control

import (
	"time"
)

// UserAccessLog 用户访问记录
type UserAccessLog struct {
	ID        uint64    `json:"id,string"`
	UserID    uint64    `json:"user_id,string"`
	Path      string    `json:"path"`
	Method    string    `json:"method"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Headers   string    `json:"headers"`
	Status    int32     `json:"status"`
	Latency   int64     `json:"latency"` // 耗时毫秒
	CreatedAt time.Time `json:"created_at"`
}
