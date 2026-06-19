// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package updater manages GitHub Release checks and in-place application upgrades.
package updater

const (
	errInvalidRepository       = "上游仓库地址无效"
	errReleaseRequestFailed    = "获取上游版本失败"
	errReleaseResponseInvalid  = "上游版本响应无效"
	errNoCompatibleRelease     = "未找到兼容的 Release"
	errNoCompatibleAsset       = "未找到当前系统对应的 Release 资产"
	errDevelopmentBuild        = "开发版本无法执行自动升级"
	errAlreadyUpToDate         = "当前已是最新版本"
	errUpgradeAlreadyRunning   = "已有升级任务正在执行"
	errAutomaticUpgradeBlocked = "当前平台暂不支持自动替换二进制"
)
