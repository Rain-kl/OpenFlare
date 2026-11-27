// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netquality

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/cache/ram"
	pkggeoip "github.com/Rain-kl/Wavelet/pkg/geoip"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
)

const (
	// PredictionMethodExactRegion indicates a direct match at (node, asn, country, region, ip_family).
	PredictionMethodExactRegion = "exact_region"
	// PredictionMethodCountryRollup indicates a direct fallback to country-level aggregation.
	PredictionMethodCountryRollup = "country_rollup"
	// PredictionMethodInferredUpstream indicates similarity-weighted residual transfer on the same node.
	PredictionMethodInferredUpstream = "inferred_upstream"
	// PredictionMethodSimilarNode indicates cold-start fallback to a sibling node in the same group/city.
	PredictionMethodSimilarNode = "similar_node"
	// PredictionMethodGeoPrior indicates pure geographic propagation baseline prior.
	PredictionMethodGeoPrior = "geo_prior"

	// Internal cold-start constants (kept internal until automated calibration overrides).
	geoPriorDetourFactor = 1.35
	geoPriorAddBaseMs    = 8.0
	unknownGeoPriorMs    = 120.0

	directMatchMaxAge        = 24 * time.Hour
	directFreshBypassAge     = 2 * time.Hour
	directRegionMinSamples   = 3
	directCountryMinSample   = 2
	stalePassiveAge          = 12 * time.Hour
	stalePassiveMinSamples   = 3
	freshProbeMaxAge         = 2 * time.Hour
	freshProbeMinSamples     = 3
	directDefaultConfidence  = 0.50
	directCountryDecayFactor = 0.90

	inferredMaxCandidateAge   = 48 * time.Hour
	inferredMinTotalWeight    = 0.12
	inferredShrinkageLambda   = 1.20
	inferredBaseConfidence    = 0.25
	inferredConfWeightSpan    = 0.45
	inferredDispersionScaleMs = 40.0
	inferredMinConfidence     = 0.15
	inferredMaxConfidence     = 0.72
	inferredP95SpreadWeight   = 0.50
	negativeResidualGeoFactor = 0.45
	unknownNetSimWeight       = 0.25
	netSimBaseWeight          = 0.25
	netSimSpanWeight          = 0.75
	crossFamilyWeight         = 0.65
	candidateFreshHalfLifeH   = 12.0
	candidateMinQualityW      = 0.15

	geoWeightSameRegion   = 1.0
	geoWeightSameCountry  = 0.85
	geoWeightRegional     = 0.35
	geoWeightCrossRegion  = 0.15
	siblingMaxDistanceKm  = 50.0
	siblingSameGroupW     = 1.0
	siblingSameASNW       = 0.55
	siblingConfMultiplier = 0.65
	siblingMaxSparseRows  = 2

	geoPriorMaxConfidence  = 0.25
	geoPriorUnknownConf    = 0.05
	geoPriorConfScale      = 0.18
	geoPriorMinConfFloor   = 0.25
	geoPriorP95Multiplier  = 1.25
	minPredictedRTTMs      = 1.0
	qualityDispersionShare = 0.25

	predictionCacheMaxSize    = 4096
	predictionCacheTTL        = 5 * time.Minute
	predictionCacheCoordScale = 5.0
	minCalibratedSamples      = 10
	calibrationDecileCount    = 10.0
	calibrationMaxConfClamped = 0.99
	calibrationP80Quantile    = 0.80
	calibrationMethodWide     = -1
	quartileLower             = 0.25
	quartileMedian            = 0.50
	quartileUpper             = 0.75
	topologyEdgeQueryLimit    = 10000
)

// PredictQuery specifies a client network target to evaluate against an edge node.
type PredictQuery struct {
	NodeID        string
	ClientASN     uint32
	ClientCountry string
	ClientRegion  string
	IPFamily      string
	ClientLat     *float64
	ClientLon     *float64
	GeoAccuracyKm float64
	GeoSource     string
	AnycastStatus string
}

// PredictionResult represents a unified, source-disambiguated latency prediction.
type PredictionResult struct {
	RTTMs                 float64    `json:"rtt_ms"`
	ObservedRTTMs         float64    `json:"observed_rtt_ms,omitempty"`
	PredictedRTTMs        float64    `json:"predicted_rtt_ms,omitempty"`
	RTTP95Ms              float64    `json:"rtt_p95_ms"`
	P95Weight             float64    `json:"p95_weight,omitempty"`
	CandidateDispersionMs float64    `json:"candidate_dispersion_ms"`
	LossRate              float64    `json:"loss_rate,omitempty"`
	ErrorMs               *float64   `json:"error_ms,omitempty"`
	Confidence            float64    `json:"confidence"`
	QualityState          string     `json:"quality_state,omitempty"`
	ExplorePriority       float64    `json:"explore_priority,omitempty"`
	MeasurementSource     string     `json:"measurement_source"`
	PredictionMethod      string     `json:"prediction_method"`
	GeoBaseRTTMs          float64    `json:"geo_base_rtt_ms"`
	GeoBaseKnown          bool       `json:"geo_base_known,omitempty"`
	SampleCount           uint64     `json:"sample_count"`
	DistinctPrefixes      uint32     `json:"distinct_prefixes"`
	LastObservedAt        *time.Time `json:"last_observed_at,omitempty"`
	AnycastIsolated       bool       `json:"anycast_isolated,omitempty"`
}

// CalibrationObservation pairs a predicted result against an observed ground-truth RTT.
type CalibrationObservation struct {
	Method      string
	Confidence  float64
	PredictedMs float64
	ActualMs    float64
}

// CalibrationBucket holds calibrated holdout prediction error statistics for one (method, confBin) slice.
type CalibrationBucket struct {
	Method      string
	ConfBin     int
	MAE         float64
	P80ErrorMs  float64
	SampleCount int
}

// ErrorCalibration stores empirically calibrated prediction error bounds keyed by prediction method and confidence decile.
type ErrorCalibration struct {
	Buckets map[string]CalibrationBucket
}

// BuildErrorCalibration derives calibrated MAE and P80 error tables from holdout observations.
func BuildErrorCalibration(observations []CalibrationObservation) *ErrorCalibration {
	if len(observations) == 0 {
		return nil
	}
	byKey := make(map[string][]float64)
	for _, obs := range observations {
		if obs.ActualMs <= 0 || obs.PredictedMs <= 0 {
			continue
		}
		absErr := math.Abs(obs.ActualMs - obs.PredictedMs)
		binKey := calibrationKey(obs.Method, calibrationConfBin(obs.Confidence))
		wideKey := calibrationKey(obs.Method, calibrationMethodWide)
		byKey[binKey] = append(byKey[binKey], absErr)
		byKey[wideKey] = append(byKey[wideKey], absErr)
	}
	buckets := make(map[string]CalibrationBucket, len(byKey))
	for k, errs := range byKey {
		if len(errs) < minCalibratedSamples {
			continue
		}
		sort.Float64s(errs)
		var sum float64
		for _, e := range errs {
			sum += e
		}
		p80Idx := min(len(errs)-1, int(math.Floor(calibrationP80Quantile*float64(len(errs)-1))))
		method, bin := parseCalibrationKey(k)
		buckets[k] = CalibrationBucket{
			Method:      method,
			ConfBin:     bin,
			MAE:         round2(sum / float64(len(errs))),
			P80ErrorMs:  round2(errs[p80Idx]),
			SampleCount: len(errs),
		}
	}
	if len(buckets) == 0 {
		return nil
	}
	return &ErrorCalibration{Buckets: buckets}
}

// Lookup returns the calibrated error estimate in ms when calibration data for (method, confidence) is sufficient.
func (c *ErrorCalibration) Lookup(method string, confidence float64) *float64 {
	if c == nil || len(c.Buckets) == 0 {
		return nil
	}
	bin := calibrationConfBin(confidence)
	if b, ok := c.Buckets[calibrationKey(method, bin)]; ok && b.SampleCount >= minCalibratedSamples {
		val := round2(b.P80ErrorMs)
		return &val
	}
	if b, ok := c.Buckets[calibrationKey(method, calibrationMethodWide)]; ok && b.SampleCount >= minCalibratedSamples {
		val := round2(b.P80ErrorMs)
		return &val
	}
	return nil
}

func calibrationConfBin(confidence float64) int {
	return int(math.Floor(math.Max(0, math.Min(calibrationMaxConfClamped, confidence)) * calibrationDecileCount))
}

func calibrationKey(method string, bin int) string {
	return fmt.Sprintf("%s:%d", method, bin)
}

func parseCalibrationKey(key string) (string, int) {
	idx := strings.LastIndex(key, ":")
	if idx < 0 {
		return key, calibrationMethodWide
	}
	var bin int
	if _, err := fmt.Sscanf(key[idx+1:], "%d", &bin); err != nil {
		return key[:idx], calibrationMethodWide
	}
	return key[:idx], bin
}

type predictionCacheKey struct {
	generation    uint64
	nodeID        string
	clientASN     uint32
	clientCountry string
	clientRegion  string
	ipFamily      string
	hasCoords     bool
	latBucket     int
	lonBucket     int
	geoSource     string
}

func buildPredictionCacheKey(generation uint64, q PredictQuery) predictionCacheKey {
	ck := predictionCacheKey{
		generation:    generation,
		nodeID:        q.NodeID,
		clientASN:     q.ClientASN,
		clientCountry: q.ClientCountry,
		clientRegion:  q.ClientRegion,
		ipFamily:      q.IPFamily,
	}
	if q.ClientLat != nil && q.ClientLon != nil && isValidCoord(*q.ClientLat, *q.ClientLon) {
		ck.hasCoords = true
		ck.latBucket = int(math.Round(*q.ClientLat * predictionCacheCoordScale))
		ck.lonBucket = int(math.Round(*q.ClientLon * predictionCacheCoordScale))
		ck.geoSource = strings.ToLower(strings.TrimSpace(q.GeoSource))
	}
	return ck
}

func isValidCoord(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsInf(lat, 0) && !math.IsNaN(lon) && !math.IsInf(lon, 0)
}

func hasKnownRowGeoBase(row *model.OpenFlareNetworkQualityMatrix) bool {
	if row == nil || row.GeoBaseRTTMs < 0 {
		return false
	}
	return row.GeoBaseRTTMs > 0 || row.GeoConfidence > 0
}

type cachedPrediction struct {
	result    PredictionResult
	expiresAt time.Time
}

var (
	predictionCache = ram.MustNew[predictionCacheKey, cachedPrediction](ram.Options{
		MaximumSize: predictionCacheMaxSize,
	})
	calibrationMu     sync.RWMutex
	activeCalibration *ErrorCalibration
)

// SetErrorCalibration installs an empirically calibrated error table (intended for evaluation/startup calibration).
func SetErrorCalibration(cal *ErrorCalibration) {
	calibrationMu.Lock()
	activeCalibration = cal
	calibrationMu.Unlock()
	ResetPredictorCacheForTest()
}

func currentErrorCalibration() *ErrorCalibration {
	calibrationMu.RLock()
	cal := activeCalibration
	calibrationMu.RUnlock()
	return cal
}

// ResetPredictorCacheForTest clears the bounded TTL prediction cache.
func ResetPredictorCacheForTest() {
	predictionCache.InvalidateAll()
}

// Predictor evaluates node-to-network latency using the 5-level measured + residual similarity pipeline.
type Predictor struct {
	now          time.Time
	nodes        map[string]model.OpenFlareNode
	nodeProfiles map[string]NodeNetworkProfile
	rowsByNode   map[string][]*model.OpenFlareNetworkQualityMatrix
	exactIndex   map[matrixGroupKey]*model.OpenFlareNetworkQualityMatrix
	observedAdj  map[uint32][]uint32
	asProfiles   map[uint32]pkgnetintel.ASUpstreamProfile
	calibration  *ErrorCalibration
	useCache     bool
	generation   uint64
}

// NewPredictor constructs a Predictor over an in-memory snapshot of measured matrix rows, nodes, and topology edges.
func NewPredictor(
	now time.Time,
	nodes []model.OpenFlareNode,
	measuredRows []*model.OpenFlareNetworkQualityMatrix,
	observedAdj map[uint32][]uint32,
	calibration *ErrorCalibration,
) *Predictor {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	nodeMap := make(map[string]model.OpenFlareNode, len(nodes))
	profiles := make(map[string]NodeNetworkProfile, len(nodes))
	for i := range nodes {
		n := nodes[i]
		if n.ASN == 0 && strings.TrimSpace(n.IP) != "" {
			netintel.EnrichNodeASN(&n)
		}
		nodeMap[n.NodeID] = n
		profiles[n.NodeID] = BuildNodeNetworkProfile(n, observedAdj)
	}

	rowsByNode := make(map[string][]*model.OpenFlareNetworkQualityMatrix)
	exactIndex := make(map[matrixGroupKey]*model.OpenFlareNetworkQualityMatrix, len(measuredRows))
	for _, r := range measuredRows {
		if r == nil || r.NodeID == "" || r.ClientASN == 0 || r.DataSource != model.QualityMatrixDataSourceMeasured {
			continue
		}
		rowsByNode[r.NodeID] = append(rowsByNode[r.NodeID], r)
		key := matrixGroupKey{
			nodeID:           r.NodeID,
			clientASN:        r.ClientASN,
			clientCountry:    strings.ToUpper(strings.TrimSpace(r.ClientCountry)),
			clientRegion:     strings.ToUpper(strings.TrimSpace(r.ClientRegion)),
			ipFamily:         normalizePredictorIPFamily(r.IPFamily),
			aggregationLevel: normalizePredictorAggLevel(r.AggregationLevel),
		}
		exactIndex[key] = r
	}

	return &Predictor{
		now:          now.UTC(),
		nodes:        nodeMap,
		nodeProfiles: profiles,
		rowsByNode:   rowsByNode,
		exactIndex:   exactIndex,
		observedAdj:  observedAdj,
		asProfiles:   make(map[uint32]pkgnetintel.ASUpstreamProfile),
		calibration:  calibration,
		generation:   CurrentRollupGeneration(),
	}
}

// LoadLivePredictor builds a Predictor backed by current DB state and enables prediction_cache.
func LoadLivePredictor(ctx context.Context) (*Predictor, error) {
	nodes, err := repository.ListOpenFlareNodes(ctx)
	if err != nil {
		return nil, err
	}
	measured, err := repository.ListOpenFlareNetworkQualityMatrixMeasured(ctx)
	if err != nil {
		return nil, err
	}
	edges, _ := repository.ListOpenFlareASTopologyEdges(ctx, topologyEdgeQueryLimit)
	adj := buildObservedAdjacency(edges)
	p := NewPredictor(time.Now().UTC(), nodes, measured, adj, currentErrorCalibration())
	p.useCache = true
	return p, nil
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

// Predict estimates latency from query.NodeID to the queried client network using levels 1..5.
func (p *Predictor) Predict(query PredictQuery) PredictionResult {
	norm := p.normalizeQuery(query)
	if norm.NodeID == "" {
		return PredictionResult{
			RTTMs:             unknownGeoPriorMs,
			Confidence:        geoPriorUnknownConf,
			MeasurementSource: model.MeasurementSourceNone,
			PredictionMethod:  PredictionMethodGeoPrior,
		}
	}

	if p.useCache && norm.AnycastStatus != pkgnetintel.AnycastStatusConfirmed {
		ck := buildPredictionCacheKey(p.generation, norm)
		if cached, ok := predictionCache.GetIfPresent(ck); ok && p.now.Before(cached.expiresAt) {
			return cached.result
		}
		res := p.predictUncached(norm, true)
		predictionCache.Set(ck, cachedPrediction{result: res, expiresAt: p.now.Add(predictionCacheTTL)})
		return res
	}
	return p.predictUncached(norm, true)
}

func (p *Predictor) predictUncached(q PredictQuery, allowSibling bool) PredictionResult {
	targetGeoBase, geoConf, geoKnown := p.computeTargetGeoBase(q)

	// Level 1: exact (node_id, client_asn, client_country, client_region, ip_family, 'region')
	if q.ClientASN > 0 && q.ClientRegion != "" {
		if row := p.lookupDirectRow(q.NodeID, q.ClientASN, q.ClientCountry, q.ClientRegion, q.IPFamily, model.QualityAggregationLevelRegion); row != nil {
			if res, ok := p.buildDirectPrediction(row, targetGeoBase, geoKnown, PredictionMethodExactRegion, directRegionMinSamples); ok {
				return p.attachError(res)
			}
		}
	}

	// Level 2: country-level aggregate (node_id, client_asn, client_country, "", ip_family, 'country')
	if q.ClientASN > 0 {
		if row := p.lookupDirectRow(q.NodeID, q.ClientASN, q.ClientCountry, "", q.IPFamily, model.QualityAggregationLevelCountry); row != nil {
			if res, ok := p.buildDirectPrediction(row, targetGeoBase, geoKnown, PredictionMethodCountryRollup, directCountryMinSample); ok {
				return p.attachError(res)
			}
		}
	}

	// Level 3: same-node similar network residual transfer
	if res, ok := p.predictInferredUpstream(q, targetGeoBase, geoKnown); ok {
		return p.attachError(res)
	}

	// Level 4: sibling node cold-start reference (only when current node has sparse measurements)
	if allowSibling && len(p.rowsByNode[q.NodeID]) <= siblingMaxSparseRows {
		if res, ok := p.predictFromSimilarNode(q, targetGeoBase, geoKnown); ok {
			return p.attachError(res)
		}
	}

	// Level 5: geographic propagation baseline prior
	return p.attachError(buildGeoPriorPrediction(targetGeoBase, geoConf, geoKnown, q.AnycastStatus == pkgnetintel.AnycastStatusConfirmed))
}

func (p *Predictor) normalizeQuery(q PredictQuery) PredictQuery {
	q.NodeID = strings.TrimSpace(q.NodeID)
	q.ClientCountry = strings.ToUpper(strings.TrimSpace(q.ClientCountry))
	q.ClientRegion = pkgnetintel.NormalizeMatrixRegion(q.ClientCountry, q.ClientRegion)
	q.IPFamily = normalizePredictorIPFamily(q.IPFamily)
	if q.ClientCountry == "" && q.ClientASN > 0 {
		if info := netintel.LookupASN(q.ClientASN); info != nil {
			q.ClientCountry = strings.ToUpper(strings.TrimSpace(info.Country))
		}
	}
	return q
}

func (p *Predictor) lookupDirectRow(nodeID string, asn uint32, country, region, ipFamily, level string) *model.OpenFlareNetworkQualityMatrix {
	key := matrixGroupKey{
		nodeID:           nodeID,
		clientASN:        asn,
		clientCountry:    country,
		clientRegion:     region,
		ipFamily:         ipFamily,
		aggregationLevel: level,
	}
	if row := p.exactIndex[key]; row != nil {
		return row
	}
	if ipFamily != pkgnetintel.IPFamilyUnknown {
		key.ipFamily = pkgnetintel.IPFamilyUnknown
		return p.exactIndex[key]
	}
	return nil
}

func (p *Predictor) buildDirectPrediction(
	row *model.OpenFlareNetworkQualityMatrix,
	targetGeoBase float64,
	geoKnown bool,
	method string,
	minSamples int,
) (PredictionResult, bool) {
	rttMs, p95Ms, src, sampleCnt, obsAt := p.selectDirectSource(row)
	if rttMs <= 0 || obsAt == nil {
		return PredictionResult{}, false
	}
	age := max(time.Duration(0), p.now.Sub(*obsAt))
	if age > directMatchMaxAge {
		return PredictionResult{}, false
	}
	if minSamples > 0 && sampleCnt < uint64(minSamples) && age > directFreshBypassAge {
		return PredictionResult{}, false
	}

	geoBase := row.GeoBaseRTTMs
	geoBaseKnown := hasKnownRowGeoBase(row)
	if geoKnown {
		geoBase = targetGeoBase
		geoBaseKnown = true
	}
	conf := row.Confidence
	if conf <= 0 {
		conf = directDefaultConfidence
	}
	conf *= ageFreshnessFactor(age)
	if method == PredictionMethodCountryRollup {
		conf *= directCountryDecayFactor
	}
	if row.StatsVersion < model.NetworkQualityStatsVersionHist64 {
		conf = math.Min(conf, legacyMaxConfidence)
	}

	obsRTT := row.ObservedRTTMs
	if obsRTT <= 0 {
		obsRTT = rttMs
	}
	predRTT := row.PredictedRTTMs
	if predRTT <= 0 {
		predRTT = rttMs
	}
	state := strings.TrimSpace(row.QualityState)
	if state == "" {
		state = model.QualityStateHealthy
	}
	if age > qualityStaleAge {
		state = model.QualityStateStale
	}
	var p95Weight float64
	if src == model.NetworkQualitySourceTCPPassive {
		p95Weight = computeDynamicP95Weight(int(sampleCnt), row.DistinctPrefixCount) //nolint:gosec // bounded sample count
	}

	return PredictionResult{
		RTTMs:             round2(rttMs),
		ObservedRTTMs:     round2(obsRTT),
		PredictedRTTMs:    round2(predRTT),
		RTTP95Ms:          round2(max(rttMs, p95Ms)),
		P95Weight:         p95Weight,
		LossRate:          round4(selectDirectLossRate(row, src)),
		Confidence:        round4(math.Max(minSampleConfidence, math.Min(maxSampleConfidence, conf))),
		QualityState:      state,
		ExplorePriority:   round4(clampUnit(row.ExplorePriority, exploreMinPriority, exploreMaxPriority)),
		MeasurementSource: src,
		PredictionMethod:  method,
		GeoBaseRTTMs:      round2(geoBase),
		GeoBaseKnown:      geoBaseKnown,
		SampleCount:       sampleCnt,
		DistinctPrefixes:  uint32(max(0, row.DistinctPrefixCount)), //nolint:gosec // clamped >= 0
		LastObservedAt:    obsAt,
	}, true
}

func selectDirectLossRate(row *model.OpenFlareNetworkQualityMatrix, src string) float64 {
	if row == nil {
		return 0
	}
	var loss float64
	switch src {
	case model.NetworkQualitySourceTCPPassive:
		loss = row.TCPRetransRate
	case model.NetworkQualitySourceTCPActive, model.NetworkQualitySourceICMPActive:
		loss = row.ProbeLossRate
	case model.NetworkQualitySourceTraceroute:
		loss = row.TraceLossRate
	default:
		loss = max(row.ProbeLossRate, max(row.TraceLossRate, row.TCPRetransRate))
	}
	if loss < 0 || math.IsNaN(loss) || math.IsInf(loss, 0) {
		return 0
	}
	if loss > 1 {
		return 1
	}
	return loss
}

func (p *Predictor) selectDirectSource(row *model.OpenFlareNetworkQualityMatrix) (rttMs, p95Ms float64, source string, samples uint64, obsAt *time.Time) {
	tcpFreshOK := row.TCPRTTP50Ms > 0 && row.TCPLastSeenAt != nil
	probeFreshOK := row.ProbeRTTP50Ms > 0 && row.ProbeLastSeenAt != nil

	if tcpFreshOK && probeFreshOK {
		tcpAge := p.now.Sub(*row.TCPLastSeenAt)
		probeAge := p.now.Sub(*row.ProbeLastSeenAt)
		if tcpAge > stalePassiveAge && row.TCPSampleCount < stalePassiveMinSamples &&
			probeAge <= freshProbeMaxAge && row.ProbeSampleCount >= freshProbeMinSamples {
			return row.ProbeRTTP50Ms, row.ProbeRTTP50Ms, activeSourceName(row.ProbeProto), uint64(row.ProbeSampleCount), row.ProbeLastSeenAt //nolint:gosec // > 0
		}
	}

	if tcpFreshOK {
		rtt, p95, samples := selectDirectTCPMetrics(row)
		return rtt, p95, model.NetworkQualitySourceTCPPassive, samples, row.TCPLastSeenAt
	}
	if probeFreshOK {
		rtt := row.ProbeRTTP50Ms
		if row.EffectiveRTTMs > 0 && row.MeasurementSource == activeSourceName(row.ProbeProto) {
			rtt = row.EffectiveRTTMs
		}
		return rtt, rtt, activeSourceName(row.ProbeProto), uint64(max(1, row.ProbeSampleCount)), row.ProbeLastSeenAt //nolint:gosec // >= 1
	}
	if row.EffectiveRTTMs > 0 {
		obs := row.LastObservedAt
		if obs == nil {
			t := row.UpdatedAt
			obs = &t
		}
		cnt := uint64(max(1, row.TCPSampleCount+row.ProbeSampleCount+row.TraceSampleCount)) //nolint:gosec // >= 1
		return row.EffectiveRTTMs, row.EffectiveRTTMs, row.MeasurementSource, cnt, obs
	}
	return 0, 0, model.MeasurementSourceNone, 0, nil
}

func selectDirectTCPMetrics(row *model.OpenFlareNetworkQualityMatrix) (rtt, p95 float64, samples uint64) {
	rtt = row.TCPRTTP50Ms
	if row.EffectiveRTTMs > 0 && (row.MeasurementSource == "" || row.MeasurementSource == model.NetworkQualitySourceTCPPassive) {
		rtt = row.EffectiveRTTMs
	}
	p95 = row.TCPRTTP95Ms
	if row.ShortRTTP95Ms > 0 && row.ShortSampleCount >= p95MinSamples {
		p95 = row.ShortRTTP95Ms
	}
	if p95 < rtt {
		p95 = rtt
	}
	sampleCnt := row.TCPSampleCount
	if row.ShortSampleCount > 0 && row.EffectiveRTTMs > 0 && row.QualityState == model.QualityStateDegraded {
		sampleCnt = row.ShortSampleCount
	}
	return rtt, p95, uint64(max(1, sampleCnt)) //nolint:gosec // >= 1
}

func activeSourceName(proto string) string {
	switch strings.ToLower(strings.TrimSpace(proto)) {
	case probeProtoTCP:
		return model.NetworkQualitySourceTCPActive
	case probeProtoICMP:
		return model.NetworkQualitySourceICMPActive
	case model.NetworkQualitySourceTraceroute:
		return model.NetworkQualitySourceTraceroute
	default:
		return model.NetworkQualitySourceICMPActive
	}
}

type weightedCandidate struct {
	extra  float64
	weight float64
	row    *model.OpenFlareNetworkQualityMatrix
}

func (p *Predictor) predictInferredUpstream(q PredictQuery, targetGeoBase float64, geoKnown bool) (PredictionResult, bool) {
	rows := p.rowsByNode[q.NodeID]
	if len(rows) == 0 || !geoKnown || targetGeoBase < 0 {
		return PredictionResult{}, false
	}

	targetProf := p.getASProfile(q.ClientASN)
	candidates := make([]weightedCandidate, 0, len(rows))
	var totalWeight float64
	var latestObs *time.Time
	var totalSamples uint64

	for _, row := range rows {
		cand, obs, ok := p.scoreCandidateRow(q, targetProf, row)
		if !ok {
			continue
		}
		candidates = append(candidates, cand)
		totalWeight += cand.weight
		totalSamples += uint64(max(1, row.TCPSampleCount+row.ProbeSampleCount+row.TraceSampleCount)) //nolint:gosec // >= 1
		latestObs = maxTimePtr(latestObs, obs)
	}

	if len(candidates) == 0 || totalWeight < inferredMinTotalWeight {
		return PredictionResult{}, false
	}

	medExtra, iqrExtra := weightedMedianAndIQR(candidates, totalWeight)
	inferredRTT := targetGeoBase + medExtra
	priorRTT := targetGeoBase*geoPriorDetourFactor + geoPriorAddBaseMs
	alpha := totalWeight / (totalWeight + inferredShrinkageLambda)
	finalRTT := math.Max(minPredictedRTTMs, alpha*inferredRTT+(1.0-alpha)*priorRTT)

	conf := inferredBaseConfidence + inferredConfWeightSpan*alpha*(1.0/(1.0+iqrExtra/inferredDispersionScaleMs))
	conf = math.Max(inferredMinConfidence, math.Min(inferredMaxConfidence, conf))

	return PredictionResult{
		RTTMs:                 round2(finalRTT),
		RTTP95Ms:              round2(finalRTT + inferredP95SpreadWeight*iqrExtra),
		CandidateDispersionMs: round2(iqrExtra),
		Confidence:            round4(conf),
		MeasurementSource:     model.MeasurementSourceNone,
		PredictionMethod:      PredictionMethodInferredUpstream,
		GeoBaseRTTMs:          round2(targetGeoBase),
		GeoBaseKnown:          true,
		SampleCount:           totalSamples,
		LastObservedAt:        latestObs,
		AnycastIsolated:       q.AnycastStatus == pkgnetintel.AnycastStatusConfirmed,
	}, true
}

func (p *Predictor) scoreCandidateRow(
	q PredictQuery,
	targetProf pkgnetintel.ASUpstreamProfile,
	row *model.OpenFlareNetworkQualityMatrix,
) (weightedCandidate, time.Time, bool) {
	if row == nil || row.EffectiveRTTMs <= 0 || !hasKnownRowGeoBase(row) {
		return weightedCandidate{}, time.Time{}, false
	}
	if row.AggregationLevel == model.QualityAggregationLevelRegion && q.ClientCountry != row.ClientCountry {
		return weightedCandidate{}, time.Time{}, false
	}
	obs := row.LastObservedAt
	var obsTime time.Time
	if obs != nil {
		obsTime = *obs
	} else {
		obsTime = row.UpdatedAt
	}
	age := max(time.Duration(0), p.now.Sub(obsTime))
	if age > inferredMaxCandidateAge {
		return weightedCandidate{}, time.Time{}, false
	}

	extra := row.EffectiveRTTMs - row.GeoBaseRTTMs
	wGeo := computeCandidateGeoWeight(q, row, extra < 0)
	wNet := p.computeCandidateNetWeight(q.ClientASN, targetProf, row)
	wFam := 1.0
	if q.IPFamily != pkgnetintel.IPFamilyUnknown && row.IPFamily != pkgnetintel.IPFamilyUnknown && q.IPFamily != row.IPFamily {
		wFam = crossFamilyWeight
	}
	wFresh := math.Exp(-math.Ln2 * age.Hours() / candidateFreshHalfLifeH)
	wQual := math.Max(candidateMinQualityW, math.Min(1.0, row.Confidence))

	w := wGeo * wNet * wFam * wFresh * wQual
	if w <= 0 {
		return weightedCandidate{}, time.Time{}, false
	}
	return weightedCandidate{
		extra:  extra,
		weight: w,
		row:    row,
	}, obsTime, true
}

func computeCandidateGeoWeight(q PredictQuery, row *model.OpenFlareNetworkQualityMatrix, negativeResidual bool) float64 {
	var w float64
	switch {
	case q.ClientRegion != "" && strings.EqualFold(q.ClientRegion, row.ClientRegion):
		w = geoWeightSameRegion
	case q.ClientCountry != "" && strings.EqualFold(q.ClientCountry, row.ClientCountry):
		w = geoWeightSameCountry
	case q.ClientCountry != "" && row.ClientCountry != "" && hasKnownRowGeoBase(row):
		w = geoWeightRegional
	default:
		w = geoWeightCrossRegion
	}
	if negativeResidual {
		w *= negativeResidualGeoFactor
	}
	return w
}

func (p *Predictor) computeCandidateNetWeight(targetASN uint32, targetProf pkgnetintel.ASUpstreamProfile, row *model.OpenFlareNetworkQualityMatrix) float64 {
	if targetASN == 0 || row.ClientASN == 0 {
		return unknownNetSimWeight
	}
	if targetASN == row.ClientASN {
		return 1.0
	}
	candProf := p.getASProfile(row.ClientASN)
	sim := pkgnetintel.ComputeNetworkSimilarity(targetProf, candProf, 0)
	if !sim.Known {
		return unknownNetSimWeight
	}
	return netSimBaseWeight + netSimSpanWeight*sim.Score
}

func (p *Predictor) predictFromSimilarNode(q PredictQuery, targetGeoBase float64, geoKnown bool) (PredictionResult, bool) {
	curProf, ok := p.nodeProfiles[q.NodeID]
	if !ok || curProf.GeoLatitude == nil || curProf.GeoLongitude == nil {
		return PredictionResult{}, false
	}

	type siblingCand struct {
		nodeID string
		weight float64
	}
	var siblings []siblingCand
	for id, prof := range p.nodeProfiles {
		if id == q.NodeID || len(p.rowsByNode[id]) == 0 || prof.GeoLatitude == nil || prof.GeoLongitude == nil {
			continue
		}
		dist := pkgnetintel.DistanceKm(*curProf.GeoLatitude, *curProf.GeoLongitude, *prof.GeoLatitude, *prof.GeoLongitude)
		if dist > siblingMaxDistanceKm || curProf.NetworkGroup != prof.NetworkGroup {
			continue
		}
		if curProf.NetworkGroup != "" {
			siblings = append(siblings, siblingCand{nodeID: id, weight: siblingSameGroupW})
			continue
		}
		if curProf.ASN > 0 && curProf.ASN == prof.ASN {
			siblings = append(siblings, siblingCand{nodeID: id, weight: siblingSameASNW})
		}
	}
	if len(siblings) == 0 {
		return PredictionResult{}, false
	}
	sort.Slice(siblings, func(i, j int) bool {
		if siblings[i].weight != siblings[j].weight {
			return siblings[i].weight > siblings[j].weight
		}
		return siblings[i].nodeID < siblings[j].nodeID
	})

	subQ := q
	subQ.NodeID = siblings[0].nodeID
	res := p.predictUncached(subQ, false)
	if res.PredictionMethod == PredictionMethodGeoPrior {
		return PredictionResult{}, false
	}
	res.PredictionMethod = PredictionMethodSimilarNode
	res.MeasurementSource = model.MeasurementSourceNone
	res.Confidence = round4(math.Max(minSampleConfidence, res.Confidence*siblingConfMultiplier*siblings[0].weight))
	if geoKnown {
		res.GeoBaseRTTMs = round2(targetGeoBase)
		res.GeoBaseKnown = true
	}
	return res, true
}

func buildGeoPriorPrediction(targetGeoBase, geoConf float64, geoKnown, anycastIsolated bool) PredictionResult {
	if !geoKnown || targetGeoBase < 0 {
		return PredictionResult{
			RTTMs:             unknownGeoPriorMs,
			RTTP95Ms:          unknownGeoPriorMs,
			Confidence:        geoPriorUnknownConf,
			MeasurementSource: model.MeasurementSourceNone,
			PredictionMethod:  PredictionMethodGeoPrior,
			GeoBaseRTTMs:      0,
			GeoBaseKnown:      false,
			AnycastIsolated:   anycastIsolated,
		}
	}
	rtt := math.Max(minPredictedRTTMs, targetGeoBase*geoPriorDetourFactor+geoPriorAddBaseMs)
	conf := math.Max(geoPriorUnknownConf, math.Min(geoPriorMaxConfidence, geoPriorConfScale*max(geoPriorMinConfFloor, geoConf)))
	return PredictionResult{
		RTTMs:             round2(rtt),
		RTTP95Ms:          round2(rtt * geoPriorP95Multiplier),
		Confidence:        round4(conf),
		MeasurementSource: model.MeasurementSourceNone,
		PredictionMethod:  PredictionMethodGeoPrior,
		GeoBaseRTTMs:      round2(targetGeoBase),
		GeoBaseKnown:      true,
		AnycastIsolated:   anycastIsolated,
	}
}

func (p *Predictor) computeTargetGeoBase(q PredictQuery) (baseRTT, geoConf float64, geoKnown bool) {
	node, ok := p.nodes[q.NodeID]
	if !ok || node.GeoLatitude == nil || node.GeoLongitude == nil || !isValidCoord(*node.GeoLatitude, *node.GeoLongitude) {
		return 0, 0, false
	}
	cLat, cLon, cSource, cAccKm := resolveTargetGeoCoords(q)
	if cLat == nil || cLon == nil || !isValidCoord(*cLat, *cLon) {
		return 0, 0, false
	}
	distKm := pkgnetintel.DistanceKm(*node.GeoLatitude, *node.GeoLongitude, *cLat, *cLon)
	baseRTT = pkgnetintel.GeoBaseRTTMs(distKm)
	nodeConf := pkgnetintel.GeoLocationConfidence(resolveNodeGeoSource(node), defaultCityAccuracyKm)
	clientConf := pkgnetintel.GeoLocationConfidence(cSource, cAccKm)
	return baseRTT, math.Min(nodeConf, clientConf), true
}

func resolveTargetGeoCoords(q PredictQuery) (lat, lon *float64, geoSource string, accKm float64) {
	if q.ClientLat != nil && q.ClientLon != nil && isValidCoord(*q.ClientLat, *q.ClientLon) {
		return q.ClientLat, q.ClientLon, q.GeoSource, q.GeoAccuracyKm
	}
	if q.ClientCountry != "" {
		if cLat, cLon, ok := pkggeoip.CountryCentroidByISO(q.ClientCountry); ok {
			return &cLat, &cLon, pkgnetintel.GeoSourceCountry, minGeoSourceWeightFallback
		}
	}
	return nil, nil, pkgnetintel.GeoSourceUnknown, 0
}

func (p *Predictor) getASProfile(asn uint32) pkgnetintel.ASUpstreamProfile {
	if asn == 0 {
		return pkgnetintel.ASUpstreamProfile{}
	}
	if prof, ok := p.asProfiles[asn]; ok {
		return prof
	}
	prof := netintel.ExtractASUpstreamProfile(asn)
	p.asProfiles[asn] = prof
	return prof
}

func (p *Predictor) attachError(res PredictionResult) PredictionResult {
	if p.calibration != nil {
		res.ErrorMs = p.calibration.Lookup(res.PredictionMethod, res.Confidence)
	}
	return res
}

func weightedMedianAndIQR(cands []weightedCandidate, totalW float64) (median, iqr float64) {
	if len(cands) == 0 || totalW <= 0 {
		return 0, 0
	}
	sorted := make([]weightedCandidate, len(cands))
	copy(sorted, cands)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].extra < sorted[j].extra
	})
	q25 := weightedQuantile(sorted, totalW, quartileLower)
	q50 := weightedQuantile(sorted, totalW, quartileMedian)
	q75 := weightedQuantile(sorted, totalW, quartileUpper)
	return q50, math.Max(0, q75-q25)
}

func weightedQuantile(sorted []weightedCandidate, totalW, q float64) float64 {
	target := q * totalW
	var cum float64
	for _, c := range sorted {
		cum += c.weight
		if cum >= target {
			return c.extra
		}
	}
	return sorted[len(sorted)-1].extra
}

func normalizePredictorIPFamily(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case pkgnetintel.IPFamilyIPv4:
		return pkgnetintel.IPFamilyIPv4
	case pkgnetintel.IPFamilyIPv6:
		return pkgnetintel.IPFamilyIPv6
	default:
		return pkgnetintel.IPFamilyUnknown
	}
}

func normalizePredictorAggLevel(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), model.QualityAggregationLevelRegion) {
		return model.QualityAggregationLevelRegion
	}
	return model.QualityAggregationLevelCountry
}

// ComputeQualityScore computes a [1, 100] node ranking score from a PredictionResult (keeping RTTMs unpenalized).
func ComputeQualityScore(res PredictionResult) float64 {
	rtt := res.RTTMs
	if rtt <= 0 {
		rtt = unknownGeoPriorMs
	}
	if res.CandidateDispersionMs > 0 {
		rtt += qualityDispersionShare * res.CandidateDispersionMs
	}
	return round2(math.Max(minQualityScore, maxQualityScore/(1.0+rtt/qualityScoreRTTScale)))
}
