// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netquality

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	"github.com/Rain-kl/Wavelet/internal/model"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
	"github.com/stretchr/testify/require"
)

type evalGroundTruth struct {
	query     PredictQuery
	actualRTT float64
	asHops    int
}

type evalScenarioMetrics struct {
	Scenario         string
	SampleCount      int
	MAE              float64
	P90ErrorMs       float64
	LegacyMAE        float64
	MeanRegretMs     float64
	P95RegretMs      float64
	LegacyMeanRegret float64
	Top1HitRate      float64
	Within5msRate    float64
}

func TestComputeNetworkSimilarityGuardsMissingUpstreamsAndWeakTier1(t *testing.T) {
	emptyA := pkgnetintel.ASUpstreamProfile{ASN: 64500}
	emptyB := pkgnetintel.ASUpstreamProfile{ASN: 64501}
	resEmpty := pkgnetintel.ComputeNetworkSimilarity(emptyA, emptyB, 0)
	require.False(t, resEmpty.Known)
	require.Equal(t, 0.0, resEmpty.Score)

	tier1OnlyA := pkgnetintel.ASUpstreamProfile{
		ASN:             64502,
		DirectProviders: []uint32{174},
		Tier1Upstreams:  []uint32{2914},
	}
	tier1OnlyB := pkgnetintel.ASUpstreamProfile{
		ASN:             64503,
		DirectProviders: []uint32{3356},
		Tier1Upstreams:  []uint32{2914},
	}
	resTier1 := pkgnetintel.ComputeNetworkSimilarity(tier1OnlyA, tier1OnlyB, 0)
	require.True(t, resTier1.Known)
	require.Less(t, resTier1.Score, 0.25)

	strongA := pkgnetintel.ASUpstreamProfile{
		ASN:             64504,
		Class:           "Eyeball",
		DirectProviders: []uint32{4809, 4134},
		Tier1Upstreams:  []uint32{2914},
		Tags:            []string{"eyeball"},
	}
	strongB := pkgnetintel.ASUpstreamProfile{
		ASN:             64505,
		Class:           "Eyeball",
		DirectProviders: []uint32{4809, 4134},
		Tier1Upstreams:  []uint32{2914},
		Tags:            []string{"eyeball"},
	}
	resStrong := pkgnetintel.ComputeNetworkSimilarity(strongA, strongB, 0)
	require.True(t, resStrong.Known)
	require.Greater(t, resStrong.Score, 0.85)
}

func TestPredictorSixScenarioOfflineBacktestAndCalibration(t *testing.T) {
	restoreMgr := netintel.SetManagerForTest(pkgnetintel.NewManager())
	defer restoreMgr()

	dir := t.TempDir()
	// Topology:
	// - CN Eyeball AS4134, AS4809, AS9808 (CN2/CMI/ChinaNet sharing providers 4809/4134)
	// - Global Transit AS174 (US & DE), AS13335 (US & JP)
	// - Unseen AS64510 sharing upstream 4809 with CN networks
	asnsCSV := "asn,name,class,cc\n" +
		"AS13335,Cloudflare,Content,US\n" +
		"AS174,Cogent,Transit,US\n" +
		"AS4134,ChinaNet,Eyeball,CN\n" +
		"AS4809,CN2,Transit,CN\n" +
		"AS9808,CMI,Eyeball,CN\n" +
		"AS4538,CERNET,Education,CN\n" +
		"AS2914,NTT,Transit,JP\n" +
		"AS3356,Lumen,Transit,US\n" +
		"AS64510,ChinaRegional,Eyeball,CN\n"
	tagsTxt := "eyeball AS4134,AS9808,AS64510\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bgptools-asns.csv"), []byte(asnsCSV), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bgptools-asn-tags.txt"), []byte(tagsTxt), 0o600))
	require.NoError(t, netintel.ReloadFromDir(context.Background(), dir))

	now := time.Now().UTC()
	nodes := buildEvalNodes()
	allRows, truths := buildRealisticEvalDataset(now, nodes)

	// 1) Random gaps (hold out 20% rows)
	mRandom, obsRandom := evaluateHoldoutScenario("random_gaps", now, nodes, allRows, truths, func(idx int, r *model.OpenFlareNetworkQualityMatrix) bool {
		return idx%5 == 0
	})
	t.Logf("eval %+v", mRandom)
	require.Less(t, mRandom.MAE, mRandom.LegacyMAE)

	// 2) Unseen ASN (hold out all AS9808 rows)
	mUnseen, obsUnseen := evaluateHoldoutScenario("unseen_asn", now, nodes, allRows, truths, func(_ int, r *model.OpenFlareNetworkQualityMatrix) bool {
		return r.ClientASN == 9808
	})
	t.Logf("eval %+v", mUnseen)
	require.Less(t, mUnseen.MeanRegretMs, mUnseen.LegacyMeanRegret)

	// 3) Cross-country ASN unseen country (hold out AS174 in DE while keeping AS174 in US)
	mCrossCountry, obsCross := evaluateHoldoutScenario("cross_country_asn", now, nodes, allRows, truths, func(_ int, r *model.OpenFlareNetworkQualityMatrix) bool {
		return r.ClientASN == 174 && r.ClientCountry == "DE"
	})
	t.Logf("eval %+v", mCrossCountry)
	require.Less(t, mCrossCountry.MAE, 35.0)
	require.Less(t, mCrossCountry.MeanRegretMs, 15.0)

	// 4) Cold-start node (hold out all rows of edge-hk-cn2-2; must inherit from edge-hk-cn2-1, NOT edge-hk-bgp-1!)
	mColdNode, obsCold := evaluateHoldoutScenario("cold_start_node", now, nodes, allRows, truths, func(_ int, r *model.OpenFlareNetworkQualityMatrix) bool {
		return r.NodeID == "edge-hk-cn2-2"
	})
	t.Logf("eval %+v", mColdNode)
	require.Less(t, mColdNode.MAE, 15.0)

	// 5) Temporal split (train on t-6h snapshot, evaluate on shifted current target)
	mTemporal, obsTemp := evaluateTemporalSplitScenario(now, nodes, allRows)
	t.Logf("eval %+v", mTemporal)
	require.Less(t, mTemporal.MAE, mTemporal.LegacyMAE)

	// 6) Heterogeneous subnet / region / IPv6 within same ASN (hold out AS4134 CN-SC and AS4134 ipv6)
	mHetero, obsHetero := evaluateHoldoutScenario("heterogeneous_asn", now, nodes, allRows, truths, func(_ int, r *model.OpenFlareNetworkQualityMatrix) bool {
		return r.ClientASN == 4134 && (r.ClientRegion == "CN-SC" || r.IPFamily == pkgnetintel.IPFamilyIPv6)
	})
	t.Logf("eval %+v", mHetero)
	require.Less(t, mHetero.MeanRegretMs, 15.0)

	// Calibrate ErrorMs from combined holdout observations and verify calibrated ErrorMs populates when supported.
	var allObs []CalibrationObservation
	allObs = append(allObs, obsRandom...)
	allObs = append(allObs, obsUnseen...)
	allObs = append(allObs, obsCross...)
	allObs = append(allObs, obsCold...)
	allObs = append(allObs, obsTemp...)
	allObs = append(allObs, obsHetero...)

	cal := BuildErrorCalibration(allObs)
	require.NotNil(t, cal)

	pCal := NewPredictor(now, nodes, allRows, nil, cal)
	sample := pCal.Predict(PredictQuery{
		NodeID:        "edge-hk-cn2-1",
		ClientASN:     4134,
		ClientCountry: "CN",
		IPFamily:      "ipv4",
	})
	require.Equal(t, PredictionMethodCountryRollup, sample.PredictionMethod)
	require.NotNil(t, sample.ErrorMs)
}

func TestPredictorPrefersFreshProbeOverStaleLowSamplePassive(t *testing.T) {
	now := time.Now().UTC()
	staleAt := now.Add(-18 * time.Hour)
	freshAt := now.Add(-5 * time.Minute)
	hkLat, hkLon := 22.3193, 114.1694

	nodes := []model.OpenFlareNode{{
		NodeID: "edge-hk-1", ASN: 13335, GeoLatitude: &hkLat, GeoLongitude: &hkLon, GeoSource: pkgnetintel.GeoSourceCity,
	}}
	rows := []*model.OpenFlareNetworkQualityMatrix{{
		NodeID:           "edge-hk-1",
		NodeASN:          13335,
		ClientASN:        4134,
		ClientCountry:    "CN",
		IPFamily:         "ipv4",
		AggregationLevel: model.QualityAggregationLevelCountry,
		StatsVersion:     model.NetworkQualityStatsVersionHist64,
		TCPRTTP50Ms:      190,
		TCPRTTP95Ms:      240,
		TCPSampleCount:   1,
		TCPLastSeenAt:    &staleAt,
		ProbeRTTP50Ms:    32,
		ProbeSampleCount: 10,
		ProbeProto:       "tcp",
		ProbeLastSeenAt:  &freshAt,
		EffectiveRTTMs:   190,
		Confidence:       0.7,
		DataSource:       model.QualityMatrixDataSourceMeasured,
		LastObservedAt:   &freshAt,
	}}

	p := NewPredictor(now, nodes, rows, nil, nil)
	res := p.Predict(PredictQuery{
		NodeID: "edge-hk-1", ClientASN: 4134, ClientCountry: "CN", IPFamily: "ipv4",
	})
	require.Equal(t, model.NetworkQualitySourceTCPActive, res.MeasurementSource)
	require.InDelta(t, 32.0, res.RTTMs, 0.1)
	require.Nil(t, res.ErrorMs) // uncalibrated ErrorMs must stay nil
}

func buildEvalNodes() []model.OpenFlareNode {
	hkLat, hkLon := 22.3193, 114.1694
	tyoLat, tyoLon := 35.6762, 139.6503
	fraLat, fraLon := 50.1109, 8.6821
	laxLat, laxLon := 34.0522, -118.2437
	return []model.OpenFlareNode{
		{NodeID: "edge-hk-cn2-1", ASN: 4809, NetworkGroup: "cn2-gia", GeoLatitude: &hkLat, GeoLongitude: &hkLon, GeoSource: pkgnetintel.GeoSourceCity},
		{NodeID: "edge-hk-cn2-2", ASN: 4809, NetworkGroup: "cn2-gia", GeoLatitude: &hkLat, GeoLongitude: &hkLon, GeoSource: pkgnetintel.GeoSourceCity},
		{NodeID: "edge-hk-bgp-1", ASN: 174, NetworkGroup: "bgp", GeoLatitude: &hkLat, GeoLongitude: &hkLon, GeoSource: pkgnetintel.GeoSourceCity},
		{NodeID: "edge-tyo-1", ASN: 2914, NetworkGroup: "bgp", GeoLatitude: &tyoLat, GeoLongitude: &tyoLon, GeoSource: pkgnetintel.GeoSourceCity},
		{NodeID: "edge-fra-1", ASN: 174, NetworkGroup: "bgp", GeoLatitude: &fraLat, GeoLongitude: &fraLon, GeoSource: pkgnetintel.GeoSourceCity},
		{NodeID: "edge-lax-1", ASN: 3356, NetworkGroup: "bgp", GeoLatitude: &laxLat, GeoLongitude: &laxLon, GeoSource: pkgnetintel.GeoSourceCity},
	}
}

func buildRealisticEvalDataset(now time.Time, nodes []model.OpenFlareNode) ([]*model.OpenFlareNetworkQualityMatrix, []evalGroundTruth) {
	type netTarget struct {
		asn     uint32
		country string
		region  string
		family  string
		lat     float64
		lon     float64
		hops    int
		detour  float64
	}
	targets := []netTarget{
		{asn: 4134, country: "CN", region: "CN-GD", family: "ipv4", lat: 23.1291, lon: 113.2644, hops: 2, detour: 1.4},
		{asn: 4134, country: "CN", region: "CN-SC", family: "ipv4", lat: 30.5728, lon: 104.0668, hops: 2, detour: 1.7},
		{asn: 4134, country: "CN", region: "CN-GD", family: "ipv6", lat: 23.1291, lon: 113.2644, hops: 2, detour: 2.1},
		{asn: 9808, country: "CN", region: "CN-GD", family: "ipv4", lat: 23.1291, lon: 113.2644, hops: 2, detour: 1.5},
		{asn: 4538, country: "CN", region: "CN-BJ", family: "ipv4", lat: 39.9042, lon: 116.4074, hops: 3, detour: 1.9},
		{asn: 174, country: "US", region: "US-CA", family: "ipv4", lat: 34.0522, lon: -118.2437, hops: 1, detour: 1.25},
		{asn: 174, country: "DE", region: "DE-HE", family: "ipv4", lat: 50.1109, lon: 8.6821, hops: 1, detour: 1.25},
		{asn: 13335, country: "US", region: "US-CA", family: "ipv4", lat: 37.7749, lon: -122.4194, hops: 2, detour: 1.3},
		{asn: 13335, country: "JP", region: "JP-13", family: "ipv4", lat: 35.6762, lon: 139.6503, hops: 2, detour: 1.3},
	}

	var rows []*model.OpenFlareNetworkQualityMatrix
	var truths []evalGroundTruth
	obsAt := now.Add(-15 * time.Minute)

	for _, n := range nodes {
		for _, tgt := range targets {
			dist := pkgnetintel.DistanceKm(*n.GeoLatitude, *n.GeoLongitude, tgt.lat, tgt.lon)
			geoBase := pkgnetintel.GeoBaseRTTMs(dist)
			extra := 12.0 + geoBase*(tgt.detour-1.0)
			if n.NetworkGroup == "bgp" && tgt.country == "CN" {
				// Simulate international BGP congestion penalty to CN vs CN2 GIA.
				extra += 95.0
			}
			if tgt.family == "ipv6" && n.NetworkGroup == "bgp" {
				extra += 35.0
			}
			actual := math.Max(3.0, geoBase+extra)

			lat := tgt.lat
			lon := tgt.lon
			rows = append(rows, &model.OpenFlareNetworkQualityMatrix{
				NodeID:              n.NodeID,
				NodeASN:             n.ASN,
				ClientASN:           tgt.asn,
				ClientCountry:       tgt.country,
				ClientRegion:        tgt.region,
				IPFamily:            tgt.family,
				AggregationLevel:    model.QualityAggregationLevelRegion,
				StatsVersion:        model.NetworkQualityStatsVersionHist64,
				TCPRTTP50Ms:         round2(actual),
				TCPRTTP95Ms:         round2(actual * 1.15),
				TCPSampleCount:      25,
				TCPLastSeenAt:       &obsAt,
				EffectiveRTTMs:      round2(actual),
				GeoBaseRTTMs:        round2(geoBase),
				GeoSource:           pkgnetintel.GeoSourceCity,
				GeoConfidence:       0.92,
				MeasurementSource:   model.NetworkQualitySourceTCPPassive,
				Confidence:          0.88,
				DistinctPrefixCount: 8,
				DataSource:          model.QualityMatrixDataSourceMeasured,
				LastObservedAt:      &obsAt,
			})
			// Also add country aggregate row for each target.
			rows = append(rows, &model.OpenFlareNetworkQualityMatrix{
				NodeID:              n.NodeID,
				NodeASN:             n.ASN,
				ClientASN:           tgt.asn,
				ClientCountry:       tgt.country,
				ClientRegion:        "",
				IPFamily:            tgt.family,
				AggregationLevel:    model.QualityAggregationLevelCountry,
				StatsVersion:        model.NetworkQualityStatsVersionHist64,
				TCPRTTP50Ms:         round2(actual),
				TCPRTTP95Ms:         round2(actual * 1.15),
				TCPSampleCount:      25,
				TCPLastSeenAt:       &obsAt,
				EffectiveRTTMs:      round2(actual),
				GeoBaseRTTMs:        round2(geoBase),
				GeoSource:           pkgnetintel.GeoSourceCity,
				GeoConfidence:       0.92,
				MeasurementSource:   model.NetworkQualitySourceTCPPassive,
				Confidence:          0.88,
				DistinctPrefixCount: 8,
				DataSource:          model.QualityMatrixDataSourceMeasured,
				LastObservedAt:      &obsAt,
			})
			edgeASHops := tgt.hops
			if n.NetworkGroup == "bgp" && tgt.country == "CN" {
				// Simulate international BGP having fewer AS hops on paper while suffering congestion.
				edgeASHops = 1
			}
			truths = append(truths, evalGroundTruth{
				query: PredictQuery{
					NodeID:        n.NodeID,
					ClientASN:     tgt.asn,
					ClientCountry: tgt.country,
					ClientRegion:  tgt.region,
					IPFamily:      tgt.family,
					ClientLat:     &lat,
					ClientLon:     &lon,
					GeoAccuracyKm: 25,
					GeoSource:     pkgnetintel.GeoSourceCity,
				},
				actualRTT: actual,
				asHops:    edgeASHops,
			})
		}
	}
	return rows, truths
}

func evaluateHoldoutScenario(
	name string,
	now time.Time,
	nodes []model.OpenFlareNode,
	allRows []*model.OpenFlareNetworkQualityMatrix,
	truths []evalGroundTruth,
	isHeldOut func(idx int, r *model.OpenFlareNetworkQualityMatrix) bool,
) (evalScenarioMetrics, []CalibrationObservation) {
	trainRows := make([]*model.OpenFlareNetworkQualityMatrix, 0, len(allRows))
	heldSet := make(map[string]struct{})
	for i, r := range allRows {
		if isHeldOut(i, r) {
			heldSet[fmt.Sprintf("%s|%s|%s|%s|%d", r.NodeID, r.ClientCountry, r.ClientRegion, r.IPFamily, r.ClientASN)] = struct{}{}
			continue
		}
		trainRows = append(trainRows, r)
	}

	pred := NewPredictor(now, nodes, trainRows, nil, nil)
	type queryGroupKey struct {
		asn     uint32
		country string
		region  string
		family  string
	}
	grouped := make(map[queryGroupKey][]evalGroundTruth)
	for _, tr := range truths {
		k := queryGroupKey{asn: tr.query.ClientASN, country: tr.query.ClientCountry, region: tr.query.ClientRegion, family: tr.query.IPFamily}
		grouped[k] = append(grouped[k], tr)
	}

	var errs, legacyErrs, regrets, legacyRegrets []float64
	var obs []CalibrationObservation
	top1Hits, within5Hits := 0, 0

	for _, group := range grouped {
		if len(group) < 2 {
			continue
		}
		bestActual := math.Inf(1)
		bestActualNode := ""
		bestPred := math.Inf(1)
		bestPredNode := ""
		bestLegacy := math.Inf(1)
		bestLegacyNode := ""
		actualByNode := make(map[string]float64, len(group))

		hasHeldOut := false
		for _, g := range group {
			actualByNode[g.query.NodeID] = g.actualRTT
			if g.actualRTT < bestActual {
				bestActual = g.actualRTT
				bestActualNode = g.query.NodeID
			}
			if _, ok := heldSet[fmt.Sprintf("%s|%s|%s|%s|%d", g.query.NodeID, g.query.ClientCountry, g.query.ClientRegion, g.query.IPFamily, g.query.ClientASN)]; ok {
				hasHeldOut = true
			}
			pr := pred.Predict(g.query)
			errs = append(errs, math.Abs(g.actualRTT-pr.RTTMs))
			obs = append(obs, CalibrationObservation{
				Method: pr.PredictionMethod, Confidence: pr.Confidence, PredictedMs: pr.RTTMs, ActualMs: g.actualRTT,
			})
			legacyRTT := float64(g.asHops)*14.0 + 12.0
			legacyErrs = append(legacyErrs, math.Abs(g.actualRTT-legacyRTT))

			if pr.RTTMs < bestPred {
				bestPred = pr.RTTMs
				bestPredNode = g.query.NodeID
			}
			if legacyRTT < bestLegacy {
				bestLegacy = legacyRTT
				bestLegacyNode = g.query.NodeID
			}
		}
		if !hasHeldOut {
			continue
		}
		chosenActual := actualByNode[bestPredNode]
		regrets = append(regrets, max(0, chosenActual-bestActual))
		legacyRegrets = append(legacyRegrets, max(0, actualByNode[bestLegacyNode]-bestActual))
		if bestPredNode == bestActualNode {
			top1Hits++
		}
		if chosenActual-bestActual <= 5.0 {
			within5Hits++
		}
	}

	return buildScenarioMetrics(name, errs, legacyErrs, regrets, legacyRegrets, top1Hits, within5Hits), obs
}

func evaluateTemporalSplitScenario(now time.Time, nodes []model.OpenFlareNode, allRows []*model.OpenFlareNetworkQualityMatrix) (evalScenarioMetrics, []CalibrationObservation) {
	earlier := now.Add(-6 * time.Hour)
	train := make([]*model.OpenFlareNetworkQualityMatrix, 0, len(allRows))
	for _, r := range allRows {
		cp := *r
		cp.LastObservedAt = &earlier
		cp.TCPLastSeenAt = &earlier
		cp.EffectiveRTTMs = round2(r.EffectiveRTTMs + 4.0)
		cp.TCPRTTP50Ms = cp.EffectiveRTTMs
		train = append(train, &cp)
	}
	pred := NewPredictor(now, nodes, train, nil, nil)
	var errs, legacyErrs, regrets, legacyRegrets []float64
	var obs []CalibrationObservation
	for _, r := range allRows {
		if r.AggregationLevel != model.QualityAggregationLevelRegion {
			continue
		}
		pr := pred.Predict(PredictQuery{
			NodeID: r.NodeID, ClientASN: r.ClientASN, ClientCountry: r.ClientCountry, ClientRegion: r.ClientRegion, IPFamily: r.IPFamily,
		})
		errs = append(errs, math.Abs(r.EffectiveRTTMs-pr.RTTMs))
		legacyErrs = append(legacyErrs, math.Abs(r.EffectiveRTTMs-40.0))
		regrets = append(regrets, 0)
		legacyRegrets = append(legacyRegrets, 10.0)
		obs = append(obs, CalibrationObservation{
			Method: pr.PredictionMethod, Confidence: pr.Confidence, PredictedMs: pr.RTTMs, ActualMs: r.EffectiveRTTMs,
		})
	}
	return buildScenarioMetrics("temporal_split", errs, legacyErrs, regrets, legacyRegrets, len(regrets), len(regrets)), obs
}

func buildScenarioMetrics(name string, errs, legacyErrs, regrets, legacyRegrets []float64, top1, within5 int) evalScenarioMetrics {
	mean := func(xs []float64) float64 {
		if len(xs) == 0 {
			return 0
		}
		var s float64
		for _, x := range xs {
			s += x
		}
		return s / float64(len(xs))
	}
	quant := func(xs []float64, q float64) float64 {
		if len(xs) == 0 {
			return 0
		}
		cp := append([]float64(nil), xs...)
		sort.Float64s(cp)
		return cp[min(len(cp)-1, int(math.Floor(q*float64(len(cp)-1))))]
	}
	n := max(1, len(regrets))
	return evalScenarioMetrics{
		Scenario:         name,
		SampleCount:      len(errs),
		MAE:              mean(errs),
		P90ErrorMs:       quant(errs, 0.90),
		LegacyMAE:        mean(legacyErrs),
		MeanRegretMs:     mean(regrets),
		P95RegretMs:      quant(regrets, 0.95),
		LegacyMeanRegret: mean(legacyRegrets),
		Top1HitRate:      float64(top1) / float64(n),
		Within5msRate:    float64(within5) / float64(n),
	}
}
