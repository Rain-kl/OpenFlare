// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

import "time"

const (
	verificationCodeRange  = 900000           // 验证码随机范围
	verificationCodeOffset = 100000           // 验证码偏移量（保证 6 位）
	emailCodeExpiry        = 5 * time.Minute  // 验证码有效期
	emailCodeCooldown      = 60 * time.Second // 验证码发送冷却时间
	minPasswordLength      = 8                // 密码最小长度
)
