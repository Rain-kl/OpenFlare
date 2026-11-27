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
	pkggeoip "github.com/Rain-kl/Wavelet/pkg/geoip"
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
		&model.OpenFlareProbeTarget{},
		&model.OpenFlareProbeTargetState{},
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

func TestPredictQualityMatrixAndASPathWithRegionAndIPFamily(t *testing.T) {
	cleanup := setupNetToolsTestEnv(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID:           "edge-hk-cn2",
		Name:             "Edge HK CN2",
		IP:               "1.1.1.1",
		ASN:              13335,
		NetworkGroup:     "cn2-gia",
		GeoSource:        pkgnetintel.GeoSourceCity,
		Status:           "online",
		NodeType:         "edge_node",
		LastSeenAt:       &now,
		AccessToken:      "tok-hk-cn2",
		CapabilitiesJSON: "[]",
	}))
	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID:           "edge-us-bgp",
		Name:             "Edge US BGP",
		IP:               "2.2.2.2",
		ASN:              174,
		NetworkGroup:     "bgp",
		GeoSource:        pkgnetintel.GeoSourceCountry,
		Status:           "online",
		NodeType:         "edge_node",
		LastSeenAt:       &now,
		AccessToken:      "tok-us-bgp",
		CapabilitiesJSON: "[]",
	}))

	_, err := repository.ReplaceOpenFlareNetworkQualityMatrixSnapshot(ctx, []*model.OpenFlareNetworkQualityMatrix{
		{
			NodeID:            "edge-hk-cn2",
			NodeASN:           13335,
			ClientASN:         4134,
			ClientCountry:     "CN",
			ClientRegion:      "CN-GD",
			IPFamily:          pkgnetintel.IPFamilyIPv4,
			AggregationLevel:  model.QualityAggregationLevelRegion,
			StatsVersion:      model.NetworkQualityStatsVersionHist64,
			TCPRTTP50Ms:       24.0,
			TCPRTTP95Ms:       31.0,
			TCPSampleCount:    18,
			TCPLastSeenAt:     &now,
			EffectiveRTTMs:    24.0,
			MeasurementSource: model.NetworkQualitySourceTCPPassive,
			Confidence:        0.86,
			QualityScore:      76.92,
			DataSource:        model.QualityMatrixDataSourceMeasured,
			LastObservedAt:    &now,
		},
		{
			NodeID:            "edge-hk-cn2",
			NodeASN:           13335,
			ClientASN:         4134,
			ClientCountry:     "CN",
			ClientRegion:      "",
			IPFamily:          pkgnetintel.IPFamilyIPv4,
			AggregationLevel:  model.QualityAggregationLevelCountry,
			StatsVersion:      model.NetworkQualityStatsVersionHist64,
			TCPRTTP50Ms:       35.0,
			TCPRTTP95Ms:       48.0,
			TCPSampleCount:    42,
			TCPLastSeenAt:     &now,
			EffectiveRTTMs:    35.0,
			MeasurementSource: model.NetworkQualitySourceTCPPassive,
			Confidence:        0.91,
			QualityScore:      69.57,
			DataSource:        model.QualityMatrixDataSourceMeasured,
			LastObservedAt:    &now,
		},
		{
			NodeID:            "edge-us-bgp",
			NodeASN:           174,
			ClientASN:         4134,
			ClientCountry:     "CN",
			ClientRegion:      "",
			IPFamily:          pkgnetintel.IPFamilyIPv4,
			AggregationLevel:  model.QualityAggregationLevelCountry,
			StatsVersion:      model.NetworkQualityStatsVersionHist64,
			TCPRTTP50Ms:       185.0,
			TCPRTTP95Ms:       230.0,
			TCPSampleCount:    20,
			TCPLastSeenAt:     &now,
			EffectiveRTTMs:    185.0,
			MeasurementSource: model.NetworkQualitySourceTCPPassive,
			Confidence:        0.82,
			QualityScore:      30.19,
			DataSource:        model.QualityMatrixDataSourceMeasured,
			LastObservedAt:    &now,
		},
	}, now.Add(-time.Hour))
	require.NoError(t, err)

	r := testhelper.NewTestGinEngine()
	group := r.Group("/api/v1/d/network-tools")
	group.POST("/as-path", TraceASPathHandler)
	group.GET("/quality-matrix", ListQualityMatrixHandler)
	group.POST("/quality-matrix/predict", PredictQualityMatrixHandler)

	// 1. Exact region match on AS-path comparison.
	asBody, _ := json.Marshal(ASPathInput{
		TargetASN:     4134,
		TargetCountry: "CN",
		TargetRegion:  "GD",
		IPFamily:      pkgnetintel.IPFamilyIPv4,
	})
	asReq := httptest.NewRequest(http.MethodPost, "/api/v1/d/network-tools/as-path", bytes.NewReader(asBody))
	asReq.Header.Set("Content-Type", "application/json")
	asW := httptest.NewRecorder()
	r.ServeHTTP(asW, asReq)
	require.Equal(t, http.StatusOK, asW.Code)
	var asPayload struct {
		Data ASPathResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(asW.Body.Bytes(), &asPayload))
	require.Equal(t, "CN-GD", asPayload.Data.TargetRegion)
	require.Len(t, asPayload.Data.Nodes, 2)
	require.Equal(t, "edge-hk-cn2", asPayload.Data.Nodes[0].NodeID)
	require.Equal(t, netquality.PredictionMethodExactRegion, asPayload.Data.Nodes[0].PredictionMethod)
	require.InDelta(t, 24.0, asPayload.Data.Nodes[0].EffectiveRTTMs, 0.1)

	// 2. Country-level prediction simulation via /quality-matrix/predict.
	predBody, _ := json.Marshal(QualityPredictInput{
		TargetASN:     4134,
		TargetCountry: "CN",
		IPFamily:      pkgnetintel.IPFamilyIPv4,
	})
	predReq := httptest.NewRequest(http.MethodPost, "/api/v1/d/network-tools/quality-matrix/predict", bytes.NewReader(predBody))
	predReq.Header.Set("Content-Type", "application/json")
	predW := httptest.NewRecorder()
	r.ServeHTTP(predW, predReq)
	require.Equal(t, http.StatusOK, predW.Code)
	var predPayload struct {
		Data QualityPredictResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(predW.Body.Bytes(), &predPayload))
	require.Len(t, predPayload.Data.Nodes, 2)
	require.Equal(t, "edge-hk-cn2", predPayload.Data.Nodes[0].NodeID)
	require.Equal(t, netquality.PredictionMethodCountryRollup, predPayload.Data.Nodes[0].PredictionMethod)
	require.Equal(t, "cn2-gia", predPayload.Data.Nodes[0].NetworkGroup)
	require.InDelta(t, 35.0, predPayload.Data.Nodes[0].EffectiveRTTMs, 0.1)

	// 3. Matrix query filters: aggregation_level & ip_family.
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/d/network-tools/quality-matrix?aggregation_level=region&ip_family=ipv4", nil)
	listW := httptest.NewRecorder()
	r.ServeHTTP(listW, listReq)
	require.Equal(t, http.StatusOK, listW.Code)
	var listPayload struct {
		Data QualityMatrixPage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listW.Body.Bytes(), &listPayload))
	require.Equal(t, int64(1), listPayload.Data.Total)
	require.Equal(t, "CN-GD", listPayload.Data.Results[0].ClientRegion)
	require.Equal(t, "cn2-gia", listPayload.Data.Results[0].NodeNetworkGroup)
}

func TestProbeTargetCRUDAndSSRFValidation(t *testing.T) {
	cleanup := setupNetToolsTestEnv(t)
	defer cleanup()

	r := testhelper.NewTestGinEngine()
	group := r.Group("/api/v1/d/network-tools")
	group.GET("/probe-targets", ListProbeTargetsHandler)
	group.POST("/probe-targets", CreateProbeTargetHandler)
	group.PUT("/probe-targets/:id", UpdateProbeTargetHandler)
	group.DELETE("/probe-targets/:id", DeleteProbeTargetHandler)

	// 1. SSRF validation blocks private, loopback, link-local, CGNAT, and benchmark IPs.
	for _, blocked := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "198.18.0.1", "fe80::1"} {
		payload, _ := json.Marshal(ProbeTargetInput{
			Name:       "Blocked",
			IP:         blocked,
			ProbeProto: model.ProbeProtoICMP,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/d/network-tools/probe-targets", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code, "expected SSRF rejection for %s", blocked)
	}

	// 2. Create public ISP TCP target -> succeeds and auto-enriches ASN/country/ip_family.
	createBody, _ := json.Marshal(ProbeTargetInput{
		Name:       "CT Guangdong Gateway",
		IP:         "3.3.3.3",
		ProbeProto: model.ProbeProtoTCP,
		Port:       443,
		Country:    "CN",
		Region:     "GD",
		TargetType: model.ProbeTargetTypeCustomerISP,
	})
	cReq := httptest.NewRequest(http.MethodPost, "/api/v1/d/network-tools/probe-targets", bytes.NewReader(createBody))
	cReq.Header.Set("Content-Type", "application/json")
	cW := httptest.NewRecorder()
	r.ServeHTTP(cW, cReq)
	require.Equal(t, http.StatusOK, cW.Code)

	var createResp struct {
		Data model.OpenFlareProbeTarget `json:"data"`
	}
	require.NoError(t, json.Unmarshal(cW.Body.Bytes(), &createResp))
	require.NotZero(t, createResp.Data.ID)
	require.Equal(t, pkgnetintel.IPFamilyIPv4, createResp.Data.IPFamily)
	require.Equal(t, "CN-GD", createResp.Data.Region)
	require.Equal(t, uint32(4134), createResp.Data.ASN)

	// 3. Duplicate (ip, probe_proto, port) rejected.
	dupReq := httptest.NewRequest(http.MethodPost, "/api/v1/d/network-tools/probe-targets", bytes.NewReader(createBody))
	dupReq.Header.Set("Content-Type", "application/json")
	dupW := httptest.NewRecorder()
	r.ServeHTTP(dupW, dupReq)
	require.Equal(t, http.StatusBadRequest, dupW.Code)

	// 4. Add a node backoff state row and verify GET /probe-targets joins state metadata.
	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID: "edge-test-pt", Name: "PT Edge", IP: "1.1.1.1", GeoName: "HK", Status: "online", NodeType: "edge_node", AccessToken: "tok-pt", CapabilitiesJSON: "[]",
	}))
	require.NoError(t, repository.RecordOpenFlareProbeTargetOutcome(
		ctx, createResp.Data.ID, "edge-test-pt", "3.3.3.3", "tcp", 443, false, 0, "dial timeout", now, 15*time.Minute, 24*time.Hour,
	))

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/d/network-tools/probe-targets?target_type=customer_isp", nil)
	listW := httptest.NewRecorder()
	r.ServeHTTP(listW, listReq)
	require.Equal(t, http.StatusOK, listW.Code)
	var listResp struct {
		Data ProbeTargetPage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listW.Body.Bytes(), &listResp))
	require.EqualValues(t, 1, listResp.Data.Total)
	require.Len(t, listResp.Data.Results[0].States, 1)
	require.Equal(t, 1, listResp.Data.Results[0].States[0].ConsecutiveFailures)
	require.Equal(t, "PT Edge", listResp.Data.Results[0].States[0].NodeName)

	// 5. Update enablement & anycast status, then delete cleanly.
	disabled := false
	confirmed := pkgnetintel.AnycastStatusConfirmed
	updBody, _ := json.Marshal(ProbeTargetUpdateInput{
		Enabled:       &disabled,
		AnycastStatus: &confirmed,
	})
	uReq := httptest.NewRequest(http.MethodPut, "/api/v1/d/network-tools/probe-targets/1", bytes.NewReader(updBody))
	uReq.Header.Set("Content-Type", "application/json")
	uW := httptest.NewRecorder()
	r.ServeHTTP(uW, uReq)
	require.Equal(t, http.StatusOK, uW.Code)

	dReq := httptest.NewRequest(http.MethodDelete, "/api/v1/d/network-tools/probe-targets/1", nil)
	dW := httptest.NewRecorder()
	r.ServeHTTP(dW, dReq)
	require.Equal(t, http.StatusOK, dW.Code)

	statesAfter, err := repository.ListOpenFlareProbeTargetStates(ctx, "edge-test-pt", []uint{createResp.Data.ID})
	require.NoError(t, err)
	require.Empty(t, statesAfter)
}

func TestPredictQualityMatrixCoLocatedZeroGeoPriorAndUnknownGeo(t *testing.T) {
	cleanup := setupNetToolsTestEnv(t)
	defer cleanup()
	netquality.ResetPredictorCacheForTest()
	defer netquality.ResetPredictorCacheForTest()

	usLat, usLon, ok := pkggeoip.CountryCentroidByISO("US")
	require.True(t, ok)

	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID:           "edge-us-colocated",
		Name:             "Edge US Co-located",
		IP:               "1.1.1.1",
		ASN:              13335,
		GeoLatitude:      &usLat,
		GeoLongitude:     &usLon,
		GeoSource:        pkgnetintel.GeoSourceCountry,
		Status:           "online",
		NodeType:         "edge_node",
		LastSeenAt:       &now,
		AccessToken:      "tok-us-colocated",
		CapabilitiesJSON: "[]",
	}))
	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID:           "edge-no-geo",
		Name:             "Edge No Geo",
		IP:               "2.2.2.2",
		ASN:              174,
		Status:           "online",
		NodeType:         "edge_node",
		LastSeenAt:       &now,
		AccessToken:      "tok-no-geo",
		CapabilitiesJSON: "[]",
	}))

	r := testhelper.NewTestGinEngine()
	group := r.Group("/api/v1/d/network-tools")
	group.POST("/quality-matrix/predict", PredictQualityMatrixHandler)

	body, _ := json.Marshal(QualityPredictInput{
		TargetASN:     174,
		TargetCountry: "US",
		IPFamily:      pkgnetintel.IPFamilyIPv4,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/d/network-tools/quality-matrix/predict", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data QualityPredictResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Data.Nodes, 2)

	colocated := resp.Data.Nodes[0]
	require.Equal(t, "edge-us-colocated", colocated.NodeID)
	require.Equal(t, netquality.PredictionMethodGeoPrior, colocated.PredictionMethod)
	require.Equal(t, model.MeasurementSourceNone, colocated.MeasurementSource)
	require.True(t, colocated.GeoBaseKnown)
	require.Equal(t, 0.0, colocated.GeoBaseRTTMs)
	require.InDelta(t, 8.0, colocated.EffectiveRTTMs, 0.01)
	require.InDelta(t, 10.0, colocated.RTTP95Ms, 0.01)

	noGeo := resp.Data.Nodes[1]
	require.Equal(t, "edge-no-geo", noGeo.NodeID)
	require.Equal(t, netquality.PredictionMethodGeoPrior, noGeo.PredictionMethod)
	require.Equal(t, model.MeasurementSourceNone, noGeo.MeasurementSource)
	require.False(t, noGeo.GeoBaseKnown)
	require.Equal(t, 0.0, noGeo.GeoBaseRTTMs)
	require.InDelta(t, 120.0, noGeo.EffectiveRTTMs, 0.01)
	require.InDelta(t, 120.0, noGeo.RTTP95Ms, 0.01)
}
