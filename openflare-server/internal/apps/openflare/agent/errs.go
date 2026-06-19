// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

const (
	errMissingAgentToken      = "缺少 Agent Token"
	errInvalidAgentToken      = "无权进行此操作，Agent Token 无效"
	errInvalidDiscoveryToken  = "无权进行此操作，注册 Token 无效"
	errNodeMissingFromContext = "Node object missing from context"
	errNoActiveConfig         = "当前没有激活版本"
	errNodeNotFound           = "节点不存在"
	errNodeIDRequired         = "node_id 不能为空"
	errVersionRequired        = "version 不能为空"
	errInvalidApplyResult     = "result 仅支持 success、warning 或 failed"
	errIPRequired             = "ip 不能为空"
	errIPInvalid              = "ip 格式无效"
	errAgentVersionRequired   = "version 不能为空"
	errNodeIDConflict         = "节点标识生成冲突，请重试"
)
