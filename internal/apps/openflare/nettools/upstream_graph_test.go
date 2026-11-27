// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nettools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	"github.com/Rain-kl/Wavelet/internal/repository"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
	"github.com/stretchr/testify/require"
)

func TestGetASNUpstreamGraphRIPEStatLookingGlassAndCache(t *testing.T) {
	cleanup := setupNetToolsTestEnv(t)
	defer cleanup()
	ResetUpstreamGraphCacheForTest()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch {
		case strings.Contains(r.URL.Path, "looking-glass"):
			require.Equal(t, "198.51.100.0/24", r.URL.Query().Get("resource"))
			payload := map[string]any{
				"data": map[string]any{
					"rrcs": []map[string]any{
						{
							"peers": []map[string]any{
								{"as_path": "3356 7018 174 174 979"},
								{"as_path": "6939 1299 174 979"},
								{"as_path": "6939 3491 6453 979"},
								{"as_path": "3356 2914 979"},
								{"as_path": "6762 2914 979"},
								{"as_path": "64512 979"}, // private-only hop before origin -> ignored
							},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(payload))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	restoreBase := SetRIPEStatBaseURLForTest(srv.URL)
	defer restoreBase()

	resp, err := GetASNUpstreamGraph(context.Background(), 979, "198.51.100.0/24", false)
	require.NoError(t, err)
	require.False(t, resp.Cached)
	require.Equal(t, uint32(979), resp.OriginASN)
	require.Equal(t, "198.51.100.0/24", resp.SampledPrefix)
	require.Equal(t, upstreamSourceRIPEStatLG, resp.DataSource)
	require.Equal(t, 5, resp.TotalPathsParsed)
	require.Equal(t, int32(1), calls.Load())

	nodeByASN := make(map[uint32]ASUpstreamNode, len(resp.Nodes))
	for _, n := range resp.Nodes {
		nodeByASN[n.ASN] = n
	}
	require.Equal(t, upstreamRoleOrigin, nodeByASN[979].Role)
	require.Equal(t, 0, nodeByASN[979].Layer)

	// AS174, AS6453, AS2914 connect from origin AND branch to other Tier 1 ISPs -> tier1_upstream (layer 1).
	for _, upASN := range []uint32{174, 6453, 2914} {
		require.Equal(t, upstreamRoleTier1Upstream, nodeByASN[upASN].Role, "AS%d", upASN)
		require.Equal(t, 1, nodeByASN[upASN].Layer, "AS%d", upASN)
	}
	// Downstream Tier 1 peers -> tier1 (layer 2) with canonical short names.
	require.Equal(t, upstreamRoleTier1, nodeByASN[7018].Role)
	require.Equal(t, "AT&T", nodeByASN[7018].ShortName)
	require.Equal(t, 2, nodeByASN[7018].Layer)
	require.Equal(t, "Lumen", nodeByASN[3356].ShortName)
	require.Equal(t, "Arelion", nodeByASN[1299].ShortName)
	require.Equal(t, "PCCW", nodeByASN[3491].ShortName)
	require.Equal(t, "Sparkle", nodeByASN[6762].ShortName)

	// Verify edges only go forward (layer[from] < layer[to]) and no self loops.
	for _, e := range resp.Edges {
		require.Less(t, nodeByASN[e.From].Layer, nodeByASN[e.To].Layer)
	}

	// Second request hits L1 RAM cache without invoking RIPEstat again.
	cachedResp, err := GetASNUpstreamGraph(context.Background(), 979, "198.51.100.0/24", false)
	require.NoError(t, err)
	require.True(t, cachedResp.Cached)
	require.Equal(t, int32(1), calls.Load())
}

func TestGetASNUpstreamGraphCAIDAAndNeighboursFallback(t *testing.T) {
	cleanup := setupNetToolsTestEnv(t)
	defer cleanup()
	ResetUpstreamGraphCacheForTest()

	// 1. CAIDA multi-hop extraction: Origin AS4808 -> Transit AS4134 -> Tier 1 AS174 & AS2914.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bgptools-asns.csv"), []byte(
		"asn,name,class,cc\nAS4808,China Unicom Beijing,Eyeball,CN\nAS4134,China Telecom Corporation Limited,Eyeball,CN\nAS174,Cogent Communications,Transit,US\nAS2914,NTT America Inc.,Transit,US\n",
	), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "caida-as-rel2.txt"), []byte(
		"4134|4808|-1\n174|4134|-1\n2914|4134|-1\n",
	), 0o600))
	require.NoError(t, netintel.ReloadFromDir(context.Background(), dir))

	caidaResp, err := GetASNUpstreamGraph(context.Background(), 4808, "", true)
	require.NoError(t, err)
	require.Equal(t, upstreamSourceCAIDA, caidaResp.DataSource)
	byASN := make(map[uint32]ASUpstreamNode, len(caidaResp.Nodes))
	for _, n := range caidaResp.Nodes {
		byASN[n.ASN] = n
	}
	require.Equal(t, upstreamRoleOrigin, byASN[4808].Role)
	require.Equal(t, 0, byASN[4808].Layer)
	require.Equal(t, upstreamRoleTransit, byASN[4134].Role)
	require.Equal(t, 1, byASN[4134].Layer)
	require.Equal(t, "China Telecom", byASN[4134].ShortName)
	require.Equal(t, upstreamRoleTier1, byASN[174].Role)
	require.Equal(t, 2, byASN[174].Layer)
	require.Equal(t, upstreamRoleTier1, byASN[2914].Role)
	require.Equal(t, 2, byASN[2914].Layer)
	_ = pkgnetintel.IsTier1ASN(174)

	// 2. Fallback to RIPEstat asn-neighbours when CAIDA has no data for origin and looking-glass is empty.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "looking-glass"):
			_, _ = w.Write([]byte(`{"data":{"rrcs":[]}}`))
		case strings.Contains(r.URL.Path, "asn-neighbours"):
			require.Equal(t, "AS20473", r.URL.Query().Get("resource"))
			_, _ = w.Write([]byte(`{"data":{"neighbours":[{"asn":174,"type":"left","power":40},{"asn":1299,"type":"left","power":25},{"asn":4134,"type":"right","power":100}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	restoreBase := SetRIPEStatBaseURLForTest(srv.URL)
	defer restoreBase()

	fbResp, err := GetASNUpstreamGraph(context.Background(), 20473, "", true)
	require.NoError(t, err)
	require.Equal(t, upstreamSourceRIPENeigh, fbResp.DataSource)
	require.Len(t, fbResp.Edges, 2)
}

func TestLookupASNNeighborsRIPEStatFallbackAndObservedOverlay(t *testing.T) {
	cleanup := setupNetToolsTestEnv(t)
	defer cleanup()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Contains(t, r.URL.Path, "asn-neighbours")
		require.Equal(t, "AS13335", r.URL.Query().Get("resource"))
		_, _ = w.Write([]byte(`{"data":{"neighbours":[
			{"asn":174,"type":"left","power":120},
			{"asn":1299,"type":"left","power":50},
			{"asn":1299,"type":"right","power":30},
			{"asn":4134,"type":"right","power":40}
		]}}`))
	}))
	defer srv.Close()
	restoreBase := SetRIPEStatBaseURLForTest(srv.URL)
	defer restoreBase()

	ctx := context.Background()
	require.NoError(t, repository.UpsertOpenFlareASTopologyEdges(ctx, []repository.ObservedASEdge{
		{SrcASN: 13335, DstASN: 174, HopRTTMs: 14.5},
	}))

	detail, err := LookupASN(ctx, 13335)
	require.NoError(t, err)
	require.NotNil(t, detail.ASNInfo)
	require.Equal(t, 3, detail.ASNInfo.NeighborCnt)
	require.Len(t, detail.Neighbors, 3)

	byPeer := make(map[uint32]ASNeighborItem, len(detail.Neighbors))
	for _, nb := range detail.Neighbors {
		byPeer[nb.PeerASN] = nb
	}
	require.Equal(t, pkgnetintel.RelationProvider, byPeer[174].Relation)
	require.Equal(t, 14.5, byPeer[174].ObservedRTT)
	require.Equal(t, 1, byPeer[174].SampleCount)
	require.Equal(t, pkgnetintel.RelationPeer, byPeer[1299].Relation)
	require.Equal(t, pkgnetintel.RelationCustomer, byPeer[4134].Relation)

	// Second lookup serves RIPEstat neighbors from L1 RAM cache without another HTTP call.
	_, err = LookupASN(ctx, 13335)
	require.NoError(t, err)
	require.Equal(t, int32(1), calls.Load())
}
