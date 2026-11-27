// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netquality

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/qualityapi"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
)

const (
	// QualityProviderName identifies the 5-level netquality predictor adapter.
	QualityProviderName = "netquality_5level"

	cachedPredictorMaxAge = 60 * time.Second
)

type predictorSnapshot struct {
	predictor  *Predictor
	generation uint64
	loadedAt   time.Time
}

// QualityProvider adapts the internal 5-level Predictor and matrix repository
// to the stable qualityapi.Provider interface consumed by SmartDNS.
type QualityProvider struct {
	mu       sync.Mutex
	snapshot atomic.Pointer[predictorSnapshot]
	loader   func(ctx context.Context) (*Predictor, error)
}

// NewQualityProvider constructs a QualityProvider backed by live database state.
func NewQualityProvider() *QualityProvider {
	return &QualityProvider{
		loader: LoadLivePredictor,
	}
}

// NewStaticQualityProvider constructs a QualityProvider pinned to an in-memory Predictor (for tests/backtests).
func NewStaticQualityProvider(predictor *Predictor) *QualityProvider {
	p := &QualityProvider{}
	if predictor != nil {
		p.snapshot.Store(&predictorSnapshot{
			predictor:  predictor,
			generation: predictor.generation,
			loadedAt:   time.Now().UTC(),
		})
	}
	return p
}

// Name returns the active quality provider identifier.
func (p *QualityProvider) Name() string {
	return QualityProviderName
}

// Generation returns the latest observed rollup generation.
func (p *QualityProvider) Generation() uint64 {
	if snap := p.snapshot.Load(); snap != nil && snap.generation > CurrentRollupGeneration() {
		return snap.generation
	}
	return CurrentRollupGeneration()
}

// EvaluateNode predicts unified link quality from nodeID to client without executing synchronous SQL on hot paths.
func (p *QualityProvider) EvaluateNode(ctx context.Context, nodeID string, client qualityapi.ClientProfile) qualityapi.NodeLinkQuality {
	trimmedID := strings.TrimSpace(nodeID)
	if trimmedID == "" {
		return qualityapi.DefaultFallbackQuality("")
	}
	predictor := p.getPredictor(ctx)
	if predictor == nil {
		return qualityapi.DefaultFallbackQuality(trimmedID)
	}
	return mapPredictionToLinkQuality(trimmedID, predictor.Predict(toPredictQuery(trimmedID, client)))
}

// EvaluateNodes predicts unified link quality for a batch of candidate nodes.
func (p *QualityProvider) EvaluateNodes(ctx context.Context, nodeIDs []string, client qualityapi.ClientProfile) map[string]qualityapi.NodeLinkQuality {
	out := make(map[string]qualityapi.NodeLinkQuality, len(nodeIDs))
	if len(nodeIDs) == 0 {
		return out
	}
	predictor := p.getPredictor(ctx)
	if predictor == nil {
		for _, id := range nodeIDs {
			trimmed := strings.TrimSpace(id)
			out[id] = qualityapi.DefaultFallbackQuality(trimmed)
		}
		return out
	}
	for _, id := range nodeIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			out[id] = qualityapi.DefaultFallbackQuality("")
			continue
		}
		res := predictor.Predict(toPredictQuery(trimmed, client))
		out[id] = mapPredictionToLinkQuality(trimmed, res)
	}
	return out
}

// ExportSnapshot exports measured matrix rows and per-node priors in CompactQualitySnapshot v1 format.
func (p *QualityProvider) ExportSnapshot(ctx context.Context, nodeIDs []string) (*qualityapi.CompactQualitySnapshot, error) {
	if p.loader != nil && db.DB(ctx) != nil {
		if loaded, err := p.loader(ctx); err == nil && loaded != nil {
			p.snapshot.Store(&predictorSnapshot{
				predictor:  loaded,
				generation: CurrentRollupGeneration(),
				loadedAt:   time.Now().UTC(),
			})
		}
	}

	snap := p.snapshot.Load()
	var (
		nodes        []model.OpenFlareNode
		measuredRows []*model.OpenFlareNetworkQualityMatrix
		err          error
	)
	if snap != nil && snap.predictor != nil && p.loader == nil {
		nodes = make([]model.OpenFlareNode, 0, len(snap.predictor.nodes))
		for _, n := range snap.predictor.nodes {
			nodes = append(nodes, n)
		}
		for _, rows := range snap.predictor.rowsByNode {
			measuredRows = append(measuredRows, rows...)
		}
	} else {
		if db.DB(ctx) == nil {
			return qualityapi.FallbackProvider{}.ExportSnapshot(ctx, nodeIDs)
		}
		nodes, err = repository.ListOpenFlareNodes(ctx)
		if err != nil {
			return nil, err
		}
		measuredRows, err = repository.ListOpenFlareNetworkQualityMatrixMeasured(ctx)
		if err != nil {
			return nil, err
		}
	}

	allowed := makeNodeFilterSet(nodeIDs)
	nodeDefaults := make(map[string]qualityapi.CompactNodePrior, len(nodes))
	extrasByNode := make(map[string][]float64, len(nodes))

	entries := make([]qualityapi.CompactQualityEntry, 0, len(measuredRows))
	for _, row := range measuredRows {
		if row == nil || row.NodeID == "" || row.ClientASN == 0 || row.DataSource != model.QualityMatrixDataSourceMeasured {
			continue
		}
		if len(allowed) > 0 && !allowed[row.NodeID] {
			continue
		}
		if row.EffectiveRTTMs <= 0 {
			continue
		}
		p95 := row.EffectiveRTTMs
		if row.MeasurementSource == model.NetworkQualitySourceTCPPassive && row.TCPRTTP95Ms > p95 {
			p95 = row.TCPRTTP95Ms
		}
		geoKnown := hasKnownRowGeoBase(row)
		if geoKnown {
			extrasByNode[row.NodeID] = append(extrasByNode[row.NodeID], row.EffectiveRTTMs-row.GeoBaseRTTMs)
		}
		entries = append(entries, qualityapi.CompactQualityEntry{
			NodeID:         row.NodeID,
			ClientASN:      row.ClientASN,
			ClientCountry:  strings.ToUpper(strings.TrimSpace(row.ClientCountry)),
			ClientRegion:   strings.ToUpper(strings.TrimSpace(row.ClientRegion)),
			IPFamily:       normalizePredictorIPFamily(row.IPFamily),
			Level:          normalizePredictorAggLevel(row.AggregationLevel),
			EffectiveRTTMs: round2(row.EffectiveRTTMs),
			RTTP95Ms:       round2(p95),
			LossRate:       round4(selectDirectLossRate(row, row.MeasurementSource)),
			Confidence:     round4(clampConfidence(row.Confidence)),
			GeoBaseRTTMs:   round2(max(0.0, row.GeoBaseRTTMs)),
			GeoBaseKnown:   geoKnown,
			SourceTier:     tierFromAggregationLevel(row.AggregationLevel),
		})
	}

	for _, n := range nodes {
		if len(allowed) > 0 && !allowed[n.NodeID] {
			continue
		}
		nodeDefaults[n.NodeID] = qualityapi.CompactNodePrior{
			NetworkGroup: strings.TrimSpace(n.NetworkGroup),
			ASN:          n.ASN,
			Latitude:     n.GeoLatitude,
			Longitude:    n.GeoLongitude,
			MedianExtra:  round2(medianFloat64(extrasByNode[n.NodeID])),
		}
	}

	return &qualityapi.CompactQualitySnapshot{
		SchemaVersion: qualityapi.CompactSnapshotSchemaVersion,
		Generation:    p.Generation(),
		GeneratedAt:   time.Now().UTC(),
		ProviderName:  QualityProviderName,
		Entries:       entries,
		NodeDefaults:  nodeDefaults,
	}, nil
}

func (p *QualityProvider) getPredictor(ctx context.Context) *Predictor {
	curGen := CurrentRollupGeneration()
	now := time.Now().UTC()
	if snap := p.snapshot.Load(); snap != nil && snap.predictor != nil {
		if snap.generation == curGen && now.Sub(snap.loadedAt) <= cachedPredictorMaxAge {
			return snap.predictor
		}
	}
	if p.loader == nil || db.DB(ctx) == nil {
		if snap := p.snapshot.Load(); snap != nil {
			return snap.predictor
		}
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	curGen = CurrentRollupGeneration()
	now = time.Now().UTC()
	if snap := p.snapshot.Load(); snap != nil && snap.predictor != nil {
		if snap.generation == curGen && now.Sub(snap.loadedAt) <= cachedPredictorMaxAge {
			return snap.predictor
		}
	}
	loaded, err := p.loader(ctx)
	if err != nil || loaded == nil {
		if snap := p.snapshot.Load(); snap != nil {
			return snap.predictor
		}
		return nil
	}
	p.snapshot.Store(&predictorSnapshot{
		predictor:  loaded,
		generation: curGen,
		loadedAt:   now,
	})
	return loaded
}

func toPredictQuery(nodeID string, client qualityapi.ClientProfile) PredictQuery {
	ipFamily := strings.ToLower(strings.TrimSpace(client.IPFamily))
	if ipFamily == "" && strings.TrimSpace(client.ClientIP) != "" {
		ipFamily = pkgnetintel.DetectIPFamily(client.ClientIP)
	}
	return PredictQuery{
		NodeID:        nodeID,
		ClientASN:     client.ASN,
		ClientCountry: client.CountryISO,
		ClientRegion:  client.RegionCode,
		IPFamily:      ipFamily,
		ClientLat:     client.Latitude,
		ClientLon:     client.Longitude,
		GeoAccuracyKm: client.GeoAccuracyKm,
		GeoSource:     client.GeoSource,
		AnycastStatus: client.AnycastStatus,
	}
}

func mapPredictionToLinkQuality(nodeID string, res PredictionResult) qualityapi.NodeLinkQuality {
	tier := qualityapi.SourceTierGeoPrior
	switch res.PredictionMethod {
	case PredictionMethodExactRegion:
		tier = qualityapi.SourceTierMeasuredRegion
	case PredictionMethodCountryRollup:
		tier = qualityapi.SourceTierMeasuredCountry
	case PredictionMethodInferredUpstream, PredictionMethodSimilarNode:
		tier = qualityapi.SourceTierInferred
	}
	return qualityapi.NodeLinkQuality{
		NodeID:          nodeID,
		EffectiveRTTMs:  res.RTTMs,
		RTTP95Ms:        res.RTTP95Ms,
		DispersionMs:    res.CandidateDispersionMs,
		LossRate:        res.LossRate,
		Confidence:      res.Confidence,
		QualityScore:    ComputeQualityScore(res),
		GeoBaseRTTMs:    res.GeoBaseRTTMs,
		GeoBaseKnown:    res.GeoBaseKnown,
		SourceTier:      tier,
		ProviderDetail:  res.PredictionMethod,
		AnycastIsolated: res.AnycastIsolated,
		LastObservedAt:  res.LastObservedAt,
	}
}

func tierFromAggregationLevel(level string) string {
	if strings.EqualFold(strings.TrimSpace(level), model.QualityAggregationLevelRegion) {
		return qualityapi.SourceTierMeasuredRegion
	}
	return qualityapi.SourceTierMeasuredCountry
}

func clampConfidence(conf float64) float64 {
	if conf <= 0 || math.IsNaN(conf) || math.IsInf(conf, 0) {
		return directDefaultConfidence
	}
	return math.Max(minSampleConfidence, math.Min(maxSampleConfidence, conf))
}

func makeNodeFilterSet(nodeIDs []string) map[string]bool {
	if len(nodeIDs) == 0 {
		return nil
	}
	allowed := make(map[string]bool, len(nodeIDs))
	for _, id := range nodeIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			allowed[trimmed] = true
		}
	}
	return allowed
}

func medianFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2.0
}
