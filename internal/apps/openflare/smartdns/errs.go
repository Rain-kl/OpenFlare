// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"errors"
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
)

const (
	errSmartDNSDisabled        = "SmartDNS 服务当前已关闭"
	errInstanceNotFound        = "DNS 实例不存在"
	errInstanceEmbeddedLocked  = "不允许删除或降级内置 SmartDNS 实例"
	errInstanceIDInvalid       = "DNS 实例 ID 格式无效"
	errInstanceNameRequired    = "DNS 实例名称不能为空且不能超过 128 个字符"
	errInstanceListenInvalid   = "DNS 实例监听地址格式无效"
	errInstanceQuarantined     = "DNS 实例已被隔离"
	errTokenInvalid            = "无效的 DNS 实例认证令牌"
	errDiscoveryTokenInvalid   = "无效的 DNS 自动发现令牌"
	errPolicyNotFound          = "解析策略不存在"
	errPolicyNameRequired      = "策略名称不能为空且不能超过 128 个字符"
	errPolicyDefaultLocked     = "默认解析策略不可删除"
	errPolicyInUse             = "该策略仍被 Cloudflare SmartDNS 分组引用，请先解除关联"
	errPolicyGraphInvalid      = "决策流图或 JDM 配置格式无效"
	errPolicyStrategyInvalid   = "不支持的调度策略类型"
	errPolicyFallbackInvalid   = "不支持的应急切换策略类型"
	errPolicyScopeInvalid      = "不支持的策略生效范围类型"
	errPolicyScopeDomainEmpty  = "按域名范围生效时必须至少配置一个域名匹配模式"
	errPolicyTTLInvalid        = "策略 TTL 必须在 10 到 86400 秒之间"
	errPolicyMaxAnswersInvalid = "策略最大应答 IP 数量必须在 1 到 16 之间"
	errSimulatorDomainInvalid  = "请输入有效的模拟解析域名"
	errSimulatorIPInvalid      = "请输入有效的客户端 IPv4 或 IPv6 地址"
	errSimulatorQTypeInvalid   = "仅支持 A 与 AAAA 记录类型模拟"
	errSettingsInvalid         = "SmartDNS 系统配置参数无效"
	errDelegationDomainInvalid = "全局委派域名格式无效"
	errOverviewQueryFailed     = "获取 SmartDNS 概览统计失败"
	errListInstancesFailed     = "获取 DNS 实例列表失败"
	errCreateInstanceFailed    = "创建 DNS 实例失败"
	errUpdateInstanceFailed    = "更新 DNS 实例失败"
	errDeleteInstanceFailed    = "删除 DNS 实例失败"
	errListPoliciesFailed      = "获取解析策略列表失败"
	errGetPolicyFailed         = "获取解析策略详情失败"
	errCreatePolicyFailed      = "创建解析策略失败"
	errUpdatePolicyFailed      = "更新解析策略失败"
	errDeletePolicyFailed      = "删除解析策略失败"
	errGetSettingsFailed       = "读取 SmartDNS 设置失败"
	errSaveSettingsFailed      = "保存 SmartDNS 设置失败"
	errVerifyDelegationFailed  = "校验域名委派状态失败"
	errHeartbeatFailed         = "上报 DNS 实例心跳失败"
	errSnapshotLoadFailed      = "生成 SmartDNS 运行时快照失败"
)

type logicError struct {
	status int
	msg    string
}

func (e *logicError) Error() string {
	return e.msg
}

func newLogicError(status int, msg string) error {
	return &logicError{status: status, msg: msg}
}

func abortSmartDNSError(c *gin.Context, err error, internalMsg string) {
	ctx := c.Request.Context()
	var le *logicError
	if errors.As(err, &le) {
		switch le.status {
		case http.StatusBadRequest:
			response.AbortBadRequest(c, le.msg)
		case http.StatusUnauthorized:
			response.AbortUnauthorized(c, le.msg)
		case http.StatusForbidden:
			response.AbortForbidden(c, le.msg)
		case http.StatusNotFound:
			response.AbortNotFound(c, le.msg)
		case http.StatusConflict:
			response.AbortConflict(c, le.msg)
		default:
			response.AbortInternal(c, le.msg)
		}
		return
	}
	logger.ErrorF(ctx, "%s: %v", internalMsg, err)
	response.AbortInternal(c, internalMsg)
}
