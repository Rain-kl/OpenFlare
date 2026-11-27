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
		&model.OpenFlareProbeTarget{},
		&model.OpenFlareProbeTargetState{},
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

func TestRunAutoProbeSchedulerBackoffBudgetsAndAnycastIsolation(t *testing.T) {
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
	now := time.Now().UTC()
	for _, n := range []model.OpenFlareNode{
		{NodeID: "edge-best", Name: "Best", IP: "1.1.1.1", ASN: 13335, Status: "online", NodeType: "edge_node", LastSeenAt: &now, AccessToken: "tok-1", CapabilitiesJSON: "[]"},
		{NodeID: "edge-backup", Name: "Backup", IP: "2.2.2.2", ASN: 174, Status: "online", NodeType: "edge_node", LastSeenAt: &now, AccessToken: "tok-2", CapabilitiesJSON: "[]"},
	} {
		require.NoError(t, repository.CreateOpenFlareNode(ctx, &n))
	}

	// Mark edge-best as already high-confidence for AS4134 so backup node gets exploration priority,
	// while edge-best still probes the confirmed-anycast anchor.
	_, err := repository.ReplaceOpenFlareNetworkQualityMatrixSnapshot(ctx, []*model.OpenFlareNetworkQualityMatrix{
		{
			NodeID: "edge-best", NodeASN: 13335, ClientASN: 4134, ClientCountry: "CN", IPFamily: pkgnetintel.IPFamilyIPv4,
			AggregationLevel: model.QualityAggregationLevelCountry, StatsVersion: model.NetworkQualityStatsVersionHist64,
			TCPRTTP50Ms: 32, TCPSampleCount: 150, TCPLastSeenAt: &now, EffectiveRTTMs: 32,
			Confidence: 0.92, MeasurementSource: model.NetworkQualitySourceTCPPassive, DataSource: model.QualityMatrixDataSourceMeasured, LastObservedAt: &now,
		},
	}, now.Add(-time.Hour))
	require.NoError(t, err)

	ispTarget := &model.OpenFlareProbeTarget{
		Name: "ChinaNet ISP", IP: "3.3.3.3", IPFamily: pkgnetintel.IPFamilyIPv4, ProbeProto: model.ProbeProtoTraceroute,
		ASN: 4134, Country: "CN", TargetType: model.ProbeTargetTypeCustomerISP, AnycastStatus: pkgnetintel.AnycastStatusUnicast, Enabled: true,
	}
	anycastAnchor := &model.OpenFlareProbeTarget{
		Name: "CF Anchor", IP: "1.1.1.1", IPFamily: pkgnetintel.IPFamilyIPv4, ProbeProto: model.ProbeProtoICMP,
		ASN: 13335, Country: "US", TargetType: model.ProbeTargetTypeAnchor, AnycastStatus: pkgnetintel.AnycastStatusConfirmed, Enabled: true,
	}
	require.NoError(t, repository.CreateOpenFlareProbeTarget(ctx, ispTarget))
	require.NoError(t, repository.CreateOpenFlareProbeTarget(ctx, anycastAnchor))

	// Put edge-backup on active failure backoff for ispTarget; verify RunAutoProbe skips only that pair.
	futureBackoff := now.Add(30 * time.Minute)
	require.NoError(t, repository.MarkOpenFlareProbeDispatched(ctx, "edge-backup", ispTarget.ID, now.Add(-40*time.Minute), futureBackoff))

	type dispatchedItem struct {
		nodeID string
		kind   string
		target string
	}
	var sent []dispatchedItem
	origDispatch := dispatchNetProbeFn
	dispatchNetProbeFn = func(_ context.Context, nodeID string, task protocol.NetProbeTaskDispatch) bool {
		tgt := ""
		if task.Traceroute != nil {
			tgt = task.Traceroute.Target
		} else if task.Ping != nil {
			tgt = task.Ping.Target
		}
		sent = append(sent, dispatchedItem{nodeID: nodeID, kind: task.Kind, target: tgt})
		return true
	}
	defer func() { dispatchNetProbeFn = origDispatch }()

	summary, err := RunAutoProbe(ctx)
	require.NoError(t, err)
	require.True(t, summary.Enabled)
	require.Equal(t, 2, summary.OnlineNodes)
	require.GreaterOrEqual(t, summary.SkippedBackoff, 1)
	require.GreaterOrEqual(t, summary.Dispatched, 2)
	for _, s := range sent {
		require.False(t, s.nodeID == "edge-backup" && s.target == "3.3.3.3", "backed-off pair must be skipped")
	}

	// Simulate failed ping_batch against ispTarget from edge-best -> must increment backoff WITHOUT creating a 0ms matrix/sample row!
	beforeSamples, err := repository.ListOpenFlareNetworkQualitySamplesSince(ctx, now.Add(-time.Hour), 50)
	require.NoError(t, err)
	IngestProbeResult(ctx, "edge-best", protocol.NetProbeResultPayload{
		ProbeID: "auto-fail-1",
		Kind:    "ping_batch",
		Ping: &protocol.PingBatchResult{
			Protocol: "icmp",
			Target:   "3.3.3.3",
			Sent:     4,
			Received: 0,
			LossRate: 100,
			Error:    "request timed out",
		},
	})
	afterSamples, err := repository.ListOpenFlareNetworkQualitySamplesSince(ctx, now.Add(-time.Hour), 50)
	require.NoError(t, err)
	require.Len(t, afterSamples, len(beforeSamples), "failed/0ms probes must NEVER write quality samples")

	states, err := repository.ListOpenFlareProbeTargetStates(ctx, "edge-best", []uint{ispTarget.ID})
	require.NoError(t, err)
	require.Len(t, states, 1)
	require.GreaterOrEqual(t, states[0].ConsecutiveFailures, 1)
	require.NotNil(t, states[0].NextProbeAt)
	require.True(t, states[0].NextProbeAt.After(now))

	// Confirmed anycast anchor traceroute must NOT pollute AS topology edges or quality samples, while still resetting failure state.
	avg1 := 3.0
	avg2 := 8.0
	IngestProbeResult(ctx, "edge-backup", protocol.NetProbeResultPayload{
		ProbeID: "auto-anycast-1",
		Kind:    "traceroute",
		Traceroute: &protocol.TracerouteResult{
			Target:     "1.1.1.1",
			ResolvedIP: "1.1.1.1",
			Protocol:   "icmp",
			HopCount:   2,
			Reached:    true,
			TotalRTTMs: 8.0,
			Hops: []protocol.TracerouteHop{
				{TTL: 1, IP: "2.2.2.2", AvgMs: &avg1, Sent: 2, Recv: 2},
				{TTL: 2, IP: "1.1.1.1", AvgMs: &avg2, Sent: 2, Recv: 2},
			},
		},
	})
	edges, err := repository.ListOpenFlareASTopologyEdges(ctx, 20)
	require.NoError(t, err)
	require.Empty(t, edges, "confirmed anycast target must be isolated from AS topology edges")

	anchorStates, err := repository.ListOpenFlareProbeTargetStates(ctx, "edge-backup", []uint{anycastAnchor.ID})
	require.NoError(t, err)
	require.Len(t, anchorStates, 1)
	require.Zero(t, anchorStates[0].ConsecutiveFailures)
	require.InDelta(t, 8.0, anchorStates[0].LastRTTMs, 0.1)
}

func TestDynamicP95WeightScaling(t *testing.T) {
	require.Equal(t, 0.0, computeDynamicP95Weight(9, 10))
	wSmall := computeDynamicP95Weight(10, 1)
	wMid := computeDynamicP95Weight(25, 3)
	wLarge := computeDynamicP95Weight(120, 20)
	require.Greater(t, wSmall, 0.0)
	require.Less(t, wSmall, wMid)
	require.Less(t, wMid, wLarge)
	require.InDelta(t, p95MaxTailWeight, wLarge, 0.015)
}

func TestPaginatedRollupReadsPastLegacy20kLimit(t *testing.T) {
	cleanup := setupNetQualityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID: "edge-page-1", Name: "Page", IP: "198.51.100.1", ASN: 13335,
		Status: "online", NodeType: "edge_node", LastSeenAt: &now, AccessToken: "tok-p",
	}))

	// Insert > rollupSamplePageSize (5000) rows to force keyset pagination across pages.
	var pageHist LogHistogram64
	pageHist.Add(30)
	encodedHist := pageHist.Encode()

	const totalRows = rollupSamplePageSize + 55
	batch := make([]analyticsmodel.NetworkQualitySample, 0, totalRows)
	for i := range totalRows {
		ts := now.Add(-time.Duration(i%300) * time.Second)
		batch = append(batch, analyticsmodel.NetworkQualitySample{
			ID:            uint64(100000 + i), //nolint:gosec // positive test ID
			NodeID:        "edge-page-1",
			NodeASN:       13335,
			ClientASN:     4134,
			ClientCountry: "CN",
			IPFamily:      pkgnetintel.IPFamilyIPv4,
			StatsVersion:  model.NetworkQualityStatsVersionHist64,
			Source:        model.NetworkQualitySourceTCPPassive,
			SampleCount:   1,
			RTTMinMs:      28,
			RTTP50Ms:      30,
			RTTP95Ms:      38,
			RTTHist:       encodedHist,
			CapturedAt:    ts,
			CreatedAt:     ts,
		})
	}
	s, err := repository.ListOpenFlareNetworkQualitySamplesPage(ctx, now.Add(-time.Hour), nil, 0, 10)
	require.NoError(t, err)
	require.Empty(t, s)
	require.NoError(t, db.DB(ctx).CreateInBatches(batch, 500).Error)

	summary, err := RunRollup(ctx)
	require.NoError(t, err)
	require.Equal(t, totalRows, summary.SamplesRead)

	rows, _, err := repository.ListOpenFlareNetworkQualityMatrix(ctx, model.NetworkQualityMatrixQuery{Page: 1, PageSize: 10, NodeID: "edge-page-1"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, totalRows, rows[0].TCPSampleCount)
}

func TestAdaptiveStateMachineSpikeStaleRecoveryAndExplorePriority(t *testing.T) {
	cleanup := setupNetQualityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()
	for _, n := range []model.OpenFlareNode{
		{NodeID: "hk-a", Name: "HK-A", IP: "203.0.113.1", ASN: 4134, Status: "online", NodeType: "edge_node", LastSeenAt: &now, AccessToken: "tok-a"},
		{NodeID: "jp-b", Name: "JP-B", IP: "203.0.113.2", ASN: 2914, Status: "online", NodeType: "edge_node", LastSeenAt: &now, AccessToken: "tok-b"},
		{NodeID: "us-d", Name: "US-D", IP: "203.0.113.4", ASN: 174, Status: "online", NodeType: "edge_node", LastSeenAt: &now, AccessToken: "tok-d"},
	} {
		require.NoError(t, repository.CreateOpenFlareNode(ctx, &n))
	}

	insertPassive := func(id uint64, nodeID string, age time.Duration, minMs, p50Ms, p95Ms float64, cnt int32) {
		var h LogHistogram64
		for range max(1, int(cnt)) {
			h.Add(p50Ms)
		}
		var sk PrefixSketch
		sk.AddIP("113.108.10.1")
		sk.AddIP("113.109.10.1")
		sk.AddIP("113.110.10.1")
		sk.AddIP("113.111.10.1")
		ts := now.Add(-age)
		require.NoError(t, db.DB(ctx).Create(&analyticsmodel.NetworkQualitySample{
			ID:                  id,
			NodeID:              nodeID,
			ClientASN:           9808,
			ClientCountry:       "CN",
			IPFamily:            pkgnetintel.IPFamilyIPv4,
			StatsVersion:        model.NetworkQualityStatsVersionHist64,
			Source:              model.NetworkQualitySourceTCPPassive,
			SampleCount:         cnt,
			RTTMinMs:            minMs,
			RTTP50Ms:            p50Ms,
			RTTP95Ms:            p95Ms,
			RTTHist:             h.Encode(),
			PrefixSketch:        sk.Encode(),
			DistinctPrefixCount: int32(sk.DistinctCount()), //nolint:gosec // bounded <= 64
			CapturedAt:          ts,
			CreatedAt:           ts,
		}).Error)
	}

	// Daytime history on hk-a (30ms, 3h-6h ago) + evening spike in last 5m (180ms, 10 samples).
	insertPassive(1, "hk-a", 6*time.Hour, 28, 30, 36, 120)
	insertPassive(2, "hk-a", 3*time.Hour, 28, 30, 36, 120)
	insertPassive(3, "hk-a", 5*time.Minute, 28, 180, 230, 10)
	// Stable JP-B (65ms) and hopelessly slow US-D (220ms).
	insertPassive(4, "jp-b", 3*time.Minute, 60, 65, 80, 40)
	insertPassive(5, "us-d", 4*time.Hour, 200, 220, 250, 40)

	_, err := RunRollup(ctx)
	require.NoError(t, err)

	fetchRow := func(nodeID string) *model.OpenFlareNetworkQualityMatrix {
		rows, _, listErr := repository.ListOpenFlareNetworkQualityMatrix(ctx, model.NetworkQualityMatrixQuery{
			Page: 1, PageSize: 10, NodeID: nodeID, ClientASN: 9808, ClientCountry: "CN", IPFamily: "ipv4",
		})
		require.NoError(t, listErr)
		require.Len(t, rows, 1)
		return rows[0]
	}

	hk1 := fetchRow("hk-a")
	require.Equal(t, model.QualityStateDegraded, hk1.QualityState)
	require.Greater(t, hk1.EffectiveRTTMs, 150.0)
	require.Greater(t, hk1.ObservedRTTMs, 150.0)
	require.Less(t, hk1.PredictedRTTMs, 45.0)
	// Freshly verified degraded node (5m < 15m cooldown, 10 >= 5 samples) must auto-brake exploration.
	require.Equal(t, 0.0, hk1.ExplorePriority)
	// Hopeless US-D (> 65*1.5 + 30 = 127.5ms) must also get 0 exploration priority.
	us1 := fetchRow("us-d")
	require.Equal(t, 0.0, us1.ExplorePriority)

	// Move hk-a's spike sample to 90m ago so hk-a becomes stale (> 60m without samples).
	staleTS := now.Add(-90 * time.Minute)
	require.NoError(t, db.DB(ctx).Model(&analyticsmodel.NetworkQualitySample{}).
		Where("id = ?", 3).
		Updates(map[string]any{"captured_at": staleTS, "created_at": staleTS}).Error)

	_, err = RunRollup(ctx)
	require.NoError(t, err)

	hkStale := fetchRow("hk-a")
	require.Equal(t, model.QualityStateStale, hkStale.QualityState)
	// Critical invariant: staleness must NEVER drop observed/effective RTT down to daytime 30ms!
	require.Greater(t, hkStale.EffectiveRTTMs, 150.0)
	require.Greater(t, hkStale.ObservedRTTMs, 150.0)
	require.Less(t, hkStale.Confidence, hk1.Confidence)
	require.Greater(t, hkStale.ExplorePriority, 0.25)

	// Predictor must honor EffectiveRTTMs while marking stale and preserving impaired RTT.
	p, err := LoadLivePredictor(ctx)
	require.NoError(t, err)
	predRes := p.Predict(PredictQuery{NodeID: "hk-a", ClientASN: 9808, ClientCountry: "CN", IPFamily: "ipv4"})
	require.Equal(t, model.QualityStateStale, predRes.QualityState)
	require.Greater(t, predRes.RTTMs, 150.0)
	require.Greater(t, predRes.ExplorePriority, 0.25)

	// Window 1 of next-day recovery: 4 fresh 35ms samples -> enters recovering (holds high effective RTT!).
	insertPassive(6, "hk-a", 3*time.Minute, 28, 35, 42, 4)
	_, err = RunRollup(ctx)
	require.NoError(t, err)

	hkRec1 := fetchRow("hk-a")
	require.Equal(t, model.QualityStateRecovering, hkRec1.QualityState)
	require.Equal(t, 1, hkRec1.ConsecutiveGoodWindows)
	require.Less(t, hkRec1.ObservedRTTMs, 45.0)
	require.Greater(t, hkRec1.EffectiveRTTMs, 150.0)
	require.Greater(t, hkRec1.ExplorePriority, 0.5)

	// Repeat rollup with NO newer sample -> must NOT double-count confirmation window!
	_, err = RunRollup(ctx)
	require.NoError(t, err)
	hkRecStill := fetchRow("hk-a")
	require.Equal(t, model.QualityStateRecovering, hkRecStill.QualityState)
	require.Equal(t, 1, hkRecStill.ConsecutiveGoodWindows)

	// Window 2 of recovery: newer fresh 35ms samples -> confirmed healthy & releases traffic!
	insertPassive(7, "hk-a", 30*time.Second, 28, 35, 42, 4)
	_, err = RunRollup(ctx)
	require.NoError(t, err)

	hkHealthy := fetchRow("hk-a")
	require.Equal(t, model.QualityStateHealthy, hkHealthy.QualityState)
	require.Equal(t, 2, hkHealthy.ConsecutiveGoodWindows)
	require.Less(t, hkHealthy.EffectiveRTTMs, 45.0)
	require.Equal(t, 0.0, hkHealthy.ExplorePriority)
}
