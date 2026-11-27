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
	"sync/atomic"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
)

const (
	// NetQualityRollupTask is the Asynq task name for blending quality samples into the matrix.
	NetQualityRollupTask = "openflare:netquality_rollup"
	// TaskTypeNetQualityRollup is the admin task type for quality matrix rollup.
	TaskTypeNetQualityRollup = "of_netquality_rollup"

	rollupLookbackWindow    = 24 * time.Hour
	rollupStaleRetention    = 7 * 24 * time.Hour
	rollupHalfLifeHours     = 6.0
	ewmaHalfLifeHours       = 2.0
	ewmaZeroDtAlpha         = 0.25
	maxRollupSamples        = 20000
	maxRollupTraces         = 5000
	onlineWindow            = 2 * time.Minute
	minSampleConfidence     = 0.1
	maxSampleConfidence     = 0.99
	legacyMaxConfidence     = 0.45
	confidenceHalfSamples   = 20.0
	maxLossPenaltyRatio     = 0.85
	tcpRetransPenaltyFactor = 1.5
	probeLossPenaltyFactor  = 2.0
	qualityScoreRTTScale    = 80.0
	minQualityScore         = 1.0
	maxQualityScore         = 100.0
	defaultCityAccuracyKm   = 25.0
	prefixConfidenceBase    = 0.55
	prefixConfidenceSpan    = 0.45
	prefixConfidenceHalf    = 4.0
	probeProtoTCP           = "tcp"
	probeProtoICMP          = "icmp"
)

var rollupGeneration atomic.Uint64

// CurrentRollupGeneration returns the monotonically increasing Rollup snapshot generation counter.
func CurrentRollupGeneration() uint64 {
	return rollupGeneration.Load()
}

// NetQualityRollupMeta describes the quality matrix aggregation task in the admin task catalog.
var NetQualityRollupMeta = task.TaskMeta{
	Type:         TaskTypeNetQualityRollup,
	AsynqTask:    NetQualityRollupTask,
	Name:         "OpenFlare network quality matrix rollup",
	Description:  "Aggregate passive TCP RTT and active probes into two-level (country/region) measured rows in of_network_quality_matrix",
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
		"Network quality rollup finished: samples=%d measured_rows=%d pruned=%d (%dms)",
		summary.SamplesRead,
		summary.MeasuredRows,
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

type matrixGroupKey struct {
	nodeID           string
	clientASN        uint32
	clientCountry    string
	clientRegion     string
	ipFamily         string
	aggregationLevel string
}

type tracePathKey struct {
	nodeID        string
	targetASN     uint32
	targetCountry string
	targetRegion  string
	ipFamily      string
}

type ewmaObservation struct {
	capturedAt time.Time
	rttMs      float64
	source     string
}

type pairAccum struct {
	nodeID           string
	nodeASN          uint32
	clientASN        uint32
	clientCountry    string
	clientRegion     string
	ipFamily         string
	aggregationLevel string

	hasV2Hist    bool
	prefixSketch PrefixSketch
	legacyMaxPfx int

	centroid     pkgnetintel.SphericalCentroid
	geoAccSum    float64
	geoAccWeight float64
	geoSource    string

	// Passive TCP (v2 histogram + v1 fallback)
	tcpHist         WeightedLogHistogram64
	tcpWeightSum    float64
	tcpMinRTTMs     float64
	tcpRetransSum   float64
	tcpSamples      int
	tcpLastSeenAt   *time.Time
	tcpLegacyWeight float64
	tcpLegacyP50Sum float64
	tcpLegacyP95Sum float64

	// Active TCP
	tcpActiveHist       WeightedLogHistogram64
	tcpActiveWeightSum  float64
	tcpActiveLossSum    float64
	tcpActiveSamples    int
	tcpActiveLastSeenAt *time.Time
	tcpActiveLegWeight  float64
	tcpActiveLegP50Sum  float64

	// Active ICMP
	icmpActiveHist       WeightedLogHistogram64
	icmpActiveWeightSum  float64
	icmpActiveLossSum    float64
	icmpActiveSamples    int
	icmpActiveLastSeenAt *time.Time
	icmpActiveLegWeight  float64
	icmpActiveLegP50Sum  float64

	// Active Traceroute
	traceHist       WeightedLogHistogram64
	traceWeightSum  float64
	traceLossSum    float64
	traceHopSum     float64
	traceSamples    int
	traceLastSeenAt *time.Time
	traceASPathJSON string
	traceLegWeight  float64
	traceLegP50Sum  float64

	ewmaPoints []ewmaObservation
}

type rollupInputs struct {
	samples    []analyticsmodel.NetworkQualitySample
	nodeByID   map[string]model.OpenFlareNode
	tracePaths map[tracePathKey]string
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

	nodeByID := make(map[string]model.OpenFlareNode, len(nodes))
	for i := range nodes {
		n := nodes[i]
		if n.ASN == 0 && strings.TrimSpace(n.IP) != "" {
			netintel.EnrichNodeASN(&n)
		}
		nodeByID[n.NodeID] = n
	}

	return &rollupInputs{
		samples:    samples,
		nodeByID:   nodeByID,
		tracePaths: latestTracePaths(traces),
	}, nil
}

func accumulateRollupGroups(inputs *rollupInputs, now time.Time) map[matrixGroupKey]*pairAccum {
	groups := make(map[matrixGroupKey]*pairAccum)

	for i := range inputs.samples {
		s := inputs.samples[i]
		nodeID := strings.TrimSpace(s.NodeID)
		if nodeID == "" || s.ClientASN == 0 {
			continue
		}

		country := strings.ToUpper(strings.TrimSpace(s.ClientCountry))
		if country == "" {
			if info := netintel.LookupASN(s.ClientASN); info != nil && info.Country != "" {
				country = strings.ToUpper(strings.TrimSpace(info.Country))
			}
		}
		rawRegion := pkgnetintel.FormatSubdivisionCode(country, s.ClientRegion)
		ipFamily := strings.ToLower(strings.TrimSpace(s.IPFamily))
		if ipFamily == "" {
			ipFamily = pkgnetintel.IPFamilyUnknown
		}

		capturedAt := resolveSampleTime(s.CapturedAt, s.CreatedAt, now)
		ageHours := max(0.0, now.Sub(capturedAt).Hours())
		decay := math.Exp(-math.Ln2 * ageHours / rollupHalfLifeHours)

		// Always aggregate into country-level row (region = "").
		countryKey := matrixGroupKey{
			nodeID:           nodeID,
			clientASN:        s.ClientASN,
			clientCountry:    country,
			clientRegion:     "",
			ipFamily:         ipFamily,
			aggregationLevel: model.QualityAggregationLevelCountry,
		}
		countryAcc := ensurePairAccum(groups, countryKey, inputs, s.NodeASN)
		applySampleToPair(countryAcc, &s, decay, capturedAt)

		// Additionally aggregate into region-level row only when country is in drilldown whitelist and region is known.
		if rawRegion != "" && pkgnetintel.IsRegionDrilldownCountry(country) {
			regionKey := matrixGroupKey{
				nodeID:           nodeID,
				clientASN:        s.ClientASN,
				clientCountry:    country,
				clientRegion:     rawRegion,
				ipFamily:         ipFamily,
				aggregationLevel: model.QualityAggregationLevelRegion,
			}
			regionAcc := ensurePairAccum(groups, regionKey, inputs, s.NodeASN)
			applySampleToPair(regionAcc, &s, decay, capturedAt)
		}
	}
	return groups
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
	groups map[matrixGroupKey]*pairAccum,
	key matrixGroupKey,
	inputs *rollupInputs,
	sampleNodeASN uint32,
) *pairAccum {
	acc := groups[key]
	if acc != nil {
		return acc
	}
	nodeASN := sampleNodeASN
	if n, ok := inputs.nodeByID[key.nodeID]; ok && n.ASN > 0 {
		nodeASN = n.ASN
	}
	acc = &pairAccum{
		nodeID:           key.nodeID,
		nodeASN:          nodeASN,
		clientASN:        key.clientASN,
		clientCountry:    key.clientCountry,
		clientRegion:     key.clientRegion,
		ipFamily:         key.ipFamily,
		aggregationLevel: key.aggregationLevel,
		traceASPathJSON:  resolveTracePath(inputs.tracePaths, key),
	}
	groups[key] = acc
	return acc
}

func resolveTracePath(paths map[tracePathKey]string, key matrixGroupKey) string {
	candidates := []tracePathKey{
		{nodeID: key.nodeID, targetASN: key.clientASN, targetCountry: key.clientCountry, targetRegion: key.clientRegion, ipFamily: key.ipFamily},
		{nodeID: key.nodeID, targetASN: key.clientASN, targetCountry: key.clientCountry, targetRegion: "", ipFamily: key.ipFamily},
		{nodeID: key.nodeID, targetASN: key.clientASN, targetCountry: key.clientCountry, targetRegion: "", ipFamily: pkgnetintel.IPFamilyUnknown},
	}
	for _, c := range candidates {
		if path, ok := paths[c]; ok && strings.TrimSpace(path) != "" {
			return path
		}
	}
	return ""
}

func applySampleToPair(acc *pairAccum, s *analyticsmodel.NetworkQualitySample, decay float64, capturedAt time.Time) {
	sampleCount := max(1, int(s.SampleCount))
	bucketWeight := decay * math.Max(1.0, math.Log1p(float64(sampleCount)))

	accumulateSamplePrefixes(acc, s)
	accumulateSampleGeo(acc, s, decay)
	decodedHist := decodeSampleHist(acc, s)

	pointRTT := s.RTTP50Ms
	if decodedHist != nil {
		pointRTT = decodedHist.Quantile(percentileMedian)
	}
	if pointRTT > 0 {
		acc.ewmaPoints = append(acc.ewmaPoints, ewmaObservation{
			capturedAt: capturedAt,
			rttMs:      pointRTT,
			source:     s.Source,
		})
	}

	accumulateSampleSource(acc, s, decodedHist, decay, bucketWeight, sampleCount, capturedAt)
}

func accumulateSamplePrefixes(acc *pairAccum, s *analyticsmodel.NetworkQualitySample) {
	if strings.TrimSpace(s.PrefixSketch) != "" {
		if sk, err := DecodePrefixSketch(s.PrefixSketch); err == nil {
			acc.prefixSketch.Union(sk)
		}
		return
	}
	if int(s.DistinctPrefixCount) > acc.legacyMaxPfx {
		acc.legacyMaxPfx = int(s.DistinctPrefixCount)
	}
}

func accumulateSampleGeo(acc *pairAccum, s *analyticsmodel.NetworkQualitySample, decay float64) {
	if s.ClientLat == nil || s.ClientLon == nil {
		return
	}
	accKm := normalizeSampleGeoAccuracy(s.GeoAccuracyKm, s.GeoSource)
	geoW := decay / math.Max(minAccuracyWeightFloorKm, accKm)
	acc.centroid.Add(*s.ClientLat, *s.ClientLon, geoW)
	acc.geoAccSum += accKm * geoW
	acc.geoAccWeight += geoW
	if acc.geoSource == "" || (acc.geoSource != pkgnetintel.GeoSourceCity && s.GeoSource == pkgnetintel.GeoSourceCity) {
		acc.geoSource = strings.TrimSpace(s.GeoSource)
	}
}

func normalizeSampleGeoAccuracy(accKm float64, geoSource string) float64 {
	if accKm > 0 {
		return accKm
	}
	if strings.EqualFold(geoSource, pkgnetintel.GeoSourceCity) {
		return defaultCityAccuracyKm
	}
	return minGeoSourceWeightFallback
}

func decodeSampleHist(acc *pairAccum, s *analyticsmodel.NetworkQualitySample) *LogHistogram64 {
	if s.StatsVersion < model.NetworkQualityStatsVersionHist64 || strings.TrimSpace(s.RTTHist) == "" {
		return nil
	}
	h, err := DecodeLogHistogram64(s.RTTHist)
	if err != nil || h.Count == 0 {
		return nil
	}
	acc.hasV2Hist = true
	return h
}

func accumulateSampleSource(
	acc *pairAccum,
	s *analyticsmodel.NetworkQualitySample,
	decodedHist *LogHistogram64,
	decay float64,
	bucketWeight float64,
	sampleCount int,
	capturedAt time.Time,
) {
	t := capturedAt
	switch s.Source {
	case model.NetworkQualitySourceTCPPassive:
		acc.tcpSamples += sampleCount
		addHistOrLegacy(decodedHist, &acc.tcpHist, decay, s.RTTP50Ms, bucketWeight, &acc.tcpLegacyWeight, &acc.tcpLegacyP50Sum)
		if decodedHist == nil && s.RTTP50Ms > 0 {
			acc.tcpLegacyP95Sum += s.RTTP95Ms * bucketWeight
		}
		acc.tcpWeightSum += bucketWeight
		acc.tcpRetransSum += s.RetransRate * bucketWeight
		if s.RTTMinMs > 0 && (acc.tcpMinRTTMs == 0 || s.RTTMinMs < acc.tcpMinRTTMs) {
			acc.tcpMinRTTMs = s.RTTMinMs
		}
		acc.tcpLastSeenAt = maxTimePtr(acc.tcpLastSeenAt, t)
	case model.NetworkQualitySourceTCPActive:
		acc.tcpActiveSamples += sampleCount
		addHistOrLegacy(decodedHist, &acc.tcpActiveHist, decay, s.RTTP50Ms, bucketWeight, &acc.tcpActiveLegWeight, &acc.tcpActiveLegP50Sum)
		acc.tcpActiveWeightSum += bucketWeight
		acc.tcpActiveLossSum += s.LossRate * bucketWeight
		acc.tcpActiveLastSeenAt = maxTimePtr(acc.tcpActiveLastSeenAt, t)
	case model.NetworkQualitySourceICMPActive:
		acc.icmpActiveSamples += sampleCount
		addHistOrLegacy(decodedHist, &acc.icmpActiveHist, decay, s.RTTP50Ms, bucketWeight, &acc.icmpActiveLegWeight, &acc.icmpActiveLegP50Sum)
		acc.icmpActiveWeightSum += bucketWeight
		acc.icmpActiveLossSum += s.LossRate * bucketWeight
		acc.icmpActiveLastSeenAt = maxTimePtr(acc.icmpActiveLastSeenAt, t)
	default:
		acc.traceSamples += sampleCount
		addHistOrLegacy(decodedHist, &acc.traceHist, decay, s.RTTP50Ms, bucketWeight, &acc.traceLegWeight, &acc.traceLegP50Sum)
		acc.traceWeightSum += bucketWeight
		acc.traceLossSum += s.LossRate * bucketWeight
		acc.traceHopSum += float64(s.HopCount) * bucketWeight
		acc.traceLastSeenAt = maxTimePtr(acc.traceLastSeenAt, t)
	}
}

func addHistOrLegacy(
	decodedHist *LogHistogram64,
	targetHist *WeightedLogHistogram64,
	decay float64,
	p50Ms float64,
	bucketWeight float64,
	legWeight *float64,
	legP50Sum *float64,
) {
	if decodedHist != nil {
		targetHist.AddHistogram(decodedHist, decay)
		return
	}
	if p50Ms > 0 {
		*legWeight += bucketWeight
		*legP50Sum += p50Ms * bucketWeight
	}
}

func maxTimePtr(existing *time.Time, candidate time.Time) *time.Time {
	if existing == nil || candidate.After(*existing) {
		t := candidate
		return &t
	}
	return existing
}

// RunRollup aggregates passive and active observations from the last 24h into measured-only
// two-level (country/region) rows in of_network_quality_matrix without creating an empty-table window.
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
	groups := accumulateRollupGroups(inputs, now)

	rows := make([]*model.OpenFlareNetworkQualityMatrix, 0, len(groups))
	for key, acc := range groups {
		rows = append(rows, finalizeMeasuredRow(acc, inputs.nodeByID[key.nodeID]))
	}

	pruned, err := repository.ReplaceOpenFlareNetworkQualityMatrixSnapshot(ctx, rows, now.Add(-rollupStaleRetention))
	if err != nil {
		return nil, fmt.Errorf("replace network quality matrix snapshot: %w", err)
	}
	rollupGeneration.Add(1)

	return &RollupSummary{
		SamplesRead:   len(inputs.samples),
		MeasuredRows:  len(rows),
		PredictedRows: 0,
		PrunedRows:    pruned,
		DurationMs:    time.Since(started).Milliseconds(),
	}, nil
}

func selectPrimaryProbeMetric(
	acc *pairAccum,
	tcpActiveP50, tcpActiveLoss, icmpActiveP50, icmpActiveLoss, traceP50, traceLoss float64,
) (rttP50, loss float64, samples int, proto string, lastSeen *time.Time) {
	switch {
	case acc.tcpActiveSamples > 0 && tcpActiveP50 > 0:
		return tcpActiveP50, tcpActiveLoss, acc.tcpActiveSamples, probeProtoTCP, acc.tcpActiveLastSeenAt
	case acc.icmpActiveSamples > 0 && icmpActiveP50 > 0:
		return icmpActiveP50, icmpActiveLoss, acc.icmpActiveSamples, probeProtoICMP, acc.icmpActiveLastSeenAt
	case acc.traceSamples > 0 && traceP50 > 0:
		return traceP50, traceLoss, acc.traceSamples, model.NetworkQualitySourceTraceroute, acc.traceLastSeenAt
	default:
		return 0, 0, 0, "", nil
	}
}

func selectEffectiveLatencySource(
	tcpP50, tcpRetrans, tcpActiveP50, tcpActiveLoss, icmpActiveP50, icmpActiveLoss, traceP50, traceLoss float64,
) (effectiveRTT float64, measurementSource string, primaryLoss float64) {
	switch {
	case tcpP50 > 0:
		return tcpP50, model.NetworkQualitySourceTCPPassive, tcpRetrans
	case tcpActiveP50 > 0:
		return tcpActiveP50, model.NetworkQualitySourceTCPActive, tcpActiveLoss
	case icmpActiveP50 > 0:
		return icmpActiveP50, model.NetworkQualitySourceICMPActive, icmpActiveLoss
	case traceP50 > 0:
		return traceP50, model.NetworkQualitySourceTraceroute, traceLoss
	default:
		return 0, model.MeasurementSourceNone, 0
	}
}

func computeRowConfidenceAndScore(
	totalSamples, distinctPrefixes, statsVersion int,
	effectiveRTT, tcpP50, tcpP95, tcpRetrans, primaryLoss float64,
	measurementSource string,
) (confidence, qualityScore float64) {
	sampleFactor := 1.0 - math.Exp(-float64(totalSamples)/confidenceHalfSamples)
	prefixFactor := 1.0
	if distinctPrefixes > 0 {
		prefixFactor = math.Min(1.0, prefixConfidenceBase+prefixConfidenceSpan*(1.0-math.Exp(-float64(distinctPrefixes)/prefixConfidenceHalf)))
	}
	confidence = math.Max(minSampleConfidence, math.Min(maxSampleConfidence, sampleFactor*prefixFactor))
	if statsVersion < model.NetworkQualityStatsVersionHist64 {
		confidence = math.Min(confidence, legacyMaxConfidence)
	}

	scoredRTT := effectiveRTT
	if tcpP95 > tcpP50 && tcpP50 > 0 && measurementSource == model.NetworkQualitySourceTCPPassive {
		scoredRTT += 0.35 * (tcpP95 - tcpP50)
	}
	lossPenalty := math.Min(maxLossPenaltyRatio, tcpRetrans*tcpRetransPenaltyFactor+primaryLoss*probeLossPenaltyFactor)
	qualityScore = math.Max(minQualityScore, (maxQualityScore/(1.0+scoredRTT/qualityScoreRTTScale))*(1.0-lossPenalty))
	return confidence, qualityScore
}

func finalizeMeasuredRow(
	acc *pairAccum,
	node model.OpenFlareNode,
) *model.OpenFlareNetworkQualityMatrix {
	var tcpP50, tcpP95, tcpRetrans float64
	if acc.tcpHist.TotalWeight > 0 {
		tcpP50 = acc.tcpHist.Quantile(percentileMedian)
		tcpP95 = acc.tcpHist.Quantile(percentileP95)
	} else if acc.tcpLegacyWeight > 0 {
		tcpP50 = acc.tcpLegacyP50Sum / acc.tcpLegacyWeight
		tcpP95 = acc.tcpLegacyP95Sum / acc.tcpLegacyWeight
	}
	if acc.tcpWeightSum > 0 {
		tcpRetrans = acc.tcpRetransSum / acc.tcpWeightSum
	}

	tcpActiveP50, tcpActiveLoss := resolveSourceRTTAndLoss(&acc.tcpActiveHist, acc.tcpActiveLegP50Sum, acc.tcpActiveLegWeight, acc.tcpActiveLossSum, acc.tcpActiveWeightSum)
	icmpActiveP50, icmpActiveLoss := resolveSourceRTTAndLoss(&acc.icmpActiveHist, acc.icmpActiveLegP50Sum, acc.icmpActiveLegWeight, acc.icmpActiveLossSum, acc.icmpActiveWeightSum)
	traceP50, traceLoss := resolveSourceRTTAndLoss(&acc.traceHist, acc.traceLegP50Sum, acc.traceLegWeight, acc.traceLossSum, acc.traceWeightSum)

	var traceHops int
	if acc.traceWeightSum > 0 {
		traceHops = int(math.Round(acc.traceHopSum / acc.traceWeightSum))
	}

	probeRTTP50, probeLoss, probeSamples, probeProto, probeLastSeen := selectPrimaryProbeMetric(
		acc, tcpActiveP50, tcpActiveLoss, icmpActiveP50, icmpActiveLoss, traceP50, traceLoss,
	)
	effectiveRTT, measurementSource, primaryLoss := selectEffectiveLatencySource(
		tcpP50, tcpRetrans, tcpActiveP50, tcpActiveLoss, icmpActiveP50, icmpActiveLoss, traceP50, traceLoss,
	)

	rttEWMA := computeOrderedEWMA(acc.ewmaPoints, measurementSource)
	if rttEWMA <= 0 {
		rttEWMA = effectiveRTT
	}

	asnName := ""
	if asnInfo := netintel.LookupASN(acc.clientASN); asnInfo != nil {
		asnName = asnInfo.Name
	}
	nodeASN := acc.nodeASN
	if nodeASN == 0 {
		nodeASN = node.ASN
	}
	asPathJSON := acc.traceASPathJSON
	if strings.TrimSpace(asPathJSON) == "" {
		asPathJSON = "[]"
	}
	distinctPrefixes := acc.prefixSketch.DistinctCount()
	if distinctPrefixes == 0 && acc.legacyMaxPfx > 0 {
		distinctPrefixes = acc.legacyMaxPfx
	}

	geoBaseRTTMs, geoAccuracyKm, geoSource, geoConfidence := computeGeoBaselineForRow(acc, node)
	lastObservedAt := latestTimePtr(acc.tcpLastSeenAt, acc.tcpActiveLastSeenAt, acc.icmpActiveLastSeenAt, acc.traceLastSeenAt)

	statsVersion := model.NetworkQualityStatsVersionLegacy
	if acc.hasV2Hist {
		statsVersion = model.NetworkQualityStatsVersionHist64
	}
	totalSamples := acc.tcpSamples + acc.tcpActiveSamples + acc.icmpActiveSamples + acc.traceSamples
	confidence, qualityScore := computeRowConfidenceAndScore(
		totalSamples, distinctPrefixes, statsVersion,
		effectiveRTT, tcpP50, tcpP95, tcpRetrans, primaryLoss, measurementSource,
	)

	return &model.OpenFlareNetworkQualityMatrix{
		NodeID:              acc.nodeID,
		NodeASN:             nodeASN,
		ClientASN:           acc.clientASN,
		ClientCountry:       acc.clientCountry,
		ClientRegion:        acc.clientRegion,
		IPFamily:            acc.ipFamily,
		AggregationLevel:    acc.aggregationLevel,
		StatsVersion:        statsVersion,
		ClientASNName:       asnName,
		TCPRTTP50Ms:         round2(tcpP50),
		TCPRTTP95Ms:         round2(tcpP95),
		TCPMinRTTMs:         round2(acc.tcpMinRTTMs),
		TCPRetransRate:      round4(tcpRetrans),
		TCPSampleCount:      acc.tcpSamples,
		TCPLastSeenAt:       acc.tcpLastSeenAt,
		ProbeRTTP50Ms:       round2(probeRTTP50),
		ProbeLossRate:       round4(probeLoss),
		ProbeSampleCount:    probeSamples,
		ProbeProto:          probeProto,
		ProbeLastSeenAt:     probeLastSeen,
		TraceRTTP50Ms:       round2(traceP50),
		TraceLossRate:       round4(traceLoss),
		TraceHopCount:       traceHops,
		TraceASPath:         asPathJSON,
		TraceSampleCount:    acc.traceSamples,
		TraceLastSeenAt:     acc.traceLastSeenAt,
		EffectiveRTTMs:      round2(effectiveRTT),
		PredictedRTTMs:      0,
		RTTEWMAms:           round2(rttEWMA),
		DistinctPrefixCount: distinctPrefixes,
		GeoBaseRTTMs:        round2(geoBaseRTTMs),
		GeoAccuracyKm:       round2(geoAccuracyKm),
		GeoSource:           geoSource,
		GeoConfidence:       round4(geoConfidence),
		MeasurementSource:   measurementSource,
		QualityScore:        round2(qualityScore),
		Confidence:          round4(confidence),
		DataSource:          model.QualityMatrixDataSourceMeasured,
		LastObservedAt:      lastObservedAt,
	}
}

func computeGeoBaselineForRow(acc *pairAccum, node model.OpenFlareNode) (baseRTTMs, accuracyKm float64, geoSource string, geoConfidence float64) {
	geoSource = acc.geoSource
	if acc.geoAccWeight > 0 {
		accuracyKm = acc.geoAccSum / acc.geoAccWeight
	}
	if node.GeoLatitude == nil || node.GeoLongitude == nil {
		return 0, accuracyKm, geoSource, 0
	}
	cLat, cLon, ok := acc.centroid.Result()
	if !ok {
		return 0, accuracyKm, geoSource, 0
	}
	distKm := pkgnetintel.DistanceKm(*node.GeoLatitude, *node.GeoLongitude, cLat, cLon)
	baseRTTMs = pkgnetintel.GeoBaseRTTMs(distKm)
	nodeGeoSource := resolveNodeGeoSource(node)
	nodeConf := pkgnetintel.GeoLocationConfidence(nodeGeoSource, defaultCityAccuracyKm)
	clientConf := pkgnetintel.GeoLocationConfidence(geoSource, accuracyKm)
	return baseRTTMs, accuracyKm, geoSource, math.Min(nodeConf, clientConf)
}

func resolveNodeGeoSource(node model.OpenFlareNode) string {
	if s := strings.TrimSpace(node.GeoSource); s != "" {
		return s
	}
	if node.GeoManualOverride {
		return pkgnetintel.GeoSourceManual
	}
	return pkgnetintel.GeoSourceCountry
}

func resolveSourceRTTAndLoss(hist *WeightedLogHistogram64, legP50Sum, legWeight, lossSum, weightSum float64) (float64, float64) {
	var p50, loss float64
	if hist != nil && hist.TotalWeight > 0 {
		p50 = hist.Quantile(percentileMedian)
	} else if legWeight > 0 {
		p50 = legP50Sum / legWeight
	}
	if weightSum > 0 {
		loss = lossSum / weightSum
	}
	return p50, loss
}

func computeOrderedEWMA(points []ewmaObservation, preferredSource string) float64 {
	if len(points) == 0 {
		return 0
	}
	filtered := make([]ewmaObservation, 0, len(points))
	for _, pt := range points {
		if pt.rttMs <= 0 {
			continue
		}
		if preferredSource != "" && preferredSource != model.MeasurementSourceNone && pt.source != preferredSource {
			continue
		}
		filtered = append(filtered, pt)
	}
	if len(filtered) == 0 {
		for _, pt := range points {
			if pt.rttMs > 0 {
				filtered = append(filtered, pt)
			}
		}
	}
	if len(filtered) == 0 {
		return 0
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].capturedAt.Before(filtered[j].capturedAt)
	})
	ewma := filtered[0].rttMs
	prevT := filtered[0].capturedAt
	for i := 1; i < len(filtered); i++ {
		dtHours := max(0.0, filtered[i].capturedAt.Sub(prevT).Hours())
		alpha := 1.0 - math.Exp(-dtHours/ewmaHalfLifeHours)
		if alpha < ewmaZeroDtAlpha {
			alpha = ewmaZeroDtAlpha
		}
		ewma = alpha*filtered[i].rttMs + (1.0-alpha)*ewma
		prevT = filtered[i].capturedAt
	}
	return ewma
}

func latestTimePtr(candidates ...*time.Time) *time.Time {
	var best *time.Time
	for _, c := range candidates {
		if c == nil || c.IsZero() {
			continue
		}
		if best == nil || c.After(*best) {
			t := *c
			best = &t
		}
	}
	return best
}

func latestTracePaths(traces []*model.OpenFlareTracerouteRecord) map[tracePathKey]string {
	out := make(map[tracePathKey]string)
	for _, tr := range traces {
		if tr == nil || tr.NodeID == "" || tr.TargetASN == 0 {
			continue
		}
		if strings.TrimSpace(tr.ASPathJSON) == "" || tr.ASPathJSON == "[]" {
			continue
		}
		country := strings.ToUpper(strings.TrimSpace(tr.TargetCountry))
		region := pkgnetintel.FormatSubdivisionCode(country, tr.TargetRegion)
		ipFamily := strings.ToLower(strings.TrimSpace(tr.IPFamily))
		if ipFamily == "" {
			ipFamily = pkgnetintel.DetectIPFamily(tr.ResolvedIP)
		}
		if ipFamily == "" {
			ipFamily = pkgnetintel.IPFamilyUnknown
		}

		// Populate region-specific key when applicable, plus country-level key for aggregation fallback.
		if region != "" {
			regKey := tracePathKey{
				nodeID:        tr.NodeID,
				targetASN:     tr.TargetASN,
				targetCountry: country,
				targetRegion:  region,
				ipFamily:      ipFamily,
			}
			if _, exists := out[regKey]; !exists {
				out[regKey] = tr.ASPathJSON
			}
		}
		countryKey := tracePathKey{
			nodeID:        tr.NodeID,
			targetASN:     tr.TargetASN,
			targetCountry: country,
			targetRegion:  "",
			ipFamily:      ipFamily,
		}
		if _, exists := out[countryKey]; !exists {
			out[countryKey] = tr.ASPathJSON
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
