// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nettools

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/shared/httpx"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
)

const defaultHistoryPageSize = 20

type triggerSyncRequest struct {
	Force bool `json:"force"`
}

// DispatchPingHandler dispatches a batch Ping / TCPing / HTTP latency probe to selected edge nodes.
// @Summary 多点 Ping / TCPing 探测
// @Description 向指定或全部在线边缘节点下发一次性批量 ICMP/TCP/HTTP 延迟探测任务，返回 task_id 供轮询结果
// @Tags openflare-network-tools
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body nettools.PingDispatchInput true "探测参数"
// @Success 200 {object} response.Any{data=nettools.ProbeTaskState} "探测任务状态"
// @Failure 400 {object} response.Any "参数错误或无可用在线节点"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/ping [post]
func DispatchPingHandler(c *gin.Context) {
	var input PingDispatchInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	state, err := DispatchPing(c.Request.Context(), input)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(state))
}

// DispatchTracerouteHandler dispatches a trippy-core traceroute task to selected edge nodes.
// @Summary 多点路由追踪（Traceroute）
// @Description 向指定或全部在线边缘节点下发多轮 Traceroute（ICMP/UDP/TCP）探测任务，逐跳富化 ASN 与 GeoIP 信息
// @Tags openflare-network-tools
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body nettools.TracerouteDispatchInput true "路由追踪参数"
// @Success 200 {object} response.Any{data=nettools.ProbeTaskState} "路由追踪任务状态"
// @Failure 400 {object} response.Any "参数错误或无可用在线节点"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/traceroute [post]
func DispatchTracerouteHandler(c *gin.Context) {
	var input TracerouteDispatchInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	state, err := DispatchTraceroute(c.Request.Context(), input)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(state))
}

// GetTaskHandler polls an active or completed multi-node network probe task.
// @Summary 查询网络探测任务状态与结果
// @Description 根据 task_id 获取各边缘节点上传的 Ping/TCPing、Traceroute 或 DNS 探测结果
// @Tags openflare-network-tools
// @Produce json
// @Security SessionCookie
// @Param task_id path string true "探测任务 ID"
// @Success 200 {object} response.Any{data=nettools.ProbeTaskState} "探测任务状态"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "任务不存在或已过期"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/tasks/{task_id} [get]
func GetTaskHandler(c *gin.Context) {
	state, err := GetTask(c.Request.Context(), c.Param("task_id"))
	if err != nil {
		response.AbortNotFound(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(state))
}

// ListTracerouteHistoryHandler returns paginated historical traceroute records.
// @Summary 列出路由追踪历史记录
// @Description 分页查询手动或后台自动执行的边缘节点路由追踪历史与 AS 路径
// @Tags openflare-network-tools
// @Produce json
// @Security SessionCookie
// @Param node_id query string false "节点 ID"
// @Param target query string false "目标域名或 IP"
// @Param trigger_type query string false "触发类型（manual 或 auto）"
// @Param target_asn query int false "目标 ASN"
// @Param p query int false "页码"
// @Param page_size query int false "每页条数"
// @Success 200 {object} response.Any{data=nettools.TracerouteHistoryPage} "历史路由追踪列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/traceroute/history [get]
func ListTracerouteHistoryHandler(c *gin.Context) {
	targetASN, ok := httpx.OptionalPositiveUint32Query(c, "target_asn", errInvalidASN)
	if !ok {
		return
	}
	query := TracerouteHistoryQuery{
		Page:        parsePositiveInt(c.Query("p"), 1),
		PageSize:    parsePositiveInt(c.Query("page_size"), defaultHistoryPageSize),
		NodeID:      c.Query("node_id"),
		Target:      c.Query("target"),
		TriggerType: c.Query("trigger_type"),
		TargetASN:   targetASN,
	}
	res, err := ListTracerouteHistory(c.Request.Context(), query)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// QueryDNSHandler runs a direct DNS/EDNS0-ECS query from the controller or dispatches across edge nodes.
// @Summary DNS 与 EDNS0 ECS 模拟查询
// @Description 支持主控直接探测或下发到多个边缘节点探测（支持 EDNS0 Client Subnet 视角模拟），并自动富化应答 IP 的 ASN 与 GeoIP
// @Tags openflare-network-tools
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body nettools.DNSQueryInput true "DNS 查询参数"
// @Success 200 {object} response.Any{data=nettools.DNSQueryResponse} "DNS 查询结果或异步任务"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/dns [post]
func QueryDNSHandler(c *gin.Context) {
	var input DNSQueryInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	res, err := QueryDNS(c.Request.Context(), input)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// LookupIPHandler resolves GeoIP, ASN, bgp.tools tags/prefixes, and AS topology neighbors for an IP or hostname.
// @Summary IP / 域名网络情报查询
// @Description 查询目标 IP 或域名的地理位置、所属 ASN、BGP 宣告前缀与 AS 邻接关系
// @Tags openflare-network-tools
// @Produce json
// @Security SessionCookie
// @Param ip query string true "目标 IP、CIDR 或域名"
// @Success 200 {object} response.Any{data=nettools.IPLookupResponse} "IP 网络情报"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/lookup/ip [get]
func LookupIPHandler(c *gin.Context) {
	query := c.Query("ip")
	if query == "" {
		query = c.Query("query")
	}
	res, err := LookupIP(c.Request.Context(), query)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// LookupASNHandler returns ASN metadata, prefix counts, and CAIDA/observed neighbors.
// @Summary ASN 详情查询
// @Description 查询自治系统号（ASN）的组织名称、分类标签、IPv4/IPv6 前缀规模与邻接 AS 关系
// @Tags openflare-network-tools
// @Produce json
// @Security SessionCookie
// @Param asn path int true "ASN 编号"
// @Success 200 {object} response.Any{data=nettools.ASNDetailResponse} "ASN 详情"
// @Failure 400 {object} response.Any "ASN 参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/lookup/asn/{asn} [get]
func LookupASNHandler(c *gin.Context) {
	asn, ok := parseASNParam(c)
	if !ok {
		return
	}
	res, err := LookupASN(c.Request.Context(), asn)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// GetASNUpstreamGraphHandler returns the layered upstream routing connectivity DAG for an ASN.
// @Summary ASN 上游路由接线图查询
// @Description 基于本地 CAIDA、RIPEstat Looking Glass/Neighbours 与实测路径构建 Origins -> Upstreams -> Tier 1 ISPs 分层上游拓扑图
// @Tags openflare-network-tools
// @Produce json
// @Security SessionCookie
// @Param asn path int true "ASN 编号"
// @Param prefix query string false "采样前缀（可选，默认选用可见度最高的公告前缀）"
// @Param refresh query bool false "是否忽略缓存强制刷新"
// @Success 200 {object} response.Any{data=nettools.ASNUpstreamGraphResponse} "ASN 上游路由拓扑图"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/lookup/asn/{asn}/upstream-graph [get]
func GetASNUpstreamGraphHandler(c *gin.Context) {
	asn, ok := parseASNParam(c)
	if !ok {
		return
	}
	forceRefresh := strings.EqualFold(c.Query("refresh"), "true") || c.Query("refresh") == "1"
	res, err := GetASNUpstreamGraph(c.Request.Context(), asn, c.Query("prefix"), forceRefresh)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// ListASNPrefixesHandler paginates BGP prefixes announced by an ASN.
// @Summary 分页列出 ASN 宣告的 BGP 前缀
// @Description 按 IPv4/IPv6 地址族分页返回指定 ASN 宣告的 CIDR 前缀与热度权重
// @Tags openflare-network-tools
// @Produce json
// @Security SessionCookie
// @Param asn path int true "ASN 编号"
// @Param family query string false "地址族（all、ipv4 或 ipv6）"
// @Param p query int false "页码"
// @Param page_size query int false "每页条数"
// @Success 200 {object} response.Any{data=nettools.ASNPrefixesResponse} "ASN 前缀分页列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/lookup/asn/{asn}/prefixes [get]
func ListASNPrefixesHandler(c *gin.Context) {
	asn, ok := parseASNParam(c)
	if !ok {
		return
	}
	page := parsePositiveInt(c.Query("p"), 1)
	pageSize := parsePositiveInt(c.Query("page_size"), defaultHistoryPageSize)
	res, err := ListASNPrefixes(c.Request.Context(), asn, c.Query("family"), page, pageSize)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// TraceASPathHandler compares AS paths and quality metrics across edge nodes toward a target ASN or IP.
// @Summary 多节点 AS 路径与质量对比
// @Description 基于已观测路由拓扑与 5 级质量预测器计算各边缘节点到达目标 ASN/IP（支持区域与 IP 族参数）的 AS 路径、预测延迟与评分
// @Tags openflare-network-tools
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body nettools.ASPathInput true "目标 ASN 或 IP"
// @Success 200 {object} response.Any{data=nettools.ASPathResponse} "节点 AS 路径对比结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/as-path [post]
func TraceASPathHandler(c *gin.Context) {
	var input ASPathInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	res, err := TraceASPath(c.Request.Context(), input)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// ListQualityMatrixHandler returns paginated network quality matrix rows.
// @Summary 分页查询网络质量矩阵
// @Description 列出节点×访客 ASN×地区×IP族维度的被动 TCP RTT、主动探测 RTT、有效延迟、置信度与质量评分
// @Tags openflare-network-tools
// @Produce json
// @Security SessionCookie
// @Param node_id query string false "节点 ID"
// @Param client_asn query int false "访客 ASN"
// @Param client_country query string false "国家/地区 ISO 编码"
// @Param client_region query string false "下钻省/州细分编码（如 CN-GD、US-CA）"
// @Param ip_family query string false "IP 地址族（ipv4、ipv6、unknown）"
// @Param aggregation_level query string false "聚合层级（country 或 region）"
// @Param measurement_source query string false "实测来源（tcp_passive、tcp_active、icmp_active 等）"
// @Param data_source query string false "数据来源（measured 或 predicted）"
// @Param search query string false "关键词搜索"
// @Param sort_by query string false "排序字段"
// @Param sort_order query string false "排序方向（asc 或 desc）"
// @Param p query int false "页码"
// @Param page_size query int false "每页条数"
// @Success 200 {object} response.Any{data=nettools.QualityMatrixPage} "网络质量矩阵列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/quality-matrix [get]
func ListQualityMatrixHandler(c *gin.Context) {
	clientASN, ok := httpx.OptionalPositiveUint32Query(c, "client_asn", errInvalidASN)
	if !ok {
		return
	}
	query := model.NetworkQualityMatrixQuery{
		Page:              parsePositiveInt(c.Query("p"), 1),
		PageSize:          parsePositiveInt(c.Query("page_size"), defaultHistoryPageSize),
		NodeID:            c.Query("node_id"),
		ClientASN:         clientASN,
		ClientCountry:     c.Query("client_country"),
		ClientRegion:      c.Query("client_region"),
		IPFamily:          c.Query("ip_family"),
		AggregationLevel:  c.Query("aggregation_level"),
		MeasurementSource: c.Query("measurement_source"),
		DataSource:        c.Query("data_source"),
		Search:            c.Query("search"),
		SortBy:            c.Query("sort_by"),
		SortOrder:         c.Query("sort_order"),
	}
	res, err := ListQualityMatrix(c.Request.Context(), query)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// PredictQualityMatrixHandler simulates five-level SmartDNS node latency prediction for a client network target.
// @Summary 预测客户端到各节点网络质量
// @Description 基于 5 级预测器（区域精确→国家级→同节点相似上游→同城/同组节点→地理基线）计算各节点预测延迟、离散度、置信度与评分
// @Tags openflare-network-tools
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body nettools.QualityPredictInput true "客户端目标（IP、ASN、国家/地区或 IP 族）"
// @Success 200 {object} response.Any{data=nettools.QualityPredictResponse} "各节点预测质量排序结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/quality-matrix/predict [post]
func PredictQualityMatrixHandler(c *gin.Context) {
	var input QualityPredictInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	res, err := PredictQualityMatrix(c.Request.Context(), input)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// GetNetIntelStatusHandler returns status of loaded GeoIP/ASN/bgp.tools/CAIDA database files.
// @Summary 获取网络情报数据库状态
// @Description 查看各 GeoIP/ASN MMDB、bgp.tools 路由与标签库、可选 CAIDA 数据集的加载状态、记录数与版权信息
// @Tags openflare-network-tools
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=nettools.NetIntelStatusResponse} "数据库状态列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/netintel/status [get]
func GetNetIntelStatusHandler(c *gin.Context) {
	res, err := GetNetIntelStatus(c.Request.Context())
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// TriggerNetIntelSyncHandler triggers an immediate or Asynq-queued NetIntel database sync.
// @Summary 触发网络情报数据库同步
// @Description 下发 of_netintel_sync 异步任务（若未启用任务队列则在当前请求同步执行）以更新数据库并热重载
// @Tags openflare-network-tools
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body nettools.triggerSyncRequest false "同步参数"
// @Success 200 {object} response.Any{data=nettools.TriggerSyncResponse} "同步任务已派发或完成"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "同步失败"
// @Router /api/v1/d/network-tools/netintel/sync [post]
func TriggerNetIntelSyncHandler(c *gin.Context) {
	var req triggerSyncRequest
	if c.Request.ContentLength > 0 {
		if !httpx.BindJSON(c, &req) {
			return
		}
	}
	res, err := TriggerNetIntelSync(c.Request.Context(), req.Force)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// ListProbeTargetsHandler returns paginated managed probe targets and per-node backoff states.
// @Summary 分页查询受控主动探测靶点
// @Description 列出 of_probe_targets 探测靶点及其按节点维度（of_probe_target_states）的连续失败次数、退避截止时间与最近成功延迟
// @Tags openflare-network-tools
// @Produce json
// @Security SessionCookie
// @Param asn query int false "目标 ASN"
// @Param country query string false "国家/地区 ISO 编码"
// @Param ip_family query string false "IP 地址族（ipv4 或 ipv6）"
// @Param target_type query string false "靶点类型（public_anchor、customer_isp、manual）"
// @Param anycast_status query string false "Anycast 状态（unknown、unicast、confirmed）"
// @Param enabled query bool false "是否启用"
// @Param search query string false "关键词搜索"
// @Param p query int false "页码"
// @Param page_size query int false "每页条数"
// @Success 200 {object} response.Any{data=nettools.ProbeTargetPage} "受控探测靶点列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/probe-targets [get]
func ListProbeTargetsHandler(c *gin.Context) {
	asn, ok := httpx.OptionalPositiveUint32Query(c, "asn", errInvalidASN)
	if !ok {
		return
	}
	query := model.ProbeTargetQuery{
		Page:          parsePositiveInt(c.Query("p"), 1),
		PageSize:      parsePositiveInt(c.Query("page_size"), defaultHistoryPageSize),
		ASN:           asn,
		Country:       c.Query("country"),
		IPFamily:      c.Query("ip_family"),
		TargetType:    c.Query("target_type"),
		AnycastStatus: c.Query("anycast_status"),
		Enabled:       parseOptionalBool(c.Query("enabled")),
		Search:        c.Query("search"),
	}
	res, err := ListProbeTargets(c.Request.Context(), query)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// CreateProbeTargetHandler validates and registers a new public active probe target.
// @Summary 新增受控主动探测靶点
// @Description 执行公网 IP 与 SSRF 校验并保存受控探测靶点（禁止私网、回环、CGNAT 及内部地址）
// @Tags openflare-network-tools
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body nettools.ProbeTargetInput true "探测靶点配置"
// @Success 200 {object} response.Any{data=model.OpenFlareProbeTarget} "已创建的探测靶点"
// @Failure 400 {object} response.Any "参数错误或 SSRF 校验失败"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/probe-targets [post]
func CreateProbeTargetHandler(c *gin.Context) {
	var input ProbeTargetInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	res, err := CreateProbeTarget(c.Request.Context(), input)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// UpdateProbeTargetHandler updates settings or enablement for an existing managed probe target.
// @Summary 更新受控主动探测靶点
// @Description 修改探测靶点名称、协议、端口、Anycast 标注或启停状态
// @Tags openflare-network-tools
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "靶点 ID"
// @Param body body nettools.ProbeTargetUpdateInput true "靶点更新参数"
// @Success 200 {object} response.Any{data=model.OpenFlareProbeTarget} "更新后的探测靶点"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/probe-targets/{id} [put]
func UpdateProbeTargetHandler(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var input ProbeTargetUpdateInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	res, err := UpdateProbeTarget(c.Request.Context(), id, input)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// DeleteProbeTargetHandler removes a managed probe target and its node backoff state records.
// @Summary 删除受控主动探测靶点
// @Description 删除指定探测靶点及其关联的节点探测退避状态
// @Tags openflare-network-tools
// @Produce json
// @Security SessionCookie
// @Param id path int true "靶点 ID"
// @Success 200 {object} response.Any "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/network-tools/probe-targets/{id} [delete]
func DeleteProbeTargetHandler(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if httpx.AbortBadRequestOnError(c, DeleteProbeTarget(c.Request.Context(), id)) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	id, err := httpx.ParseUint(c.Param(name))
	if err != nil || id == 0 {
		response.AbortBadRequest(c, errProbeTargetNotFound)
		return 0, false
	}
	return id, true
}

func parseOptionalBool(raw string) *bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1":
		v := true
		return &v
	case "false", "0":
		v := false
		return &v
	default:
		return nil
	}
}

func parseASNParam(c *gin.Context) (uint32, bool) {
	return httpx.PositiveUint32Param(c, "asn", errInvalidASN)
}

func parsePositiveInt(raw string, def int) int {
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return def
	}
	return v
}
