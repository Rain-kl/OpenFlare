// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package util

import "strings"

// emailPartsCount 邮箱地址由 @ 分割为两部分
const (
	emailPartsCount    = 2
	emailLocalMinChars = 2 // 邮箱 local 部分掩码显示的最小字符数
)

// DerefString 安全地解引用字符串指针，nil 返回空字符串
func DerefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// MaskEmail 安全脱敏用户的邮箱地址（例如 us***@example.com）
func MaskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != emailPartsCount {
		return email
	}
	local := parts[0]
	domain := parts[1]
	if len(local) <= emailLocalMinChars {
		return "**@" + domain
	}
	return local[:2] + "***" + local[len(local)-1:] + "@" + domain
}
