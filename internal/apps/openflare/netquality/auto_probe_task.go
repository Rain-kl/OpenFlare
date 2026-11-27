// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netquality

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/netip"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
	"github.com/google/uuid"
)

const (
	// NetQualityProbeTask is the Asynq task name for periodic automated edge traceroutes.
	NetQualityProbeTask = "openflare:netquality_probe"
	// TaskTypeNetQualityProbe is the admin task type for automated network quality probing.
	TaskTypeNetQualityProbe = "of_netquality_probe"

	autoProbeIDPrefix          = "auto-"
	maxAutoProbeTargetsPerNode = 5
	autoProbeTimeoutMs         = uint64(1500)
	autoProbeMaxHops           = uint8(24)
	autoProbeProbesPerHop      = uint8(2)
	autoProbeMatrixLookback    = 25
	prefixCandidateLimit       = 5
	percentDivisor             = 100.0
)

var anchorProbeTargets = []struct {
	asn uint32
	ip  string
}{
	{asn: 13335, ip: "1.1.1.1"},
	{asn: 15169, ip: "8.8.8.8"},
	{asn: 19281, ip: "9.9.9.9"},
}

// NetQualityProbeMeta describes the background auto-probe task in the admin task catalog.
var NetQualityProbeMeta = task.TaskMeta{
	Type:         TaskTypeNetQualityProbe,
	AsynqTask:    NetQualityProbeTask,
	Name:         "OpenFlare automated network quality probe",
	Description:  "Dispatch rate-limited traceroute probes from online edge nodes to active client ASNs to discover AS paths and enrich quality matrix",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
}

// ProbeHandler executes periodic automated edge traceroutes.
type ProbeHandler struct{}

// Execute runs one auto-probe dispatch pass.
func (h *ProbeHandler) Execute(ctx context.Context, _ []byte) (*task.TaskResult, error) {
	task.AppendLog(ctx, "Starting automated network quality probe dispatch")
	summary, err := RunAutoProbe(ctx)
	if err != nil {
		task.AppendLog(ctx, "Automated network quality probe failed: %v", err)
		return nil, err
	}
	detailBytes, _ := json.Marshal(summary)
	msg := fmt.Sprintf(
		"Automated probe finished: enabled=%t online_nodes=%d dispatched=%d",
		summary.Enabled,
		summary.OnlineNodes,
		summary.Dispatched,
	)
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{
		Message: msg,
		Detail:  string(detailBytes),
	}, nil
}

// AutoProbeSummary summarizes one automated probe dispatch run.
type AutoProbeSummary struct {
	Enabled     bool `json:"enabled"`
	OnlineNodes int  `json:"online_nodes"`
	TargetsUsed int  `json:"targets_used"`
	Dispatched  int  `json:"dispatched"`
}

// RunAutoProbe dispatches rate-limited traceroute probes from online edge nodes when enabled.
func RunAutoProbe(ctx context.Context) (*AutoProbeSummary, error) {
	if db.DB(ctx) == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	cfg, cfgErr := repository.GetSystemConfigByKey(ctx, model.ConfigKeyNetIntelAutoProbeEnabled)
	if cfgErr == nil && strings.EqualFold(strings.TrimSpace(cfg.Value), "false") {
		return &AutoProbeSummary{Enabled: false}, nil
	}

	nodes, err := repository.ListOpenFlareNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	now := time.Now().UTC()
	var onlineNodes []model.OpenFlareNode
	for i := range nodes {
		n := nodes[i]
		if n.IsQuarantined() {
			continue
		}
		if n.NodeType != "" && n.NodeType != "edge_node" {
			continue
		}
		if n.Status == "online" || (n.LastSeenAt != nil && now.Sub(*n.LastSeenAt) <= onlineWindow) {
			onlineNodes = append(onlineNodes, n)
		}
	}
	if len(onlineNodes) == 0 {
		return &AutoProbeSummary{Enabled: true}, nil
	}

	targets := selectAutoProbeTargets(ctx, maxAutoProbeTargetsPerNode)
	dispatched := 0
	timeoutMs := autoProbeTimeoutMs
	for _, node := range onlineNodes {
		for _, targetIP := range targets {
			dispatch := protocol.NetProbeTaskDispatch{
				ProbeID: autoProbeIDPrefix + uuid.NewString(),
				Kind:    "traceroute",
				Traceroute: &protocol.TracerouteRequest{
					Target:       targetIP,
					Protocol:     "icmp",
					MaxHops:      autoProbeMaxHops,
					ProbesPerHop: autoProbeProbesPerHop,
					TimeoutMs:    &timeoutMs,
				},
			}
			if websocket.DispatchNetProbeTask(ctx, node.NodeID, dispatch) {
				dispatched++
			}
		}
	}
	return &AutoProbeSummary{
		Enabled:     true,
		OnlineNodes: len(onlineNodes),
		TargetsUsed: len(targets),
		Dispatched:  dispatched,
	}, nil
}

func selectAutoProbeTargets(ctx context.Context, limit int) []string {
	seenIP := make(map[string]struct{})
	seenASN := make(map[uint32]struct{})
	targets := make([]string, 0, limit)

	rows, _, err := repository.ListOpenFlareNetworkQualityMatrix(ctx, model.NetworkQualityMatrixQuery{
		Page:      1,
		PageSize:  autoProbeMatrixLookback,
		SortBy:    "tcp_sample_count",
		SortOrder: "desc",
	})
	if err == nil {
		targets = appendMatrixTargets(targets, rows, seenIP, seenASN, limit)
	}

	for _, anchor := range anchorProbeTargets {
		if len(targets) >= limit {
			break
		}
		if _, ok := seenIP[anchor.ip]; ok {
			continue
		}
		seenIP[anchor.ip] = struct{}{}
		targets = append(targets, anchor.ip)
	}
	return targets
}

func appendMatrixTargets(
	targets []string,
	rows []*model.OpenFlareNetworkQualityMatrix,
	seenIP map[string]struct{},
	seenASN map[uint32]struct{},
	limit int,
) []string {
	for _, row := range rows {
		if len(targets) >= limit {
			break
		}
		if row == nil || row.ClientASN == 0 {
			continue
		}
		if _, ok := seenASN[row.ClientASN]; ok {
			continue
		}
		seenASN[row.ClientASN] = struct{}{}
		ip := firstProbeablePrefixIP(row.ClientASN)
		if ip == "" {
			continue
		}
		if _, ok := seenIP[ip]; ok {
			continue
		}
		seenIP[ip] = struct{}{}
		targets = append(targets, ip)
	}
	return targets
}

func firstProbeablePrefixIP(asn uint32) string {
	prefixes, _ := netintel.ListASNPrefixes(asn, "ipv4", 1, prefixCandidateLimit)
	for _, p := range prefixes {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(p.CIDR))
		if err != nil || !prefix.Addr().Is4() {
			continue
		}
		addr := prefix.Addr().Next()
		if addr.IsGlobalUnicast() && !addr.IsPrivate() {
			return addr.String()
		}
	}
	return ""
}

// EnrichedTracerouteHop represents a traceroute hop enriched with geolocation, ASN, and bgp.tools tags.
type EnrichedTracerouteHop struct {
	protocol.TracerouteHop
	CountryISO string   `json:"country_iso,omitempty"`
	RegionCode string   `json:"region_code,omitempty"`
	CityName   string   `json:"city_name,omitempty"`
	ASN        uint32   `json:"asn,omitempty"`
	ASNName    string   `json:"asn_name,omitempty"`
	ASNTags    []string `json:"asn_tags,omitempty"`
}

// IngestProbeResult records completed traceroute and batch ping observations into
// of_as_topology_edges, of_traceroute_records, and of_network_quality_samples.
func IngestProbeResult(ctx context.Context, nodeID string, payload protocol.NetProbeResultPayload) {
	defer func() {
		if r := recover(); r != nil {
			logger.ErrorF(ctx, "[NetQuality] IngestProbeResult panic recovered: %v", r)
		}
	}()

	trimmedNode := strings.TrimSpace(nodeID)
	if trimmedNode == "" || db.DB(ctx) == nil {
		return
	}

	switch payload.Kind {
	case "traceroute":
		if payload.Traceroute != nil {
			ingestTracerouteResult(ctx, trimmedNode, payload.ProbeID, payload.Traceroute)
		}
	case "ping_batch":
		if payload.Ping != nil {
			ingestPingBatchResult(ctx, trimmedNode, payload.Ping)
		}
	}
}

func ingestTracerouteResult(ctx context.Context, nodeID, probeID string, tr *protocol.TracerouteResult) {
	enrichedHops := EnrichTracerouteHops(tr.Hops)
	asPath := make([]uint32, 0, len(enrichedHops))
	var edges []repository.ObservedASEdge
	var prevASN uint32
	var prevRTT float64
	seenAt := time.Now().UTC()

	for _, hop := range enrichedHops {
		if hop.ASN == 0 {
			continue
		}
		if len(asPath) == 0 || asPath[len(asPath)-1] != hop.ASN {
			asPath = append(asPath, hop.ASN)
		}
		var hopRTT float64
		if hop.AvgMs != nil {
			hopRTT = *hop.AvgMs
		}
		if prevASN != 0 && prevASN != hop.ASN {
			delta := math.Max(0, hopRTT-prevRTT)
			edges = append(edges, repository.ObservedASEdge{
				SrcASN:   prevASN,
				DstASN:   hop.ASN,
				EdgeType: "observed",
				HopRTTMs: round2(delta),
				SeenAt:   seenAt,
			})
		}
		prevASN = hop.ASN
		prevRTT = hopRTT
	}

	var targetASN uint32
	var targetCountry, targetRegion, geoSource string
	var clientLat, clientLon *float64
	var geoAccuracyKm float64
	var anycastStatus string
	lookupIP := strings.TrimSpace(tr.ResolvedIP)
	if lookupIP == "" {
		lookupIP = strings.TrimSpace(tr.Target)
	}
	ipFamily := pkgnetintel.DetectIPFamily(lookupIP)
	if lookupIP != "" {
		if res := netintel.LookupIP(lookupIP); res != nil {
			targetASN = res.ASN
			targetCountry = res.CountryISO
			targetRegion = res.RegionCode
			clientLat = res.Latitude
			clientLon = res.Longitude
			geoAccuracyKm = res.AccuracyRadiusKm
			geoSource = res.GeoSource
			anycastStatus = res.AnycastStatus
		}
	}

	// Isolate confirmed Anycast targets from AS topology edges to prevent misleading shortcut links.
	if len(edges) > 0 && anycastStatus != pkgnetintel.AnycastStatusConfirmed {
		if err := repository.UpsertOpenFlareASTopologyEdges(ctx, edges); err != nil {
			logger.WarnF(ctx, "[NetQuality] upsert AS topology edges failed (node=%s): %v", nodeID, err)
		}
	}

	triggerType := model.TracerouteTriggerManual
	if strings.HasPrefix(probeID, autoProbeIDPrefix) {
		triggerType = model.TracerouteTriggerAuto
	}
	hopsJSON, _ := json.Marshal(enrichedHops)
	record := &model.OpenFlareTracerouteRecord{
		TaskID:        strings.TrimSpace(probeID),
		TriggerType:   triggerType,
		NodeID:        nodeID,
		Target:        strings.TrimSpace(tr.Target),
		ResolvedIP:    strings.TrimSpace(tr.ResolvedIP),
		TargetASN:     targetASN,
		TargetCountry: targetCountry,
		TargetRegion:  targetRegion,
		IPFamily:      ipFamily,
		Protocol:      strings.TrimSpace(tr.Protocol),
		HopCount:      int(tr.HopCount),
		Reached:       tr.Reached,
		TotalRTTMs:    round2(tr.TotalRTTMs),
		HopsJSON:      string(hopsJSON),
		ASPathJSON:    encodeUint32SliceJSON(asPath),
		ErrorMessage:  strings.TrimSpace(tr.Error),
	}
	if err := repository.CreateOpenFlareTracerouteRecord(ctx, record); err != nil {
		logger.WarnF(ctx, "[NetQuality] persist traceroute record failed (node=%s): %v", nodeID, err)
	}

	if targetASN > 0 && tr.Reached && tr.TotalRTTMs > 0 && anycastStatus != pkgnetintel.AnycastStatusConfirmed {
		var lossRate float64
		if len(tr.Hops) > 0 {
			last := tr.Hops[len(tr.Hops)-1]
			lossRate = math.Min(1.0, math.Max(0, last.LossPct/percentDivisor))
		}
		var hist LogHistogram64
		hist.Add(tr.TotalRTTMs)
		var sketch PrefixSketch
		sketch.AddIP(lookupIP)

		sample := &model.OpenFlareNetworkQualitySample{
			NodeID:              nodeID,
			NodeASN:             lookupCachedNodeASNs(ctx)[nodeID],
			ClientASN:           targetASN,
			ClientCountry:       targetCountry,
			ClientRegion:        targetRegion,
			IPFamily:            ipFamily,
			StatsVersion:        model.NetworkQualityStatsVersionHist64,
			Source:              model.NetworkQualitySourceTraceroute,
			SampleCount:         1,
			RTTMinMs:            round2(tr.TotalRTTMs),
			RTTP50Ms:            round2(tr.TotalRTTMs),
			RTTP95Ms:            round2(tr.TotalRTTMs),
			RTTHist:             hist.Encode(),
			PrefixSketch:        sketch.Encode(),
			DistinctPrefixCount: sketch.DistinctCount(),
			ClientLat:           clientLat,
			ClientLon:           clientLon,
			GeoAccuracyKm:       geoAccuracyKm,
			GeoSource:           geoSource,
			LossRate:            round4(lossRate),
			HopCount:            int(tr.HopCount),
			CapturedAt:          seenAt,
		}
		if err := repository.InsertOpenFlareNetworkQualitySample(ctx, sample); err != nil {
			logger.WarnF(ctx, "[NetQuality] insert traceroute sample failed (node=%s): %v", nodeID, err)
		}
	}
}

func ingestPingBatchResult(ctx context.Context, nodeID string, ping *protocol.PingBatchResult) {
	if ping.Received == 0 || ping.AvgMs <= 0 {
		return
	}
	lookupIP := strings.TrimSpace(ping.ResolvedIP)
	if lookupIP == "" {
		lookupIP = strings.TrimSpace(ping.Target)
	}
	res := netintel.LookupIP(lookupIP)
	if res == nil || res.ASN == 0 {
		return
	}
	if res.AnycastStatus == pkgnetintel.AnycastStatusConfirmed {
		return
	}

	source := model.NetworkQualitySourceICMPActive
	if strings.EqualFold(strings.TrimSpace(ping.Protocol), "tcp") {
		source = model.NetworkQualitySourceTCPActive
	}

	var hist LogHistogram64
	for _, s := range ping.Samples {
		if s.Success && s.RTTMs > 0 {
			hist.Add(float64(s.RTTMs))
		}
	}
	if hist.Count == 0 {
		hist.Add(ping.AvgMs)
	}
	var sketch PrefixSketch
	sketch.AddIP(lookupIP)

	p50 := hist.Quantile(percentileMedian)
	p95 := hist.Quantile(percentileP95)
	if ping.P95Ms > 0 && len(ping.Samples) == 0 {
		p50 = ping.AvgMs
		p95 = ping.P95Ms
	}

	sample := &model.OpenFlareNetworkQualitySample{
		NodeID:              nodeID,
		NodeASN:             lookupCachedNodeASNs(ctx)[nodeID],
		ClientASN:           res.ASN,
		ClientCountry:       res.CountryISO,
		ClientRegion:        res.RegionCode,
		IPFamily:            pkgnetintel.DetectIPFamily(lookupIP),
		StatsVersion:        model.NetworkQualityStatsVersionHist64,
		Source:              source,
		SampleCount:         max(1, int(ping.Received)),
		RTTMinMs:            round2(ping.MinMs),
		RTTP50Ms:            round2(p50),
		RTTP95Ms:            round2(p95),
		RTTHist:             hist.Encode(),
		PrefixSketch:        sketch.Encode(),
		DistinctPrefixCount: sketch.DistinctCount(),
		ClientLat:           res.Latitude,
		ClientLon:           res.Longitude,
		GeoAccuracyKm:       res.AccuracyRadiusKm,
		GeoSource:           res.GeoSource,
		LossRate:            round4(math.Min(1.0, math.Max(0, ping.LossRate/percentDivisor))),
		CapturedAt:          time.Now().UTC(),
	}
	if err := repository.InsertOpenFlareNetworkQualitySample(ctx, sample); err != nil {
		logger.WarnF(ctx, "[NetQuality] insert ping_batch sample failed (node=%s): %v", nodeID, err)
	}
}

// EnrichTracerouteHops annotates each traceroute hop with geolocation, ASN, org, and bgp.tools tags.
func EnrichTracerouteHops(hops []protocol.TracerouteHop) []EnrichedTracerouteHop {
	out := make([]EnrichedTracerouteHop, len(hops))
	for i, hop := range hops {
		enriched := EnrichedTracerouteHop{TracerouteHop: hop}
		ip := strings.TrimSpace(hop.IP)
		if ip != "" {
			if res := netintel.LookupIP(ip); res != nil {
				enriched.CountryISO = res.CountryISO
				enriched.RegionCode = res.RegionCode
				enriched.CityName = res.CityName
				enriched.ASN = res.ASN
				enriched.ASNName = res.ASNName
				enriched.ASNTags = res.ASNTags
			}
		}
		out[i] = enriched
	}
	return out
}
