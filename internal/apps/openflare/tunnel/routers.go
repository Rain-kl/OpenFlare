// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/shared/httpx"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func handleTunnelLogicError(c *gin.Context, err error, notFoundMsg string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.AbortNotFound(c, notFoundMsg)
		return true
	}
	switch err.Error() {
	case errPoolKeyExists, errPoolDefaultCannotDelete, errPoolHasRelaysOrTunnels, errTunnelInUseByProxyRoutes:
		response.AbortConflict(c, err.Error())
		return true
	case errRelayQuarantined, errTunnelQuarantined, errTunnelConnectorRevoked:
		c.Header(quarantinedHeader, quarantinedHeaderTrue)
		response.AbortForbidden(c, err.Error())
		return true
	default:
		response.AbortBadRequest(c, err.Error())
		return true
	}
}

func bindOptionalJSONBody(body io.Reader, target any) error {
	if err := json.NewDecoder(body).Decode(target); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// ListPoolsHandler lists all tunnel relay pools.
// @Summary 获取中继节点池列表
// @Description 返回全部中继节点池及其中继、隧道数量统计
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]tunnel.PoolView} "中继节点池列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Router /api/v1/d/tunnel-pools [get]
func ListPoolsHandler(c *gin.Context) {
	pools, err := ListPools(c.Request.Context())
	if handleTunnelLogicError(c, err, errPoolNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(pools))
}

// GetPoolHandler gets one tunnel relay pool by ID.
// @Summary 获取中继节点池详情
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Param id path int true "中继池 ID"
// @Success 200 {object} response.Any{data=tunnel.PoolView} "中继池详情"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 404 {object} response.Any "中继池不存在"
// @Router /api/v1/d/tunnel-pools/{id} [get]
func GetPoolHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	pool, err := GetPool(c.Request.Context(), id)
	if handleTunnelLogicError(c, err, errPoolNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(pool))
}

// CreatePoolHandler creates a new tunnel relay pool.
// @Summary 创建中继节点池
// @Tags openflare-tunnel
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body tunnel.PoolInput true "中继池参数"
// @Success 200 {object} response.Any{data=tunnel.PoolView} "创建成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 409 {object} response.Any "pool_key 已存在"
// @Router /api/v1/d/tunnel-pools [post]
func CreatePoolHandler(c *gin.Context) {
	var input PoolInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := CreatePool(c.Request.Context(), input)
	if handleTunnelLogicError(c, err, errPoolNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// UpdatePoolHandler updates an existing tunnel relay pool.
// @Summary 更新中继节点池
// @Tags openflare-tunnel
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "中继池 ID"
// @Param body body tunnel.PoolInput true "中继池参数"
// @Success 200 {object} response.Any{data=tunnel.PoolView} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 404 {object} response.Any "中继池不存在"
// @Router /api/v1/d/tunnel-pools/{id}/update [post]
func UpdatePoolHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input PoolInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := UpdatePool(c.Request.Context(), id, input)
	if handleTunnelLogicError(c, err, errPoolNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// DeletePoolHandler deletes an empty non-default relay pool.
// @Summary 删除中继节点池
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Param id path int true "中继池 ID"
// @Success 200 {object} response.Any{data=string} "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 404 {object} response.Any "中继池不存在"
// @Failure 409 {object} response.Any "默认池或非空池不可删除"
// @Router /api/v1/d/tunnel-pools/{id}/delete [post]
func DeletePoolHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	if err := DeletePool(c.Request.Context(), id); handleTunnelLogicError(c, err, errPoolNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// ListRelaysHandler lists all tunnel relay PoPs.
// @Summary 获取中继 PoP 列表
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]tunnel.RelayView} "中继节点列表"
// @Failure 401 {object} response.Any "未登录"
// @Router /api/v1/d/tunnel-relays [get]
func ListRelaysHandler(c *gin.Context) {
	relays, err := ListRelays(c.Request.Context())
	if handleTunnelLogicError(c, err, errRelayNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(relays))
}

// GetRelayHandler gets one tunnel relay PoP by ID.
// @Summary 获取中继 PoP 详情
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Param id path int true "中继节点 ID"
// @Success 200 {object} response.Any{data=tunnel.RelayView} "中继详情"
// @Failure 404 {object} response.Any "中继不存在"
// @Router /api/v1/d/tunnel-relays/{id} [get]
func GetRelayHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	view, err := GetRelay(c.Request.Context(), id)
	if handleTunnelLogicError(c, err, errRelayNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// GetRelayDeployHandler gets deployment command for an existing relay PoP.
// @Summary 获取中继 PoP 部署命令
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Param id path int true "中继节点 ID"
// @Success 200 {object} response.Any{data=tunnel.RelayDeployView} "部署命令与凭证"
// @Failure 404 {object} response.Any "中继不存在"
// @Router /api/v1/d/tunnel-relays/{id}/deploy [get]
func GetRelayDeployHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	view, err := GetRelayDeploy(c.Request.Context(), id)
	if handleTunnelLogicError(c, err, errRelayNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// CreateRelayHandler creates a new tunnel relay PoP.
// @Summary 创建中继 PoP
// @Tags openflare-tunnel
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body tunnel.RelayInput true "中继节点参数"
// @Success 200 {object} response.Any{data=tunnel.RelayDeployView} "创建成功及部署命令"
// @Failure 400 {object} response.Any "参数错误"
// @Router /api/v1/d/tunnel-relays [post]
func CreateRelayHandler(c *gin.Context) {
	var input RelayInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := CreateRelay(c.Request.Context(), input)
	if handleTunnelLogicError(c, err, errRelayNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// UpdateRelayHandler updates a tunnel relay PoP.
// @Summary 更新中继 PoP
// @Tags openflare-tunnel
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "中继节点 ID"
// @Param body body tunnel.RelayInput true "中继节点参数"
// @Success 200 {object} response.Any{data=tunnel.RelayView} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 404 {object} response.Any "中继不存在"
// @Router /api/v1/d/tunnel-relays/{id}/update [post]
func UpdateRelayHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input RelayInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := UpdateRelay(c.Request.Context(), id, input)
	if handleTunnelLogicError(c, err, errRelayNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// DeleteRelayHandler deletes a tunnel relay PoP.
// @Summary 删除中继 PoP
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Param id path int true "中继节点 ID"
// @Success 200 {object} response.Any{data=string} "删除成功"
// @Failure 404 {object} response.Any "中继不存在"
// @Router /api/v1/d/tunnel-relays/{id}/delete [post]
func DeleteRelayHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	if err := DeleteRelay(c.Request.Context(), id); handleTunnelLogicError(c, err, errRelayNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// RotateRelayTokenHandler rotates a relay PoP access token.
// @Summary 轮换中继访问令牌
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Param id path int true "中继节点 ID"
// @Success 200 {object} response.Any{data=tunnel.RelayDeployView} "轮换后的凭证"
// @Failure 404 {object} response.Any "中继不存在"
// @Router /api/v1/d/tunnel-relays/{id}/rotate-token [post]
func RotateRelayTokenHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	view, err := RotateRelayToken(c.Request.Context(), id)
	if handleTunnelLogicError(c, err, errRelayNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// SetRelayDrainingHandler sets draining mode on a relay PoP.
// @Summary 设置中继排空状态
// @Tags openflare-tunnel
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "中继节点 ID"
// @Param body body tunnel.RelayDrainInput true "排空开关"
// @Success 200 {object} response.Any{data=tunnel.RelayView} "更新后的中继"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 404 {object} response.Any "中继不存在"
// @Router /api/v1/d/tunnel-relays/{id}/drain [post]
func SetRelayDrainingHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input RelayDrainInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := SetRelayDraining(c.Request.Context(), id, input.Draining)
	if handleTunnelLogicError(c, err, errRelayNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// QuarantineRelayHandler quarantines a relay PoP.
// @Summary 紧急隔离中继 PoP
// @Tags openflare-tunnel
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "中继节点 ID"
// @Param body body tunnel.RelayQuarantineInput false "隔离原因"
// @Success 200 {object} response.Any{data=tunnel.RelayView} "已隔离的中继"
// @Failure 404 {object} response.Any "中继不存在"
// @Router /api/v1/d/tunnel-relays/{id}/quarantine [post]
func QuarantineRelayHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input RelayQuarantineInput
	if c.Request.ContentLength > 0 {
		if err := bindOptionalJSONBody(c.Request.Body, &input); err != nil {
			response.AbortBadRequest(c, "invalid parameters")
			return
		}
	}
	view, err := QuarantineRelay(c.Request.Context(), id, input.Reason)
	if handleTunnelLogicError(c, err, errRelayNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// UnquarantineRelayHandler unquarantines a relay PoP and issues a fresh access token.
// @Summary 解除中继隔离并签发新令牌
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Param id path int true "中继节点 ID"
// @Success 200 {object} response.Any{data=tunnel.RelayDeployView} "解除隔离后的中继"
// @Failure 404 {object} response.Any "中继不存在"
// @Router /api/v1/d/tunnel-relays/{id}/unquarantine [post]
func UnquarantineRelayHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	view, err := UnquarantineRelay(c.Request.Context(), id)
	if handleTunnelLogicError(c, err, errRelayNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// RequestRelayBinaryUpdateHandler schedules a self-update on a relay PoP.
// @Summary 下发中继二进制更新
// @Tags openflare-tunnel
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "中继节点 ID"
// @Param body body tunnel.BinaryUpdateInput false "更新参数"
// @Success 200 {object} response.Any{data=tunnel.RelayView} "更新已登记"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 404 {object} response.Any "中继不存在"
// @Router /api/v1/d/tunnel-relays/{id}/update-binary [post]
func RequestRelayBinaryUpdateHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input BinaryUpdateInput
	if c.Request.ContentLength > 0 {
		if err := bindOptionalJSONBody(c.Request.Body, &input); err != nil {
			response.AbortBadRequest(c, "invalid parameters")
			return
		}
	}
	view, err := RequestRelayBinaryUpdate(c.Request.Context(), id, input)
	if handleTunnelLogicError(c, err, errRelayNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// GetRelayBootstrapTokenHandler gets the global relay discovery bootstrap token.
// @Summary 获取中继发现引导令牌
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=tunnel.RelayBootstrapView} "中继引导令牌"
// @Router /api/v1/d/tunnel-relays/bootstrap-token [get]
func GetRelayBootstrapTokenHandler(c *gin.Context) {
	view, err := GetRelayBootstrapToken(c.Request.Context())
	if handleTunnelLogicError(c, err, errRelayNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// RotateRelayBootstrapTokenHandler rotates the global relay discovery bootstrap token.
// @Summary 轮换中继发现引导令牌
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=tunnel.RelayBootstrapView} "新中继引导令牌"
// @Router /api/v1/d/tunnel-relays/bootstrap-token/rotate [post]
func RotateRelayBootstrapTokenHandler(c *gin.Context) {
	view, err := RotateRelayBootstrapToken(c.Request.Context())
	if handleTunnelLogicError(c, err, errRelayNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// GetTunnelReleaseHandler returns the latest tunnel-v* release metadata.
// @Summary 获取隧道组件最新版本信息
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Param channel query string false "发布渠道（stable/preview）"
// @Success 200 {object} response.Any{data=tunnel.TunnelReleaseInfo} "版本信息"
// @Failure 400 {object} response.Any "参数或查询错误"
// @Router /api/v1/d/tunnels/release [get]
func GetTunnelReleaseHandler(c *gin.Context) {
	info, err := GetLatestTunnelRelease(c.Request.Context(), c.Query("channel"))
	if handleTunnelLogicError(c, err, errTunnelNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(info))
}

// ListTunnelsHandler lists all logical tunnels.
// @Summary 获取隧道列表
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]tunnel.TunnelView} "隧道列表"
// @Router /api/v1/d/tunnels [get]
func ListTunnelsHandler(c *gin.Context) {
	views, err := ListTunnels(c.Request.Context())
	if handleTunnelLogicError(c, err, errTunnelNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(views))
}

// GetTopologyHandler returns global tunnel mesh topology.
// @Summary 获取隧道全局拓扑视图
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=tunnel.TunnelTopologyView} "拓扑数据"
// @Router /api/v1/d/tunnels/topology [get]
func GetTopologyHandler(c *gin.Context) {
	topology, err := GetTopology(c.Request.Context())
	if handleTunnelLogicError(c, err, errTunnelNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(topology))
}

// GetTunnelHandler returns full details for a single logical tunnel.
// @Summary 获取隧道详情
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Param id path int true "隧道 ID"
// @Success 200 {object} response.Any{data=tunnel.TunnelDetailView} "隧道详情"
// @Failure 404 {object} response.Any "隧道不存在"
// @Router /api/v1/d/tunnels/{id} [get]
func GetTunnelHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	detail, err := GetTunnelDetail(c.Request.Context(), id)
	if handleTunnelLogicError(c, err, errTunnelNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(detail))
}

// GetTunnelDeployHandler returns deployment command and credentials for a logical tunnel.
// @Summary 获取隧道连接器部署命令
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Param id path int true "隧道 ID"
// @Success 200 {object} response.Any{data=tunnel.TunnelDeployView} "部署命令与凭证"
// @Failure 404 {object} response.Any "隧道不存在"
// @Router /api/v1/d/tunnels/{id}/deploy [get]
func GetTunnelDeployHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	view, err := GetTunnelDeploy(c.Request.Context(), id)
	if handleTunnelLogicError(c, err, errTunnelNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// CreateTunnelHandler creates a logical tunnel.
// @Summary 创建隧道
// @Tags openflare-tunnel
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body tunnel.TunnelInput true "隧道参数"
// @Success 200 {object} response.Any{data=tunnel.TunnelDeployView} "创建成功及部署命令"
// @Failure 400 {object} response.Any "参数错误"
// @Router /api/v1/d/tunnels [post]
func CreateTunnelHandler(c *gin.Context) {
	var input TunnelInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := CreateTunnel(c.Request.Context(), input)
	if handleTunnelLogicError(c, err, errTunnelNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// UpdateTunnelHandler updates a logical tunnel.
// @Summary 更新隧道配置
// @Tags openflare-tunnel
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "隧道 ID"
// @Param body body tunnel.TunnelInput true "隧道参数"
// @Success 200 {object} response.Any{data=tunnel.TunnelView} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 404 {object} response.Any "隧道不存在"
// @Router /api/v1/d/tunnels/{id}/update [post]
func UpdateTunnelHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input TunnelInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := UpdateTunnel(c.Request.Context(), id, input)
	if handleTunnelLogicError(c, err, errTunnelNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// DeleteTunnelHandler deletes a logical tunnel.
// @Summary 删除隧道
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Param id path int true "隧道 ID"
// @Success 200 {object} response.Any{data=string} "删除成功"
// @Failure 404 {object} response.Any "隧道不存在"
// @Failure 409 {object} response.Any "隧道仍被路由引用"
// @Router /api/v1/d/tunnels/{id}/delete [post]
func DeleteTunnelHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	if err := DeleteTunnel(c.Request.Context(), id); handleTunnelLogicError(c, err, errTunnelNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// RotateTunnelTokenHandler rotates a logical tunnel access token.
// @Summary 轮换隧道接入令牌
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Param id path int true "隧道 ID"
// @Success 200 {object} response.Any{data=tunnel.TunnelDeployView} "新凭证与部署命令"
// @Failure 404 {object} response.Any "隧道不存在"
// @Router /api/v1/d/tunnels/{id}/rotate-token [post]
func RotateTunnelTokenHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	view, err := RotateTunnelToken(c.Request.Context(), id)
	if handleTunnelLogicError(c, err, errTunnelNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// QuarantineTunnelHandler isolates a logical tunnel.
// @Summary 紧急隔离隧道
// @Tags openflare-tunnel
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "隧道 ID"
// @Param body body tunnel.TunnelQuarantineInput false "隔离原因"
// @Success 200 {object} response.Any{data=tunnel.TunnelView} "已隔离的隧道"
// @Failure 404 {object} response.Any "隧道不存在"
// @Router /api/v1/d/tunnels/{id}/quarantine [post]
func QuarantineTunnelHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input TunnelQuarantineInput
	if c.Request.ContentLength > 0 {
		if err := bindOptionalJSONBody(c.Request.Body, &input); err != nil {
			response.AbortBadRequest(c, "invalid parameters")
			return
		}
	}
	view, err := QuarantineTunnel(c.Request.Context(), id, input.Reason)
	if handleTunnelLogicError(c, err, errTunnelNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// UnquarantineTunnelHandler clears tunnel quarantine and issues a fresh token.
// @Summary 解除隧道隔离并签发新令牌
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Param id path int true "隧道 ID"
// @Success 200 {object} response.Any{data=tunnel.TunnelDeployView} "解除隔离后的隧道"
// @Failure 404 {object} response.Any "隧道不存在"
// @Router /api/v1/d/tunnels/{id}/unquarantine [post]
func UnquarantineTunnelHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	view, err := UnquarantineTunnel(c.Request.Context(), id)
	if handleTunnelLogicError(c, err, errTunnelNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// RequestTunnelBinaryUpdateHandler requests self-update across connectors of a tunnel.
// @Summary 下发连接器二进制更新
// @Tags openflare-tunnel
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "隧道 ID"
// @Param body body tunnel.BinaryUpdateInput false "更新参数"
// @Success 200 {object} response.Any{data=tunnel.TunnelView} "更新已登记"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 404 {object} response.Any "隧道不存在"
// @Router /api/v1/d/tunnels/{id}/update-binary [post]
func RequestTunnelBinaryUpdateHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input BinaryUpdateInput
	if c.Request.ContentLength > 0 {
		if err := bindOptionalJSONBody(c.Request.Body, &input); err != nil {
			response.AbortBadRequest(c, "invalid parameters")
			return
		}
	}
	view, err := RequestTunnelBinaryUpdate(c.Request.Context(), id, input)
	if handleTunnelLogicError(c, err, errTunnelNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// RevokeConnectorHandler revokes a single openflared replica belonging to a tunnel.
// @Summary 吊销指定连接器副本
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Param id path int true "隧道 ID"
// @Param connector_id path string true "连接器实例 ID"
// @Success 200 {object} response.Any{data=string} "吊销成功"
// @Failure 404 {object} response.Any "隧道或连接器不存在"
// @Router /api/v1/d/tunnels/{id}/connectors/{connector_id}/revoke [post]
func RevokeConnectorHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	connectorID := strings.TrimSpace(c.Param("connector_id"))
	if err := RevokeConnector(c.Request.Context(), id, connectorID); handleTunnelLogicError(c, err, errTunnelConnectorNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// CreateTunnelServiceHandler creates an L4 TCP/UDP exposed service under a tunnel.
// @Summary 创建隧道 L4 端口转发服务
// @Tags openflare-tunnel
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "隧道 ID"
// @Param body body tunnel.TunnelServiceInput true "L4 服务参数"
// @Success 200 {object} response.Any{data=tunnel.TunnelServiceView} "创建成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 404 {object} response.Any "隧道不存在"
// @Router /api/v1/d/tunnels/{id}/services [post]
func CreateTunnelServiceHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input TunnelServiceInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := CreateTunnelService(c.Request.Context(), id, input)
	if handleTunnelLogicError(c, err, errTunnelNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// UpdateTunnelServiceHandler updates an L4 TCP/UDP exposed service under a tunnel.
// @Summary 更新隧道 L4 端口转发服务
// @Tags openflare-tunnel
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "隧道 ID"
// @Param service_id path int true "服务 ID"
// @Param body body tunnel.TunnelServiceInput true "L4 服务参数"
// @Success 200 {object} response.Any{data=tunnel.TunnelServiceView} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 404 {object} response.Any "服务不存在"
// @Router /api/v1/d/tunnels/{id}/services/{service_id}/update [post]
func UpdateTunnelServiceHandler(c *gin.Context) {
	tunnelID, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	serviceID, ok := httpx.NamedIDParam(c, "service_id")
	if !ok {
		return
	}
	var input TunnelServiceInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := UpdateTunnelService(c.Request.Context(), tunnelID, serviceID, input)
	if handleTunnelLogicError(c, err, errTunnelServiceNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// DeleteTunnelServiceHandler deletes an L4 TCP/UDP exposed service under a tunnel.
// @Summary 删除隧道 L4 端口转发服务
// @Tags openflare-tunnel
// @Produce json
// @Security SessionCookie
// @Param id path int true "隧道 ID"
// @Param service_id path int true "服务 ID"
// @Success 200 {object} response.Any{data=string} "删除成功"
// @Failure 404 {object} response.Any "服务不存在"
// @Router /api/v1/d/tunnels/{id}/services/{service_id}/delete [post]
func DeleteTunnelServiceHandler(c *gin.Context) {
	tunnelID, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	serviceID, ok := httpx.NamedIDParam(c, "service_id")
	if !ok {
		return
	}
	if err := DeleteTunnelService(c.Request.Context(), tunnelID, serviceID); handleTunnelLogicError(c, err, errTunnelServiceNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// RelayRegisterHandler handles bootstrap or credentialed registration for openflare-relay.
// @Summary 注册隧道中继 PoP
// @Tags openflare-relay
// @Accept json
// @Produce json
// @Param X-Relay-Token header string false "中继令牌或发现引导令牌"
// @Param body body protocol.RelayRegisterRequest true "中继注册信息"
// @Success 200 {object} response.Any{data=protocol.RelayRegisterResponse} "注册响应"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Failure 403 {object} response.Any "中继已隔离"
// @Router /api/v1/relay/register [post]
func RelayRegisterHandler(c *gin.Context) {
	var req protocol.RelayRegisterRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	authRelay, _ := RelayFromContext(c)
	resp, err := RegisterRelay(c.Request.Context(), authRelay, req, clientIPFromRequest(c))
	if handleTunnelLogicError(c, err, errRelayNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(resp))
}

// RelayHeartbeatHandler records relay PoP metrics and verifies active Rathole connector leases.
// @Summary 中继 PoP 心跳与租约续期
// @Tags openflare-relay
// @Accept json
// @Produce json
// @Param X-Relay-Token header string true "中继访问令牌"
// @Param body body protocol.RelayHeartbeatPayload true "中继心跳载荷"
// @Success 200 {object} response.Any{data=protocol.RelayHeartbeatResponse} "心跳响应"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Failure 403 {object} response.Any "中继已隔离"
// @Router /api/v1/relay/heartbeat [post]
func RelayHeartbeatHandler(c *gin.Context) {
	var payload protocol.RelayHeartbeatPayload
	if !httpx.BindJSON(c, &payload) {
		return
	}
	relay, ok := RelayFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errRelayTokenInvalid)
		return
	}
	resp, err := HeartbeatRelay(c.Request.Context(), relay, payload, clientIPFromRequest(c))
	if handleTunnelLogicError(c, err, errRelayNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(resp))
}

// RelayConfigHandler returns active authorized Rathole services and gateway config for openflare-relay.
// @Summary 拉取中继运行时配置
// @Tags openflare-relay
// @Produce json
// @Param X-Relay-Token header string true "中继访问令牌"
// @Success 200 {object} response.Any{data=protocol.RelayRuntimeConfigResponse} "中继运行时配置"
// @Failure 401 {object} response.Any "Token 无效"
// @Failure 403 {object} response.Any "中继已隔离"
// @Router /api/v1/relay/config [get]
func RelayConfigHandler(c *gin.Context) {
	relay, ok := RelayFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errRelayTokenInvalid)
		return
	}
	resp, err := GetRelayRuntimeConfig(c.Request.Context(), relay)
	if handleTunnelLogicError(c, err, errRelayNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(resp))
}

// RelayWebSocketHandler upgrades a control-plane wake WebSocket for openflare-relay.
// @Summary 中继控制面 WebSocket
// @Tags openflare-relay
// @Param X-Relay-Token header string true "中继访问令牌"
// @Failure 401 {object} response.Any "Token 无效"
// @Router /api/v1/relay/ws [get]
func RelayWebSocketHandler(c *gin.Context) {
	relay, ok := RelayFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errRelayTokenInvalid)
		return
	}
	websocket.ServeRelay(c, relay.RelayID)
}

// TunnelHeartbeatHandler records an openflared connector heartbeat.
// @Summary 连接器心跳上报
// @Tags openflare-tunnel-agent
// @Accept json
// @Produce json
// @Param X-Tunnel-Token header string true "隧道访问令牌"
// @Param body body protocol.TunnelConnectorHeartbeatPayload true "连接器心跳载荷"
// @Success 200 {object} response.Any{data=protocol.TunnelConnectorHeartbeatResponse} "心跳响应"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Failure 403 {object} response.Any "隧道已隔离或连接器已吊销"
// @Router /api/v1/tunnel/heartbeat [post]
func TunnelHeartbeatHandler(c *gin.Context) {
	var payload protocol.TunnelConnectorHeartbeatPayload
	if !httpx.BindJSON(c, &payload) {
		return
	}
	tun, ok := TunnelFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errTunnelTokenInvalid)
		return
	}
	resp, err := HeartbeatConnector(c.Request.Context(), tun, payload, clientIPFromRequest(c))
	if handleTunnelLogicError(c, err, errTunnelNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(resp))
}

// TunnelConfigHandler returns candidate relays, active grants, CIDR rules, and routes for openflared.
// @Summary 拉取连接器配置与候选中继目录
// @Tags openflare-tunnel-agent
// @Produce json
// @Param X-Tunnel-Token header string true "隧道访问令牌"
// @Param connector_id query string false "连接器实例 ID"
// @Success 200 {object} response.Any{data=protocol.TunnelConnectorConfigResponse} "连接器配置"
// @Failure 401 {object} response.Any "Token 无效"
// @Failure 403 {object} response.Any "隧道已隔离"
// @Router /api/v1/tunnel/config [get]
func TunnelConfigHandler(c *gin.Context) {
	tun, ok := TunnelFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errTunnelTokenInvalid)
		return
	}
	resp, err := GetConnectorConfig(c.Request.Context(), tun, connectorIDFromRequest(c))
	if handleTunnelLogicError(c, err, errTunnelNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(resp))
}

// TunnelNegotiateAttachmentsHandler issues per-relay Rathole session grants for closest-K relays.
// @Summary 协商连接器中继挂载租约
// @Tags openflare-tunnel-agent
// @Accept json
// @Produce json
// @Param X-Tunnel-Token header string true "隧道访问令牌"
// @Param body body protocol.TunnelNegotiateAttachmentsRequest true "选定中继请求"
// @Success 200 {object} response.Any{data=protocol.TunnelNegotiateAttachmentsResponse} "签发会话凭证"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Failure 403 {object} response.Any "隧道已隔离"
// @Router /api/v1/tunnel/attachments [post]
func TunnelNegotiateAttachmentsHandler(c *gin.Context) {
	var req protocol.TunnelNegotiateAttachmentsRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	tun, ok := TunnelFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errTunnelTokenInvalid)
		return
	}
	resp, err := NegotiateConnectorAttachments(c.Request.Context(), tun, req, clientIPFromRequest(c))
	if handleTunnelLogicError(c, err, errTunnelNotFound) {
		return
	}
	c.JSON(http.StatusOK, response.OK(resp))
}

// TunnelWebSocketHandler upgrades a control-plane wake WebSocket for an openflared connector.
// @Summary 连接器控制面 WebSocket
// @Tags openflare-tunnel-agent
// @Param X-Tunnel-Token header string true "隧道访问令牌"
// @Param X-Connector-ID header string false "连接器实例 ID"
// @Param connector_id query string false "连接器实例 ID"
// @Failure 400 {object} response.Any "缺少连接器 ID"
// @Failure 401 {object} response.Any "Token 无效"
// @Failure 403 {object} response.Any "隧道已隔离或连接器已吊销"
// @Router /api/v1/tunnel/ws [get]
func TunnelWebSocketHandler(c *gin.Context) {
	tun, ok := TunnelFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errTunnelTokenInvalid)
		return
	}
	connectorID := connectorIDFromRequest(c)
	if connectorID == "" {
		response.AbortBadRequest(c, errTunnelConnectorRequired)
		return
	}
	websocket.ServeConnector(c, tun.ID, connectorID)
}
