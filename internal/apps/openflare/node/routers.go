// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/shared/httpx"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func handleLogicError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	return httpx.AbortNotFoundIfMissing(c, err, errNodeNotFound)
}

// ListNodesHandler lists all nodes.
// @Summary 获取节点列表
// @Description 返回所有节点及最新配置下发记录，需要管理员权限
// @Tags openflare-node
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]node.View} "节点列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Router /api/v1/d/nodes [get]
func ListNodesHandler(c *gin.Context) {
	nodes, err := ListNodes(c.Request.Context())
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(nodes))
}

// CreateNodeHandler creates a node.
// @Summary 创建节点
// @Description 创建新的边缘节点记录，需要管理员权限
// @Tags openflare-node
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body node.Input true "节点参数"
// @Success 200 {object} response.Any{data=node.View} "创建成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Router /api/v1/d/nodes [post]
func CreateNodeHandler(c *gin.Context) {
	var input Input
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := CreateNode(c.Request.Context(), input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// UpdateNodeHandler updates a node.
// @Summary 更新节点
// @Description 更新指定节点的配置信息，需要管理员权限
// @Tags openflare-node
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "节点 ID"
// @Param body body node.Input true "节点参数"
// @Success 200 {object} response.Any{data=node.View} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或节点不存在"
// @Router /api/v1/d/nodes/{id}/update [post]
func UpdateNodeHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input Input
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := UpdateNode(c.Request.Context(), id, input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// DeleteNodeHandler deletes a node.
// @Summary 删除节点
// @Description 删除指定节点记录，需要管理员权限
// @Tags openflare-node
// @Produce json
// @Security SessionCookie
// @Param id path int true "节点 ID"
// @Success 200 {object} response.Any{data=string} "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或节点不存在"
// @Router /api/v1/d/nodes/{id}/delete [post]
func DeleteNodeHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	if err := DeleteNode(c.Request.Context(), id); handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// GetBootstrapTokenHandler returns the global discovery token.
// @Summary 获取引导令牌
// @Description 返回全局节点发现引导令牌，需要管理员权限
// @Tags openflare-node
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=node.BootstrapView} "引导令牌"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Router /api/v1/d/nodes/bootstrap-token [get]
func GetBootstrapTokenHandler(c *gin.Context) {
	view, err := GetBootstrapToken(c.Request.Context())
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// RotateBootstrapTokenHandler rotates the global discovery token.
// @Summary 轮换引导令牌
// @Description 重新生成全局节点发现引导令牌，需要管理员权限
// @Tags openflare-node
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=node.BootstrapView} "新引导令牌"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Router /api/v1/d/nodes/bootstrap-token/rotate [post]
func RotateBootstrapTokenHandler(c *gin.Context) {
	view, err := RotateBootstrapToken(c.Request.Context())
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// GetAgentReleaseHandler returns the latest agent release for a node.
// @Summary 获取节点发布信息
// @Description 按节点类型返回指定发布渠道的最新版本，需要管理员权限
// @Tags openflare-node
// @Produce json
// @Security SessionCookie
// @Param id path int true "节点 ID"
// @Param channel query string false "发布渠道"
// @Success 200 {object} response.Any{data=node.AgentReleaseInfo} "节点发布信息"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或节点不存在"
// @Router /api/v1/d/nodes/{id}/agent-release [get]
func GetAgentReleaseHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	release, err := GetAgentRelease(c.Request.Context(), id, c.Query("channel"))
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(release))
}

// RequestAgentUpdateHandler requests agent self-update on a node.
// @Summary 请求更新节点
// @Description 为指定节点登记更新请求，边缘节点的请求固定到目标版本，需要管理员权限
// @Tags openflare-node
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "节点 ID"
// @Param body body node.AgentUpdateInput false "更新参数（可选）"
// @Success 200 {object} response.Any{data=node.View} "更新请求已登记"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或节点不存在"
// @Router /api/v1/d/nodes/{id}/agent-update [post]
func RequestAgentUpdateHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var request AgentUpdateInput
	if c.Request.ContentLength > 0 {
		if err := bindOptionalJSON(c.Request.Body, &request); err != nil {
			response.AbortBadRequest(c, "invalid parameters")
			return
		}
	}
	view, err := RequestAgentUpdate(c.Request.Context(), id, request)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// RequestEdgeRestartHandler requests openresty restart on a node.
// @Summary 请求重启 OpenFlare Edge
// @Description 向指定节点下发 OpenResty 重启指令，需要管理员权限
// @Tags openflare-node
// @Produce json
// @Security SessionCookie
// @Param id path int true "节点 ID"
// @Success 200 {object} response.Any{data=node.View} "重启请求已下发"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或节点不存在"
// @Router /api/v1/d/nodes/{id}/edge-restart [post]
func RequestEdgeRestartHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	view, err := RequestEdgeRestart(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// RequestForceSyncHandler requests force sync on a node.
// @Summary 请求强制同步配置
// @Description 向指定节点下发强制同步当前活跃配置的指令，需要管理员权限
// @Tags openflare-node
// @Produce json
// @Security SessionCookie
// @Param id path int true "节点 ID"
// @Success 200 {object} response.Any{data=node.View} "同步请求已下发"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或节点不存在"
// @Router /api/v1/d/nodes/{id}/force-sync [post]
func RequestForceSyncHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	view, err := RequestForceSync(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// GetObservabilityHandler returns node observability details.
// @Summary 获取节点可观测性数据
// @Description 返回指定节点的指标、健康事件与流量分析数据，需要管理员权限
// @Tags openflare-node
// @Produce json
// @Security SessionCookie
// @Param id path int true "节点 ID"
// @Param hours query int false "统计时间范围（小时，最大 720）"
// @Param limit query int false "返回记录数量上限"
// @Param realtime query bool false "实时模式（近 30 分钟高分辨率快照）"
// @Success 200 {object} response.Any{data=node.ObservabilityView} "可观测性数据"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或节点不存在"
// @Router /api/v1/d/nodes/{id}/observability [get]
func GetObservabilityHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var query ObservabilityQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.AbortBadRequest(c, "invalid parameters")
		return
	}
	view, err := GetObservability(c.Request.Context(), id, query)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// CleanupHealthEventsHandler cleans up node health events.
// @Summary 清理节点健康事件
// @Description 清理指定节点的历史健康事件记录，需要管理员权限
// @Tags openflare-node
// @Produce json
// @Security SessionCookie
// @Param id path int true "节点 ID"
// @Success 200 {object} response.Any{data=node.HealthEventCleanupResult} "清理结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或节点不存在"
// @Router /api/v1/d/nodes/{id}/observability/cleanup [post]
func CleanupHealthEventsHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	result, err := CleanupHealthEvents(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

func bindOptionalJSON(body io.Reader, target any) error {
	if err := json.NewDecoder(body).Decode(target); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// ListNodePageHandler lists a bounded page of nodes.
// @Summary 获取分页节点列表
// @Description 返回分页节点和筛选后的总数；旧版 /nodes 列表接口保持兼容，需要管理员权限
// @Tags openflare-node
// @Produce json
// @Security SessionCookie
// @Param page query int false "页码（默认 1）"
// @Param page_size query int false "每页数量（默认 20，最大 100）"
// @Param search query string false "名称、节点标识或 IP 的字面子串（最多 256 字节）"
// @Param status query string false "pending、online、offline 或 quarantined"
// @Param node_type query string false "节点类型（edge_node）"
// @Success 200 {object} response.Any{data=node.ListResult} "节点分页"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Failure 500 {object} response.Any "查询失败"
// @Router /api/v1/d/nodes/page [get]
func ListNodePageHandler(c *gin.Context) {
	var query ListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.AbortBadRequest(c, errInvalidNodeListQuery)
		return
	}
	normalized, err := normalizeListQuery(query)
	if err != nil {
		response.AbortBadRequest(c, errInvalidNodeListQuery)
		return
	}
	result, err := ListPage(c.Request.Context(), normalized)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "Failed to list node page: %v", err)
		response.AbortInternal(c, errListNodesFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

// GetNodeHandler returns a single node.
// @Summary 获取节点详情
// @Description 返回指定节点及最新配置下发记录，需要管理员权限
// @Tags openflare-node
// @Produce json
// @Security SessionCookie
// @Param id path int true "节点 ID"
// @Success 200 {object} response.Any{data=node.View} "节点详情"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "查询失败"
// @Router /api/v1/d/nodes/{id} [get]
func GetNodeHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	view, err := GetNode(c.Request.Context(), id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.AbortNotFound(c, errNodeNotFound)
		return
	}
	if err != nil {
		logger.ErrorF(c.Request.Context(), "Failed to get node: %v", err)
		response.AbortInternal(c, errListNodesFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// GetCertificateDeliveriesHandler lists TLS private-key deliveries audited for a node.
// @Summary 获取节点证书密钥下发记录
// @Description 返回控制面向指定节点内存下发过的 TLS 证书密钥审计台账，需要管理员权限
// @Tags openflare-node
// @Produce json
// @Security SessionCookie
// @Param id path int true "节点 ID"
// @Success 200 {object} response.Any{data=[]node.CertificateDeliveryView} "下发审计记录"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或节点不存在"
// @Router /api/v1/d/nodes/{id}/certificate-deliveries [get]
func GetCertificateDeliveriesHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	views, err := ListNodeCertificateDeliveries(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(views))
}

// QuarantineNodeHandler isolates a compromised or suspect edge node.
// @Summary 紧急隔离节点
// @Description 轮换并吊销节点接入令牌、断开 WebSocket、触发 DNS 备用节点切换，并按需重签且吊销受影响的 ACME 证书，需要管理员权限
// @Tags openflare-node
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "节点 ID"
// @Param body body node.QuarantineInput false "隔离参数（可选）"
// @Success 200 {object} response.Any{data=node.QuarantineResult} "隔离结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或节点不存在"
// @Router /api/v1/d/nodes/{id}/quarantine [post]
func QuarantineNodeHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input QuarantineInput
	if c.Request.ContentLength > 0 {
		if err := bindOptionalJSON(c.Request.Body, &input); err != nil {
			response.AbortBadRequest(c, "invalid parameters")
			return
		}
	}
	result, err := QuarantineNode(c.Request.Context(), id, input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

// UnquarantineNodeHandler clears node quarantine and issues a fresh access token.
// @Summary 解除节点隔离并重置令牌
// @Description 清除节点隔离状态并生成全新的接入令牌以供重建节点使用，需要管理员权限
// @Tags openflare-node
// @Produce json
// @Security SessionCookie
// @Param id path int true "节点 ID"
// @Success 200 {object} response.Any{data=node.View} "解除隔离后的节点"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或节点不存在"
// @Router /api/v1/d/nodes/{id}/unquarantine [post]
func UnquarantineNodeHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	view, err := UnquarantineNode(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}
