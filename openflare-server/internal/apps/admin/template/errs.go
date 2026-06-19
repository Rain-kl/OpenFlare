// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package template 提供模板管理功能
package template

// 模板管理相关错误消息
const (
	TemplateNotFound              = "模板不存在"
	TemplateKeyRequired           = "模板标识符不能为空"
	TemplateNameRequired          = "模板名称不能为空"
	TemplateContentRequired       = "模板内容不能为空"
	TemplateKeyExists             = "模板标识符已存在"
	SystemTemplateCannotDelete    = "系统预置模板不可删除"
	SystemTemplateCannotModifyKey = "系统预置模板不可修改标识符"
)
