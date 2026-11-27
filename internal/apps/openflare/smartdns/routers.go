// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"compress/gzip"
	"net/http"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/shared/httpx"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
)

// OverviewHandler returns SmartDNS cluster KPIs, snapshot state, and latency/QPS timeseries.
// @Summary 获取 SmartDNS 运行概览
// @Description 返回 SmartDNS 实例在线数、QPS、延迟分位数、缓存命中率与时序指标，需要管理员权限
// @Tags openflare-smartdns
// @Produce json
// @Security SessionCookie
// @Param instance_id query string false "按实例 ID 过滤"
// @Param window query string false "时间窗口：1h|6h|24h|7d"
// @Success 200 {object} response.Any{data=smartdns.OverviewResult} "概览统计"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/overview [get]
func OverviewHandler(c *gin.Context) {
	ctx := c.Request.Context()
	res, err := GetOverview(ctx, c.Query("instance_id"), c.Query("window"))
	if err != nil {
		logger.ErrorF(ctx, "smartdns overview failed: %v", err)
		response.AbortInternal(c, errOverviewQueryFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// GetSettingsHandler returns SmartDNS global configuration.
// @Summary 获取 SmartDNS 系统设置
// @Description 返回委派主域名、NS 主机列表、默认 TTL 与快照刷新周期，需要管理员权限
// @Tags openflare-smartdns
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=smartdns.SettingsView} "SmartDNS 设置"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/settings [get]
func GetSettingsHandler(c *gin.Context) {
	view, err := GetSettings(c.Request.Context())
	if err != nil {
		abortSmartDNSError(c, err, errGetSettingsFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// SaveSettingsHandler saves SmartDNS global configuration.
// @Summary 保存 SmartDNS 系统设置
// @Description 更新全局委派域名、NS 主机、默认 TTL 与快照周期，并触发快照重建，需要管理员权限
// @Tags openflare-smartdns
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body smartdns.SettingsInput true "SmartDNS 设置参数"
// @Success 200 {object} response.Any{data=smartdns.SettingsView} "保存成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/settings [post]
func SaveSettingsHandler(c *gin.Context) {
	var input SettingsInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := SaveSettings(c.Request.Context(), input)
	if err != nil {
		abortSmartDNSError(c, err, errSaveSettingsFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// VerifyDelegationRequest optionally overrides delegation domain for one-off DNS checks.
type VerifyDelegationRequest struct {
	DelegationDomain string `json:"delegation_domain"`
}

// VerifyDelegationHandler verifies live DNS NS delegation for the SmartDNS delegation domain.
// @Summary 校验 SmartDNS 域名委派状态
// @Description 查询委派主域名的实时 NS 记录并与配置主机名比对，需要管理员权限
// @Tags openflare-smartdns
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body smartdns.VerifyDelegationRequest false "可选校验域名"
// @Success 200 {object} response.Any{data=smartdns.DelegationVerifyResult} "校验结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/settings/verify-delegation [post]
func VerifyDelegationHandler(c *gin.Context) {
	var req VerifyDelegationRequest
	if c.Request.ContentLength > 0 {
		if !httpx.BindJSON(c, &req) {
			return
		}
	}
	res, err := VerifyDelegation(c.Request.Context(), req.DelegationDomain)
	if err != nil {
		abortSmartDNSError(c, err, errVerifyDelegationFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// ListInstancesHandler returns all embedded and standalone SmartDNS instances.
// @Summary 获取 SmartDNS 实例列表
// @Description 列出内置与独立部署的 SmartDNS 解析实例，需要管理员权限
// @Tags openflare-smartdns
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]smartdns.InstanceView} "实例列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/instances [get]
func ListInstancesHandler(c *gin.Context) {
	items, err := ListInstances(c.Request.Context())
	if err != nil {
		abortSmartDNSError(c, err, errListInstancesFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(items))
}

// CreateInstanceHandler creates a standalone SmartDNS worker record.
// @Summary 创建独立 SmartDNS 实例
// @Description 预注册独立 SmartDNS 解析实例并返回专属令牌与部署命令，需要管理员权限
// @Tags openflare-smartdns
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body smartdns.InstanceInput true "DNS 实例参数"
// @Success 200 {object} response.Any{data=smartdns.InstanceView} "创建成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/instances [post]
func CreateInstanceHandler(c *gin.Context) {
	var input InstanceInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := CreateInstance(c.Request.Context(), input)
	if err != nil {
		abortSmartDNSError(c, err, errCreateInstanceFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// UpdateInstanceHandler updates a SmartDNS instance.
// @Summary 更新 SmartDNS 实例
// @Description 更新 DNS 实例名称、监听地址、公网地址与地理坐标，需要管理员权限
// @Tags openflare-smartdns
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "实例 ID"
// @Param body body smartdns.InstanceInput true "DNS 实例参数"
// @Success 200 {object} response.Any{data=smartdns.InstanceView} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "实例不存在"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/instances/{id}/update [post]
func UpdateInstanceHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input InstanceInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := UpdateInstance(c.Request.Context(), id, input)
	if err != nil {
		abortSmartDNSError(c, err, errUpdateInstanceFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// DeleteInstanceHandler deletes a standalone SmartDNS instance.
// @Summary 删除 SmartDNS 实例
// @Description 删除指定的独立部署 SmartDNS 实例（内置实例不可删除），需要管理员权限
// @Tags openflare-smartdns
// @Produce json
// @Security SessionCookie
// @Param id path int true "实例 ID"
// @Success 200 {object} response.Any{data=string} "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "实例不存在"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/instances/{id}/delete [post]
func DeleteInstanceHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	if err := DeleteInstance(c.Request.Context(), id); err != nil {
		abortSmartDNSError(c, err, errDeleteInstanceFailed)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// QuarantineInstanceHandler isolates a SmartDNS instance.
// @Summary 隔离 SmartDNS 实例
// @Description 隔离指定 SmartDNS 实例并断开其控制通道，需要管理员权限
// @Tags openflare-smartdns
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "实例 ID"
// @Param body body smartdns.QuarantineInput false "隔离原因"
// @Success 200 {object} response.Any{data=smartdns.InstanceView} "隔离成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "实例不存在"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/instances/{id}/quarantine [post]
func QuarantineInstanceHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input QuarantineInput
	if c.Request.ContentLength > 0 {
		if !httpx.BindJSON(c, &input) {
			return
		}
	}
	view, err := QuarantineInstance(c.Request.Context(), id, input.Reason)
	if err != nil {
		abortSmartDNSError(c, err, errUpdateInstanceFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// UnquarantineInstanceHandler lifts quarantine on a SmartDNS instance.
// @Summary 解除 SmartDNS 实例隔离
// @Description 恢复被隔离 SmartDNS 实例的接入资格，需要管理员权限
// @Tags openflare-smartdns
// @Produce json
// @Security SessionCookie
// @Param id path int true "实例 ID"
// @Success 200 {object} response.Any{data=smartdns.InstanceView} "解除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "实例不存在"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/instances/{id}/unquarantine [post]
func UnquarantineInstanceHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	view, err := UnquarantineInstance(c.Request.Context(), id)
	if err != nil {
		abortSmartDNSError(c, err, errUpdateInstanceFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// RotateInstanceTokenHandler rotates a standalone SmartDNS instance's access token.
// @Summary 轮换 SmartDNS 实例令牌
// @Description 为独立 SmartDNS 实例生成新认证令牌并下发新部署命令，需要管理员权限
// @Tags openflare-smartdns
// @Produce json
// @Security SessionCookie
// @Param id path int true "实例 ID"
// @Success 200 {object} response.Any{data=smartdns.InstanceView} "轮换成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "实例不存在"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/instances/{id}/rotate-token [post]
func RotateInstanceTokenHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	view, err := RotateInstanceToken(c.Request.Context(), id)
	if err != nil {
		abortSmartDNSError(c, err, errUpdateInstanceFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// GetDiscoveryTokenHandler returns the SmartDNS standalone discovery token.
// @Summary 获取 SmartDNS 自动发现令牌
// @Description 返回用于独立 openflare-dns 实例自动注册的全局令牌，需要管理员权限
// @Tags openflare-smartdns
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=smartdns.DiscoveryTokenView} "发现令牌"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/discovery-token [get]
func GetDiscoveryTokenHandler(c *gin.Context) {
	view, err := GetDiscoveryToken(c.Request.Context())
	if err != nil {
		abortSmartDNSError(c, err, errGetSettingsFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// RotateDiscoveryTokenHandler rotates the SmartDNS standalone discovery token.
// @Summary 轮换 SmartDNS 自动发现令牌
// @Description 重新生成独立 SmartDNS 实例注册用发现令牌，需要管理员权限
// @Tags openflare-smartdns
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=smartdns.DiscoveryTokenView} "新发现令牌"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/discovery-token/rotate [post]
func RotateDiscoveryTokenHandler(c *gin.Context) {
	view, err := RotateDiscoveryToken(c.Request.Context())
	if err != nil {
		abortSmartDNSError(c, err, errSaveSettingsFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// ListPoliciesHandler lists all SmartDNS routing policies.
// @Summary 获取 SmartDNS 解析策略列表
// @Description 按优先级顺序返回所有 SmartDNS 解析策略，需要管理员权限
// @Tags openflare-smartdns
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]smartdns.PolicyView} "策略列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/policies [get]
func ListPoliciesHandler(c *gin.Context) {
	items, err := ListPolicies(c.Request.Context())
	if err != nil {
		abortSmartDNSError(c, err, errListPoliciesFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(items))
}

// GetPolicyHandler fetches a single SmartDNS policy by ID.
// @Summary 获取 SmartDNS 解析策略详情
// @Description 获取单个解析策略及其可视化流图与编译后的 JDM JSON，需要管理员权限
// @Tags openflare-smartdns
// @Produce json
// @Security SessionCookie
// @Param id path int true "策略 ID"
// @Success 200 {object} response.Any{data=smartdns.PolicyView} "策略详情"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "策略不存在"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/policies/{id} [get]
func GetPolicyHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	view, err := GetPolicy(c.Request.Context(), id)
	if err != nil {
		abortSmartDNSError(c, err, errGetPolicyFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// CreatePolicyHandler creates a new SmartDNS policy.
// @Summary 创建 SmartDNS 解析策略
// @Description 创建并编译可视化或预设 SmartDNS 调度策略，需要管理员权限
// @Tags openflare-smartdns
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body smartdns.PolicyInput true "策略参数"
// @Success 200 {object} response.Any{data=smartdns.PolicyView} "创建成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/policies [post]
func CreatePolicyHandler(c *gin.Context) {
	var input PolicyInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := CreatePolicy(c.Request.Context(), input)
	if err != nil {
		abortSmartDNSError(c, err, errCreatePolicyFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// UpdatePolicyHandler updates an existing SmartDNS policy.
// @Summary 更新 SmartDNS 解析策略
// @Description 更新并重新编译 SmartDNS 解析策略，递增加载版本号，需要管理员权限
// @Tags openflare-smartdns
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "策略 ID"
// @Param body body smartdns.PolicyInput true "策略参数"
// @Success 200 {object} response.Any{data=smartdns.PolicyView} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "策略不存在"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/policies/{id}/update [post]
func UpdatePolicyHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input PolicyInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := UpdatePolicy(c.Request.Context(), id, input)
	if err != nil {
		abortSmartDNSError(c, err, errUpdatePolicyFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// DeletePolicyHandler deletes an unreferenced non-default SmartDNS policy.
// @Summary 删除 SmartDNS 解析策略
// @Description 删除未设默认且未被 Cloudflare 分组引用的解析策略，需要管理员权限
// @Tags openflare-smartdns
// @Produce json
// @Security SessionCookie
// @Param id path int true "策略 ID"
// @Success 200 {object} response.Any{data=string} "删除成功"
// @Failure 400 {object} response.Any "默认策略不可删除"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "策略不存在"
// @Failure 409 {object} response.Any "策略正在被使用"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/policies/{id}/delete [post]
func DeletePolicyHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	if err := DeletePolicy(c.Request.Context(), id); err != nil {
		abortSmartDNSError(c, err, errDeletePolicyFailed)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// SetDefaultPolicyHandler sets one SmartDNS policy as the default policy.
// @Summary 设为默认 SmartDNS 策略
// @Description 将指定策略设为全局默认解析策略并启用，需要管理员权限
// @Tags openflare-smartdns
// @Produce json
// @Security SessionCookie
// @Param id path int true "策略 ID"
// @Success 200 {object} response.Any{data=string} "设置成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "策略不存在"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/policies/{id}/set-default [post]
func SetDefaultPolicyHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	if err := SetDefaultPolicy(c.Request.Context(), id); err != nil {
		abortSmartDNSError(c, err, errUpdatePolicyFailed)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// ValidatePolicyHandler validates and compiles a visual flow graph or JDM JSON without saving.
// @Summary 校验并编译 SmartDNS 策略流图
// @Description 校验策略决策流图是否无环且表达式合法，并返回编译后的 GoRules JDM JSON，需要管理员权限
// @Tags openflare-smartdns
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body smartdns.PolicyValidateInput true "校验请求"
// @Success 200 {object} response.Any{data=smartdns.PolicyValidateResult} "校验通过"
// @Failure 400 {object} response.Any "流图无效"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Router /api/v1/d/smartdns/policies/validate [post]
func ValidatePolicyHandler(c *gin.Context) {
	var input PolicyValidateInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	res, err := ValidatePolicyGraph(input)
	if err != nil {
		abortSmartDNSError(c, err, errPolicyGraphInvalid)
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// SimulateHandler runs SmartDNS routing simulation for a client IP and domain.
// @Summary 模拟 SmartDNS 智能解析结果
// @Description 基于客户端 IP、域名和记录类型模拟执行策略流图与四阶段评分，返回候选节点打分详情，需要管理员权限
// @Tags openflare-smartdns
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body smartdns.SimulateInput true "模拟参数"
// @Success 200 {object} response.Any{data=smartdns.SimulateResult} "模拟结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "策略不存在"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/smartdns/simulate [post]
func SimulateHandler(c *gin.Context) {
	var input SimulateInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	res, err := SimulateResolution(c.Request.Context(), input)
	if err != nil {
		abortSmartDNSError(c, err, errSnapshotLoadFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// AgentRegisterHandler self-registers a standalone openflare-dns worker.
// @Summary 独立 SmartDNS 实例自注册
// @Description 独立部署的 openflare-dns 实例通过自动发现令牌或实例令牌注册接入，返回专属令牌
// @Tags openflare-dns-agent
// @Accept json
// @Produce json
// @Param X-DNS-Discovery-Token header string false "全局自动发现令牌"
// @Param body body protocol.DNSInstanceRegisterRequest true "注册请求"
// @Success 200 {object} response.Any{data=protocol.DNSInstanceRegisterResponse} "注册成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "认证失败"
// @Failure 403 {object} response.Any "实例已被隔离"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/dns-agent/register [post]
func AgentRegisterHandler(c *gin.Context) {
	var req protocol.DNSInstanceRegisterRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	existing, _ := InstanceFromContext(c)
	resp, err := RegisterDNSInstance(c.Request.Context(), existing, req)
	if err != nil {
		abortSmartDNSError(c, err, errCreateInstanceFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(resp))
}

// AgentHeartbeatHandler ingests runtime metrics and status heartbeats from SmartDNS workers.
// @Summary 上报 SmartDNS 实例心跳
// @Description SmartDNS 实例周期上报 QPS、延迟分位、缓存命中率与系统资源状态
// @Tags openflare-dns-agent
// @Accept json
// @Produce json
// @Param X-DNS-Token header string true "DNS 实例令牌"
// @Param body body protocol.DNSHeartbeatRequest true "心跳载荷"
// @Success 200 {object} response.Any{data=protocol.DNSHeartbeatResponse} "确认结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "认证失败"
// @Failure 403 {object} response.Any "实例已被隔离"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/dns-agent/heartbeat [post]
func AgentHeartbeatHandler(c *gin.Context) {
	inst, ok := InstanceFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errTokenInvalid)
		return
	}
	var req protocol.DNSHeartbeatRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	resp, err := RecordDNSHeartbeat(c.Request.Context(), inst, req)
	if err != nil {
		abortSmartDNSError(c, err, errHeartbeatFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(resp))
}

// AgentPullSnapshotHandler returns the compiled immutable SmartDNS snapshot with ETag & optional gzip support.
// @Summary 拉取 SmartDNS 编译快照
// @Description 返回包含节点状态、策略 JDM、域名绑定与 CompactQualitySnapshot 的不可变运行时快照，支持 ETag 协商与 Gzip 压缩
// @Tags openflare-dns-agent
// @Produce json
// @Param X-DNS-Token header string true "DNS 实例令牌"
// @Param If-None-Match header string false "客户端当前快照 ETag"
// @Success 200 {object} response.Any{data=protocol.DNSSnapshotEnvelope} "快照载荷"
// @Success 304 "快照未变更"
// @Failure 401 {object} response.Any "认证失败"
// @Failure 403 {object} response.Any "实例已被隔离"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/dns-agent/snapshot [get]
func AgentPullSnapshotHandler(c *gin.Context) {
	ctx := c.Request.Context()
	bundle, err := DefaultSnapshotManager().EnsureCurrent(ctx)
	if err != nil {
		logger.ErrorF(ctx, "smartdns snapshot ensure failed: %v", err)
		response.AbortInternal(c, errSnapshotLoadFailed)
		return
	}
	if match := strings.TrimSpace(c.GetHeader("If-None-Match")); match != "" && match == bundle.ETag {
		c.Header("ETag", bundle.ETag)
		c.Status(http.StatusNotModified)
		return
	}
	c.Header("ETag", bundle.ETag)
	c.Header("Content-Type", "application/json; charset=utf-8")
	if strings.Contains(strings.ToLower(c.GetHeader("Accept-Encoding")), "gzip") {
		// Return enveloped response or raw envelope? Standardize envelope for gzip or let client decode HTTP JSON.
		// Serving raw snapshot JSON in standard envelope or directly? Wait: our OpenFlare API uses {error_msg, data} envelope,
		// OR gzipped raw JSON. Returning standard envelope compressed via gzip.Writer ensures response.OK compatibility AND compression!
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")
		c.Status(http.StatusOK)
		gz := gzip.NewWriter(c.Writer)
		_, _ = gz.Write([]byte(`{"error_msg":"","data":`))
		_, _ = gz.Write(bundle.JSONBytes)
		_, _ = gz.Write([]byte(`}`))
		_ = gz.Close()
		return
	}
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write([]byte(`{"error_msg":"","data":`))
	_, _ = c.Writer.Write(bundle.JSONBytes)
	_, _ = c.Writer.Write([]byte(`}`))
}

// AgentWebSocketHandler upgrades a SmartDNS control WebSocket to receive instant snapshot change events.
// @Summary 建立 SmartDNS 控制 WebSocket 长连接
// @Description 为独立 SmartDNS 实例推送快照变更唤醒与隔离指令
// @Tags openflare-dns-agent
// @Param X-DNS-Token header string true "DNS 实例令牌"
// @Success 101 "Switching Protocols"
// @Failure 401 {object} response.Any "认证失败"
// @Failure 403 {object} response.Any "实例已被隔离"
// @Router /api/v1/dns-agent/ws [get]
func AgentWebSocketHandler(c *gin.Context) {
	inst, ok := InstanceFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errTokenInvalid)
		return
	}
	websocket.ServeDNSInstance(c, inst.InstanceID)
}
