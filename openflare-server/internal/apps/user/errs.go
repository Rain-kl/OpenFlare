// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

const (
	errBindParamsFailed             = "参数绑定失败"
	errInvalidParams                = "无效的参数"
	errPasswordLoginDisabled        = "管理员关闭了密码登录" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errUsernameOrPasswordWrong      = "用户名或密码错误"   //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errLoginEmailMissing            = "该账号未绑定邮箱，请联系管理员绑定邮箱后再登录"
	errNeedEmailCodePrefix          = "need_email_code:"
	errSMTPInvalidUseTempCodePrefix = "smtp_invalid:"
	errSMTPInvalidUseTempCode       = "smtp 配置无效，使用临时码登录"
	errEmailCodeInvalidOrExpired    = "验证码错误或已过期"
	errSaveSessionFailed            = "无法保存会话信息，请重试"
	errRegistrationDisabled         = "管理员关闭了注册"
	errPasswordTooShort             = "密码长度不能少于 8 位" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errEmailOrCodeRequired          = "邮箱或验证码未填写"
	errNewPasswordTooShort          = "新密码长度不能少于 8 位" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errLoginRequired                = "请先登录"
	errUserNotFound                 = "未找到该用户"
	errOldPasswordIncorrect         = "原密码不正确"     //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errPasswordEncryptFailed        = "密码加密失败，请重试" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errEmailRequired                = "邮箱地址不能为空"
	errUnsupportedEmailScene        = "不支持的验证场景"
	errEmailAlreadyRegistered       = "该邮箱已被注册"
	errEmailCodeCooldown            = "验证码发送频繁，请稍后再试"
	errEmailFormatInvalid           = "邮箱格式不正确"
	errEmailAlreadyBound            = "该邮箱已被其他账号绑定"
	errRenderEmailTemplateFailed    = "渲染验证邮件模板失败：%w"
	errGenerateEmailCodeFailed      = "生成验证码失败，请重试"
	errDispatchEmailTaskFailed      = "投递验证邮件发送任务失败，请重试"
	errTokenNameRequired            = "令牌名称不能为空"            //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errAccessTokenLimitReached      = "已达到访问令牌最大创建数量限制"     //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errGenerateTokenFailed          = "生成令牌失败"              //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errInvalidTokenID               = "无效的令牌ID"             //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errTokenNotFoundOrForbidden     = "令牌不存在或无权操作"          //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errAdminTokenRequiresAdmin      = "只有管理员才能创建具有管理员权限的令牌" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errTaskPayloadRequired          = "任务参数不能为空"
	errInvalidJSONFormat            = "无效的 JSON 格式: %w"
	errEmailTaskFieldsRequired      = "to、subject、body 不能为空"
	errParseEmailPayloadFailed      = "解析邮件发送参数失败: %w"
	errSMTPConfigIncomplete         = "系统 SMTP 邮件服务配置不完整"
	errSendMailFailed               = "发送邮件失败: %w"
)
