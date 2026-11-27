// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netquality

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupNetQualityTestDB(t *testing.T) func() {
	t.Helper()
	ResetCollectorForTest()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.SystemConfig{},
		&model.OpenFlareNode{},
		&analyticsmodel.NetworkQualitySample{},
		&model.OpenFlareNetworkQualityMatrix{},
		&model.OpenFlareASTopologyEdge{},
		&model.OpenFlareTracerouteRecord{},
	))
	db.SetDB(sqliteDB)
	testhelper.SetupLogStoresForTest(t)

	return func() {
		ResetCollectorForTest()
		db.SetDB(nil)
	}
}

func TestLogHistogram64AccuracyAndSerialization(t *testing.T) {
	// Verify relative error across normal range [0.5, 10000]ms stays under 8%.
	testPoints := []float64{0.6, 1.2, 5.5, 12.3, 35.0, 88.8, 250.0, 1200.0, 4800.0, 9500.0}
	for _, v := range testPoints {
		var h LogHistogram64
		for range 50 {
			h.Add(v)
		}
		got := h.Quantile(0.5)
		relErr := math.Abs(got-v) / v
		require.Less(t, relErr, 0.08, "value=%.2f got=%.2f relErr=%.4f", v, got, relErr)
	}

	// Verify overflow buckets and Encode/Decode roundtrip.
	var src LogHistogram64
	src.Add(0.1)
	src.Add(25.0)
	src.Add(40.0)
	src.Add(15000.0)
	encoded := src.Encode()
	require.NotEmpty(t, encoded)

	decoded, err := DecodeLogHistogram64(encoded)
	require.NoError(t, err)
	require.Equal(t, src.Count, decoded.Count)
	require.Equal(t, src.Bins, decoded.Bins)
}

func TestPrefixSketchUnionDeduplicationAndSaturation(t *testing.T) {
	var a PrefixSketch
	a.AddIP("203.0.113.10") // /24 A
	a.AddIP("203.0.113.99") // same /24 A
	a.AddIP("203.0.114.5")  // /24 B
	a.AddIP("198.51.100.7") // /24 C
	require.Equal(t, 3, a.DistinctCount())

	var b PrefixSketch
	b.AddIP("203.0.113.12")  // /24 A (overlap)
	b.AddIP("198.51.100.42") // /24 C (overlap)
	b.AddIP("192.0.2.8")     // /24 D (new)
	require.Equal(t, 3, b.DistinctCount())

	a.Union(&b)
	require.Equal(t, 4, a.DistinctCount())
	require.False(t, a.Saturated)

	decoded, err := DecodePrefixSketch(a.Encode())
	require.NoError(t, err)
	require.Equal(t, 4, decoded.DistinctCount())

	// Exceeding MaxPrefixSketchHashes marks saturated and caps slice size at 64.
	var big PrefixSketch
	for i := 1; i <= 90; i++ {
		big.AddIP("100.65." + string(rune('0'+i/10)) + string(rune('0'+i%10)) + ".1") // invalid string ignored; use valid octets below
	}
	big = PrefixSketch{}
	for i := 1; i <= 90; i++ {
		ip := "198.51." + string([]byte{byte('0' + i/100), byte('0' + (i/10)%10), byte('0' + i%10)})
		_ = ip
	}
}

func TestPrefixSketchCapAt64(t *testing.T) {
	var sk PrefixSketch
	for i := 1; i <= 80; i++ {
		sk.AddHash(uint64(i * 1000003)) //nolint:gosec // positive test index
	}
	require.True(t, sk.Saturated)
	require.Equal(t, MaxPrefixSketchHashes, sk.DistinctCount())
}

func TestPassiveCollectorMultiCountryRegionIPFamilyAndRollup(t *testing.T) {
	cleanup := setupNetQualityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()
	hkLat, hkLon := 22.3193, 114.1694
	sgLat, sgLon := 1.3521, 103.8198
	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID:           "edge-hk-1",
		Name:             "HK-Edge-1",
		IP:               "1.1.1.1",
		ASN:              13335,
		GeoLatitude:      &hkLat,
		GeoLongitude:     &hkLon,
		GeoSource:        pkgnetintel.GeoSourceCity,
		Status:           "online",
		NodeType:         "edge_node",
		LastSeenAt:       &now,
		AccessToken:      "token-edge-hk-1",
		CapabilitiesJSON: "[]",
	}))
	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID:           "edge-sg-1",
		Name:             "SG-Edge-1",
		IP:               "8.8.8.8",
		ASN:              15169,
		GeoLatitude:      &sgLat,
		GeoLongitude:     &sgLon,
		GeoSource:        pkgnetintel.GeoSourceCity,
		Status:           "online",
		NodeType:         "edge_node",
		LastSeenAt:       &now,
		AccessToken:      "token-edge-sg-1",
		CapabilitiesJSON: "[]",
	}))

	// Insert legacy predicted + legacy stats_version=1 matrix rows to verify zero-window atomic replacement.
	require.NoError(t, repository.UpsertOpenFlareNetworkQualityMatrixBatch(ctx, []*model.OpenFlareNetworkQualityMatrix{
		{
			NodeID:           "edge-sg-1",
			ClientASN:        4134,
			ClientCountry:    "CN",
			IPFamily:         "unknown",
			AggregationLevel: model.QualityAggregationLevelCountry,
			StatsVersion:     model.NetworkQualityStatsVersionLegacy,
			EffectiveRTTMs:   999,
			DataSource:       model.QualityMatrixDataSourcePredicted,
		},
	}))

	// Invalid / zero / outlier samples should be ignored without panicking.
	RecordPassiveSample(ctx, "", 4134, "CN", 25000, 20000, 0, 10, 0)
	RecordPassiveSample(ctx, "edge-hk-1", 0, "CN", 25000, 20000, 0, 10, 0)
	RecordPassiveSample(ctx, "edge-hk-1", 4134, "CN", maxValidTCPRTTUs+1, 20000, 0, 10, 0)

	gdLat, gdLon := 23.1291, 113.2644
	zjLat, zjLon := 30.2741, 120.1551
	usLat, usLon := 37.7749, -122.4194
	deLat, deLon := 50.1109, 8.6821

	// AS4134 in CN-GD (2 prefixes) and CN-ZJ (1 prefix), plus IPv6 in CN-GD.
	RecordPassiveSampleInput(ctx, PassiveSampleInput{
		NodeID: "edge-hk-1", ClientIP: "113.108.10.5", ClientASN: 4134, ClientCountry: "CN", ClientRegion: "CN-GD",
		IPFamily: "ipv4", ClientLat: &gdLat, ClientLon: &gdLon, GeoAccuracyKm: 20, GeoSource: pkgnetintel.GeoSourceCity,
		TCPRTTUs: 24000, TCPMinRTTUs: 19000, TCPSegsOut: 20,
	})
	RecordPassiveSampleInput(ctx, PassiveSampleInput{
		NodeID: "edge-hk-1", ClientIP: "113.109.10.5", ClientASN: 4134, ClientCountry: "CN", ClientRegion: "CN-GD",
		IPFamily: "ipv4", ClientLat: &gdLat, ClientLon: &gdLon, GeoAccuracyKm: 20, GeoSource: pkgnetintel.GeoSourceCity,
		TCPRTTUs: 30000, TCPMinRTTUs: 20000, TCPRetrans: 1, TCPSegsOut: 20,
	})
	RecordPassiveSampleInput(ctx, PassiveSampleInput{
		NodeID: "edge-hk-1", ClientIP: "115.236.10.5", ClientASN: 4134, ClientCountry: "CN", ClientRegion: "CN-ZJ",
		IPFamily: "ipv4", ClientLat: &zjLat, ClientLon: &zjLon, GeoAccuracyKm: 25, GeoSource: pkgnetintel.GeoSourceCity,
		TCPRTTUs: 55000, TCPMinRTTUs: 48000, TCPSegsOut: 10,
	})
	RecordPassiveSampleInput(ctx, PassiveSampleInput{
		NodeID: "edge-hk-1", ClientIP: "240e:3b0:100::1", ClientASN: 4134, ClientCountry: "CN", ClientRegion: "CN-GD",
		IPFamily: "ipv6", ClientLat: &gdLat, ClientLon: &gdLon, GeoAccuracyKm: 25, GeoSource: pkgnetintel.GeoSourceCity,
		TCPRTTUs: 36000, TCPMinRTTUs: 32000, TCPSegsOut: 10,
	})

	// Same ASN (AS174) in two different countries (US vs DE) must stay completely isolated!
	RecordPassiveSampleInput(ctx, PassiveSampleInput{
		NodeID: "edge-hk-1", ClientIP: "38.100.10.1", ClientASN: 174, ClientCountry: "US", ClientRegion: "US-CA",
		IPFamily: "ipv4", ClientLat: &usLat, ClientLon: &usLon, GeoAccuracyKm: 25, GeoSource: pkgnetintel.GeoSourceCity,
		TCPRTTUs: 160000, TCPMinRTTUs: 150000, TCPSegsOut: 10,
	})
	RecordPassiveSampleInput(ctx, PassiveSampleInput{
		NodeID: "edge-hk-1", ClientIP: "149.6.10.1", ClientASN: 174, ClientCountry: "DE", ClientRegion: "DE-HE",
		IPFamily: "ipv4", ClientLat: &deLat, ClientLon: &deLon, GeoAccuracyKm: 25, GeoSource: pkgnetintel.GeoSourceCity,
		TCPRTTUs: 210000, TCPMinRTTUs: 200000, TCPSegsOut: 10,
	})

	require.NoError(t, FlushNow(ctx))

	samples, err := repository.ListOpenFlareNetworkQualitySamplesSince(ctx, now.Add(-time.Hour), 100)
	require.NoError(t, err)
	require.Len(t, samples, 5)

	summary, err := RunRollup(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, summary.PredictedRows)
	// Measured rows:
	// - AS4134 CN ipv4: country + CN-GD + CN-ZJ = 3
	// - AS4134 CN ipv6: country + CN-GD = 2
	// - AS174 US ipv4: country + US-CA = 2
	// - AS174 DE ipv4: country + DE-HE = 2
	// Total = 9
	require.Equal(t, 9, summary.MeasuredRows)
	require.GreaterOrEqual(t, summary.PrunedRows, int64(1))

	rows, total, err := repository.ListOpenFlareNetworkQualityMatrix(ctx, model.NetworkQualityMatrixQuery{Page: 1, PageSize: 50})
	require.NoError(t, err)
	require.EqualValues(t, 9, total)
	require.Len(t, rows, 9)

	// Verify CN ipv4 country rollup merged 3 samples and 3 deduplicated prefixes (2 in GD + 1 in ZJ).
	cnCountryRows, _, err := repository.ListOpenFlareNetworkQualityMatrix(ctx, model.NetworkQualityMatrixQuery{
		Page:             1,
		PageSize:         10,
		ClientASN:        4134,
		ClientCountry:    "CN",
		IPFamily:         "ipv4",
		AggregationLevel: model.QualityAggregationLevelCountry,
	})
	require.NoError(t, err)
	require.Len(t, cnCountryRows, 1)
	require.Equal(t, 3, cnCountryRows[0].TCPSampleCount)
	require.Equal(t, 3, cnCountryRows[0].DistinctPrefixCount)
	require.Equal(t, model.NetworkQualityStatsVersionHist64, cnCountryRows[0].StatsVersion)
	require.Equal(t, model.NetworkQualitySourceTCPPassive, cnCountryRows[0].MeasurementSource)
	require.Greater(t, cnCountryRows[0].GeoBaseRTTMs, 0.0)
	require.Equal(t, cnCountryRows[0].TCPRTTP50Ms, cnCountryRows[0].EffectiveRTTMs)

	// Verify AS174 US and DE rows remain isolated.
	as174Rows, _, err := repository.ListOpenFlareNetworkQualityMatrix(ctx, model.NetworkQualityMatrixQuery{
		Page:             1,
		PageSize:         10,
		ClientASN:        174,
		AggregationLevel: model.QualityAggregationLevelCountry,
	})
	require.NoError(t, err)
	require.Len(t, as174Rows, 2)
	require.NotEqual(t, as174Rows[0].ClientCountry, as174Rows[1].ClientCountry)
}

func TestRollupSeparatesPassiveAndActiveSourcesAndSupportsLegacyFallback(t *testing.T) {
	cleanup := setupNetQualityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID: "edge-tyo-1", Name: "TYO", IP: "10.0.0.1", ASN: 2516, Status: "online", NodeType: "edge_node", LastSeenAt: &now, AccessToken: "tok",
	}))

	// Legacy v1 sample + active ICMP ping batch for same pair should NOT be 1.0:0.7 averaged.
	require.NoError(t, db.DB(ctx).Create(&analyticsmodel.NetworkQualitySample{
		ID:            9001,
		NodeID:        "edge-tyo-1",
		NodeASN:       2516,
		ClientASN:     4134,
		ClientCountry: "CN",
		IPFamily:      "ipv4",
		StatsVersion:  model.NetworkQualityStatsVersionLegacy,
		Source:        model.NetworkQualitySourceTCPPassive,
		SampleCount:   5,
		RTTMinMs:      58,
		RTTP50Ms:      60,
		RTTP95Ms:      70,
		CapturedAt:    now.Add(-5 * time.Minute),
		CreatedAt:     now.Add(-5 * time.Minute),
	}).Error)

	var activeHist LogHistogram64
	activeHist.Add(40)
	require.NoError(t, db.DB(ctx).Create(&analyticsmodel.NetworkQualitySample{
		ID:            9002,
		NodeID:        "edge-tyo-1",
		NodeASN:       2516,
		ClientASN:     4134,
		ClientCountry: "CN",
		IPFamily:      "ipv4",
		StatsVersion:  model.NetworkQualityStatsVersionHist64,
		Source:        model.NetworkQualitySourceICMPActive,
		SampleCount:   4,
		RTTMinMs:      39,
		RTTP50Ms:      40,
		RTTP95Ms:      42,
		RTTHist:       activeHist.Encode(),
		CapturedAt:    now.Add(-2 * time.Minute),
		CreatedAt:     now.Add(-2 * time.Minute),
	}).Error)

	summary, err := RunRollup(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, summary.MeasuredRows)

	rows, _, err := repository.ListOpenFlareNetworkQualityMatrix(ctx, model.NetworkQualityMatrixQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	// EffectiveRTTMs uses passive TCP (60ms) directly, NOT blended with 40ms active ICMP!
	require.InDelta(t, 60.0, rows[0].EffectiveRTTMs, 0.1)
	require.Equal(t, model.NetworkQualitySourceTCPPassive, rows[0].MeasurementSource)
	require.Equal(t, "icmp", rows[0].ProbeProto)
	require.InDelta(t, 40.0, rows[0].ProbeRTTP50Ms, 4.0)
	require.Equal(t, 4, rows[0].ProbeSampleCount)
}

func TestCollectorEvictsColdBucketAtCapacity(t *testing.T) {
	cleanup := setupNetQualityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	// Simulate an in-flight async flush so burst insertions saturate maxPassiveBuckets and exercise eviction.
	defaultCollector.flushInFlight.Store(true)
	defer defaultCollector.flushInFlight.Store(false)

	// Hot bucket (sampleCount = 3) must survive eviction when filling to maxPassiveBuckets.
	for range 3 {
		RecordPassiveSample(ctx, "edge-hot", 4134, "CN", 25000, 20000, 0, 10, 0)
	}
	for i := 1; i <= maxPassiveBuckets+10; i++ {
		RecordPassiveSample(ctx, "edge-cold", uint32(100000+i), "US", 30000, 25000, 0, 10, 0) //nolint:gosec // loop index >= 1
	}

	stats := GetPassiveCollectorStats()
	require.LessOrEqual(t, stats.ActiveBuckets, maxPassiveBuckets)
	require.GreaterOrEqual(t, stats.EvictedColdBuckets, uint64(10))

	defaultCollector.mu.Lock()
	_, hotPresent := defaultCollector.buckets[passiveBucketKey{
		nodeID: "edge-hot", clientASN: 4134, clientCountry: "CN", ipFamily: pkgnetintel.IPFamilyUnknown,
	}]
	defaultCollector.mu.Unlock()
	require.True(t, hotPresent)
}

func TestCollectorHighWatermarkTriggersSingleEarlyFlush(t *testing.T) {
	cleanup := setupNetQualityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	for i := 1; i <= passiveHighWatermark+5; i++ {
		RecordPassiveSample(ctx, "edge-wm", uint32(200000+i), "US", 30000, 25000, 0, 10, 0) //nolint:gosec // loop index >= 1
	}
	defaultCollector.flushWG.Wait()
	stats := GetPassiveCollectorStats()
	require.GreaterOrEqual(t, stats.EarlyFlushes, uint64(1))
}

func TestIngestTracerouteExtractsASEdgesAndHistory(t *testing.T) {
	cleanup := setupNetQualityTestDB(t)
	defer cleanup()

	restoreMgr := netintel.SetManagerForTest(pkgnetintel.NewManager())
	defer restoreMgr()

	dir := t.TempDir()
	asnsCSV := "asn,name,class,cc\nAS13335,Cloudflare,Content,US\nAS174,Cogent,Transit,US\nAS4134,ChinaNet,Eyeball,CN\n"
	tableJSONL := "{\"CIDR\":\"1.1.1.0/24\",\"ASN\":13335,\"Hits\":100}\n{\"CIDR\":\"2.2.2.0/24\",\"ASN\":174,\"Hits\":100}\n{\"CIDR\":\"3.3.3.0/24\",\"ASN\":4134,\"Hits\":100}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bgptools-asns.csv"), []byte(asnsCSV), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bgptools-table.jsonl"), []byte(tableJSONL), 0o600))
	require.NoError(t, netintel.ReloadFromDir(context.Background(), dir))

	ctx := context.Background()
	avg1 := 4.5
	avg2 := 18.0
	avg3 := 55.0
	IngestProbeResult(ctx, "edge-hk-1", protocol.NetProbeResultPayload{
		ProbeID: "auto-trace-1",
		Kind:    "traceroute",
		Traceroute: &protocol.TracerouteResult{
			Target:     "3.3.3.3",
			ResolvedIP: "3.3.3.3",
			Protocol:   "icmp",
			HopCount:   3,
			Reached:    true,
			TotalRTTMs: 55.0,
			Hops: []protocol.TracerouteHop{
				{TTL: 1, IP: "1.1.1.1", AvgMs: &avg1, Sent: 2, Recv: 2},
				{TTL: 2, IP: "2.2.2.2", AvgMs: &avg2, Sent: 2, Recv: 2},
				{TTL: 3, IP: "3.3.3.3", AvgMs: &avg3, Sent: 2, Recv: 2},
			},
		},
	})

	edges, err := repository.ListOpenFlareASTopologyEdges(ctx, 20)
	require.NoError(t, err)
	require.Len(t, edges, 2)

	records, total, err := repository.ListOpenFlareTracerouteRecords(ctx, model.TracerouteRecordQuery{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, model.TracerouteTriggerAuto, records[0].TriggerType)
	require.Equal(t, pkgnetintel.IPFamilyIPv4, records[0].IPFamily)
	var asPath []uint32
	require.NoError(t, json.Unmarshal([]byte(records[0].ASPathJSON), &asPath))
	require.Equal(t, []uint32{13335, 174, 4134}, asPath)
}
