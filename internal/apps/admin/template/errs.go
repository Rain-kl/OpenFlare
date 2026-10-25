// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package template 提供模板管理功能
package template

// 模板管理相关错误消息
const (
	TemplateNotFound              = "Template was not found"
	TemplateKeyRequired           = "Template identifier must not be empty"
	TemplateNameRequired          = "Template name must not be empty"
	TemplateContentRequired       = "Template content must not be empty"
	TemplateKeyExists             = "template identifier already exists"
	SystemTemplateCannotDelete    = "built-in templates cannot be deleted"
	SystemTemplateCannotModifyKey = "Built-in template identifiers cannot be changed"
)
