// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package system_config 提供系统配置管理功能
package system_config

// 系统配置错误消息常量
const (
	SystemConfigNotFound                 = "系统配置不存在"
	ConfigKeyRequired                    = "配置键不能为空"
	ConfigValueRequired                  = "配置值不能为空"
	ConfigKeyExists                      = "配置键已存在"
	StorageDriverSwitchRequiresMigration = "存在存量文件，请通过存储迁移任务切换存储引擎"
)
