// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netquality

import (
	"math"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

const (
	rollupSamplePageSize    = 5000
	maxRollupTotalSamples   = 100000
	instantLatencyWindow    = 10 * time.Minute
	shortLatencyWindow      = 45 * time.Minute
	shortHalfLifeMinutes    = 20.0
	instantMinSamples       = 5
	shortMinSamples         = 5
	recoveryMinSamples      = 3
	recoveryConfirmWindows  = 2
	qualityStaleAge         = 60 * time.Minute
	confidenceFreshGrace    = 30 * time.Minute
	staleConfidenceHalfLife = 2.0

	degradeInstantRatio   = 1.35
	degradeInstantDeltaMs = 15.0
	degradeShortRatio     = 1.30
	degradeShortDeltaMs   = 12.0
	degradeRetransFloor   = 0.05
	degradeRetransDelta   = 0.03
	recoverRatioCeiling   = 1.25
	recoverDeltaCeilingMs = 15.0
	predictedMinRTTFactor = 1.15

	p95MaxTailWeight      = 0.35
	p95MinSamples         = 10
	p95SampleHalfScale    = 20.0
	p95PrefixHalfScale    = 3.0
	p95PrefixFactorBase   = 0.40
	p95PrefixFactorSpan   = 0.60
	p95LegacyPrefixFactor = 0.50

	exploreBestTolerance    = 1.05
	exploreCooldownWindow   = 15 * time.Minute
	exploreEnoughSamples    = 5
	exploreMaxRatio         = 1.50
	exploreMaxOverheadMs    = 30.0
	exploreMinUpside        = 0.15
	exploreStaleMinNeed     = 0.40
	exploreStaleMaxNeed     = 0.95
	exploreDegradedBaseNeed = 0.25
	exploreDegradedAgeScale = 0.20
	exploreDegradedMaxNeed  = 0.85
	exploreHealthyStaleAge  = 30 * time.Minute
	exploreHealthyNeedScale = 0.55
	exploreMinPriority      = 0.0
	exploreMaxPriority      = 1.0
	minOnlineNodesToExplore = 2
	minutesPerHour          = 60.0
	floatEqualityEpsilon    = 1e-9
)

type windowBucket struct {
	hist      WeightedLogHistogram64
	weightSum float64
	lossSum   float64
	samples   int
	legWeight float64
	legP50Sum float64
	legP95Sum float64
	prefixes  PrefixSketch
	legMaxPfx int
}

type windowGroup struct {
	tcp        windowBucket
	tcpActive  windowBucket
	icmpActive windowBucket
	trace      windowBucket
}

func (w *windowGroup) bucketForSource(source string) *windowBucket {
	switch source {
	case model.NetworkQualitySourceTCPPassive:
		return &w.tcp
	case model.NetworkQualitySourceTCPActive:
		return &w.tcpActive
	case model.NetworkQualitySourceICMPActive:
		return &w.icmpActive
	default:
		return &w.trace
	}
}

type windowMetrics struct {
	p50      float64
	p95      float64
	loss     float64
	samples  int
	prefixes int
}

func applySampleToWindowGroup(
	wg *windowGroup,
	s *analyticsmodel.NetworkQualitySample,
	decodedHist *LogHistogram64,
	decay, bucketWeight float64,
	sampleCount int,
) {
	b := wg.bucketForSource(s.Source)
	b.samples += sampleCount
	addHistOrLegacy(decodedHist, &b.hist, decay, s.RTTP50Ms, bucketWeight, &b.legWeight, &b.legP50Sum)
	if decodedHist == nil && s.RTTP50Ms > 0 {
		p95Val := s.RTTP95Ms
		if p95Val < s.RTTP50Ms {
			p95Val = s.RTTP50Ms
		}
		b.legP95Sum += p95Val * bucketWeight
	}
	b.weightSum += bucketWeight
	if s.Source == model.NetworkQualitySourceTCPPassive {
		b.lossSum += s.RetransRate * bucketWeight
	} else {
		b.lossSum += s.LossRate * bucketWeight
	}
	if strings.TrimSpace(s.PrefixSketch) != "" {
		if sk, err := DecodePrefixSketch(s.PrefixSketch); err == nil {
			b.prefixes.Union(sk)
		}
	} else if int(s.DistinctPrefixCount) > b.legMaxPfx {
		b.legMaxPfx = int(s.DistinctPrefixCount)
	}
}

func resolveWindowMetrics(wg *windowGroup, measurementSource string) windowMetrics {
	if wg == nil || measurementSource == "" || measurementSource == model.MeasurementSourceNone {
		return windowMetrics{}
	}
	b := wg.bucketForSource(measurementSource)
	if b.samples <= 0 {
		return windowMetrics{}
	}
	var p50, p95, loss float64
	if b.hist.TotalWeight > 0 {
		p50 = b.hist.Quantile(percentileMedian)
		p95 = b.hist.Quantile(percentileP95)
	} else if b.legWeight > 0 {
		p50 = b.legP50Sum / b.legWeight
		p95 = b.legP95Sum / b.legWeight
	}
	if p95 < p50 {
		p95 = p50
	}
	if b.weightSum > 0 {
		loss = b.lossSum / b.weightSum
	}
	prefixes := b.prefixes.DistinctCount()
	if prefixes == 0 && b.legMaxPfx > 0 {
		prefixes = b.legMaxPfx
	}
	return windowMetrics{
		p50:      p50,
		p95:      p95,
		loss:     loss,
		samples:  b.samples,
		prefixes: prefixes,
	}
}

// computeDynamicP95Weight scales the P95 tail penalty smoothly in [0, 0.35] based on sample count and distinct prefixes.
func computeDynamicP95Weight(sampleCount, distinctPrefixes int) float64 {
	if sampleCount < p95MinSamples {
		return 0
	}
	sampleFactor := 1.0 - math.Exp(-float64(sampleCount-p95MinSamples+1)/p95SampleHalfScale)
	prefixFactor := p95LegacyPrefixFactor
	if distinctPrefixes > 0 {
		prefixFactor = p95PrefixFactorBase + p95PrefixFactorSpan*(1.0-math.Exp(-float64(distinctPrefixes)/p95PrefixHalfScale))
	}
	return round4(math.Min(p95MaxTailWeight, p95MaxTailWeight*sampleFactor*prefixFactor))
}

func ageFreshnessFactor(age time.Duration) float64 {
	if age <= confidenceFreshGrace {
		return 1.0
	}
	excessHours := (age - confidenceFreshGrace).Hours()
	return math.Exp(-math.Ln2 * excessHours / staleConfidenceHalfLife)
}

type adaptiveInputs struct {
	baselineRTT       float64
	baselineP95       float64
	baselineLoss      float64
	baselineSamples   int
	baselinePrefixes  int
	rttEWMA           float64
	tcpMinRTT         float64
	geoBaseRTT        float64
	geoBaseKnown      bool
	measurementSource string
	lastObservedAt    *time.Time
}

type adaptiveResolution struct {
	effectiveRTT           float64
	observedRTT            float64
	predictedRTT           float64
	shortP50               float64
	shortP95               float64
	shortSamples           int
	activeP95              float64
	activeLoss             float64
	activeSamples          int
	activePrefixes         int
	qualityState           string
	consecutiveGoodWindows int
	age                    time.Duration
}

func resolveAdaptiveQualityState(
	acc *pairAccum,
	prev *model.OpenFlareNetworkQualityMatrix,
	now time.Time,
	in adaptiveInputs,
) adaptiveResolution {
	short := resolveWindowMetrics(&acc.shortWin, in.measurementSource)
	instant := resolveWindowMetrics(&acc.instantWin, in.measurementSource)
	predictedRTT := computePredictedBaseRTT(in.tcpMinRTT, in.geoBaseRTT, in.geoBaseKnown, in.baselineRTT)

	var age time.Duration
	if in.lastObservedAt != nil && now.After(*in.lastObservedAt) {
		age = now.Sub(*in.lastObservedAt)
	}

	res := adaptiveResolution{
		predictedRTT:   predictedRTT,
		shortP50:       short.p50,
		shortP95:       short.p95,
		shortSamples:   short.samples,
		activeP95:      in.baselineP95,
		activeLoss:     in.baselineLoss,
		activeSamples:  in.baselineSamples,
		activePrefixes: in.baselinePrefixes,
		qualityState:   model.QualityStateHealthy,
		age:            age,
	}

	if in.lastObservedAt != nil && age > qualityStaleAge {
		return resolveStaleState(res, prev, in)
	}
	return resolveFreshState(res, prev, in, short, instant)
}

func computePredictedBaseRTT(tcpMinRTT, geoBaseRTT float64, geoBaseKnown bool, baselineRTT float64) float64 {
	if tcpMinRTT > 0 {
		pred := tcpMinRTT * predictedMinRTTFactor
		if geoBaseKnown && geoBaseRTT > 0 && pred < geoBaseRTT {
			pred = geoBaseRTT
		}
		if baselineRTT > 0 && baselineRTT < pred {
			pred = baselineRTT
		}
		return pred
	}
	if geoBaseKnown && geoBaseRTT > 0 {
		return geoBaseRTT*geoPriorDetourFactor + geoPriorAddBaseMs
	}
	return baselineRTT
}

func resolveStaleState(
	res adaptiveResolution,
	prev *model.OpenFlareNetworkQualityMatrix,
	in adaptiveInputs,
) adaptiveResolution {
	res.qualityState = model.QualityStateStale
	res.consecutiveGoodWindows = 0

	// Never lower effective/observed RTT due to staleness; preserve prior impaired RTT when present.
	if prev != nil && prev.EffectiveRTTMs > 0 && isImpairedState(prev.QualityState) {
		res.effectiveRTT = prev.EffectiveRTTMs
		res.observedRTT = prev.ObservedRTTMs
		if res.observedRTT <= 0 {
			res.observedRTT = prev.EffectiveRTTMs
		}
		if prev.ShortSampleCount > 0 {
			res.activeSamples = prev.ShortSampleCount
		}
		if prev.ShortRTTP95Ms > res.activeP95 {
			res.activeP95 = prev.ShortRTTP95Ms
		}
		return res
	}
	if in.rttEWMA > 0 {
		res.observedRTT = in.rttEWMA
		res.effectiveRTT = in.rttEWMA
		return res
	}
	res.observedRTT = in.baselineRTT
	res.effectiveRTT = in.baselineRTT
	return res
}

func resolveFreshState(
	res adaptiveResolution,
	prev *model.OpenFlareNetworkQualityMatrix,
	in adaptiveInputs,
	short, instant windowMetrics,
) adaptiveResolution {
	refRTT := res.predictedRTT
	if refRTT <= 0 {
		refRTT = in.baselineRTT
	}

	instantDegraded := isWindowDegraded(instant, instantMinSamples, degradeInstantRatio, degradeInstantDeltaMs, in.baselineRTT, in.baselineLoss)
	instantHealthy := instant.samples >= recoveryMinSamples && instant.p50 > 0 &&
		!instantDegraded && isWindowHealthy(instant, refRTT)
	shortDegraded := !instantHealthy && isWindowDegraded(short, shortMinSamples, degradeShortRatio, degradeShortDeltaMs, in.baselineRTT, in.baselineLoss)

	if instantDegraded {
		return applyDegradedResolution(res, instant)
	}
	if shortDegraded {
		return applyDegradedResolution(res, short)
	}
	if wasPreviouslyImpaired(prev, res.predictedRTT) {
		return resolveRecoveryState(res, prev, in, short, instant, refRTT)
	}
	return applyHealthyResolution(res, in, short)
}

func isImpairedState(state string) bool {
	return state == model.QualityStateDegraded ||
		state == model.QualityStateRecovering ||
		state == model.QualityStateStale
}

func isWindowDegraded(w windowMetrics, minSamples int, ratio, delta, baselineRTT, baselineLoss float64) bool {
	if w.samples < minSamples || w.p50 <= 0 {
		return false
	}
	rttSpike := baselineRTT > 0 && w.p50 >= baselineRTT*ratio && (w.p50-baselineRTT) >= delta
	lossSpike := w.loss >= degradeRetransFloor && w.loss >= baselineLoss+degradeRetransDelta
	return rttSpike || lossSpike
}

func isWindowHealthy(w windowMetrics, refRTT float64) bool {
	if w.p50 <= 0 {
		return false
	}
	if w.loss >= degradeRetransFloor {
		return false
	}
	if refRTT <= 0 {
		return true
	}
	return w.p50 <= refRTT*recoverRatioCeiling+recoverDeltaCeilingMs
}

func wasPreviouslyImpaired(prev *model.OpenFlareNetworkQualityMatrix, predictedRTT float64) bool {
	if prev == nil {
		return false
	}
	if prev.QualityState == model.QualityStateDegraded || prev.QualityState == model.QualityStateRecovering {
		return true
	}
	if prev.QualityState == model.QualityStateStale && prev.EffectiveRTTMs > 0 && predictedRTT > 0 {
		return prev.EffectiveRTTMs >= predictedRTT*degradeShortRatio && (prev.EffectiveRTTMs-predictedRTT) >= degradeShortDeltaMs
	}
	return false
}

func applyDegradedResolution(res adaptiveResolution, w windowMetrics) adaptiveResolution {
	res.qualityState = model.QualityStateDegraded
	res.consecutiveGoodWindows = 0
	res.observedRTT = w.p50
	res.effectiveRTT = w.p50
	if w.p95 > 0 {
		res.activeP95 = w.p95
	} else {
		res.activeP95 = w.p50
	}
	res.activeLoss = w.loss
	res.activeSamples = w.samples
	if w.prefixes > 0 {
		res.activePrefixes = w.prefixes
	}
	return res
}

func resolveRecoveryState(
	res adaptiveResolution,
	prev *model.OpenFlareNetworkQualityMatrix,
	in adaptiveInputs,
	short, instant windowMetrics,
	refRTT float64,
) adaptiveResolution {
	freshest := selectFreshestRecoveryWindow(short, instant)
	if freshest.samples == 0 || freshest.p50 <= 0 {
		res.qualityState = prev.QualityState
		if res.qualityState == "" || res.qualityState == model.QualityStateStale {
			res.qualityState = model.QualityStateDegraded
		}
		res.consecutiveGoodWindows = prev.ConsecutiveGoodWindows
		res.observedRTT = prev.ObservedRTTMs
		res.effectiveRTT = prev.EffectiveRTTMs
		if res.effectiveRTT <= 0 {
			res.effectiveRTT = in.baselineRTT
		}
		if res.observedRTT <= 0 {
			res.observedRTT = res.effectiveRTT
		}
		return res
	}

	if !isWindowHealthy(freshest, refRTT) {
		return applyDegradedResolution(res, freshest)
	}

	hasFreshEvidence := prev.LastObservedAt == nil || (in.lastObservedAt != nil && in.lastObservedAt.After(*prev.LastObservedAt))
	nextGood := prev.ConsecutiveGoodWindows
	if hasFreshEvidence {
		nextGood++
	}
	res.consecutiveGoodWindows = nextGood
	res.observedRTT = freshest.p50
	if freshest.p95 > 0 {
		res.activeP95 = freshest.p95
	}
	res.activeLoss = freshest.loss
	res.activeSamples = freshest.samples
	if freshest.prefixes > 0 {
		res.activePrefixes = freshest.prefixes
	}

	if nextGood >= recoveryConfirmWindows {
		res.qualityState = model.QualityStateHealthy
		res.effectiveRTT = freshest.p50
		return res
	}

	// First confirmation window: record good observed RTT, keep conservative effective RTT.
	res.qualityState = model.QualityStateRecovering
	res.effectiveRTT = prev.EffectiveRTTMs
	if res.effectiveRTT <= 0 {
		res.effectiveRTT = in.baselineRTT
	}
	return res
}

func selectFreshestRecoveryWindow(short, instant windowMetrics) windowMetrics {
	if instant.samples >= recoveryMinSamples && instant.p50 > 0 {
		return instant
	}
	if short.samples >= recoveryMinSamples && short.p50 > 0 {
		return short
	}
	return windowMetrics{}
}

func applyHealthyResolution(res adaptiveResolution, in adaptiveInputs, short windowMetrics) adaptiveResolution {
	res.qualityState = model.QualityStateHealthy
	res.consecutiveGoodWindows = 0

	if short.samples >= shortMinSamples && short.p50 > 0 {
		res.observedRTT = short.p50
		res.effectiveRTT = short.p50
		if short.p95 > 0 {
			res.activeP95 = short.p95
		}
		res.activeLoss = short.loss
		res.activeSamples = short.samples
		if short.prefixes > 0 {
			res.activePrefixes = short.prefixes
		}
		return res
	}

	if short.samples > 0 && short.p50 > 0 && in.baselineRTT > 0 {
		if math.Abs(short.p50-in.baselineRTT) < floatEqualityEpsilon {
			res.observedRTT = in.baselineRTT
			res.effectiveRTT = in.baselineRTT
		} else {
			w := float64(short.samples) / float64(shortMinSamples)
			blended := w*short.p50 + (1.0-w)*in.baselineRTT
			res.observedRTT = blended
			res.effectiveRTT = blended
		}
		return res
	}

	res.observedRTT = in.baselineRTT
	res.effectiveRTT = in.baselineRTT
	return res
}

type exploreSliceKey struct {
	clientASN        uint32
	clientCountry    string
	clientRegion     string
	ipFamily         string
	aggregationLevel string
}

// assignExplorePriorities computes per-slice exploration priorities so SmartDNS focuses
// ECS canary traffic on competitive stale/recovering/cooled-off links instead of hopeless paths.
func assignExplorePriorities(rows []*model.OpenFlareNetworkQualityMatrix, nodeByID map[string]model.OpenFlareNode, now time.Time) {
	if len(rows) == 0 {
		return
	}
	slices := make(map[exploreSliceKey][]*model.OpenFlareNetworkQualityMatrix)
	for _, r := range rows {
		if r == nil || r.NodeID == "" || r.ClientASN == 0 {
			continue
		}
		r.ExplorePriority = exploreMinPriority
		k := exploreSliceKey{
			clientASN:        r.ClientASN,
			clientCountry:    r.ClientCountry,
			clientRegion:     r.ClientRegion,
			ipFamily:         r.IPFamily,
			aggregationLevel: r.AggregationLevel,
		}
		slices[k] = append(slices[k], r)
	}

	for _, group := range slices {
		assignSliceExplorePriorities(group, nodeByID, now)
	}
}

func assignSliceExplorePriorities(group []*model.OpenFlareNetworkQualityMatrix, nodeByID map[string]model.OpenFlareNode, now time.Time) {
	bestRTT := math.Inf(1)
	onlineCount := 0
	for _, r := range group {
		n, ok := nodeByID[r.NodeID]
		if !ok || !isNodeOnlineForExplore(n, now) || r.EffectiveRTTMs <= 0 {
			continue
		}
		onlineCount++
		if r.EffectiveRTTMs < bestRTT {
			bestRTT = r.EffectiveRTTMs
		}
	}
	if onlineCount < minOnlineNodesToExplore || math.IsInf(bestRTT, 1) {
		return
	}

	for _, r := range group {
		n, ok := nodeByID[r.NodeID]
		if !ok || !isNodeOnlineForExplore(n, now) {
			continue
		}
		r.ExplorePriority = computeRowExplorePriority(r, bestRTT, now)
	}
}

func isNodeOnlineForExplore(n model.OpenFlareNode, now time.Time) bool {
	if n.IsQuarantined() {
		return false
	}
	if n.NodeType != "" && n.NodeType != "edge_node" {
		return false
	}
	return n.Status == "online" || (n.LastSeenAt != nil && now.Sub(*n.LastSeenAt) <= onlineWindow)
}

func computeRowExplorePriority(r *model.OpenFlareNetworkQualityMatrix, bestRTT float64, now time.Time) float64 {
	var age time.Duration
	if r.LastObservedAt != nil && now.After(*r.LastObservedAt) {
		age = now.Sub(*r.LastObservedAt)
	}

	// Current primary healthy node needs no exploration budget.
	if r.EffectiveRTTMs > 0 && r.EffectiveRTTMs <= bestRTT*exploreBestTolerance &&
		r.QualityState == model.QualityStateHealthy && age <= qualityStaleAge {
		return exploreMinPriority
	}

	// Auto-brake cooldown after sufficient fresh samples (except recovering links awaiting confirmation).
	if r.QualityState != model.QualityStateRecovering && age < exploreCooldownWindow && r.ShortSampleCount >= exploreEnoughSamples {
		return exploreMinPriority
	}

	pred := r.PredictedRTTMs
	if pred <= 0 {
		pred = r.EffectiveRTTMs
	}
	maxCompetitive := bestRTT*exploreMaxRatio + exploreMaxOverheadMs
	if pred <= 0 || pred > maxCompetitive {
		return exploreMinPriority
	}

	upside := clampUnit((maxCompetitive-pred)/(bestRTT+exploreMaxOverheadMs), exploreMinUpside, exploreMaxPriority)
	need := computeExploreNeed(r, age)
	if need <= 0 {
		return exploreMinPriority
	}
	return round4(clampUnit(upside*need, exploreMinPriority, exploreMaxPriority))
}

func computeExploreNeed(r *model.OpenFlareNetworkQualityMatrix, age time.Duration) float64 {
	switch r.QualityState {
	case model.QualityStateRecovering:
		return exploreMaxPriority
	case model.QualityStateStale:
		return clampUnit(1.0-r.Confidence, exploreStaleMinNeed, exploreStaleMaxNeed)
	case model.QualityStateDegraded:
		return clampUnit(
			exploreDegradedBaseNeed+exploreDegradedAgeScale*(age.Minutes()/minutesPerHour),
			exploreDegradedBaseNeed,
			exploreDegradedMaxNeed,
		)
	default:
		if age >= exploreHealthyStaleAge {
			return clampUnit(exploreHealthyNeedScale*(1.0-r.Confidence), exploreMinPriority, exploreMaxPriority)
		}
		return exploreMinPriority
	}
}

func clampUnit(v, minV, maxV float64) float64 {
	if math.IsNaN(v) || v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
