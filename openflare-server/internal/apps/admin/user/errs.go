// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package user 提供用户管理功能
package user

const (
	userNotFound     = "用户不存在"
	cannotDisable    = "不能禁用管理员用户"
	cannotDelete     = "不能删除管理员用户"
	cannotDeleteSelf = "不能删除当前登录用户"
	updateUserFailed = "更新用户状态失败"
	deleteUserFailed = "删除用户失败"
	usernameExists   = "用户名已存在"
	usernameRequired = "用户名不能为空"
	passwordTooShort = "密码长度不能少于 8 位" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	createUserFailed = "创建用户失败"
	emailRequired    = "邮箱不能为空"
	emailExists      = "邮箱已被注册"
)
