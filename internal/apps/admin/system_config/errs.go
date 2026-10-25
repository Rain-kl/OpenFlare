// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package system_config 提供系统配置管理功能
package system_config

// 系统配置错误消息常量
const (
	SystemConfigNotFound                 = "System configuration was not found"
	ConfigKeyRequired                    = "Configuration key must not be empty"
	ConfigValueRequired                  = "Configuration value must not be empty"
	ConfigKeyExists                      = "configuration key already exists"
	StorageDriverSwitchRequiresMigration = "existing files were found; use the storage migration task to switch storage backends"
)
