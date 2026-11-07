// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/config"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/state"
)

func TestDiscoveryDoesNotRegisterBeforeRetrySecretPersists(t *testing.T) {
	blockedParent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blockedParent, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := &fakeHeartbeatService{}
	runner := &Runner{Config: &config.Config{DiscoveryToken: "discovery"},
		StateStore: state.NewStore(filepath.Join(blockedParent, "state.json")), HeartbeatService: service}
	nodeID := "local"
	if err := runner.tryRegister(context.Background(), &nodeID); err == nil {
		t.Fatal("tryRegister(unwritable state) succeeded, want persistence error")
	}
	if service.registerCalls != 0 {
		t.Errorf("tryRegister(unwritable retry key) sent %d requests, want 0", service.registerCalls)
	}
}

func TestDiscoveryStateWriteFailureDoesNotActivateCredentials(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "agent.json")
	if err := os.WriteFile(configPath, []byte(`{"server_url":"http://127.0.0.1:3000","discovery_token":"discovery","node_ip":"10.0.0.8"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	runner, nodeID := newDiscoveryTestRunner(t, configPath)
	statePath := runner.Config.StatePath
	backup := statePath + ".original"
	service := &fakeHeartbeatService{registerResp: &protocol.RegisterNodeResponse{NodeID: "server-assigned", AccessToken: "issued"}}
	var originalKey string
	service.onRegister = func() {
		snapshot, err := runner.StateStore.Load()
		if err != nil {
			t.Fatal(err)
		}
		originalKey = snapshot.RegistrationKey
		if err = os.Rename(statePath, backup); err != nil {
			t.Fatal(err)
		}
		if err = os.Mkdir(statePath, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	runner.HeartbeatService = service
	runner.HeartbeatCycle.Heartbeat = service
	if err := runner.tryRegister(context.Background(), &nodeID); err == nil {
		t.Fatal("tryRegister(state write failure) succeeded, want error")
	}
	if runner.hasAccessToken() || service.lastToken != "" {
		t.Fatal("Failed state persistence activated credentials")
	}
	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(backup, statePath); err != nil {
		t.Fatal(err)
	}
	restarted, restartedID := newDiscoveryTestRunner(t, configPath)
	service.onRegister = nil
	restarted.HeartbeatService = service
	restarted.HeartbeatCycle.Heartbeat = service
	if err := restarted.tryRegister(context.Background(), &restartedID); err != nil {
		t.Fatal(err)
	}
	key, err := restarted.StateStore.EnsureRegistrationKey()
	if err != nil || key != originalKey {
		t.Errorf("Key after state write failure/restart changed: %v", err)
	}
	if restartedID != "server-assigned" || service.lastToken != "issued" {
		t.Errorf("Recovered identity/token = %q/%q, want server-assigned/issued", restartedID, service.lastToken)
	}
}
