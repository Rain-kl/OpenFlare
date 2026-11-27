// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nettools

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netquality"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupNetToolsTestEnv(t *testing.T) func() {
	t.Helper()
	ResetTaskStoreForTest()
	ResetUpstreamGraphCacheForTest()
	netquality.ResetCollectorForTest()
	restoreRIPE := SetRIPEStatBaseURLForTest("http://127.0.0.1:1")

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

	restoreMgr := netintel.SetManagerForTest(pkgnetintel.NewManager())
	dir := t.TempDir()
	asnsCSV := "asn,name,class,cc\nAS13335,Cloudflare,Content,US\nAS174,Cogent,Transit,US\nAS4134,ChinaNet,Eyeball,CN\n"
	tagsTxt := "anycast AS13335\neyeball AS4134\n"
	tableJSONL := "{\"CIDR\":\"1.1.1.0/24\",\"ASN\":13335,\"Hits\":500}\n{\"CIDR\":\"3.3.3.0/24\",\"ASN\":4134,\"Hits\":100}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bgptools-asns.csv"), []byte(asnsCSV), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bgptools-asn-tags.txt"), []byte(tagsTxt), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bgptools-table.jsonl"), []byte(tableJSONL), 0o600))
	require.NoError(t, netintel.ReloadFromDir(context.Background(), dir))

	return func() {
		restoreRIPE()
		restoreMgr()
		ResetTaskStoreForTest()
		ResetUpstreamGraphCacheForTest()
		netquality.ResetCollectorForTest()
		db.SetDB(nil)
	}
}

func TestSSRFTargetAndResolverValidation(t *testing.T) {
	ctx := context.Background()

	blockedTargets := []string{
		"127.0.0.1",
		"::1",
		"10.0.0.1",
		"172.16.5.4",
		"192.168.1.1",
		"169.254.169.254",
		"100.64.0.1",
		"0.0.0.0",
		"localhost",
		"meta.internal",
		"file:///etc/passwd",
	}
	for _, target := range blockedTargets {
		_, _, err := ValidateProbeTarget(ctx, target, false, false)
		require.Errorf(t, err, "expected target %q to be rejected", target)
	}

	norm, host, err := ValidateProbeTarget(ctx, "1.1.1.1", false, false)
	require.NoError(t, err)
	require.Equal(t, "1.1.1.1", norm)
	require.Equal(t, "1.1.1.1", host)

	_, host, err = ValidateProbeTarget(ctx, "https://example.com/healthz", true, false)
	require.NoError(t, err)
	require.Equal(t, "example.com", host)

	_, err = ValidateResolver(ctx, "127.0.0.1:53", false)
	require.Error(t, err)
	resolver, err := ValidateResolver(ctx, "8.8.8.8", false)
	require.NoError(t, err)
	require.Equal(t, "8.8.8.8:53", resolver)

	_, err = ValidateECSSubnet("10.0.0.0/8")
	require.Error(t, err)
	ecs, err := ValidateECSSubnet("1.1.1.5/24")
	require.NoError(t, err)
	require.Equal(t, "1.1.1.0/24", ecs)
}

func TestExecuteDirectDNSQueryWithECS(t *testing.T) {
	cleanup := setupNetToolsTestEnv(t)
	defer cleanup()

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = pc.Close() }()

	mux := dns.NewServeMux()
	mux.HandleFunc("example.com.", func(w dns.ResponseWriter, r *dns.Msg) {
		resp := new(dns.Msg)
		resp.SetReply(r)
		resp.Authoritative = true
		if len(r.Question) > 0 && r.Question[0].Qtype == dns.TypeA {
			resp.Answer = append(resp.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 120},
				A:   net.ParseIP("1.1.1.1").To4(),
			})
		}
		_ = w.WriteMsg(resp)
	})

	srv := &dns.Server{PacketConn: pc, Handler: mux}
	ready := make(chan struct{})
	srv.NotifyStartedFunc = func() { close(ready) }
	go func() { _ = srv.ActivateAndServe() }()
	defer func() { _ = srv.Shutdown() }()
	<-ready

	res, err := executeDirectDNSQuery(context.Background(), "example.com", "A", pc.LocalAddr().String(), "1.1.1.0/24", 2*time.Second)
	require.NoError(t, err)
	require.True(t, res.Authoritative)
	require.Equal(t, "NOERROR", res.RCode)
	require.Len(t, res.Records, 1)
	require.Equal(t, "1.1.1.1", res.Records[0].Value)
	require.Equal(t, uint32(13335), res.Records[0].ASN)
	require.Equal(t, "Cloudflare", res.Records[0].ASNName)
}

func TestDispatchAndResultLifecycle(t *testing.T) {
	cleanup := setupNetToolsTestEnv(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID:           "edge-test-1",
		Name:             "Edge Test 1",
		IP:               "1.1.1.1",
		ASN:              13335,
		Status:           "online",
		NodeType:         "edge_node",
		LastSeenAt:       &now,
		AccessToken:      "tok-edge-test-1",
		CapabilitiesJSON: "[]",
	}))

	state, err := DispatchTraceroute(ctx, TracerouteDispatchInput{
		Target:   "3.3.3.3",
		Protocol: "icmp",
		NodeIDs:  []string{"edge-test-1"},
	})
	require.NoError(t, err)
	require.Len(t, state.Nodes, 1)

	avg := 22.5
	HandleNetProbeResult(ctx, "edge-test-1", protocol.NetProbeResultPayload{
		ProbeID:    state.TaskID,
		Kind:       "traceroute",
		FinishedAt: now.Format(time.RFC3339),
		Traceroute: &protocol.TracerouteResult{
			Target:     "3.3.3.3",
			ResolvedIP: "3.3.3.3",
			Protocol:   "icmp",
			HopCount:   1,
			Reached:    true,
			TotalRTTMs: avg,
			Hops: []protocol.TracerouteHop{
				{TTL: 1, IP: "3.3.3.3", AvgMs: &avg, Sent: 3, Recv: 3},
			},
		},
	})

	updated, err := GetTask(ctx, state.TaskID)
	require.NoError(t, err)
	require.Equal(t, TaskStatusCompleted, updated.Status)
	require.Equal(t, NodeResultStatusCompleted, updated.Nodes[0].Status)
	require.NotNil(t, updated.Nodes[0].Traceroute)
	require.Equal(t, uint32(4134), updated.Nodes[0].Traceroute.Hops[0].ASN)

	// Verify Gin router wiring end-to-end
	r := testhelper.NewTestGinEngine()
	group := r.Group("/api/v1/d/network-tools")
	group.GET("/tasks/:task_id", GetTaskHandler)
	group.GET("/lookup/asn/:asn", LookupASNHandler)
	group.POST("/as-path", TraceASPathHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/d/network-tools/lookup/asn/13335", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, strings.Contains(w.Body.String(), "Cloudflare"))

	body, _ := json.Marshal(ASPathInput{TargetASN: 4134})
	pathReq := httptest.NewRequest(http.MethodPost, "/api/v1/d/network-tools/as-path", bytes.NewReader(body))
	pathReq.Header.Set("Content-Type", "application/json")
	pw := httptest.NewRecorder()
	r.ServeHTTP(pw, pathReq)
	require.Equal(t, http.StatusOK, pw.Code)

	_ = gin.Mode()
}
