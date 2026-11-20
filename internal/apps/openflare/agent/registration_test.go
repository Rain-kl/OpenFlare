// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

func TestRegisterWithAccessTokenMigratesAnExistingEdge(t *testing.T) {
	t.Cleanup(setupAgentAuthTestDB(t))
	ctx := context.Background()
	if err := db.DB(ctx).AutoMigrate(&model.ConfigVersion{}); err != nil {
		t.Fatal(err)
	}
	seen := time.Now()
	node := model.OpenFlareNode{
		NodeID: "existing-edge", NodeType: "edge_node", Name: "managed edge", IP: "192.0.2.2",
		IPManualOverride: true, AccessToken: "fixture-credential", Engine: engineOpenResty, LastSeenAt: &seen,
	}
	if err := repository.CreateOpenFlareNode(ctx, &node); err != nil {
		t.Fatal(err)
	}
	pingora := protocol.NodePayload{Engine: enginePingora, ContractVersion: edgeconfig.ContractVersion, ConfigSyncVersion: protocol.EdgeConfigSyncVersion, IP: "192.0.2.3", Version: "test"}
	if _, err := RegisterWithAccessToken(ctx, &node, pingora); err == nil || err.Error() != errPingoraReleaseRequired {
		t.Fatalf("migration without a release: %v", err)
	}
	release := model.ConfigVersion{
		Version: "pingora-test", Engine: enginePingora, IsActive: true,
		SourceVersion: "shared-test", PayloadChecksum: "fixture-digest",
	}
	if err := db.DB(ctx).Create(&release).Error; err != nil {
		t.Fatal(err)
	}
	result, err := RegisterWithAccessToken(ctx, &node, pingora)
	if err != nil || result.NodeID != node.NodeID || result.AccessToken != node.AccessToken {
		t.Fatalf("migration changed identity or failed: %v", err)
	}
	oldAgent := protocol.NodePayload{IP: "192.0.2.2", Version: "legacy"}
	if _, err := HeartbeatNode(ctx, &node, oldAgent); err == nil || err.Error() != errPingoraNodeRequired {
		t.Fatalf("stale OpenResty heartbeat: %v", err)
	}
	if _, err := RegisterWithAccessToken(ctx, &node, oldAgent); err == nil || err.Error() != errPingoraNodeRequired {
		t.Fatalf("OpenResty registration after migration: %v", err)
	}
	stored, err := repository.GetOpenFlareNodeByNodeID(ctx, node.NodeID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Engine != enginePingora || stored.ContractVersion != pingora.ContractVersion ||
		stored.Name != node.Name || stored.IP != node.IP || !stored.IPManualOverride || stored.Version != pingora.Version {
		t.Fatal("migration did not preserve node metadata or rejected reports changed the node")
	}
}

func TestDiscoveryRetriesKeepOneIdentityAndPreserveChanges(t *testing.T) {
	t.Cleanup(setupAgentAuthTestDB(t))
	ctx := context.Background()
	key := strings.Repeat("a1", 32)
	payload := protocol.NodePayload{Engine: enginePingora, ContractVersion: edgeconfig.ContractVersion, ConfigSyncVersion: protocol.EdgeConfigSyncVersion, IP: "10.0.0.8", Version: "test", Name: "edge"}
	first, err := RegisterWithDiscovery(ctx, payload, key)
	if err != nil {
		t.Fatal(err)
	}
	record, err := repository.GetOpenFlareNodeByNodeID(ctx, first.NodeID)
	if err != nil {
		t.Fatal(err)
	}
	record.Name = "administrator name"
	record.UpdateRequested = true
	if err = repository.SaveOpenFlareNode(ctx, record); err != nil {
		t.Fatal(err)
	}
	second, err := RegisterWithDiscovery(ctx, payload, key)
	if err != nil {
		t.Fatal(err)
	}
	if first.NodeID != second.NodeID || first.AccessToken != second.AccessToken {
		t.Errorf("RegisterWithDiscovery(retry) identity = %+v, want original identity", second)
	}
	record, err = repository.GetOpenFlareNodeByNodeID(ctx, first.NodeID)
	if err != nil {
		t.Fatal(err)
	}
	if record.Name != "administrator name" || !record.UpdateRequested {
		t.Errorf("RegisterWithDiscovery(retry) overwrote administrator metadata: %+v", record)
	}
	other, err := RegisterWithDiscovery(ctx, payload, strings.Repeat("b2", 32))
	if err != nil {
		t.Fatal(err)
	}
	if other.NodeID == first.NodeID || other.AccessToken == first.AccessToken {
		t.Error("RegisterWithDiscovery(distinct key, same host/IP) reused identity")
	}
	var count int64
	if err = db.DB(ctx).Model(&model.OpenFlareNode{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("RegisterWithDiscovery(two keys, retry) node count = %d, want 2", count)
	}
}

func TestDiscoveryConcurrentRetriesAndRevokedCredential(t *testing.T) {
	t.Cleanup(setupAgentAuthTestDB(t))
	ctx := context.Background()
	sqlDB, err := db.DB(ctx).DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	key := strings.Repeat("c3", 32)
	payload := protocol.NodePayload{Engine: enginePingora, ContractVersion: edgeconfig.ContractVersion, ConfigSyncVersion: protocol.EdgeConfigSyncVersion, IP: "10.0.0.8", Version: "test"}
	const attempts = 12
	results := make(chan *protocol.RegisterNodeResponse, attempts)
	errors := make(chan error, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Go(func() {
			result, registrationErr := RegisterWithDiscovery(ctx, payload, key)
			results <- result
			errors <- registrationErr
		})
	}
	wg.Wait()
	close(results)
	close(errors)
	for registrationErr := range errors {
		if registrationErr != nil {
			t.Fatalf("Concurrent RegisterWithDiscovery error = %v", registrationErr)
		}
	}
	var first *protocol.RegisterNodeResponse
	for result := range results {
		if first == nil {
			first = result
		}
		if result.NodeID != first.NodeID || result.AccessToken != first.AccessToken {
			t.Error("Concurrent enrollment returned different identities")
		}
	}
	var count int64
	if err = db.DB(ctx).Model(&model.OpenFlareNode{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("Concurrent enrollment node count = %d, want 1", count)
	}
	record, err := repository.GetOpenFlareNodeByNodeID(ctx, first.NodeID)
	if err != nil {
		t.Fatal(err)
	}
	record.AccessToken = "rotated-by-administrator"
	if err = repository.SaveOpenFlareNode(ctx, record); err != nil {
		t.Fatal(err)
	}
	if result, retryErr := RegisterWithDiscovery(ctx, payload, key); retryErr == nil || result != nil {
		t.Errorf("RegisterWithDiscovery(rotated credential) = %+v, %v; want rejection", result, retryErr)
	}
	for _, invalid := range []string{"short", strings.Repeat("g", 64), first.NodeID} {
		if result, retryErr := RegisterWithDiscovery(ctx, payload, invalid); retryErr == nil || result != nil {
			t.Errorf("RegisterWithDiscovery(invalid/public key) = %+v, %v; want rejection", result, retryErr)
		}
	}
}

func TestEdgeHeartbeatDeliversAndAcknowledgesUpdates(t *testing.T) {
	t.Cleanup(setupAgentAuthTestDB(t))
	ctx := context.Background()
	record := &model.OpenFlareNode{
		NodeID: "update-edge", Name: "edge", Engine: enginePingora, AccessToken: "update-token",
	}
	if err := repository.CreateOpenFlareNode(ctx, record); err != nil {
		t.Fatal(err)
	}
	payload := protocol.NodePayload{
		Engine: enginePingora, ContractVersion: edgeconfig.ContractVersion, ConfigSyncVersion: protocol.EdgeConfigSyncVersion,
		IP: "192.0.2.100", Version: "0.1.0", EdgeUpdateSupported: true,
	}
	stale := *record
	if err := repository.UpdateOpenFlareNodeColumns(ctx, record, map[string]any{
		"auto_update_enabled": true, "update_requested": true, "update_tag": "edge-v0.2.0",
	}); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		version string
		ready   bool
		tag     string
	}{
		{"0.1.0", true, "edge-v0.2.0"},
		{"0.2.0", false, "edge-v0.2.0"},
		{"0.2.0", true, ""},
	} {
		payload.Version = step.version
		payload.EdgeUpdateReady = step.ready
		result, err := HeartbeatNode(ctx, &stale, payload)
		if err != nil {
			t.Fatal(err)
		}
		settings := result.AgentSettings.EdgeUpdate
		if settings == nil || !settings.AutoUpdate || settings.Tag != step.tag {
			t.Fatalf("version %s, ready %t: unexpected settings %+v", step.version, step.ready, settings)
		}
	}
	stored, err := repository.GetOpenFlareNodeByID(ctx, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.UpdateRequested || stored.UpdateTag != "" || !stored.HasCapability(model.EdgeUpdateCapability) {
		t.Fatalf("unexpected state after update acknowledgment: %+v", stored)
	}
}
