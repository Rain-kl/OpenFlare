package service

import (
	"openflare/model"
	"testing"
	"time"
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
	if observability.RelayDashboard.TotalConnections != 7 || observability.RelayDashboard.TotalProxies != 3 {
		t.Fatalf("unexpected relay dashboard: %+v", observability.RelayDashboard)
	}
}
