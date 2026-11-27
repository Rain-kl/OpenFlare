// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netquality

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
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
	// NetQualityProbeTask is the Asynq task name for periodic automated edge active probes.
	NetQualityProbeTask = "openflare:netquality_probe"
	// TaskTypeNetQualityProbe is the admin task type for automated network quality probing.
	TaskTypeNetQualityProbe = "of_netquality_probe"

	autoProbeIDPrefix               = "auto-"
	autoProbeGlobalDispatchBudget   = 24
	autoProbeMaxTargetsPerNode      = 4
	autoProbeMaxNodesPerTarget      = 4
	autoProbeMaxPacketCostPerNode   = 120
	autoProbeTraceroutePacketCost   = 48
	autoProbePingPacketCost         = 4
	autoProbeTimeoutMs              = uint64(1500)
	autoProbePingCount              = uint8(4)
	autoProbePingIntervalMs         = uint64(200)
	autoProbeMaxHops                = uint8(24)
	autoProbeProbesPerHop           = uint8(2)
	autoProbeBackoffBase            = 15 * time.Minute
	autoProbeBackoffMax             = 24 * time.Hour
	autoProbeMinCooldown            = 20 * time.Minute
	autoProbeFreshSkipAge           = 2 * time.Hour
	autoProbeFreshMinConfidence     = 0.85
	autoProbeExploreStaleAge        = 6 * time.Hour
	autoProbeExploreBonus           = 1.35
	autoProbeNeverProbedStaleFactor = 2.0
	autoProbeStaleBaseFactor        = 0.5
	autoProbeStaleHalfHours         = 12.0
	autoProbeMinUncertainty         = 0.15
	autoProbeDispersionScaleMs      = 50.0
	autoProbeMaxUncertainty         = 2.5
	autoProbeOptimalConfFloor       = 0.72
	percentDivisor                  = 100.0
	maxTrackedAutoProbeIDs          = 2048
)

type autoProbeTargetMeta struct {
	targetID      uint
	anycastStatus string
	port          int
}

var (
	autoProbeMetaMu          sync.Mutex
	autoProbeDispatchTargets = make(map[string]autoProbeTargetMeta)
	dispatchNetProbeFn       = websocket.DispatchNetProbeTask
)

func recordAutoProbeMeta(probeID string, meta autoProbeTargetMeta) {
	autoProbeMetaMu.Lock()
	defer autoProbeMetaMu.Unlock()
	if len(autoProbeDispatchTargets) >= maxTrackedAutoProbeIDs {
		autoProbeDispatchTargets = make(map[string]autoProbeTargetMeta)
	}
	autoProbeDispatchTargets[probeID] = meta
}

func consumeAutoProbeMeta(probeID string) (autoProbeTargetMeta, bool) {
	autoProbeMetaMu.Lock()
	defer autoProbeMetaMu.Unlock()
	meta, ok := autoProbeDispatchTargets[probeID]
	if ok {
		delete(autoProbeDispatchTargets, probeID)
	}
	return meta, ok
}

func resetAutoProbeTracking() {
	autoProbeMetaMu.Lock()
	autoProbeDispatchTargets = make(map[string]autoProbeTargetMeta)
	autoProbeMetaMu.Unlock()
}

// NetQualityProbeMeta describes the background auto-probe task in the admin task catalog.
var NetQualityProbeMeta = task.TaskMeta{
	Type:         TaskTypeNetQualityProbe,
	AsynqTask:    NetQualityProbeTask,
	Name:         "OpenFlare automated network quality probe",
	Description:  "Dispatch rate-limited active probes to managed probe targets using traffic heat, prediction uncertainty, staleness, and non-optimal exploration",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
}

// ProbeHandler executes periodic automated edge active probes.
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
		"Automated probe finished: enabled=%t online_nodes=%d targets=%d dispatched=%d skipped_backoff=%d",
		summary.Enabled,
		summary.OnlineNodes,
		summary.TargetsUsed,
		summary.Dispatched,
		summary.SkippedBackoff,
	)
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{
		Message: msg,
		Detail:  string(detailBytes),
	}, nil
}

// AutoProbeSummary summarizes one automated probe dispatch run.
type AutoProbeSummary struct {
	Enabled        bool `json:"enabled"`
	OnlineNodes    int  `json:"online_nodes"`
	TargetsUsed    int  `json:"targets_used"`
	Dispatched     int  `json:"dispatched"`
	SkippedBackoff int  `json:"skipped_backoff"`
	PacketBudget   int  `json:"packet_budget_used"`
}

type probeStateKey struct {
	nodeID   string
	targetID uint
}

type probeHeatKey struct {
	asn     uint32
	country string
	region  string
}

type probeScheduleCandidate struct {
	node       model.OpenFlareNode
	target     *model.OpenFlareProbeTarget
	score      float64
	packetCost int
}

// RunAutoProbe dispatches budget-bounded active probes against managed of_probe_targets using
// traffic heat, prediction uncertainty, staleness, and non-optimal node exploration.
func RunAutoProbe(ctx context.Context) (*AutoProbeSummary, error) {
	if db.DB(ctx) == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	cfg, cfgErr := repository.GetSystemConfigByKey(ctx, model.ConfigKeyNetIntelAutoProbeEnabled)
	if cfgErr == nil && strings.EqualFold(strings.TrimSpace(cfg.Value), "false") {
		return &AutoProbeSummary{Enabled: false}, nil
	}

	now := time.Now().UTC()
	onlineNodes, err := listOnlineEdgeNodes(ctx, now)
	if err != nil {
		return nil, err
	}
	if len(onlineNodes) == 0 {
		return &AutoProbeSummary{Enabled: true}, nil
	}

	targets, err := repository.ListEnabledOpenFlareProbeTargets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list enabled probe targets: %w", err)
	}
	if len(targets) == 0 {
		return &AutoProbeSummary{
			Enabled:     true,
			OnlineNodes: len(onlineNodes),
		}, nil
	}

	candidates, skippedBackoff := buildProbeScheduleCandidates(ctx, now, onlineNodes, targets)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].node.NodeID != candidates[j].node.NodeID {
			return candidates[i].node.NodeID < candidates[j].node.NodeID
		}
		return candidates[i].target.ID < candidates[j].target.ID
	})

	dispatched, usedTargets, usedPackets := dispatchBudgetedCandidates(ctx, now, candidates)
	return &AutoProbeSummary{
		Enabled:        true,
		OnlineNodes:    len(onlineNodes),
		TargetsUsed:    usedTargets,
		Dispatched:     dispatched,
		SkippedBackoff: skippedBackoff,
		PacketBudget:   usedPackets,
	}, nil
}

func listOnlineEdgeNodes(ctx context.Context, now time.Time) ([]model.OpenFlareNode, error) {
	nodes, err := repository.ListOpenFlareNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	online := make([]model.OpenFlareNode, 0, len(nodes))
	for i := range nodes {
		n := nodes[i]
		if n.IsQuarantined() {
			continue
		}
		if n.NodeType != "" && n.NodeType != "edge_node" {
			continue
		}
		if n.Status == "online" || (n.LastSeenAt != nil && now.Sub(*n.LastSeenAt) <= onlineWindow) {
			online = append(online, n)
		}
	}
	return online, nil
}

func buildProbeScheduleCandidates(
	ctx context.Context,
	now time.Time,
	onlineNodes []model.OpenFlareNode,
	targets []*model.OpenFlareProbeTarget,
) ([]probeScheduleCandidate, int) {
	targetIDs := make([]uint, 0, len(targets))
	for _, t := range targets {
		if t != nil && t.ID > 0 {
			targetIDs = append(targetIDs, t.ID)
		}
	}
	states, _ := repository.ListOpenFlareProbeTargetStates(ctx, "", targetIDs)
	stateMap := make(map[probeStateKey]*model.OpenFlareProbeTargetState, len(states))
	for _, st := range states {
		if st != nil {
			stateMap[probeStateKey{nodeID: st.NodeID, targetID: st.TargetID}] = st
		}
	}

	measured, _ := repository.ListOpenFlareNetworkQualityMatrixMeasured(ctx)
	heatMap := buildTrafficHeatMap(measured)
	predictor, _ := LoadLivePredictor(ctx)
	if predictor == nil {
		predictor = NewPredictor(now, onlineNodes, measured, nil, currentErrorCalibration())
	}

	var candidates []probeScheduleCandidate
	skippedBackoff := 0
	for _, target := range targets {
		if target == nil || !target.Enabled || strings.TrimSpace(target.IP) == "" {
			continue
		}
		optimalNodeID, predsByNode := evaluateTargetNodesPrediction(predictor, onlineNodes, target)
		heat := lookupTargetTrafficHeat(heatMap, target)
		pktCost := probePacketCost(target.ProbeProto)
		for _, node := range onlineNodes {
			st := stateMap[probeStateKey{nodeID: node.NodeID, targetID: target.ID}]
			if isProbeStateBlocked(now, st) {
				skippedBackoff++
				continue
			}
			pred := predsByNode[node.NodeID]
			if isProbeMeasurementFresh(now, st, pred) {
				continue
			}
			score := scoreProbeCandidate(now, heat, node.NodeID == optimalNodeID, st, pred)
			candidates = append(candidates, probeScheduleCandidate{
				node:       node,
				target:     target,
				score:      score,
				packetCost: pktCost,
			})
		}
	}
	return candidates, skippedBackoff
}

func buildTrafficHeatMap(rows []*model.OpenFlareNetworkQualityMatrix) map[probeHeatKey]int {
	heat := make(map[probeHeatKey]int, len(rows))
	for _, r := range rows {
		if r == nil || r.ClientASN == 0 || r.TCPSampleCount <= 0 {
			continue
		}
		country := strings.ToUpper(strings.TrimSpace(r.ClientCountry))
		region := strings.ToUpper(strings.TrimSpace(r.ClientRegion))
		if region != "" {
			heat[probeHeatKey{asn: r.ClientASN, country: country, region: region}] += r.TCPSampleCount
		}
		if country != "" {
			heat[probeHeatKey{asn: r.ClientASN, country: country}] += r.TCPSampleCount
		}
		heat[probeHeatKey{asn: r.ClientASN}] += r.TCPSampleCount
	}
	return heat
}

func lookupTargetTrafficHeat(heat map[probeHeatKey]int, t *model.OpenFlareProbeTarget) float64 {
	if t == nil || t.ASN == 0 {
		return 1.0
	}
	country := strings.ToUpper(strings.TrimSpace(t.Country))
	region := pkgnetintel.NormalizeMatrixRegion(country, t.Region)
	samples := 0
	if region != "" {
		samples = heat[probeHeatKey{asn: t.ASN, country: country, region: region}]
	}
	if samples == 0 && country != "" {
		samples = heat[probeHeatKey{asn: t.ASN, country: country}]
	}
	if samples == 0 {
		samples = heat[probeHeatKey{asn: t.ASN}]
	}
	return 1.0 + math.Log1p(float64(max(0, samples)))
}

func evaluateTargetNodesPrediction(
	predictor *Predictor,
	nodes []model.OpenFlareNode,
	t *model.OpenFlareProbeTarget,
) (string, map[string]PredictionResult) {
	preds := make(map[string]PredictionResult, len(nodes))
	bestNode := ""
	bestScore := math.Inf(-1)
	country := strings.ToUpper(strings.TrimSpace(t.Country))
	region := pkgnetintel.NormalizeMatrixRegion(country, t.Region)
	ipFam := strings.ToLower(strings.TrimSpace(t.IPFamily))
	if ipFam == "" {
		ipFam = pkgnetintel.DetectIPFamily(t.IP)
	}
	for _, n := range nodes {
		res := predictor.Predict(PredictQuery{
			NodeID:        n.NodeID,
			ClientASN:     t.ASN,
			ClientCountry: country,
			ClientRegion:  region,
			IPFamily:      ipFam,
			GeoSource:     pkgnetintel.GeoSourceCountry,
			AnycastStatus: t.AnycastStatus,
		})
		preds[n.NodeID] = res
		qScore := ComputeQualityScore(res)
		if qScore > bestScore {
			bestScore = qScore
			bestNode = n.NodeID
		}
	}
	return bestNode, preds
}

func isProbeStateBlocked(now time.Time, st *model.OpenFlareProbeTargetState) bool {
	if st == nil {
		return false
	}
	if st.NextProbeAt != nil && now.Before(*st.NextProbeAt) {
		return true
	}
	if st.LastProbedAt != nil && now.Sub(*st.LastProbedAt) < autoProbeMinCooldown {
		return true
	}
	return false
}

func isProbeMeasurementFresh(now time.Time, st *model.OpenFlareProbeTargetState, pred PredictionResult) bool {
	if st == nil || st.LastSuccessAt == nil {
		return false
	}
	return now.Sub(*st.LastSuccessAt) < autoProbeFreshSkipAge && pred.Confidence >= autoProbeFreshMinConfidence
}

func scoreProbeCandidate(
	now time.Time,
	heat float64,
	isOptimalNode bool,
	st *model.OpenFlareProbeTargetState,
	pred PredictionResult,
) float64 {
	uncertainty := math.Max(autoProbeMinUncertainty, 1.0-pred.Confidence)
	if pred.CandidateDispersionMs > 0 {
		uncertainty += pred.CandidateDispersionMs / autoProbeDispersionScaleMs
	}
	if pred.ErrorMs != nil && *pred.ErrorMs > 0 {
		uncertainty += *pred.ErrorMs / autoProbeDispersionScaleMs
	}
	uncertainty = math.Min(autoProbeMaxUncertainty, uncertainty)

	staleness := autoProbeNeverProbedStaleFactor
	if st != nil && st.LastSuccessAt != nil {
		hours := math.Max(0, now.Sub(*st.LastSuccessAt).Hours())
		staleness = math.Min(autoProbeNeverProbedStaleFactor, autoProbeStaleBaseFactor+hours/autoProbeStaleHalfHours)
	}

	explore := 1.0
	if !isOptimalNode && (st == nil || st.LastSuccessAt == nil || now.Sub(*st.LastSuccessAt) >= autoProbeExploreStaleAge) &&
		pred.Confidence < autoProbeOptimalConfFloor {
		explore = autoProbeExploreBonus
	}
	return heat * uncertainty * staleness * explore
}

func probePacketCost(proto string) int {
	switch strings.ToLower(strings.TrimSpace(proto)) {
	case model.ProbeProtoICMP, model.ProbeProtoTCP:
		return autoProbePingPacketCost
	default:
		return autoProbeTraceroutePacketCost
	}
}

func dispatchBudgetedCandidates(
	ctx context.Context,
	now time.Time,
	candidates []probeScheduleCandidate,
) (dispatched, usedTargets, usedPackets int) {
	nodeDispatches := make(map[string]int)
	nodePackets := make(map[string]int)
	targetDispatches := make(map[uint]int)
	seenTargets := make(map[uint]struct{})

	for _, c := range candidates {
		if dispatched >= autoProbeGlobalDispatchBudget {
			break
		}
		if nodeDispatches[c.node.NodeID] >= autoProbeMaxTargetsPerNode {
			continue
		}
		if nodePackets[c.node.NodeID]+c.packetCost > autoProbeMaxPacketCostPerNode {
			continue
		}
		if targetDispatches[c.target.ID] >= autoProbeMaxNodesPerTarget {
			continue
		}

		probeID := autoProbeIDPrefix + uuid.NewString()
		dispatch := buildAutoProbeDispatch(probeID, c.target)
		recordAutoProbeMeta(probeID, autoProbeTargetMeta{
			targetID:      c.target.ID,
			anycastStatus: strings.ToLower(strings.TrimSpace(c.target.AnycastStatus)),
			port:          c.target.Port,
		})
		if !dispatchNetProbeFn(ctx, c.node.NodeID, dispatch) {
			consumeAutoProbeMeta(probeID)
			continue
		}
		if err := repository.MarkOpenFlareProbeDispatched(ctx, c.node.NodeID, c.target.ID, now, now.Add(autoProbeMinCooldown)); err != nil {
			logger.WarnF(ctx, "[NetQuality] failed to mark probe dispatch state (node=%s target=%d): %v", c.node.NodeID, c.target.ID, err)
		}

		dispatched++
		usedPackets += c.packetCost
		nodeDispatches[c.node.NodeID]++
		nodePackets[c.node.NodeID] += c.packetCost
		targetDispatches[c.target.ID]++
		seenTargets[c.target.ID] = struct{}{}
	}
	return dispatched, len(seenTargets), usedPackets
}

func buildAutoProbeDispatch(probeID string, target *model.OpenFlareProbeTarget) protocol.NetProbeTaskDispatch {
	timeoutMs := autoProbeTimeoutMs
	proto := strings.ToLower(strings.TrimSpace(target.ProbeProto))
	var portPtr *uint16
	if target.Port > 0 && target.Port <= int(math.MaxUint16) {
		p := uint16(target.Port) //nolint:gosec // validated 1..65535
		portPtr = &p
	}

	if proto == model.ProbeProtoICMP || proto == model.ProbeProtoTCP {
		intervalMs := autoProbePingIntervalMs
		return protocol.NetProbeTaskDispatch{
			ProbeID: probeID,
			Kind:    "ping_batch",
			Ping: &protocol.PingBatchRequest{
				Protocol:   proto,
				Target:     target.IP,
				Port:       portPtr,
				Count:      autoProbePingCount,
				IntervalMs: intervalMs,
				TimeoutMs:  &timeoutMs,
			},
		}
	}

	traceProto := model.ProbeProtoICMP
	if portPtr != nil {
		traceProto = model.ProbeProtoTCP
	}
	return protocol.NetProbeTaskDispatch{
		ProbeID: probeID,
		Kind:    "traceroute",
		Traceroute: &protocol.TracerouteRequest{
			Target:       target.IP,
			Protocol:     traceProto,
			Port:         portPtr,
			MaxHops:      autoProbeMaxHops,
			ProbesPerHop: autoProbeProbesPerHop,
			TimeoutMs:    &timeoutMs,
		},
	}
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
		} else {
			consumeAutoProbeMeta(strings.TrimSpace(payload.ProbeID))
		}
	case "ping_batch":
		if payload.Ping != nil {
			ingestPingBatchResult(ctx, trimmedNode, payload.ProbeID, payload.Ping)
		} else {
			consumeAutoProbeMeta(strings.TrimSpace(payload.ProbeID))
		}
	}
}

func resolveEffectiveAnycastStatus(ctx context.Context, lookupIP, mmdbAnycast string, metaAnycast string) string {
	if strings.EqualFold(metaAnycast, pkgnetintel.AnycastStatusConfirmed) ||
		strings.EqualFold(mmdbAnycast, pkgnetintel.AnycastStatusConfirmed) {
		return pkgnetintel.AnycastStatusConfirmed
	}
	if lookupIP != "" {
		if target, err := repository.GetOpenFlareProbeTargetByEndpoint(ctx, lookupIP, model.ProbeProtoICMP, 0); err == nil &&
			target != nil && strings.EqualFold(target.AnycastStatus, pkgnetintel.AnycastStatusConfirmed) {
			return pkgnetintel.AnycastStatusConfirmed
		}
	}
	if mmdbAnycast != "" {
		return mmdbAnycast
	}
	return metaAnycast
}

func extractTracerouteASPathAndEdges(hops []EnrichedTracerouteHop, seenAt time.Time) ([]uint32, []repository.ObservedASEdge) {
	asPath := make([]uint32, 0, len(hops))
	var edges []repository.ObservedASEdge
	var prevASN uint32
	var prevRTT float64
	for _, hop := range hops {
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
			edges = append(edges, repository.ObservedASEdge{
				SrcASN:   prevASN,
				DstASN:   hop.ASN,
				EdgeType: "observed",
				HopRTTMs: round2(math.Max(0, hopRTT-prevRTT)),
				SeenAt:   seenAt,
			})
		}
		prevASN = hop.ASN
		prevRTT = hopRTT
	}
	return asPath, edges
}

func ingestTracerouteResult(ctx context.Context, nodeID, probeID string, tr *protocol.TracerouteResult) {
	cleanProbeID := strings.TrimSpace(probeID)
	meta, _ := consumeAutoProbeMeta(cleanProbeID)

	enrichedHops := EnrichTracerouteHops(tr.Hops)
	seenAt := time.Now().UTC()
	asPath, edges := extractTracerouteASPathAndEdges(enrichedHops, seenAt)

	lookupIP := strings.TrimSpace(tr.ResolvedIP)
	if lookupIP == "" {
		lookupIP = strings.TrimSpace(tr.Target)
	}
	ipFamily := pkgnetintel.DetectIPFamily(lookupIP)
	var geoRes *pkgnetintel.IPLookupResult
	if lookupIP != "" {
		geoRes = netintel.LookupIP(lookupIP)
	}
	targetMeta := extractTracerouteTargetGeo(geoRes)
	effectiveAnycast := resolveEffectiveAnycastStatus(ctx, lookupIP, targetMeta.anycastStatus, meta.anycastStatus)

	if len(edges) > 0 && effectiveAnycast != pkgnetintel.AnycastStatusConfirmed {
		if err := repository.UpsertOpenFlareASTopologyEdges(ctx, edges); err != nil {
			logger.WarnF(ctx, "[NetQuality] upsert AS topology edges failed (node=%s): %v", nodeID, err)
		}
	}

	persistTracerouteRecordAndOutcome(ctx, nodeID, cleanProbeID, meta, tr, lookupIP, ipFamily, targetMeta, enrichedHops, asPath, seenAt)

	trSuccess := tr.Reached && tr.TotalRTTMs > 0
	if targetMeta.asn > 0 && trSuccess && effectiveAnycast != pkgnetintel.AnycastStatusConfirmed {
		persistTracerouteQualitySample(ctx, nodeID, tr, lookupIP, ipFamily, targetMeta, seenAt)
	}
}

type tracerouteTargetGeo struct {
	asn           uint32
	country       string
	region        string
	lat           *float64
	lon           *float64
	accuracyKm    float64
	geoSource     string
	anycastStatus string
}

func extractTracerouteTargetGeo(res *pkgnetintel.IPLookupResult) tracerouteTargetGeo {
	if res == nil {
		return tracerouteTargetGeo{}
	}
	return tracerouteTargetGeo{
		asn:           res.ASN,
		country:       res.CountryISO,
		region:        res.RegionCode,
		lat:           res.Latitude,
		lon:           res.Longitude,
		accuracyKm:    res.AccuracyRadiusKm,
		geoSource:     res.GeoSource,
		anycastStatus: res.AnycastStatus,
	}
}

func persistTracerouteRecordAndOutcome(
	ctx context.Context,
	nodeID string,
	cleanProbeID string,
	meta autoProbeTargetMeta,
	tr *protocol.TracerouteResult,
	lookupIP string,
	ipFamily string,
	targetMeta tracerouteTargetGeo,
	enrichedHops []EnrichedTracerouteHop,
	asPath []uint32,
	seenAt time.Time,
) {
	triggerType := model.TracerouteTriggerManual
	if strings.HasPrefix(cleanProbeID, autoProbeIDPrefix) {
		triggerType = model.TracerouteTriggerAuto
	}
	hopsJSON, _ := json.Marshal(enrichedHops)
	record := &model.OpenFlareTracerouteRecord{
		TaskID:        cleanProbeID,
		TriggerType:   triggerType,
		NodeID:        nodeID,
		Target:        strings.TrimSpace(tr.Target),
		ResolvedIP:    strings.TrimSpace(tr.ResolvedIP),
		TargetASN:     targetMeta.asn,
		TargetCountry: targetMeta.country,
		TargetRegion:  targetMeta.region,
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

	trSuccess := tr.Reached && tr.TotalRTTMs > 0
	errMsg := strings.TrimSpace(tr.Error)
	if !trSuccess && errMsg == "" {
		errMsg = "traceroute target unreachable or timed out"
	}
	if err := repository.RecordOpenFlareProbeTargetOutcome(
		ctx, meta.targetID, nodeID, lookupIP, tr.Protocol, meta.port,
		trSuccess, round2(tr.TotalRTTMs), errMsg, seenAt, autoProbeBackoffBase, autoProbeBackoffMax,
	); err != nil {
		logger.WarnF(ctx, "[NetQuality] update probe target state failed (node=%s): %v", nodeID, err)
	}
}

func persistTracerouteQualitySample(
	ctx context.Context,
	nodeID string,
	tr *protocol.TracerouteResult,
	lookupIP string,
	ipFamily string,
	targetMeta tracerouteTargetGeo,
	seenAt time.Time,
) {
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
		ClientASN:           targetMeta.asn,
		ClientCountry:       targetMeta.country,
		ClientRegion:        targetMeta.region,
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
		ClientLat:           targetMeta.lat,
		ClientLon:           targetMeta.lon,
		GeoAccuracyKm:       targetMeta.accuracyKm,
		GeoSource:           targetMeta.geoSource,
		LossRate:            round4(lossRate),
		HopCount:            int(tr.HopCount),
		CapturedAt:          seenAt,
	}
	if err := repository.InsertOpenFlareNetworkQualitySample(ctx, sample); err != nil {
		logger.WarnF(ctx, "[NetQuality] insert traceroute sample failed (node=%s): %v", nodeID, err)
	}
}

func ingestPingBatchResult(ctx context.Context, nodeID, probeID string, ping *protocol.PingBatchResult) {
	meta, _ := consumeAutoProbeMeta(strings.TrimSpace(probeID))
	lookupIP := strings.TrimSpace(ping.ResolvedIP)
	if lookupIP == "" {
		lookupIP = strings.TrimSpace(ping.Target)
	}
	now := time.Now().UTC()
	pingSuccess := ping.Received > 0 && ping.AvgMs > 0
	errMsg := strings.TrimSpace(ping.Error)
	if !pingSuccess && errMsg == "" {
		errMsg = "all ping probes timed out or failed"
	}
	if err := repository.RecordOpenFlareProbeTargetOutcome(
		ctx, meta.targetID, nodeID, lookupIP, ping.Protocol, meta.port,
		pingSuccess, round2(ping.AvgMs), errMsg, now, autoProbeBackoffBase, autoProbeBackoffMax,
	); err != nil {
		logger.WarnF(ctx, "[NetQuality] update ping probe target state failed (node=%s): %v", nodeID, err)
	}
	if !pingSuccess {
		// Never persist 0ms or unreachable outcomes into quality samples or matrix rows.
		return
	}

	res := netintel.LookupIP(lookupIP)
	if res == nil || res.ASN == 0 {
		return
	}
	if resolveEffectiveAnycastStatus(ctx, lookupIP, res.AnycastStatus, meta.anycastStatus) == pkgnetintel.AnycastStatusConfirmed {
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
		CapturedAt:          now,
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
