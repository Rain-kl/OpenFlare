// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netquality

import (
	"context"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/qualityapi"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityProviderMapsTiersLossAndExportsCompactSnapshot(t *testing.T) {
	cleanup := setupNetQualityTestDB(t)
	defer cleanup()
	ctx := context.Background()

	latNode := 22.3193
	lonNode := 114.1694
	latClient := 23.1291
	lonClient := 113.2644
	now := time.Now().UTC()

	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID:       "hk-1",
		Name:         "HK Edge",
		IP:           "203.0.113.10",
		NetworkGroup: "cn2-gia",
		ASN:          4809,
		GeoLatitude:  &latNode,
		GeoLongitude: &lonNode,
		GeoSource:    pkgnetintel.GeoSourceCity,
		Status:       "online",
	}))
	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID:       "hk-2",
		Name:         "HK Peer Edge",
		IP:           "203.0.113.11",
		NetworkGroup: "cn2-gia",
		ASN:          4809,
		GeoLatitude:  &latNode,
		GeoLongitude: &lonNode,
		GeoSource:    pkgnetintel.GeoSourceCity,
		Status:       "online",
	}))
	require.NoError(t, repository.UpsertOpenFlareNetworkQualityMatrixBatch(ctx, []*model.OpenFlareNetworkQualityMatrix{
		{
			NodeID:            "hk-1",
			NodeASN:           4809,
			ClientASN:         4134,
			ClientCountry:     "CN",
			ClientRegion:      "CN-GD",
			IPFamily:          "ipv4",
			AggregationLevel:  model.QualityAggregationLevelRegion,
			StatsVersion:      model.NetworkQualityStatsVersionHist64,
			TCPRTTP50Ms:       24.5,
			TCPRTTP95Ms:       34.0,
			TCPRetransRate:    0.012,
			TCPSampleCount:    20,
			TCPLastSeenAt:     &now,
			EffectiveRTTMs:    24.5,
			GeoBaseRTTMs:      1.2,
			GeoConfidence:     0.95,
			MeasurementSource: model.NetworkQualitySourceTCPPassive,
			Confidence:        0.88,
			DataSource:        model.QualityMatrixDataSourceMeasured,
			LastObservedAt:    &now,
		},
		{
			NodeID:            "hk-1",
			NodeASN:           4809,
			ClientASN:         4134,
			ClientCountry:     "CN",
			ClientRegion:      "",
			IPFamily:          "ipv4",
			AggregationLevel:  model.QualityAggregationLevelCountry,
			StatsVersion:      model.NetworkQualityStatsVersionHist64,
			ProbeRTTP50Ms:     29.0,
			ProbeLossRate:     0.03,
			ProbeSampleCount:  8,
			ProbeProto:        model.ProbeProtoTCP,
			ProbeLastSeenAt:   &now,
			EffectiveRTTMs:    29.0,
			GeoBaseRTTMs:      5.0,
			GeoConfidence:     0.9,
			MeasurementSource: model.NetworkQualitySourceTCPActive,
			Confidence:        0.74,
			DataSource:        model.QualityMatrixDataSourceMeasured,
			LastObservedAt:    &now,
		},
		{
			NodeID:            "hk-2",
			NodeASN:           4809,
			ClientASN:         99999,
			ClientCountry:     "US",
			ClientRegion:      "",
			IPFamily:          "ipv4",
			AggregationLevel:  model.QualityAggregationLevelCountry,
			StatsVersion:      model.NetworkQualityStatsVersionHist64,
			TCPRTTP50Ms:       140.0,
			TCPSampleCount:    12,
			TCPLastSeenAt:     &now,
			EffectiveRTTMs:    140.0,
			GeoBaseRTTMs:      70.0,
			GeoConfidence:     0.85,
			MeasurementSource: model.NetworkQualitySourceTCPPassive,
			Confidence:        0.72,
			DataSource:        model.QualityMatrixDataSourceMeasured,
			LastObservedAt:    &now,
		},
	}))

	rollupGeneration.Add(1)
	provider := NewQualityProvider()
	assert.Equal(t, QualityProviderName, provider.Name())

	qRegion := provider.EvaluateNode(ctx, "hk-1", qualityapi.ClientProfile{
		ASN:        4134,
		CountryISO: "CN",
		RegionCode: "CN-GD",
		IPFamily:   "ipv4",
		Latitude:   &latClient,
		Longitude:  &lonClient,
		GeoSource:  pkgnetintel.GeoSourceCity,
	})
	assert.Equal(t, qualityapi.SourceTierMeasuredRegion, qRegion.SourceTier)
	assert.InDelta(t, 24.5, qRegion.EffectiveRTTMs, 0.01)
	assert.InDelta(t, 34.0, qRegion.RTTP95Ms, 0.01)
	assert.InDelta(t, 0.012, qRegion.LossRate, 0.0001)
	assert.Greater(t, qRegion.QualityScore, 50.0)

	qCountry := provider.EvaluateNode(ctx, "hk-1", qualityapi.ClientProfile{
		ASN:        4134,
		CountryISO: "CN",
		RegionCode: "CN-BJ",
		IPFamily:   "ipv4",
	})
	assert.Equal(t, qualityapi.SourceTierMeasuredCountry, qCountry.SourceTier)
	assert.InDelta(t, 29.0, qCountry.EffectiveRTTMs, 0.01)
	assert.InDelta(t, 0.03, qCountry.LossRate, 0.0001)

	qInferred := provider.EvaluateNode(ctx, "hk-1", qualityapi.ClientProfile{
		ASN:        99999,
		CountryISO: "US",
		IPFamily:   "ipv4",
	})
	assert.Equal(t, qualityapi.SourceTierInferred, qInferred.SourceTier)

	qGeoPrior := provider.EvaluateNode(ctx, "hk-1", qualityapi.ClientProfile{
		ASN:        88888,
		CountryISO: "DE",
		IPFamily:   "ipv4",
	})
	assert.Equal(t, qualityapi.SourceTierGeoPrior, qGeoPrior.SourceTier)

	snap, err := provider.ExportSnapshot(ctx, []string{"hk-1"})
	require.NoError(t, err)
	assert.Equal(t, qualityapi.CompactSnapshotSchemaVersion, snap.SchemaVersion)
	assert.Equal(t, QualityProviderName, snap.ProviderName)
	require.Len(t, snap.Entries, 2)
	prior, ok := snap.NodeDefaults["hk-1"]
	require.True(t, ok)
	assert.Equal(t, "cn2-gia", prior.NetworkGroup)
	assert.Equal(t, uint32(4809), prior.ASN)
	// median of (24.5-1.2=23.3, 29.0-5.0=24.0) -> 23.65
	assert.InDelta(t, 23.65, prior.MedianExtra, 0.05)
}
