// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

// HTTP 响应错误文案
const (
	errInvalidParams = "无效的请求参数"
	errUserNotFound  = "用户不存在"
	//nolint:gosec // error message, not hardcoded credentials
	errPasswordMismatch     = "用户名或密码错误"
	errTooManyLoginAttempts = "登录尝试过于频繁，请稍后重试"
	//nolint:gosec // error message, not hardcoded credentials
	//nolint:gosec // error message, not hardcoded credentials
	errOldPasswordIncorrect = "原密码不正确"
	//nolint:gosec // error message, not hardcoded credentials
	errTokenNotFound    = "访问令牌不存在"
	errCreateUserFailed = "创建用户失败: "
	//nolint:gosec // error message, not hardcoded credentials
	errCreateTokenFailed = "创建令牌失败"
	//nolint:gosec // error message, not hardcoded credentials
	errPasswordEncryptFailed = "密码加密失败"
	//nolint:gosec // error message, not hardcoded credentials
	errPasswordUpdateFailed = "密码更新失败"
	// error message, not hardcoded credentials
	errPasswordEmpty = "password cannot be empty"
)

// Service 层业务校验错误文案，取值被上游插件按字符串精确匹配消费，禁止改写
const (
	errUsernameEmpty        = "用户名不能为空"
	errEmailEmpty           = "邮箱不能为空"
	errUsernameTaken        = "用户名已被使用"
	errEmailTaken           = "邮箱已被使用"
	errCannotRevokeSelf     = "不能取消自己的管理员权限"
	errAdminCannotDisable   = "管理员账号无法被禁用"
	errAdminCannotDelete    = "管理员账号无法被删除"
	errCannotDeleteSelf     = "不能删除当前登录用户"
	errServiceUsernameEmpty = "user: username cannot be empty"
	// error message, not hardcoded credentials
	errServiceOldPasswordIncorrect = "user: incorrect old password"
	//nolint:gosec // error message, not hardcoded credentials
	errServicePasswordTooShort = "密码长度至少为 8 位"
	errUniqueUsernameFailed    = "failed to generate unique username"
	errInvalidEmail            = "邮箱地址无效"
	errInvalidEmailCode        = "验证码必须是 6 位数字"
	errInvalidTaskPayload      = "任务参数无效"
	errMailSubjectRequired     = "邮件主题不能为空"
	errMailBodyRequired        = "邮件内容不能为空"
	errSMTPNotConfigured       = "SMTP 未配置"
	errEmailCacheUnavailable   = "缓存服务不可用，无法保存验证码"
	errSendEmailFailed         = "邮件发送失败"
)
