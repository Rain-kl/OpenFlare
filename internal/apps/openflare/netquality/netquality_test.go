// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netquality

import (
	"context"
	"encoding/json"
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

func TestPassiveCollectorBucketsFlushAndRollup(t *testing.T) {
	cleanup := setupNetQualityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID:           "edge-hk-1",
		Name:             "HK-Edge-1",
		IP:               "1.1.1.1",
		ASN:              13335,
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
		Status:           "online",
		NodeType:         "edge_node",
		LastSeenAt:       &now,
		AccessToken:      "token-edge-sg-1",
		CapabilitiesJSON: "[]",
	}))

	// Invalid / zero / outlier samples should be ignored without panicking.
	RecordPassiveSample(ctx, "", 4134, "CN", 25000, 20000, 0, 10, 0)
	RecordPassiveSample(ctx, "edge-hk-1", 0, "CN", 25000, 20000, 0, 10, 0)
	RecordPassiveSample(ctx, "edge-hk-1", 4134, "CN", maxValidTCPRTTUs+1, 20000, 0, 10, 0)

	RecordPassiveSample(ctx, "edge-hk-1", 4134, "CN", 24000, 19000, 0, 20, 8)
	RecordPassiveSample(ctx, "edge-hk-1", 4134, "CN", 30000, 20000, 1, 20, 10)
	RecordPassiveSample(ctx, "edge-hk-1", 4134, "CN", 42000, 21000, 0, 10, 12)

	require.NoError(t, FlushNow(ctx))

	samples, err := repository.ListOpenFlareNetworkQualitySamplesSince(ctx, now.Add(-time.Hour), 100)
	require.NoError(t, err)
	require.Len(t, samples, 1)
	require.Equal(t, int32(3), samples[0].SampleCount)
	require.Equal(t, uint32(4134), samples[0].ClientASN)
	require.InDelta(t, 19.0, samples[0].RTTMinMs, 0.1)
	require.InDelta(t, 30.0, samples[0].RTTP50Ms, 0.1)

	summary, err := RunRollup(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, summary.MeasuredRows)
	require.Equal(t, 1, summary.PredictedRows)

	rows, total, err := repository.ListOpenFlareNetworkQualityMatrix(ctx, model.NetworkQualityMatrixQuery{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, rows, 2)
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
	var asPath []uint32
	require.NoError(t, json.Unmarshal([]byte(records[0].ASPathJSON), &asPath))
	require.Equal(t, []uint32{13335, 174, 4134}, asPath)
}
