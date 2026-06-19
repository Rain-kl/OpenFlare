// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

const (
	errNoActiveVersion       = "当前没有激活版本"
	errNoEnabledRoutes       = "没有可发布的启用规则"
	errNoChangesToPublish    = "当前规则没有变更，不能重复发布"
	errVersionConflict       = "版本号生成冲突，请重试"
	errInvalidSnapshotFormat = "历史版本快照格式不合法"
)
