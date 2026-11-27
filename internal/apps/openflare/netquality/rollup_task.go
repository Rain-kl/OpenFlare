// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netquality

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	// NetQualityRollupTask is the Asynq task name for blending quality samples into the matrix.
	NetQualityRollupTask = "openflare:netquality_rollup"
	// TaskTypeNetQualityRollup is the admin task type for quality matrix rollup.
	TaskTypeNetQualityRollup = "of_netquality_rollup"

	rollupLookbackWindow    = 24 * time.Hour
	rollupStaleRetention    = 7 * 24 * time.Hour
	rollupHalfLifeHours     = 6.0
	maxRollupSamples        = 20000
	maxRollupTraces         = 5000
	maxObservedEdges        = 10000
	defaultPriorRTTMs       = 120.0
	onlineWindow            = 2 * time.Minute
	minSampleConfidence     = 0.1
	maxSampleConfidence     = 0.99
	confidenceHalfSamples   = 20.0
	maxLossPenaltyRatio     = 0.85
	tcpRetransPenaltyFactor = 1.5
	traceLossPenaltyFactor  = 2.0
	qualityScoreRTTScale    = 80.0
	minQualityScore         = 1.0
	maxQualityScore         = 100.0
)

// NetQualityRollupMeta describes the quality matrix aggregation task in the admin task catalog.
var NetQualityRollupMeta = task.TaskMeta{
	Type:         TaskTypeNetQualityRollup,
	AsynqTask:    NetQualityRollupTask,
	Name:         "OpenFlare network quality matrix rollup",
	Description:  "Blend passive TCP RTT, active traceroute/ping measurements, and AS topology paths into of_network_quality_matrix",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
}

// RollupHandler executes scheduled or manual network quality matrix rollups.
type RollupHandler struct{}

// Execute runs network quality matrix aggregation.
func (h *RollupHandler) Execute(ctx context.Context, _ []byte) (*task.TaskResult, error) {
	task.AppendLog(ctx, "Starting network quality matrix rollup")
	summary, err := RunRollup(ctx)
	if err != nil {
		task.AppendLog(ctx, "Network quality matrix rollup failed: %v", err)
		return nil, err
	}
	detailBytes, _ := json.Marshal(summary)
	msg := fmt.Sprintf(
		"Network quality rollup finished: samples=%d measured_rows=%d predicted_rows=%d pruned=%d (%dms)",
		summary.SamplesRead,
		summary.MeasuredRows,
		summary.PredictedRows,
		summary.PrunedRows,
		summary.DurationMs,
	)
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{
		Message: msg,
		Detail:  string(detailBytes),
	}, nil
}

// RollupSummary reports counts from one matrix rollup cycle.
type RollupSummary struct {
	SamplesRead   int   `json:"samples_read"`
	MeasuredRows  int   `json:"measured_rows"`
	PredictedRows int   `json:"predicted_rows"`
	PrunedRows    int64 `json:"pruned_rows"`
	DurationMs    int64 `json:"duration_ms"`
}

type matrixPairKey struct {
	nodeID    string
	clientASN uint32
}

type pairAccum struct {
	nodeID        string
	nodeASN       uint32
	clientASN     uint32
	clientCountry string

	tcpWeightSum  float64
	tcpP50Sum     float64
	tcpP95Sum     float64
	tcpMinRTTMs   float64
	tcpRetransSum float64
	tcpSamples    int
	tcpLastSeenAt *time.Time

	traceWeightSum  float64
	traceP50Sum     float64
	traceLossSum    float64
	traceHopSum     float64
	traceSamples    int
	traceLastSeenAt *time.Time
	traceASPathJSON string
}

type rollupInputs struct {
	samples     []analyticsmodel.NetworkQualitySample
	nodeByID    map[string]model.OpenFlareNode
	edgeNodes   []model.OpenFlareNode
	observedAdj map[uint32][]uint32
	tracePaths  map[matrixPairKey]string
}

func loadRollupInputs(ctx context.Context, now time.Time) (*rollupInputs, error) {
	since := now.Add(-rollupLookbackWindow)
	samples, err := repository.ListOpenFlareNetworkQualitySamplesSince(ctx, since, maxRollupSamples)
	if err != nil {
		return nil, fmt.Errorf("list quality samples: %w", err)
	}
	traces, err := repository.ListRecentOpenFlareTracerouteRecordsSince(ctx, since, maxRollupTraces)
	if err != nil {
		logger.WarnF(ctx, "[NetQuality] list recent traceroute records warning: %v", err)
		traces = nil
	}
	nodes, err := repository.ListOpenFlareNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	edges, err := repository.ListOpenFlareASTopologyEdges(ctx, maxObservedEdges)
	if err != nil {
		logger.WarnF(ctx, "[NetQuality] list AS topology edges warning: %v", err)
		edges = nil
	}

	nodeByID := make(map[string]model.OpenFlareNode, len(nodes))
	edgeNodes := make([]model.OpenFlareNode, 0, len(nodes))
	for i := range nodes {
		n := nodes[i]
		if n.ASN == 0 && strings.TrimSpace(n.IP) != "" {
			netintel.EnrichNodeASN(&n)
		}
		nodeByID[n.NodeID] = n
		if n.NodeType == "" || n.NodeType == "edge_node" {
			edgeNodes = append(edgeNodes, n)
		}
	}

	return &rollupInputs{
		samples:     samples,
		nodeByID:    nodeByID,
		edgeNodes:   edgeNodes,
		observedAdj: buildObservedAdjacency(edges),
		tracePaths:  latestTracePaths(traces),
	}, nil
}

func accumulateRollupPairs(inputs *rollupInputs, now time.Time) (map[matrixPairKey]*pairAccum, map[uint32]string) {
	pairs := make(map[matrixPairKey]*pairAccum)
	activeASNs := make(map[uint32]string)

	for i := range inputs.samples {
		s := inputs.samples[i]
		nodeID := strings.TrimSpace(s.NodeID)
		if nodeID == "" || s.ClientASN == 0 {
			continue
		}
		recordActiveASNCountry(activeASNs, s.ClientASN, s.ClientCountry)

		capturedAt := resolveSampleTime(s.CapturedAt, s.CreatedAt, now)
		ageHours := max(0.0, now.Sub(capturedAt).Hours())
		decay := math.Exp(-math.Ln2 * ageHours / rollupHalfLifeHours)
		sampleWeight := decay * math.Max(1.0, math.Log1p(float64(max(1, int(s.SampleCount)))))

		key := matrixPairKey{nodeID: nodeID, clientASN: s.ClientASN}
		acc := ensurePairAccum(pairs, key, inputs, s.NodeASN, s.ClientCountry)
		applySampleToPair(acc, &s, sampleWeight, capturedAt)
	}
	return pairs, activeASNs
}

func recordActiveASNCountry(activeASNs map[uint32]string, asn uint32, rawCountry string) {
	country := strings.ToUpper(strings.TrimSpace(rawCountry))
	if existing, exists := activeASNs[asn]; !exists || (existing == "" && country != "") {
		activeASNs[asn] = country
	}
}

func resolveSampleTime(captured, created, fallback time.Time) time.Time {
	if !captured.IsZero() {
		return captured
	}
	if !created.IsZero() {
		return created
	}
	return fallback
}

func ensurePairAccum(
	pairs map[matrixPairKey]*pairAccum,
	key matrixPairKey,
	inputs *rollupInputs,
	sampleNodeASN uint32,
	clientCountry string,
) *pairAccum {
	acc := pairs[key]
	if acc == nil {
		nodeASN := sampleNodeASN
		if n, ok := inputs.nodeByID[key.nodeID]; ok && n.ASN > 0 {
			nodeASN = n.ASN
		}
		acc = &pairAccum{
			nodeID:          key.nodeID,
			nodeASN:         nodeASN,
			clientASN:       key.clientASN,
			clientCountry:   strings.ToUpper(strings.TrimSpace(clientCountry)),
			traceASPathJSON: inputs.tracePaths[key],
		}
		pairs[key] = acc
	}
	if acc.clientCountry == "" && clientCountry != "" {
		acc.clientCountry = strings.ToUpper(strings.TrimSpace(clientCountry))
	}
	return acc
}

func applySampleToPair(acc *pairAccum, s *analyticsmodel.NetworkQualitySample, weight float64, capturedAt time.Time) {
	t := capturedAt
	if s.Source == model.NetworkQualitySourceTCPPassive {
		acc.tcpWeightSum += weight
		acc.tcpP50Sum += s.RTTP50Ms * weight
		acc.tcpP95Sum += s.RTTP95Ms * weight
		acc.tcpRetransSum += s.RetransRate * weight
		acc.tcpSamples += max(1, int(s.SampleCount))
		if s.RTTMinMs > 0 && (acc.tcpMinRTTMs == 0 || s.RTTMinMs < acc.tcpMinRTTMs) {
			acc.tcpMinRTTMs = s.RTTMinMs
		}
		if acc.tcpLastSeenAt == nil || t.After(*acc.tcpLastSeenAt) {
			acc.tcpLastSeenAt = &t
		}
		return
	}
	acc.traceWeightSum += weight
	acc.traceP50Sum += s.RTTP50Ms * weight
	acc.traceLossSum += s.LossRate * weight
	acc.traceHopSum += float64(s.HopCount) * weight
	acc.traceSamples += max(1, int(s.SampleCount))
	if acc.traceLastSeenAt == nil || t.After(*acc.traceLastSeenAt) {
		acc.traceLastSeenAt = &t
	}
}

// RunRollup blends passive and active quality observations from the last 24h into of_network_quality_matrix.
func RunRollup(ctx context.Context) (*RollupSummary, error) {
	started := time.Now()
	if db.DB(ctx) == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if err := FlushNow(ctx); err != nil {
		logger.WarnF(ctx, "[NetQuality] pre-rollup passive flush warning: %v", err)
	}

	now := time.Now().UTC()
	inputs, err := loadRollupInputs(ctx, now)
	if err != nil {
		return nil, err
	}
	pairs, activeASNs := accumulateRollupPairs(inputs, now)

	rows := make([]*model.OpenFlareNetworkQualityMatrix, 0, len(pairs)+len(activeASNs)*len(inputs.edgeNodes))
	measuredCount := 0
	for key, acc := range pairs {
		rows = append(rows, finalizeMeasuredRow(acc, inputs.nodeByID[key.nodeID], inputs.observedAdj))
		measuredCount++
	}

	predictedCount := 0
	for _, n := range inputs.edgeNodes {
		if n.IsQuarantined() {
			continue
		}
		if n.Status != "online" && (n.LastSeenAt == nil || now.Sub(*n.LastSeenAt) > rollupLookbackWindow) {
			continue
		}
		for clientASN, country := range activeASNs {
			if _, exists := pairs[matrixPairKey{nodeID: n.NodeID, clientASN: clientASN}]; exists {
				continue
			}
			rows = append(rows, buildPredictedRow(n, clientASN, country, inputs.observedAdj))
			predictedCount++
		}
	}

	if err := repository.UpsertOpenFlareNetworkQualityMatrixBatch(ctx, rows); err != nil {
		return nil, fmt.Errorf("upsert network quality matrix: %w", err)
	}
	pruned, pruneErr := repository.DeleteStaleOpenFlareNetworkQualityMatrixBefore(ctx, now.Add(-rollupStaleRetention))
	if pruneErr != nil {
		logger.WarnF(ctx, "[NetQuality] prune stale matrix rows failed: %v", pruneErr)
	}

	return &RollupSummary{
		SamplesRead:   len(inputs.samples),
		MeasuredRows:  measuredCount,
		PredictedRows: predictedCount,
		PrunedRows:    pruned,
		DurationMs:    time.Since(started).Milliseconds(),
	}, nil
}

func finalizeMeasuredRow(
	acc *pairAccum,
	node model.OpenFlareNode,
	observedAdj map[uint32][]uint32,
) *model.OpenFlareNetworkQualityMatrix {
	var tcpP50, tcpP95, tcpRetrans float64
	if acc.tcpWeightSum > 0 {
		tcpP50 = acc.tcpP50Sum / acc.tcpWeightSum
		tcpP95 = acc.tcpP95Sum / acc.tcpWeightSum
		tcpRetrans = acc.tcpRetransSum / acc.tcpWeightSum
	}
	var traceP50, traceLoss float64
	var traceHops int
	if acc.traceWeightSum > 0 {
		traceP50 = acc.traceP50Sum / acc.traceWeightSum
		traceLoss = acc.traceLossSum / acc.traceWeightSum
		traceHops = int(math.Round(acc.traceHopSum / acc.traceWeightSum))
	}

	asnInfo := netintel.LookupASN(acc.clientASN)
	asnName := ""
	country := acc.clientCountry
	if asnInfo != nil {
		asnName = asnInfo.Name
		if country == "" {
			country = asnInfo.Country
		}
	}

	nodeASN := acc.nodeASN
	if nodeASN == 0 {
		nodeASN = node.ASN
	}
	predictedRTT, estHops, estPath := estimatePriorRTT(node, nodeASN, acc.clientASN, observedAdj)
	if traceHops == 0 && estHops > 0 {
		traceHops = estHops
	}

	asPathJSON := acc.traceASPathJSON
	if strings.TrimSpace(asPathJSON) == "" {
		asPathJSON = encodeUint32SliceJSON(estPath)
	}

	// Blend passive TCP RTT (weight 1.0) and active traceroute RTT (weight 0.7).
	var blendedRTT, weightSum float64
	if acc.tcpWeightSum > 0 && tcpP50 > 0 {
		blendedRTT += tcpP50 * 1.0
		weightSum += 1.0
	}
	if acc.traceWeightSum > 0 && traceP50 > 0 {
		blendedRTT += traceP50 * 0.7
		weightSum += 0.7
	}
	var effectiveRTT float64
	if weightSum > 0 {
		effectiveRTT = blendedRTT / weightSum
	} else {
		effectiveRTT = predictedRTT
	}

	// Jitter spread + packet loss / retransmission penalty.
	if tcpP95 > tcpP50 && tcpP50 > 0 {
		effectiveRTT += 0.35 * (tcpP95 - tcpP50)
	}
	effectiveRTT += tcpRetrans*150.0 + traceLoss*300.0

	totalSamples := acc.tcpSamples + acc.traceSamples
	confidence := math.Max(minSampleConfidence, math.Min(maxSampleConfidence, 1.0-math.Exp(-float64(totalSamples)/confidenceHalfSamples)))
	lossPenalty := math.Min(maxLossPenaltyRatio, tcpRetrans*tcpRetransPenaltyFactor+traceLoss*traceLossPenaltyFactor)
	qualityScore := math.Max(minQualityScore, (maxQualityScore/(1.0+effectiveRTT/qualityScoreRTTScale))*(1.0-lossPenalty))

	return &model.OpenFlareNetworkQualityMatrix{
		NodeID:           acc.nodeID,
		NodeASN:          nodeASN,
		ClientASN:        acc.clientASN,
		ClientCountry:    country,
		ClientASNName:    asnName,
		TCPRTTP50Ms:      round2(tcpP50),
		TCPRTTP95Ms:      round2(tcpP95),
		TCPMinRTTMs:      round2(acc.tcpMinRTTMs),
		TCPRetransRate:   round4(tcpRetrans),
		TCPSampleCount:   acc.tcpSamples,
		TCPLastSeenAt:    acc.tcpLastSeenAt,
		TraceRTTP50Ms:    round2(traceP50),
		TraceLossRate:    round4(traceLoss),
		TraceHopCount:    traceHops,
		TraceASPath:      asPathJSON,
		TraceSampleCount: acc.traceSamples,
		TraceLastSeenAt:  acc.traceLastSeenAt,
		EffectiveRTTMs:   round2(effectiveRTT),
		PredictedRTTMs:   round2(predictedRTT),
		QualityScore:     round2(qualityScore),
		Confidence:       round4(confidence),
		DataSource:       model.QualityMatrixDataSourceMeasured,
	}
}

func buildPredictedRow(
	node model.OpenFlareNode,
	clientASN uint32,
	country string,
	observedAdj map[uint32][]uint32,
) *model.OpenFlareNetworkQualityMatrix {
	asnInfo := netintel.LookupASN(clientASN)
	asnName := ""
	if asnInfo != nil {
		asnName = asnInfo.Name
		if country == "" {
			country = asnInfo.Country
		}
	}
	predictedRTT, estHops, estPath := estimatePriorRTT(node, node.ASN, clientASN, observedAdj)
	confidence := 0.25
	if len(estPath) > 0 {
		confidence = 0.40
	}
	qualityScore := math.Max(minQualityScore, maxQualityScore/(1.0+predictedRTT/qualityScoreRTTScale))

	return &model.OpenFlareNetworkQualityMatrix{
		NodeID:         node.NodeID,
		NodeASN:        node.ASN,
		ClientASN:      clientASN,
		ClientCountry:  country,
		ClientASNName:  asnName,
		TraceHopCount:  estHops,
		TraceASPath:    encodeUint32SliceJSON(estPath),
		EffectiveRTTMs: round2(predictedRTT),
		PredictedRTTMs: round2(predictedRTT),
		QualityScore:   round2(qualityScore),
		Confidence:     round4(confidence),
		DataSource:     model.QualityMatrixDataSourcePredicted,
	}
}

func estimatePriorRTT(
	node model.OpenFlareNode,
	nodeASN uint32,
	clientASN uint32,
	observedAdj map[uint32][]uint32,
) (float64, int, []uint32) {
	if nodeASN > 0 && clientASN > 0 {
		if hops, path, ok := netintel.EstimateASPath(nodeASN, clientASN, observedAdj); ok {
			prior := float64(hops)*14.0 + 12.0
			return prior, hops, path
		}
	}
	// Fallback when AS path cannot be inferred: if node is in same country as ASN, use local prior.
	_ = node
	return defaultPriorRTTMs, 0, nil
}

func buildObservedAdjacency(edges []*model.OpenFlareASTopologyEdge) map[uint32][]uint32 {
	adj := make(map[uint32][]uint32)
	for _, e := range edges {
		if e == nil || e.SrcASN == 0 || e.DstASN == 0 || e.SrcASN == e.DstASN {
			continue
		}
		adj[e.SrcASN] = append(adj[e.SrcASN], e.DstASN)
		adj[e.DstASN] = append(adj[e.DstASN], e.SrcASN)
	}
	return adj
}

func latestTracePaths(traces []*model.OpenFlareTracerouteRecord) map[matrixPairKey]string {
	out := make(map[matrixPairKey]string)
	for _, tr := range traces {
		if tr == nil || tr.NodeID == "" || tr.TargetASN == 0 {
			continue
		}
		key := matrixPairKey{nodeID: tr.NodeID, clientASN: tr.TargetASN}
		if _, exists := out[key]; !exists && strings.TrimSpace(tr.ASPathJSON) != "" && tr.ASPathJSON != "[]" {
			out[key] = tr.ASPathJSON
		}
	}
	return out
}

func encodeUint32SliceJSON(values []uint32) string {
	if len(values) == 0 {
		return "[]"
	}
	b, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(b)
}
