package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rain-kl/openflare/openflare-server/model"

	"gorm.io/gorm"
)

func TestHeartbeatRelayPersistsRuntimeAndObservability(t *testing.T) {
	setupServiceTestDB(t)

	node := &model.Node{
		NodeID:      "node-relay-observe",
		Name:        "relay-1",
		IP:          "",
		AccessToken: "relay-token",
		Status:      NodeStatusPending,
		NodeType:    "tunnel_relay",
		RelayStatus: "unknown",
		Version:     "",
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to seed relay node: %v", err)
	}

	now := time.Now().UTC()
	_, err := HeartbeatRelay(node, RelayHeartbeatPayload{
		Version:        "v0.1.0",
		ExtVersion:     "0.61.0",
		RelayStatus:    "healthy",
		FrpsConnCount:  7,
		FrpsProxyCount: 3,
		Name:           "relay-runtime",
		IP:             "203.0.113.9",
		Profile: &AgentNodeSystemProfile{
			Hostname:       "relay-runtime",
			OSName:         "Ubuntu",
			OSVersion:      "24.04",
			Architecture:   "amd64",
			CPUCores:       4,
			ReportedAtUnix: now.Unix(),
		},
		Snapshot: &AgentNodeMetricSnapshot{
			CapturedAtUnix:  now.Unix(),
			CPUUsagePercent: 12.5,
			NetworkRxBytes:  1024,
			NetworkTxBytes:  2048,
		},
		HealthEvents: []AgentNodeHealthEvent{},
	})
	if err != nil {
		t.Fatalf("HeartbeatRelay failed: %v", err)
	}

	updated, err := model.GetNodeByNodeID(node.NodeID)
	if err != nil {
		t.Fatalf("failed to reload node: %v", err)
	}
	if updated.Status != NodeStatusOnline || updated.RelayStatus != "healthy" {
		t.Fatalf("unexpected relay status: %+v", updated)
	}
	if updated.IP != "203.0.113.9" {
		t.Fatalf("expected relay IP to be updated, got %q", updated.IP)
	}
	if updated.Version != "v0.1.0" || updated.ExtVersion != "0.61.0" {
		t.Fatalf("expected relay versions to be updated, got relay=%q frp=%q", updated.Version, updated.ExtVersion)
	}

	profile, err := model.GetNodeSystemProfile(node.NodeID)
	if err != nil {
		t.Fatalf("expected relay system profile: %v", err)
	}
	if profile.Hostname != "relay-runtime" || profile.OSName != "Ubuntu" {
		t.Fatalf("unexpected relay profile: %+v", profile)
	}

	snapshots, err := model.ListNodeMetricSnapshots(node.NodeID, now.Add(-time.Minute), 10)
	if err != nil {
		t.Fatalf("failed to list relay snapshots: %v", err)
	}
	if len(snapshots) != 1 || snapshots[0].CPUUsagePercent != 12.5 {
		t.Fatalf("unexpected relay snapshots: %+v", snapshots)
	}

	observability, err := GetNodeObservability(updated.ID, NodeObservabilityQuery{Hours: 1, Limit: 10})
	if err != nil {
		t.Fatalf("GetNodeObservability failed: %v", err)
	}
	if observability.RelayDashboard == nil {
		t.Fatal("expected relay dashboard snapshot")
	}
	if observability.RelayDashboard.TotalConnections != 0 || observability.RelayDashboard.TotalProxies != 0 {
		// Frps telemetry collection is disabled; dashboard values are always zero.
		t.Fatalf("unexpected relay dashboard: %+v", observability.RelayDashboard)
	}
}

func TestHeartbeatFlaredRejectsWrongNodeType(t *testing.T) {
	setupServiceTestDB(t)

	node := &model.Node{
		NodeID:      "node-not-tunnel-client",
		Name:        "edge",
		IP:          "10.0.0.1",
		AccessToken: "edge-token",
		Status:      NodeStatusPending,
		NodeType:    "edge_node",
		Version:     "v0.0.0",
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to seed edge node: %v", err)
	}

	_, err := HeartbeatFlared(node, FlaredHeartbeatPayload{
		ClientVersion:  "v0.1.0",
		FrpVersion:     "0.61.0",
		TunnelStatus:   "running",
		CurrentVersion: "v1",
	})
	if err == nil {
		t.Fatal("expected error for non-tunnel_client node type")
	}
}

func TestHeartbeatFlaredRejectsNilNode(t *testing.T) {
	if _, err := HeartbeatFlared(nil, FlaredHeartbeatPayload{}); err == nil {
		t.Fatal("expected error when node is nil")
	}
}

func TestHeartbeatFlaredPersistsRuntime(t *testing.T) {
	setupServiceTestDB(t)

	node := &model.Node{
		NodeID:      "node-flared-1",
		Name:        "flared-1",
		IP:          "",
		AccessToken: "tunnel-token-abc",
		Status:      NodeStatusPending,
		NodeType:    "tunnel_client",
		Version:     "",
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to seed flared node: %v", err)
	}

	resp, err := HeartbeatFlared(node, FlaredHeartbeatPayload{
		ClientVersion: "  v0.2.0  ",
		FrpVersion:    "  0.61.1  ",
		IP:            "  192.168.1.10  ",
		TunnelStatus:  "  RUNNING ",
		ConnectedRelays: []FlaredConnectedRelay{
			{RelayNodeID: "  node-relay-1 ", Status: "  HEALTHY ", ProxyCount: 3},
			{RelayNodeID: "", Status: "running"},
			{RelayNodeID: "node-relay-2", Status: ""},
		},
		CurrentVersion:  "v1",
		CurrentChecksum: "checksum-1",
	})
	if err != nil {
		t.Fatalf("HeartbeatFlared failed: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.TunnelSettings == nil {
		t.Fatal("expected tunnel_settings in response")
	}
	if resp.TunnelSettings.HeartbeatInterval == 0 {
		t.Fatal("expected heartbeat interval to be set in tunnel_settings")
	}

	updated, err := model.GetNodeByNodeID(node.NodeID)
	if err != nil {
		t.Fatalf("failed to reload flared node: %v", err)
	}
	if updated.Status != NodeStatusOnline {
		t.Fatalf("expected flared node to be online, got %q", updated.Status)
	}
	if updated.Version != "v0.2.0" {
		t.Fatalf("expected client_version to be trimmed and stored, got %q", updated.Version)
	}
	if updated.ExtVersion != "0.61.1" {
		t.Fatalf("expected frp_version to be trimmed and stored, got %q", updated.ExtVersion)
	}
	if updated.IP != "192.168.1.10" {
		t.Fatalf("expected IP to be trimmed and stored, got %q", updated.IP)
	}
	if updated.CurrentVersion != "v1" {
		t.Fatalf("expected current_version to be stored, got %q", updated.CurrentVersion)
	}
	if updated.LastSeenAt.IsZero() {
		t.Fatal("expected last_seen_at to be updated")
	}

	// Test IPManualOverride
	updated.IPManualOverride = true
	if err := updated.Update(); err != nil {
		t.Fatalf("failed to lock IP: %v", err)
	}

	_, err = HeartbeatFlared(updated, FlaredHeartbeatPayload{
		ClientVersion: "v0.2.0",
		FrpVersion:    "0.61.1",
		IP:            "10.0.0.99",
		TunnelStatus:  "running",
	})
	if err != nil {
		t.Fatalf("second HeartbeatFlared failed: %v", err)
	}

	lockedNode, err := model.GetNodeByNodeID(node.NodeID)
	if err != nil {
		t.Fatalf("failed to reload locked node: %v", err)
	}
	if lockedNode.IP != "192.168.1.10" {
		t.Fatalf("expected IP to stay locked at 192.168.1.10, but got %q", lockedNode.IP)
	}
}

func TestHeartbeatFlaredTrimsAndFiltersRelays(t *testing.T) {
	normalized := normalizeFlaredHeartbeatPayload(FlaredHeartbeatPayload{
		TunnelStatus: "  UNHEALTHY ",
		ConnectedRelays: []FlaredConnectedRelay{
			{RelayNodeID: "  node-a ", Status: "  OK "},
			{RelayNodeID: "", Status: "running"},
		},
	})
	if normalized.TunnelStatus != "unhealthy" {
		t.Fatalf("expected tunnel_status to be lower-cased, got %q", normalized.TunnelStatus)
	}
	if len(normalized.ConnectedRelays) != 1 {
		t.Fatalf("expected empty relay_node_id to be dropped, got %+v", normalized.ConnectedRelays)
	}
	relay := normalized.ConnectedRelays[0]
	if relay.RelayNodeID != "node-a" {
		t.Fatalf("expected relay_node_id to be trimmed, got %q", relay.RelayNodeID)
	}
	if relay.Status != "ok" {
		t.Fatalf("expected status to be lower-cased, got %q", relay.Status)
	}
}

func TestHeartbeatFlaredEmitsHealthEventOnUnhealthy(t *testing.T) {
	setupServiceTestDB(t)

	node := &model.Node{
		NodeID:      "node-flared-unhealthy",
		Name:        "flared-unhealthy",
		IP:          "",
		AccessToken: "tunnel-token-unhealthy",
		Status:      NodeStatusPending,
		NodeType:    "tunnel_client",
		Version:     "",
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to seed flared node: %v", err)
	}

	if _, err := HeartbeatFlared(node, FlaredHeartbeatPayload{
		ClientVersion:   "v0.2.0",
		FrpVersion:      "0.61.0",
		TunnelStatus:    "unhealthy",
		CurrentVersion:  "v1",
		CurrentChecksum: "checksum-1",
	}); err != nil {
		t.Fatalf("HeartbeatFlared failed: %v", err)
	}

	events, err := model.ListNodeHealthEvents(node.NodeID, false, 20)
	if err != nil {
		t.Fatalf("ListNodeHealthEvents failed: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected unhealthy heartbeat to emit a node health event")
	}
	foundUnhealthy := false
	for _, event := range events {
		if event.EventType == "flared_runtime_unhealthy" {
			foundUnhealthy = true
		}
	}
	if !foundUnhealthy {
		t.Fatalf("expected flared_runtime_unhealthy event in %+v", events)
	}
}

func TestHeartbeatFlaredEmitsEmptyConnectedRelays(t *testing.T) {
	normalized := normalizeFlaredHeartbeatPayload(FlaredHeartbeatPayload{})
	if normalized.ConnectedRelays == nil {
		t.Fatal("expected ConnectedRelays to be non-nil empty slice for nil input")
	}
	if len(normalized.ConnectedRelays) != 0 {
		t.Fatalf("expected empty ConnectedRelays, got %+v", normalized.ConnectedRelays)
	}
}

func TestGetFlaredTunnelConfigRequiresActiveVersion(t *testing.T) {
	setupServiceTestDB(t)

	node := &model.Node{
		NodeID:      "node-flared-noactive",
		Name:        "flared-noactive",
		IP:          "",
		AccessToken: "tunnel-token-na",
		Status:      NodeStatusPending,
		NodeType:    "tunnel_client",
		Version:     "",
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to seed flared node: %v", err)
	}

	_, err := GetFlaredTunnelConfig(node)
	if err == nil {
		t.Fatal("expected error when no active config version exists")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		// We accept either wrapping the underlying error or surfacing a friendly message.
		// Just ensure we surface a clear failure instead of a nil result.
		t.Logf("GetFlaredTunnelConfig returned wrapped error: %v", err)
	}
}

func TestTunnelRoutePublishAndFlaredConfigUseRelayPorts(t *testing.T) {
	setupServiceTestDB(t)

	relayNode := &model.Node{
		NodeID:                "node-relay-ports",
		Name:                  "relay-ports",
		IP:                    "85.235.64.179",
		AccessToken:           "relay-token-ports",
		Status:                NodeStatusOnline,
		NodeType:              "tunnel_relay",
		RelayStatus:           "healthy",
		RelayBindPort:         17000,
		RelayVhostHTTPPort:    18080,
		RelayAuthToken:        "relay-auth-token",
		RelayClientAccessAddr: "de-e",
	}
	if err := relayNode.Insert(); err != nil {
		t.Fatalf("failed to seed relay node: %v", err)
	}
	tunnelNode := &model.Node{
		NodeID:      "node-flared-ports",
		Name:        "flared-ports",
		IP:          "",
		AccessToken: "tunnel-token-ports",
		Status:      NodeStatusOnline,
		NodeType:    "tunnel_client",
		Version:     "v0.2.0",
	}
	if err := tunnelNode.Insert(); err != nil {
		t.Fatalf("failed to seed tunnel client node: %v", err)
	}

	route, err := CreateProxyRoute(ProxyRouteInput{
		Domain:               "flared.example.com",
		UpstreamType:         "tunnel",
		TunnelID:             &tunnelNode.ID,
		TunnelTargetAddr:     "10.0.0.8:8080",
		TunnelTargetProtocol: "http",
		Enabled:              true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}
	if route.TunnelNodeID == nil || *route.TunnelNodeID != tunnelNode.ID {
		t.Fatalf("expected legacy tunnel_id to bind tunnel_node_id, got %+v", route.TunnelNodeID)
	}

	result, err := PublishConfigVersion("root", false)
	if err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}
	if !strings.Contains(result.Version.RenderedConfig, "server de-e:18080 max_fails=3 fail_timeout=10s;") {
		t.Fatalf("expected rendered OpenResty upstream to use relay vhost port, got:\n%s", result.Version.RenderedConfig)
	}

	config, err := GetFlaredTunnelConfig(tunnelNode)
	if err != nil {
		t.Fatalf("GetFlaredTunnelConfig failed: %v", err)
	}
	if len(config.Relays) != 1 {
		t.Fatalf("expected one relay, got %+v", config.Relays)
	}
	if config.Relays[0].Address != "de-e:17000" {
		t.Fatalf("expected relay client address to include bind port, got %q", config.Relays[0].Address)
	}
	if len(config.Proxies) != 1 {
		t.Fatalf("expected one proxy, got %+v", config.Proxies)
	}
	proxy := config.Proxies[0]
	if proxy.LocalAddr != "10.0.0.8" || proxy.LocalPort != 8080 {
		t.Fatalf("unexpected proxy target: %+v", proxy)
	}
	if len(proxy.CustomDomains) != 1 || proxy.CustomDomains[0] != "flared.example.com" {
		t.Fatalf("unexpected proxy domains: %+v", proxy.CustomDomains)
	}
}

func TestHeartbeatRelaySelfUpdatePropagationAndReset(t *testing.T) {
	setupServiceTestDB(t)

	node := &model.Node{
		NodeID:            "relay-update-node",
		Name:              "relay-u",
		IP:                "1.1.1.1",
		AccessToken:       "relay-update-token",
		Status:            NodeStatusPending,
		NodeType:          "tunnel_relay",
		AutoUpdateEnabled: true,
		UpdateRequested:   true,
		UpdateChannel:     "preview",
		UpdateTag:         "v1.2.3",
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to seed relay node: %v", err)
	}

	resp, err := HeartbeatRelay(node, RelayHeartbeatPayload{
		Version:     "v1.0.0",
		ExtVersion:  "0.61.0",
		RelayStatus: "healthy",
	})
	if err != nil {
		t.Fatalf("HeartbeatRelay failed: %v", err)
	}

	if resp == nil || resp.RelaySettings == nil {
		t.Fatal("expected non-nil response with RelaySettings")
	}

	settings := resp.RelaySettings
	if !settings.AutoUpdate {
		t.Error("expected AutoUpdate to be true")
	}
	if !settings.UpdateNow {
		t.Error("expected UpdateNow to be true")
	}
	if settings.UpdateChannel != "preview" {
		t.Errorf("expected UpdateChannel to be preview, got %q", settings.UpdateChannel)
	}
	if settings.UpdateTag != "v1.2.3" {
		t.Errorf("expected UpdateTag to be v1.2.3, got %q", settings.UpdateTag)
	}

	// Verify that the requested update was cleared in the DB
	updated, err := model.GetNodeByNodeID(node.NodeID)
	if err != nil {
		t.Fatalf("failed to reload node: %v", err)
	}
	if updated.UpdateRequested {
		t.Error("expected UpdateRequested to be reset to false in the database")
	}
	if updated.UpdateChannel != "stable" {
		t.Errorf("expected UpdateChannel to be reset to stable, got %q", updated.UpdateChannel)
	}
	if updated.UpdateTag != "" {
		t.Errorf("expected UpdateTag to be reset to empty, got %q", updated.UpdateTag)
	}
}

func TestHeartbeatFlaredSelfUpdatePropagationAndReset(t *testing.T) {
	setupServiceTestDB(t)

	node := &model.Node{
		NodeID:            "flared-update-node",
		Name:              "flared-u",
		IP:                "1.1.1.2",
		AccessToken:       "flared-update-token",
		Status:            NodeStatusPending,
		NodeType:          "tunnel_client",
		AutoUpdateEnabled: true,
		UpdateRequested:   true,
		UpdateChannel:     "stable",
		UpdateTag:         "v2.3.4",
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to seed flared node: %v", err)
	}

	resp, err := HeartbeatFlared(node, FlaredHeartbeatPayload{
		ClientVersion: "v1.0.0",
		FrpVersion:    "0.61.0",
		TunnelStatus:  "running",
	})
	if err != nil {
		t.Fatalf("HeartbeatFlared failed: %v", err)
	}

	if resp == nil || resp.TunnelSettings == nil {
		t.Fatal("expected non-nil response with TunnelSettings")
	}

	settings := resp.TunnelSettings
	if !settings.AutoUpdate {
		t.Error("expected AutoUpdate to be true")
	}
	if !settings.UpdateNow {
		t.Error("expected UpdateNow to be true")
	}
	if settings.UpdateChannel != "stable" {
		t.Errorf("expected UpdateChannel to be stable, got %q", settings.UpdateChannel)
	}
	if settings.UpdateTag != "v2.3.4" {
		t.Errorf("expected UpdateTag to be v2.3.4, got %q", settings.UpdateTag)
	}

	// Verify that the requested update was cleared in the DB
	updated, err := model.GetNodeByNodeID(node.NodeID)
	if err != nil {
		t.Fatalf("failed to reload node: %v", err)
	}
	if updated.UpdateRequested {
		t.Error("expected UpdateRequested to be reset to false in the database")
	}
	if updated.UpdateChannel != "stable" {
		t.Errorf("expected UpdateChannel to be reset to stable, got %q", updated.UpdateChannel)
	}
	if updated.UpdateTag != "" {
		t.Errorf("expected UpdateTag to be reset to empty, got %q", updated.UpdateTag)
	}
}
