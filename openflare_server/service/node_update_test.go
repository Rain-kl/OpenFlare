package service

import (
	"io"
	"net"
	"net/http"
	"openflare/common"
	"openflare/model"
	"openflare/utils/geoip"
	"sort"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

type fakeGeoIPProvider struct {
	info *geoip.GeoInfo
}

type fakeAccessLogGeoProvider struct {
	info *geoip.GeoInfo
}

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (f *fakeGeoIPProvider) Name() string {
	return "fake-geoip"
}

func (f *fakeGeoIPProvider) GetGeoInfo(ip net.IP) (*geoip.GeoInfo, error) {
	return f.info, nil
}

func (f *fakeGeoIPProvider) UpdateDatabase() error {
	return nil
}

func (f *fakeGeoIPProvider) Close() error {
	return nil
}

func (f *fakeAccessLogGeoProvider) Name() string {
	return "fake-access-log-geoip"
}

func (f *fakeAccessLogGeoProvider) GetGeoInfo(ip net.IP) (*geoip.GeoInfo, error) {
	return f.info, nil
}

func (f *fakeAccessLogGeoProvider) UpdateDatabase() error {
	return nil
}

func (f *fakeAccessLogGeoProvider) Close() error {
	return nil
}

func withFakeGeoIPProvider(t *testing.T, info *geoip.GeoInfo) {
	t.Helper()
	previous := geoip.CurrentProvider
	geoip.CurrentProvider = &fakeGeoIPProvider{info: info}
	t.Cleanup(func() {
		geoip.CurrentProvider = previous
	})
}

func withFakeAccessLogGeoProvider(t *testing.T, info *geoip.GeoInfo) {
	t.Helper()
	previous := accessLogGeoProviderFactory
	accessLogGeoProviderFactory = func() (geoip.GeoIPService, error) {
		return &fakeAccessLogGeoProvider{info: info}, nil
	}
	t.Cleanup(func() {
		accessLogGeoProviderFactory = previous
	})
}

func geoipFloat(value float64) *float64 {
	return &value
}

func TestRequestNodeAgentPreviewUpdate(t *testing.T) {
	setupServiceTestDB(t)

	latitude := 31.2304
	longitude := 121.4737
	node, err := CreateNode(NodeInput{
		Name:              "preview-edge-1",
		GeoManualOverride: true,
		GeoName:           "Shanghai",
		GeoLatitude:       &latitude,
		GeoLongitude:      &longitude,
	})
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	if node.GeoName != "Shanghai" || node.GeoLatitude == nil || node.GeoLongitude == nil {
		t.Fatalf("expected geo metadata to be returned, got %+v", node)
	}

	originalClient := UpdateHTTPClientForTest()
	SetUpdateHTTPClientForTest(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://api.github.com/repos/"+common.AgentUpdateRepo+"/releases/tags/v0.5.0-rc.1" {
				t.Fatalf("unexpected request url: %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v0.5.0-rc.1","prerelease":true}`)),
			}, nil
		}),
	})
	t.Cleanup(func() {
		SetUpdateHTTPClientForTest(originalClient)
	})

	updated, err := RequestNodeAgentUpdate(node.ID, NodeAgentUpdateInput{
		Channel: "preview",
		TagName: "v0.5.0-rc.1",
	})
	if err != nil {
		t.Fatalf("expected preview update request to succeed: %v", err)
	}
	if !updated.UpdateRequested {
		t.Fatal("expected update_requested to be true")
	}
	if updated.UpdateChannel != "preview" {
		t.Fatalf("unexpected update channel: %s", updated.UpdateChannel)
	}
	if updated.UpdateTag != "v0.5.0-rc.1" {
		t.Fatalf("unexpected update tag: %s", updated.UpdateTag)
	}
}

func TestHeartbeatNodeReturnsPreviewUpdateSettings(t *testing.T) {
	setupServiceTestDB(t)

	node := &model.Node{
		NodeID:                    "node-preview-1",
		Name:                      "preview-edge-1",
		IP:                        "10.0.0.8",
		AgentToken:                "agent-token",
		AgentVersion:              "v0.4.0",
		NginxVersion:              "1.27.1.2",
		Status:                    NodeStatusOnline,
		UpdateRequested:           true,
		UpdateChannel:             "preview",
		UpdateTag:                 "v0.5.0-rc.1",
		RestartOpenrestyRequested: true,
		AutoUpdateEnabled:         false,
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}
	if err := model.DB.Create(&model.ConfigVersion{
		Version:        "20260313-001",
		SnapshotJSON:   "{}",
		MainConfig:     "worker_processes auto;",
		RenderedConfig: "server { listen 80; }",
		Checksum:       "checksum-active-1",
		IsActive:       true,
		CreatedBy:      "root",
	}).Error; err != nil {
		t.Fatalf("failed to seed active config version: %v", err)
	}

	resp, err := HeartbeatNode(node, AgentNodePayload{
		NodeID:           node.NodeID,
		Name:             node.Name,
		IP:               node.IP,
		AgentVersion:     node.AgentVersion,
		NginxVersion:     node.NginxVersion,
		OpenrestyStatus:  OpenrestyStatusUnhealthy,
		OpenrestyMessage: "port 80 already allocated",
	})
	if err != nil {
		t.Fatalf("expected heartbeat to succeed: %v", err)
	}
	if resp.AgentSettings == nil {
		t.Fatal("expected agent settings in heartbeat response")
	}
	if resp.ActiveConfig == nil {
		t.Fatal("expected active config summary in heartbeat response")
	}
	if resp.ActiveConfig.Version == "" || resp.ActiveConfig.Checksum == "" {
		t.Fatal("expected active config summary to include version and checksum")
	}
	if !resp.AgentSettings.UpdateNow {
		t.Fatal("expected update_now to be true")
	}
	if resp.AgentSettings.UpdateChannel != "preview" {
		t.Fatalf("unexpected update channel: %s", resp.AgentSettings.UpdateChannel)
	}
	if resp.AgentSettings.UpdateTag != "v0.5.0-rc.1" {
		t.Fatalf("unexpected update tag: %s", resp.AgentSettings.UpdateTag)
	}
	if !resp.AgentSettings.RestartOpenrestyNow {
		t.Fatal("expected restart_openresty_now to be true")
	}
	if resp.Node.OpenrestyStatus != OpenrestyStatusUnhealthy {
		t.Fatalf("expected unhealthy openresty status, got %s", resp.Node.OpenrestyStatus)
	}
	if resp.Node.OpenrestyMessage != "port 80 already allocated" {
		t.Fatalf("unexpected openresty message: %s", resp.Node.OpenrestyMessage)
	}

	storedNode, err := model.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("failed to reload node: %v", err)
	}
	if storedNode.UpdateRequested {
		t.Fatal("expected update_requested to be reset after heartbeat")
	}
	if storedNode.UpdateChannel != "stable" {
		t.Fatalf("expected update channel to reset to stable, got %s", storedNode.UpdateChannel)
	}
	if storedNode.UpdateTag != "" {
		t.Fatalf("expected update tag to be cleared, got %s", storedNode.UpdateTag)
	}
	if storedNode.RestartOpenrestyRequested {
		t.Fatal("expected restart_openresty_requested to be reset after heartbeat")
	}
}

func TestUpdateNodeValidatesAndPersistsGeoMetadata(t *testing.T) {
	setupServiceTestDB(t)

	node, err := CreateNode(NodeInput{Name: "geo-edge"})
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	latitude := 37.7749
	longitude := -122.4194
	updated, err := UpdateNode(node.ID, NodeInput{
		Name:              "geo-edge-updated",
		AutoUpdateEnabled: true,
		GeoManualOverride: true,
		GeoName:           "San Francisco",
		GeoLatitude:       &latitude,
		GeoLongitude:      &longitude,
	})
	if err != nil {
		t.Fatalf("expected node update to succeed: %v", err)
	}
	if updated.GeoName != "San Francisco" || updated.GeoLatitude == nil || updated.GeoLongitude == nil {
		t.Fatalf("expected geo metadata in view, got %+v", updated)
	}

	stored, err := model.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("failed to load node: %v", err)
	}
	if stored.GeoName != "San Francisco" || stored.GeoLatitude == nil || stored.GeoLongitude == nil {
		t.Fatalf("expected geo metadata persisted, got %+v", stored)
	}
}

func TestUpdateNodeRejectsPartialGeoMetadata(t *testing.T) {
	setupServiceTestDB(t)

	node, err := CreateNode(NodeInput{Name: "geo-edge-invalid"})
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	latitude := 37.7749
	if _, err = UpdateNode(node.ID, NodeInput{
		Name:              "geo-edge-invalid",
		GeoManualOverride: true,
		GeoLatitude:       &latitude,
	}); err == nil {
		t.Fatal("expected partial geo metadata to be rejected")
	}
}

func TestUpdateNodeRejectsInvalidIP(t *testing.T) {
	setupServiceTestDB(t)

	node, err := CreateNode(NodeInput{Name: "geo-edge-invalid-ip"})
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if _, err = UpdateNode(node.ID, NodeInput{
		Name: "geo-edge-invalid-ip",
		IP:   "not-an-ip",
	}); err == nil {
		t.Fatal("expected invalid IP to be rejected")
	}
}

func TestUpdateNodeCanChangeIPAndAutoResolveGeo(t *testing.T) {
	setupServiceTestDB(t)
	withFakeGeoIPProvider(t, &geoip.GeoInfo{
		ISOCode:   "US",
		Name:      "United States",
		Latitude:  geoipFloat(37.7749),
		Longitude: geoipFloat(-122.4194),
	})

	node, err := CreateNode(NodeInput{Name: "geo-edge-auto-ip"})
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	updated, err := UpdateNode(node.ID, NodeInput{
		Name: "geo-edge-auto-ip",
		IP:   "8.8.8.8",
	})
	if err != nil {
		t.Fatalf("expected node update to succeed: %v", err)
	}
	if updated.IP != "8.8.8.8" {
		t.Fatalf("expected updated IP to be persisted, got %+v", updated.IP)
	}
	if updated.GeoName != "United States" {
		t.Fatalf("expected geo name to be auto resolved, got %+v", updated)
	}
	if updated.GeoLatitude == nil || updated.GeoLongitude == nil {
		t.Fatalf("expected geo coordinates to be auto resolved, got %+v", updated)
	}
}

func TestHeartbeatNodeResolvesGeoMetadataFromIPWhenNotManuallyOverridden(t *testing.T) {
	setupServiceTestDB(t)
	withFakeGeoIPProvider(t, &geoip.GeoInfo{
		ISOCode:   "US",
		Name:      "United States",
		Latitude:  geoipFloat(37.7749),
		Longitude: geoipFloat(-122.4194),
	})

	node := &model.Node{
		NodeID:       "node-geo-auto",
		Name:         "geo-auto",
		IP:           "10.0.0.8",
		AgentToken:   "agent-token",
		AgentVersion: "v0.4.0",
		NginxVersion: "1.27.1.2",
		Status:       NodeStatusOnline,
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}

	resp, err := HeartbeatNode(node, AgentNodePayload{
		NodeID:       node.NodeID,
		Name:         node.Name,
		IP:           "8.8.8.8",
		AgentVersion: node.AgentVersion,
		NginxVersion: node.NginxVersion,
	})
	if err != nil {
		t.Fatalf("expected heartbeat to succeed: %v", err)
	}
	if resp.Node.GeoName != "United States" {
		t.Fatalf("expected auto geo name, got %+v", resp.Node)
	}
	if resp.Node.GeoLatitude == nil || resp.Node.GeoLongitude == nil {
		t.Fatalf("expected auto geo coordinates, got %+v", resp.Node)
	}
}

func TestHeartbeatNodePreservesManualGeoOverride(t *testing.T) {
	setupServiceTestDB(t)
	withFakeGeoIPProvider(t, &geoip.GeoInfo{
		ISOCode:   "US",
		Name:      "United States",
		Latitude:  geoipFloat(37.7749),
		Longitude: geoipFloat(-122.4194),
	})

	latitude := 31.2304
	longitude := 121.4737
	node := &model.Node{
		NodeID:            "node-geo-manual",
		Name:              "geo-manual",
		IP:                "10.0.0.8",
		GeoName:           "Shanghai",
		GeoLatitude:       &latitude,
		GeoLongitude:      &longitude,
		GeoManualOverride: true,
		AgentToken:        "agent-token",
		AgentVersion:      "v0.4.0",
		NginxVersion:      "1.27.1.2",
		Status:            NodeStatusOnline,
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}

	resp, err := HeartbeatNode(node, AgentNodePayload{
		NodeID:       node.NodeID,
		Name:         node.Name,
		IP:           "8.8.8.8",
		AgentVersion: node.AgentVersion,
		NginxVersion: node.NginxVersion,
	})
	if err != nil {
		t.Fatalf("expected heartbeat to succeed: %v", err)
	}
	if resp.Node.GeoName != "Shanghai" {
		t.Fatalf("expected manual geo name to be preserved, got %+v", resp.Node)
	}
	if resp.Node.GeoLatitude == nil || *resp.Node.GeoLatitude != latitude {
		t.Fatalf("expected manual latitude to be preserved, got %+v", resp.Node.GeoLatitude)
	}
}

func TestRequestNodeOpenrestyRestart(t *testing.T) {
	setupServiceTestDB(t)

	node, err := CreateNode(NodeInput{Name: "restart-edge-1"})
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	updated, err := RequestNodeOpenrestyRestart(node.ID)
	if err != nil {
		t.Fatalf("expected openresty restart request to succeed: %v", err)
	}
	if !updated.RestartOpenrestyRequested {
		t.Fatal("expected restart_openresty_requested to be true")
	}
}

func TestListNodeViewsIncludesLatestApplyLogsForMultipleNodes(t *testing.T) {
	setupServiceTestDB(t)

	now := time.Now()
	nodes := []*model.Node{
		{
			NodeID:       "node-a",
			Name:         "edge-a",
			IP:           "10.0.0.11",
			GeoName:      "Shanghai",
			AgentToken:   "token-a",
			AgentVersion: "v0.5.0",
			NginxVersion: "1.27.1.2",
			Status:       NodeStatusOnline,
			LastSeenAt:   now,
		},
		{
			NodeID:       "node-b",
			Name:         "edge-b",
			IP:           "10.0.0.12",
			AgentToken:   "token-b",
			AgentVersion: "v0.5.0",
			NginxVersion: "1.27.1.2",
			Status:       NodeStatusOnline,
			LastSeenAt:   now,
		},
	}
	for _, node := range nodes {
		if err := node.Insert(); err != nil {
			t.Fatalf("failed to insert node %s: %v", node.NodeID, err)
		}
	}

	logs := []*model.ApplyLog{
		{NodeID: "node-a", Version: "20260313-001", Result: ApplyResultOK, Message: "first success", CreatedAt: now.Add(-2 * time.Minute)},
		{NodeID: "node-a", Version: "20260313-002", Result: ApplyResultFailed, Message: "latest failure", CreatedAt: now.Add(-1 * time.Minute)},
		{NodeID: "node-b", Version: "20260313-003", Result: ApplyResultOK, Message: "latest success", CreatedAt: now},
	}
	for _, log := range logs {
		if err := model.DB.Create(log).Error; err != nil {
			t.Fatalf("failed to insert apply log for %s: %v", log.NodeID, err)
		}
	}

	views, err := ListNodeViews()
	if err != nil {
		t.Fatalf("ListNodeViews failed: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("expected 2 node views, got %d", len(views))
	}

	sort.Slice(views, func(i int, j int) bool {
		return views[i].NodeID < views[j].NodeID
	})

	if views[0].NodeID != "node-a" || views[0].LatestApplyResult != ApplyResultFailed || views[0].LatestApplyMessage != "latest failure" {
		t.Fatalf("unexpected latest apply log for node-a: %+v", views[0])
	}
	if views[0].GeoName != "Shanghai" {
		t.Fatalf("expected geo name to be exposed on node view, got %+v", views[0])
	}
	if views[1].NodeID != "node-b" || views[1].LatestApplyResult != ApplyResultOK || views[1].LatestApplyMessage != "latest success" {
		t.Fatalf("unexpected latest apply log for node-b: %+v", views[1])
	}
}

func TestCollectNodeHeartbeatChangesOnlyReturnsChangedFields(t *testing.T) {
	now := time.Now()
	before := &model.Node{
		Name:                      "edge-1",
		IP:                        "10.0.0.8",
		AgentVersion:              "v0.5.0",
		NginxVersion:              "1.27.1.2",
		OpenrestyStatus:           OpenrestyStatusHealthy,
		OpenrestyMessage:          "",
		Status:                    NodeStatusOnline,
		CurrentVersion:            "20260313-001",
		LastSeenAt:                now.Add(-time.Minute),
		LastError:                 "",
		UpdateRequested:           true,
		UpdateChannel:             "preview",
		UpdateTag:                 "v0.5.0-rc.1",
		RestartOpenrestyRequested: true,
	}
	after := &model.Node{
		Name:                      "edge-1",
		IP:                        "10.0.0.8",
		AgentVersion:              "v0.5.0",
		NginxVersion:              "1.27.1.2",
		OpenrestyStatus:           OpenrestyStatusHealthy,
		OpenrestyMessage:          "",
		Status:                    NodeStatusOnline,
		CurrentVersion:            "20260313-001",
		LastSeenAt:                now,
		LastError:                 "",
		UpdateRequested:           false,
		UpdateChannel:             "stable",
		UpdateTag:                 "",
		RestartOpenrestyRequested: false,
	}

	changes := collectNodeHeartbeatChanges(before, after)
	if len(changes) != 5 {
		t.Fatalf("expected 5 changed fields, got %d: %#v", len(changes), changes)
	}
	if _, ok := changes["last_seen_at"]; !ok {
		t.Fatal("expected last_seen_at change to be included")
	}
	if value, ok := changes["update_requested"]; !ok || value != false {
		t.Fatalf("expected update_requested reset, got %#v", value)
	}
	if value, ok := changes["update_channel"]; !ok || value != "stable" {
		t.Fatalf("expected update_channel reset, got %#v", value)
	}
	if value, ok := changes["update_tag"]; !ok || value != "" {
		t.Fatalf("expected update_tag reset, got %#v", value)
	}
	if value, ok := changes["restart_openresty_requested"]; !ok || value != false {
		t.Fatalf("expected restart_openresty_requested reset, got %#v", value)
	}
	if _, ok := changes["ip"]; ok {
		t.Fatal("did not expect unchanged ip to be included")
	}
}

func TestListNodeViewsDoesNotPersistComputedStatus(t *testing.T) {
	setupServiceTestDB(t)

	node := &model.Node{
		NodeID:       "node-offline-view",
		Name:         "edge-offline",
		IP:           "10.0.0.21",
		AgentToken:   "token-offline",
		AgentVersion: "v0.5.0",
		NginxVersion: "1.27.1.2",
		Status:       NodeStatusOnline,
		LastSeenAt:   time.Now().Add(-common.NodeOfflineThreshold - time.Minute),
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to insert node: %v", err)
	}

	views, err := ListNodeViews()
	if err != nil {
		t.Fatalf("ListNodeViews failed: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected 1 node view, got %d", len(views))
	}
	if views[0].Status != NodeStatusOffline {
		t.Fatalf("expected computed offline status in view, got %s", views[0].Status)
	}

	storedNode, err := model.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("failed to reload node: %v", err)
	}
	if storedNode.Status != NodeStatusOnline {
		t.Fatalf("expected list query to avoid persisting computed status, got %s", storedNode.Status)
	}
}

func TestHeartbeatNodePersistsObservabilityPayload(t *testing.T) {
	setupServiceTestDB(t)
	withFakeAccessLogGeoProvider(t, &geoip.GeoInfo{
		ISOCode: "US",
		Name:    "United States",
	})

	node := &model.Node{
		NodeID:       "node-observe-1",
		Name:         "observe-edge-1",
		IP:           "10.0.0.31",
		AgentToken:   "token-observe",
		AgentVersion: "v0.6.0",
		NginxVersion: "1.27.1.2",
		Status:       NodeStatusOnline,
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}

	_, err := HeartbeatNode(node, AgentNodePayload{
		NodeID:       node.NodeID,
		Name:         node.Name,
		IP:           node.IP,
		AgentVersion: node.AgentVersion,
		NginxVersion: node.NginxVersion,
		Profile: &AgentNodeSystemProfile{
			Hostname:         "observe-edge-1",
			OSName:           "Ubuntu",
			OSVersion:        "24.04",
			KernelVersion:    "6.8.0",
			Architecture:     "amd64",
			CPUModel:         "Intel Xeon",
			CPUCores:         8,
			TotalMemoryBytes: 16 * 1024 * 1024 * 1024,
			TotalDiskBytes:   200 * 1024 * 1024 * 1024,
			UptimeSeconds:    3600,
			ReportedAtUnix:   time.Now().Add(-time.Minute).Unix(),
		},
		Snapshot: &AgentNodeMetricSnapshot{
			CapturedAtUnix:       time.Now().Add(-30 * time.Second).Unix(),
			CPUUsagePercent:      42.5,
			MemoryUsedBytes:      8 * 1024 * 1024 * 1024,
			MemoryTotalBytes:     16 * 1024 * 1024 * 1024,
			StorageUsedBytes:     70 * 1024 * 1024 * 1024,
			StorageTotalBytes:    200 * 1024 * 1024 * 1024,
			DiskReadBytes:        1024,
			DiskWriteBytes:       2048,
			NetworkRxBytes:       4096,
			NetworkTxBytes:       8192,
			OpenrestyConnections: 128,
		},
		TrafficReport: &AgentNodeTrafficReport{
			WindowStartedAtUnix: time.Now().Add(-time.Minute).Unix(),
			WindowEndedAtUnix:   time.Now().Unix(),
			RequestCount:        1200,
			ErrorCount:          12,
			UniqueVisitorCount:  320,
			StatusCodes:         map[string]int64{"200": 1100, "502": 12},
			TopDomains:          map[string]int64{"example.com": 900},
			SourceCountries:     map[string]int64{"CN": 700, "US": 200},
		},
		AccessLogs: []AgentNodeAccessLog{
			{
				LoggedAtUnix: time.Now().Add(-45 * time.Second).Unix(),
				RemoteAddr:   "203.0.113.10",
				Host:         "example.com",
				Path:         "/login",
				StatusCode:   200,
			},
			{
				LoggedAtUnix: time.Now().Add(-40 * time.Second).Unix(),
				RemoteAddr:   "198.51.100.20",
				Host:         "api.example.com",
				Path:         "/v1/ping",
				StatusCode:   502,
			},
		},
		HealthEvents: []AgentNodeHealthEvent{
			{
				EventType:       "openresty_unhealthy",
				Severity:        NodeHealthSeverityCritical,
				Message:         "reload failed",
				TriggeredAtUnix: time.Now().Add(-2 * time.Minute).Unix(),
			},
		},
	})
	if err != nil {
		t.Fatalf("expected heartbeat to succeed: %v", err)
	}

	profile, err := model.GetNodeSystemProfile(node.NodeID)
	if err != nil {
		t.Fatalf("expected node profile to persist: %v", err)
	}
	if profile.OSName != "Ubuntu" || profile.CPUCores != 8 {
		t.Fatalf("unexpected system profile: %+v", profile)
	}

	snapshots, err := model.ListNodeMetricSnapshots(node.NodeID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("expected node snapshots query to succeed: %v", err)
	}
	if len(snapshots) != 1 || snapshots[0].OpenrestyConnections != 128 {
		t.Fatalf("unexpected metric snapshots: %+v", snapshots)
	}

	reports, err := model.ListNodeRequestReports(node.NodeID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("expected node request reports query to succeed: %v", err)
	}
	if len(reports) != 1 || reports[0].RequestCount != 1200 {
		t.Fatalf("unexpected request reports: %+v", reports)
	}

	accessLogs, err := model.ListNodeAccessLogs(model.NodeAccessLogQuery{
		NodeID:   node.NodeID,
		Since:    time.Time{},
		Page:     0,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("expected node access logs query to succeed: %v", err)
	}
	if len(accessLogs) != 2 || accessLogs[0].Path == "" {
		t.Fatalf("unexpected access logs: %+v", accessLogs)
	}
	if accessLogs[0].Region == "" || accessLogs[1].Region == "" {
		t.Fatalf("expected access log region to persist: %+v", accessLogs)
	}

	events, err := model.ListNodeHealthEvents(node.NodeID, true, 10)
	if err != nil {
		t.Fatalf("expected node health events query to succeed: %v", err)
	}
	if len(events) != 1 || events[0].EventType != "openresty_unhealthy" {
		t.Fatalf("unexpected active health events: %+v", events)
	}
}

func TestHeartbeatNodePersistsBufferedObservabilityPayload(t *testing.T) {
	setupServiceTestDB(t)
	withFakeAccessLogGeoProvider(t, &geoip.GeoInfo{
		ISOCode: "CN",
		Name:    "China",
	})

	node := &model.Node{
		NodeID:       "node-observe-buffered",
		Name:         "observe-buffered-edge",
		IP:           "10.0.0.32",
		AgentToken:   "token-observe-buffered",
		AgentVersion: "v0.6.0",
		NginxVersion: "1.27.1.2",
		Status:       NodeStatusOnline,
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}

	now := time.Now().UTC()
	_, err := HeartbeatNode(node, AgentNodePayload{
		NodeID:       node.NodeID,
		Name:         node.Name,
		IP:           node.IP,
		AgentVersion: node.AgentVersion,
		NginxVersion: node.NginxVersion,
		Snapshot: &AgentNodeMetricSnapshot{
			CapturedAtUnix:   now.Unix(),
			CPUUsagePercent:  25,
			MemoryUsedBytes:  2 * 1024 * 1024 * 1024,
			MemoryTotalBytes: 8 * 1024 * 1024 * 1024,
		},
		TrafficReport: &AgentNodeTrafficReport{
			WindowStartedAtUnix: now.Add(-time.Minute).Unix(),
			WindowEndedAtUnix:   now.Unix(),
			RequestCount:        20,
			ErrorCount:          1,
			UniqueVisitorCount:  10,
			StatusCodes:         map[string]int64{"200": 19, "500": 1},
			TopDomains:          map[string]int64{"edge.example.com": 20},
			SourceCountries:     map[string]int64{"CN": 12},
		},
		BufferedObservability: []AgentBufferedObservabilityRecord{
			{
				WindowStartedAtUnix: now.Add(-2 * time.Minute).Unix(),
				Snapshot: &AgentNodeMetricSnapshot{
					CapturedAtUnix:   now.Add(-2 * time.Minute).Unix(),
					CPUUsagePercent:  30,
					MemoryUsedBytes:  3 * 1024 * 1024 * 1024,
					MemoryTotalBytes: 8 * 1024 * 1024 * 1024,
				},
				TrafficReport: &AgentNodeTrafficReport{
					WindowStartedAtUnix: now.Add(-2 * time.Minute).Unix(),
					WindowEndedAtUnix:   now.Add(-time.Minute).Unix(),
					RequestCount:        40,
					ErrorCount:          2,
					UniqueVisitorCount:  18,
					StatusCodes:         map[string]int64{"200": 38, "500": 2},
					TopDomains:          map[string]int64{"edge.example.com": 40},
					SourceCountries:     map[string]int64{"CN": 20},
				},
				AccessLogs: []AgentNodeAccessLog{
					{
						LoggedAtUnix: now.Add(-110 * time.Second).Unix(),
						RemoteAddr:   "203.0.113.21",
						Host:         "edge.example.com",
						Path:         "/buffered",
						StatusCode:   200,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected heartbeat to succeed: %v", err)
	}

	snapshots, err := model.ListNodeMetricSnapshots(node.NodeID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("expected node snapshots query to succeed: %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected current and buffered snapshots, got %+v", snapshots)
	}

	reports, err := model.ListNodeRequestReports(node.NodeID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("expected node request reports query to succeed: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("expected current and buffered reports, got %+v", reports)
	}

	accessLogs, err := model.ListNodeAccessLogs(model.NodeAccessLogQuery{
		NodeID:   node.NodeID,
		Since:    time.Time{},
		Page:     0,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("expected node access logs query to succeed: %v", err)
	}
	if len(accessLogs) != 1 || accessLogs[0].Path != "/buffered" {
		t.Fatalf("expected buffered access logs to persist, got %+v", accessLogs)
	}
	if accessLogs[0].Region != "China" {
		t.Fatalf("expected buffered access log region to persist, got %+v", accessLogs[0])
	}

	_, err = HeartbeatNode(node, AgentNodePayload{
		NodeID:       node.NodeID,
		Name:         node.Name,
		IP:           node.IP,
		AgentVersion: node.AgentVersion,
		NginxVersion: node.NginxVersion,
		BufferedObservability: []AgentBufferedObservabilityRecord{
			{
				WindowStartedAtUnix: now.Add(-2 * time.Minute).Unix(),
				Snapshot: &AgentNodeMetricSnapshot{
					CapturedAtUnix:   now.Add(-2 * time.Minute).Unix(),
					CPUUsagePercent:  30,
					MemoryUsedBytes:  3 * 1024 * 1024 * 1024,
					MemoryTotalBytes: 8 * 1024 * 1024 * 1024,
				},
				TrafficReport: &AgentNodeTrafficReport{
					WindowStartedAtUnix: now.Add(-2 * time.Minute).Unix(),
					WindowEndedAtUnix:   now.Add(-time.Minute).Unix(),
					RequestCount:        40,
					ErrorCount:          2,
					UniqueVisitorCount:  18,
					StatusCodes:         map[string]int64{"200": 38, "500": 2},
					TopDomains:          map[string]int64{"edge.example.com": 40},
					SourceCountries:     map[string]int64{"CN": 20},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected heartbeat replay dedupe to succeed: %v", err)
	}

	snapshots, err = model.ListNodeMetricSnapshots(node.NodeID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("expected node snapshots query to succeed after replay: %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected replay dedupe to keep snapshot count stable, got %+v", snapshots)
	}
	reports, err = model.ListNodeRequestReports(node.NodeID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("expected node request reports query to succeed after replay: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("expected replay dedupe to keep report count stable, got %+v", reports)
	}
}

func TestListAccessLogsUsesPagination(t *testing.T) {
	setupServiceTestDB(t)

	node := &model.Node{
		NodeID:       "node-access-log-page",
		Name:         "access-log-edge",
		IP:           "10.0.0.40",
		AgentToken:   "token-access-log-page",
		AgentVersion: "v0.6.0",
		NginxVersion: "1.27.1.2",
		Status:       NodeStatusOnline,
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}

	now := time.Now().UTC()
	if err := model.DB.Create([]*model.NodeAccessLog{
		{
			NodeID:     node.NodeID,
			LoggedAt:   now.Add(-10 * time.Second),
			RemoteAddr: "203.0.113.1",
			Region:     "United States",
			Host:       "example.com",
			Path:       "/one",
			StatusCode: 200,
		},
		{
			NodeID:     node.NodeID,
			LoggedAt:   now.Add(-9 * time.Second),
			RemoteAddr: "203.0.113.2",
			Region:     "China",
			Host:       "example.com",
			Path:       "/two",
			StatusCode: 200,
		},
		{
			NodeID:     node.NodeID,
			LoggedAt:   now.Add(-8 * time.Second),
			RemoteAddr: "203.0.113.3",
			Region:     "Japan",
			Host:       "example.com",
			Path:       "/three",
			StatusCode: 502,
		},
	}).Error; err != nil {
		t.Fatalf("failed to seed access logs: %v", err)
	}

	pageOne, err := ListAccessLogs(AccessLogQuery{
		NodeID:   node.NodeID,
		Page:     0,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("ListAccessLogs page 1 failed: %v", err)
	}
	if len(pageOne.Items) != 2 || !pageOne.HasMore {
		t.Fatalf("unexpected first page: %+v", pageOne)
	}
	if pageOne.Items[0].Path != "/three" || pageOne.Items[1].Path != "/two" {
		t.Fatalf("unexpected first page ordering: %+v", pageOne.Items)
	}
	if pageOne.Items[0].Region != "Japan" {
		t.Fatalf("expected paged access log region to be returned, got %+v", pageOne.Items[0])
	}

	pageTwo, err := ListAccessLogs(AccessLogQuery{
		NodeID:   node.NodeID,
		Page:     1,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("ListAccessLogs page 2 failed: %v", err)
	}
	if len(pageTwo.Items) != 1 || pageTwo.HasMore {
		t.Fatalf("unexpected second page: %+v", pageTwo)
	}
	if pageTwo.Items[0].Path != "/one" {
		t.Fatalf("unexpected second page ordering: %+v", pageTwo.Items)
	}
}

func TestHeartbeatNodeResolvesMissingHealthEvents(t *testing.T) {
	setupServiceTestDB(t)

	node := &model.Node{
		NodeID:       "node-event-1",
		Name:         "event-edge-1",
		IP:           "10.0.0.41",
		AgentToken:   "token-event",
		AgentVersion: "v0.6.0",
		NginxVersion: "1.27.1.2",
		Status:       NodeStatusOnline,
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}

	_, err := HeartbeatNode(node, AgentNodePayload{
		NodeID:       node.NodeID,
		Name:         node.Name,
		IP:           node.IP,
		AgentVersion: node.AgentVersion,
		NginxVersion: node.NginxVersion,
		HealthEvents: []AgentNodeHealthEvent{
			{
				EventType:       "sync_error",
				Severity:        NodeHealthSeverityWarning,
				Message:         "checksum mismatch",
				TriggeredAtUnix: time.Now().Add(-time.Minute).Unix(),
			},
		},
	})
	if err != nil {
		t.Fatalf("expected first heartbeat to succeed: %v", err)
	}

	_, err = HeartbeatNode(node, AgentNodePayload{
		NodeID:       node.NodeID,
		Name:         node.Name,
		IP:           node.IP,
		AgentVersion: node.AgentVersion,
		NginxVersion: node.NginxVersion,
		HealthEvents: []AgentNodeHealthEvent{},
	})
	if err != nil {
		t.Fatalf("expected second heartbeat to succeed: %v", err)
	}

	activeEvents, err := model.ListNodeHealthEvents(node.NodeID, true, 10)
	if err != nil {
		t.Fatalf("expected active node health events query to succeed: %v", err)
	}
	if len(activeEvents) != 0 {
		t.Fatalf("expected no active health events, got %+v", activeEvents)
	}

	allEvents, err := model.ListNodeHealthEvents(node.NodeID, false, 10)
	if err != nil {
		t.Fatalf("expected all node health events query to succeed: %v", err)
	}
	if len(allEvents) != 1 || allEvents[0].Status != NodeHealthEventStatusResolved || allEvents[0].ResolvedAt == nil {
		t.Fatalf("expected resolved health event record, got %+v", allEvents)
	}
}

func TestGetNodeObservability(t *testing.T) {
	setupServiceTestDB(t)

	node := &model.Node{
		NodeID:       "node-observability-query",
		Name:         "query-edge",
		IP:           "10.0.0.61",
		AgentToken:   "token-query",
		AgentVersion: "v0.6.0",
		NginxVersion: "1.27.1.2",
		Status:       NodeStatusOnline,
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to insert node: %v", err)
	}
	if err := model.UpsertNodeSystemProfile(&model.NodeSystemProfile{
		NodeID:       node.NodeID,
		Hostname:     "query-edge",
		OSName:       "Ubuntu",
		Architecture: "amd64",
		ReportedAt:   time.Now(),
	}); err != nil {
		t.Fatalf("failed to insert node system profile: %v", err)
	}
	if err := (&model.NodeMetricSnapshot{
		NodeID:            node.NodeID,
		CapturedAt:        time.Now().Add(-time.Hour),
		CPUUsagePercent:   60,
		MemoryUsedBytes:   14 * 1024 * 1024 * 1024,
		MemoryTotalBytes:  16 * 1024 * 1024 * 1024,
		StorageUsedBytes:  90 * 1024 * 1024 * 1024,
		StorageTotalBytes: 100 * 1024 * 1024 * 1024,
		DiskReadBytes:     0,
		DiskWriteBytes:    0,
		NetworkRxBytes:    2048,
		NetworkTxBytes:    4096,
		OpenrestyRxBytes:  8192,
		OpenrestyTxBytes:  16384,
	}).Insert(); err != nil {
		t.Fatalf("failed to insert node metric baseline snapshot: %v", err)
	}
	if err := (&model.NodeMetricSnapshot{
		NodeID:            node.NodeID,
		CapturedAt:        time.Now(),
		CPUUsagePercent:   81,
		MemoryUsedBytes:   15 * 1024 * 1024 * 1024,
		MemoryTotalBytes:  16 * 1024 * 1024 * 1024,
		StorageUsedBytes:  92 * 1024 * 1024 * 1024,
		StorageTotalBytes: 100 * 1024 * 1024 * 1024,
		DiskReadBytes:     1024,
		DiskWriteBytes:    2048,
		NetworkRxBytes:    4096,
		NetworkTxBytes:    8192,
		OpenrestyRxBytes:  16384,
		OpenrestyTxBytes:  32768,
	}).Insert(); err != nil {
		t.Fatalf("failed to insert node metric snapshot: %v", err)
	}
	if err := (&model.NodeRequestReport{
		NodeID:              node.NodeID,
		WindowStartedAt:     time.Now().Add(-time.Minute),
		WindowEndedAt:       time.Now(),
		RequestCount:        123,
		ErrorCount:          9,
		UniqueVisitorCount:  87,
		StatusCodesJSON:     `{"200":114,"502":9}`,
		TopDomainsJSON:      `{"example.com":80,"api.example.com":43}`,
		SourceCountriesJSON: `{"CN":90,"US":33}`,
	}).Insert(); err != nil {
		t.Fatalf("failed to insert node request report: %v", err)
	}
	if err := model.DB.Create(&model.NodeHealthEvent{
		NodeID:           node.NodeID,
		EventType:        "sync_error",
		Severity:         NodeHealthSeverityWarning,
		Status:           NodeHealthEventStatusActive,
		Message:          "checksum mismatch",
		FirstTriggeredAt: time.Now().Add(-time.Minute),
		LastTriggeredAt:  time.Now(),
		ReportedAt:       time.Now(),
	}).Error; err != nil {
		t.Fatalf("failed to insert node health event: %v", err)
	}

	view, err := GetNodeObservability(node.ID, NodeObservabilityQuery{Hours: 24, Limit: 10})
	if err != nil {
		t.Fatalf("GetNodeObservability failed: %v", err)
	}
	if view.NodeID != node.NodeID {
		t.Fatalf("unexpected node id: %s", view.NodeID)
	}
	if view.Profile == nil || view.Profile.OSName != "Ubuntu" {
		t.Fatalf("unexpected profile: %+v", view.Profile)
	}
	if len(view.MetricSnapshots) != 2 {
		t.Fatalf("expected 2 metric snapshots, got %d", len(view.MetricSnapshots))
	}
	if view.MetricSnapshots[0].DiskWriteBytes != 2048 {
		t.Fatalf("expected latest metric snapshot to stay intact, got %+v", view.MetricSnapshots[0])
	}
	if len(view.TrafficReports) != 1 || view.TrafficReports[0].RequestCount != 123 {
		t.Fatalf("unexpected traffic reports: %+v", view.TrafficReports)
	}
	if len(view.HealthEvents) != 1 || view.HealthEvents[0].EventType != "sync_error" {
		t.Fatalf("unexpected health events: %+v", view.HealthEvents)
	}
	if len(view.Trends.Traffic24h) != 24 || len(view.Trends.Capacity24h) != 24 || len(view.Trends.Network24h) != 24 || len(view.Trends.DiskIO24h) != 24 {
		t.Fatalf("expected 24-point trends, got %+v", view.Trends)
	}
	if view.Trends.Traffic24h[len(view.Trends.Traffic24h)-1].RequestCount != 123 {
		t.Fatalf("unexpected traffic trend tail: %+v", view.Trends.Traffic24h[len(view.Trends.Traffic24h)-1])
	}
	if view.Trends.Network24h[len(view.Trends.Network24h)-1].OpenrestyTxBytes != 32768 {
		t.Fatalf("unexpected network trend tail: %+v", view.Trends.Network24h[len(view.Trends.Network24h)-1])
	}
	if view.Trends.DiskIO24h[len(view.Trends.DiskIO24h)-1].DiskWriteBytes != 2048 {
		t.Fatalf("unexpected disk io trend tail: %+v", view.Trends.DiskIO24h[len(view.Trends.DiskIO24h)-1])
	}
	if view.Analytics.Traffic.RequestCount != 123 || view.Analytics.Traffic.ErrorRatePercent <= 7 {
		t.Fatalf("unexpected traffic analytics: %+v", view.Analytics.Traffic)
	}
	if len(view.Analytics.Distributions.StatusCodes) != 2 || view.Analytics.Distributions.StatusCodes[0].Key != "200" {
		t.Fatalf("unexpected traffic distributions: %+v", view.Analytics.Distributions)
	}
	if len(view.Analytics.Distributions.SourceCountries) != 2 || view.Analytics.Distributions.SourceCountries[0].Key != "CN" {
		t.Fatalf("unexpected source countries: %+v", view.Analytics.Distributions.SourceCountries)
	}
	if !view.Analytics.Health.HasCapacityRisk || !view.Analytics.Health.HasTrafficRisk || !view.Analytics.Health.HasRuntimeRisk {
		t.Fatalf("unexpected health analytics: %+v", view.Analytics.Health)
	}
}

func TestGetNodeObservabilityAllowsMissingProfile(t *testing.T) {
	setupServiceTestDB(t)

	node := &model.Node{
		NodeID:       "node-observability-empty",
		Name:         "empty-edge",
		IP:           "10.0.0.62",
		AgentToken:   "token-empty",
		AgentVersion: "v0.6.0",
		NginxVersion: "1.27.1.2",
		Status:       NodeStatusOnline,
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to insert node: %v", err)
	}

	view, err := GetNodeObservability(node.ID, NodeObservabilityQuery{})
	if err != nil {
		t.Fatalf("GetNodeObservability failed: %v", err)
	}
	if view.Profile != nil {
		t.Fatalf("expected nil profile when profile not reported, got %+v", view.Profile)
	}
	if len(view.Trends.Traffic24h) != 24 || len(view.Trends.Capacity24h) != 24 || len(view.Trends.Network24h) != 24 || len(view.Trends.DiskIO24h) != 24 {
		t.Fatalf("expected empty 24-point trends, got %+v", view.Trends)
	}
	if view.Analytics.Traffic.RequestCount != 0 || len(view.Analytics.Distributions.StatusCodes) != 0 {
		t.Fatalf("expected empty analytics, got %+v", view.Analytics)
	}
}

func TestGetDashboardOverview(t *testing.T) {
	setupServiceTestDB(t)

	now := time.Now()
	if err := model.DB.Create(&model.ConfigVersion{
		Version:        "20260314-001",
		SnapshotJSON:   "{}",
		MainConfig:     "worker_processes auto;",
		RenderedConfig: "server { listen 80; }",
		Checksum:       "checksum-dashboard",
		IsActive:       true,
		CreatedBy:      "root",
	}).Error; err != nil {
		t.Fatalf("failed to seed active config version: %v", err)
	}

	nodes := []*model.Node{
		{
			NodeID:          "node-dashboard-a",
			Name:            "edge-a",
			IP:              "10.0.0.71",
			GeoName:         "Shanghai",
			AgentToken:      "token-a",
			AgentVersion:    "v0.6.0",
			NginxVersion:    "1.27.1.2",
			OpenrestyStatus: OpenrestyStatusHealthy,
			Status:          NodeStatusOnline,
			CurrentVersion:  "20260314-001",
			LastSeenAt:      now,
		},
		{
			NodeID:          "node-dashboard-b",
			Name:            "edge-b",
			IP:              "10.0.0.72",
			GeoName:         "San Francisco",
			AgentToken:      "token-b",
			AgentVersion:    "v0.6.0",
			NginxVersion:    "1.27.1.2",
			OpenrestyStatus: OpenrestyStatusUnhealthy,
			Status:          NodeStatusOnline,
			CurrentVersion:  "20260313-001",
			LastSeenAt:      now,
		},
	}
	for _, node := range nodes {
		if err := node.Insert(); err != nil {
			t.Fatalf("failed to insert node %s: %v", node.NodeID, err)
		}
	}

	if err := (&model.NodeMetricSnapshot{
		NodeID:            "node-dashboard-a",
		CapturedAt:        now.Add(-time.Hour),
		CPUUsagePercent:   40,
		MemoryUsedBytes:   4 * 1024 * 1024 * 1024,
		MemoryTotalBytes:  8 * 1024 * 1024 * 1024,
		StorageUsedBytes:  48 * 1024 * 1024 * 1024,
		StorageTotalBytes: 100 * 1024 * 1024 * 1024,
		DiskReadBytes:     0,
		DiskWriteBytes:    0,
		NetworkRxBytes:    100,
		NetworkTxBytes:    150,
		OpenrestyRxBytes:  300,
		OpenrestyTxBytes:  450,
	}).Insert(); err != nil {
		t.Fatalf("failed to insert node a baseline metric snapshot: %v", err)
	}
	if err := (&model.NodeMetricSnapshot{
		NodeID:            "node-dashboard-a",
		CapturedAt:        now,
		CPUUsagePercent:   45,
		MemoryUsedBytes:   4 * 1024 * 1024 * 1024,
		MemoryTotalBytes:  8 * 1024 * 1024 * 1024,
		StorageUsedBytes:  50 * 1024 * 1024 * 1024,
		StorageTotalBytes: 100 * 1024 * 1024 * 1024,
		DiskReadBytes:     100,
		DiskWriteBytes:    150,
		NetworkRxBytes:    300,
		NetworkTxBytes:    500,
		OpenrestyRxBytes:  700,
		OpenrestyTxBytes:  900,
	}).Insert(); err != nil {
		t.Fatalf("failed to insert node a metric snapshot: %v", err)
	}
	if err := (&model.NodeMetricSnapshot{
		NodeID:            "node-dashboard-b",
		CapturedAt:        now.Add(-time.Hour),
		CPUUsagePercent:   88,
		MemoryUsedBytes:   14 * 1024 * 1024 * 1024,
		MemoryTotalBytes:  16 * 1024 * 1024 * 1024,
		StorageUsedBytes:  93 * 1024 * 1024 * 1024,
		StorageTotalBytes: 100 * 1024 * 1024 * 1024,
		DiskReadBytes:     0,
		DiskWriteBytes:    0,
		NetworkRxBytes:    200,
		NetworkTxBytes:    300,
		OpenrestyRxBytes:  500,
		OpenrestyTxBytes:  700,
	}).Insert(); err != nil {
		t.Fatalf("failed to insert node b baseline metric snapshot: %v", err)
	}
	if err := (&model.NodeMetricSnapshot{
		NodeID:            "node-dashboard-b",
		CapturedAt:        now,
		CPUUsagePercent:   92,
		MemoryUsedBytes:   15 * 1024 * 1024 * 1024,
		MemoryTotalBytes:  16 * 1024 * 1024 * 1024,
		StorageUsedBytes:  95 * 1024 * 1024 * 1024,
		StorageTotalBytes: 100 * 1024 * 1024 * 1024,
		DiskReadBytes:     200,
		DiskWriteBytes:    400,
		NetworkRxBytes:    600,
		NetworkTxBytes:    900,
		OpenrestyRxBytes:  1200,
		OpenrestyTxBytes:  1600,
	}).Insert(); err != nil {
		t.Fatalf("failed to insert node b metric snapshot: %v", err)
	}

	if err := (&model.NodeRequestReport{
		NodeID:             "node-dashboard-a",
		WindowStartedAt:    now.Add(-time.Minute),
		WindowEndedAt:      now,
		RequestCount:       600,
		ErrorCount:         6,
		UniqueVisitorCount: 120,
		StatusCodesJSON:    `{"200":570,"502":6,"304":24}`,
		TopDomainsJSON:     `{"app.example.com":420,"api.example.com":180}`,
	}).Insert(); err != nil {
		t.Fatalf("failed to insert node a traffic report: %v", err)
	}
	if err := (&model.NodeRequestReport{
		NodeID:             "node-dashboard-b",
		WindowStartedAt:    now.Add(-time.Minute),
		WindowEndedAt:      now,
		RequestCount:       300,
		ErrorCount:         30,
		UniqueVisitorCount: 80,
		StatusCodesJSON:    `{"200":240,"500":18,"502":12,"404":30}`,
		TopDomainsJSON:     `{"app.example.com":140,"edge.example.com":160}`,
	}).Insert(); err != nil {
		t.Fatalf("failed to insert node b traffic report: %v", err)
	}
	if err := model.DB.Create([]*model.NodeAccessLog{
		{
			NodeID:     "node-dashboard-a",
			LoggedAt:   now.Add(-30 * time.Minute),
			RemoteAddr: "203.0.113.11",
			Region:     "China",
			Host:       "app.example.com",
			Path:       "/",
			StatusCode: 200,
		},
		{
			NodeID:     "node-dashboard-a",
			LoggedAt:   now.Add(-20 * time.Minute),
			RemoteAddr: "203.0.113.12",
			Region:     "China",
			Host:       "app.example.com",
			Path:       "/login",
			StatusCode: 200,
		},
		{
			NodeID:     "node-dashboard-b",
			LoggedAt:   now.Add(-10 * time.Minute),
			RemoteAddr: "198.51.100.8",
			Region:     "United States",
			Host:       "edge.example.com",
			Path:       "/edge",
			StatusCode: 502,
		},
	}).Error; err != nil {
		t.Fatalf("failed to seed dashboard access logs: %v", err)
	}

	if err := model.DB.Create(&model.NodeHealthEvent{
		NodeID:           "node-dashboard-b",
		EventType:        "openresty_unhealthy",
		Severity:         NodeHealthSeverityCritical,
		Status:           NodeHealthEventStatusActive,
		Message:          "reload failed",
		FirstTriggeredAt: now.Add(-time.Minute),
		LastTriggeredAt:  now,
		ReportedAt:       now,
	}).Error; err != nil {
		t.Fatalf("failed to insert dashboard health event: %v", err)
	}

	view, err := GetDashboardOverview()
	if err != nil {
		t.Fatalf("GetDashboardOverview failed: %v", err)
	}
	if view.Summary.TotalNodes != 2 || view.Summary.OnlineNodes != 2 {
		t.Fatalf("unexpected dashboard summary: %+v", view.Summary)
	}
	if view.Summary.UnhealthyNodes != 1 {
		t.Fatalf("unexpected unhealthy summary: %+v", view.Summary)
	}
	if view.Traffic.RequestCount != 900 || view.Traffic.ErrorCount != 36 {
		t.Fatalf("unexpected dashboard traffic: %+v", view.Traffic)
	}
	if view.Capacity.HighCPUNodes != 1 || view.Capacity.HighMemoryNodes != 1 {
		t.Fatalf("unexpected dashboard capacity summary: %+v", view.Capacity)
	}
	if len(view.Nodes) != 2 {
		t.Fatalf("unexpected dashboard nodes: %+v", view.Nodes)
	}
	if view.Nodes[0].GeoName == "" && view.Nodes[1].GeoName == "" {
		t.Fatalf("expected dashboard nodes to expose geo metadata: %+v", view.Nodes)
	}
	if view.Nodes[0].ActiveEventCount != 1 {
		t.Fatalf("expected dashboard nodes to preserve active event counts: %+v", view.Nodes)
	}
	if len(view.Trends.Traffic24h) != 24 || len(view.Trends.Capacity24h) != 24 || len(view.Trends.Network24h) != 24 || len(view.Trends.DiskIO24h) != 24 {
		t.Fatalf("expected 24-point dashboard trends, got %+v", view.Trends)
	}
	if view.Trends.Traffic24h[len(view.Trends.Traffic24h)-1].RequestCount != 900 {
		t.Fatalf("unexpected dashboard traffic trend tail: %+v", view.Trends.Traffic24h[len(view.Trends.Traffic24h)-1])
	}
	if view.Trends.Network24h[len(view.Trends.Network24h)-1].OpenrestyRxBytes != 1900 {
		t.Fatalf("unexpected dashboard network trend tail: %+v", view.Trends.Network24h[len(view.Trends.Network24h)-1])
	}
	if view.Trends.DiskIO24h[len(view.Trends.DiskIO24h)-1].DiskWriteBytes != 550 {
		t.Fatalf("unexpected dashboard disk io trend tail: %+v", view.Trends.DiskIO24h[len(view.Trends.DiskIO24h)-1])
	}
	if len(view.Distributions.StatusCodes) == 0 || view.Distributions.StatusCodes[0].Key != "200" {
		t.Fatalf("unexpected dashboard status distributions: %+v", view.Distributions.StatusCodes)
	}
	if len(view.Distributions.SourceCountries) == 0 || view.Distributions.SourceCountries[0].Key != "China" {
		t.Fatalf("unexpected dashboard source distributions: %+v", view.Distributions.SourceCountries)
	}
	if len(view.Distributions.TopDomains) == 0 || view.Distributions.TopDomains[0].Key != "app.example.com" {
		t.Fatalf("unexpected dashboard domain distributions: %+v", view.Distributions.TopDomains)
	}
}

func TestGetDashboardOverviewReturnsEmptyNodeSlice(t *testing.T) {
	setupServiceTestDB(t)

	view, err := GetDashboardOverview()
	if err != nil {
		t.Fatalf("GetDashboardOverview failed: %v", err)
	}
	if view.Nodes == nil {
		t.Fatalf("expected nodes to be an empty slice, got nil")
	}
	if len(view.Nodes) != 0 {
		t.Fatalf("expected empty nodes, got %+v", view.Nodes)
	}
}
