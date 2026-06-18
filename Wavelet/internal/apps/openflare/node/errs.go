// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package node

const (
	errNodeNameRequired          = "节点名不能为空"
	errNodeIPTooLong             = "节点 IP 不能超过 64 个字符"
	errNodeIPInvalid             = "节点 IP 格式无效"
	errNodeIPManualRequired      = "锁定节点 IP 时必须填写节点 IP"
	errNodeGeoNameTooLong        = "节点位置名不能超过 128 个字符"
	errNodeGeoCoordinateMismatch = "地图坐标必须同时填写纬度和经度"
	errNodeGeoLatitudeInvalid    = "纬度必须在 -90 到 90 之间"
	errNodeGeoLongitudeInvalid   = "经度必须在 -180 到 180 之间"
	errNodeIDConflict            = "节点标识生成冲突，请重试"
	errNodeNotFound              = "节点不存在"
	errNodeForceSyncFailed       = "节点不在线或通过 WebSocket 发送同步指令失败"
	errNoActiveConfigVersion     = "当前没有激活版本"
	errAgentPreviewTagInvalid    = "指定版本不是 preview 发布"
	errAgentStableTagInvalid     = "正式版更新不能选择 preview 发布"
)
