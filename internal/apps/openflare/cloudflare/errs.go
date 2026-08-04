// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

const (
	errConnectionNotConfigured = "尚未配置 Cloudflare 连接"
	errConnectionSourceInvalid = "无效的 Cloudflare 连接来源"
	errStandaloneInputRequired = "请填写 Cloudflare API Token"
	errStandaloneInputInvalid  = "配置的 Cloudflare API Token 无效"
	errDNSAccountInvalid       = "请选择有效的 Cloudflare DNS 账号"
	errGroupNameRequired       = "分组名称不能为空"
	errGroupNodeSame           = "主节点和备用节点不能相同"
	errNodeInvalid             = "请选择有效的边缘节点"
	errNodeIPv4Required        = "生效节点必须配置合法 IPv4"
	errGroupDisabled           = "指向分组已停用"
	errMemberExists            = "该域名已加入其他指向分组"
	errMultipleARecords        = "检测到 Cloudflare 中存在多条同名 A 记录，请先手动清理"
	errSyncFailed              = "Cloudflare DNS 同步失败"
	errDeleteRemoteFailed      = "删除 Cloudflare DNS 记录失败"
	errTaskDispatchFailed      = "无法投递 Cloudflare 同步任务"
)
