// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	agentclient "github.com/Rain-kl/Wavelet/internal/apps/agent/httpclient"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/protocol"
)

func TestRegistrationResponseServerClientContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response.OK(RegistrationResponse{NodeID: "node-contract", AccessToken: "issued", Name: "edge"})); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	client := agentclient.New(server.URL, "discovery", time.Second)
	got, err := client.RegisterNode(context.Background(), protocol.RegisterNodeRequest{})
	if err != nil {
		t.Fatalf("RegisterNode(server response) error = %v", err)
	}
	if got.NodeID != "node-contract" || got.AccessToken != "issued" || got.Name != "edge" {
		t.Errorf("RegisterNode(server response) = %+v, want node-contract/issued/edge", got)
	}
	data, err := json.Marshal(RegistrationResponse{AccessToken: "issued"})
	if err != nil {
		t.Fatal(err)
	}
	var legacy struct {
		AccessToken string `json:"access_token"`
	}
	if err = json.Unmarshal(data, &legacy); err != nil || legacy.AccessToken != "issued" {
		t.Errorf("Decode legacy server response = %q, %v; want issued, nil", legacy.AccessToken, err)
	}
	var documented RegistrationResponseDocs
	if err = json.Unmarshal(data, &documented); err != nil {
		t.Fatal(err)
	}
	if documented.AgentToken != "issued" || documented.AccessToken != documented.AgentToken {
		t.Error("RegistrationResponseDocs does not describe both emitted token aliases")
	}
}

func TestDiscoveryRetriesKeepOneIdentityAndPreserveChanges(t *testing.T) {
	t.Cleanup(setupAgentAuthTestDB(t))
	ctx := context.Background()
	key := strings.Repeat("a1", 32)
	payload := NodePayload{IP: "10.0.0.8", Version: "test", Name: "edge"}
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
	payload := NodePayload{IP: "10.0.0.8", Version: "test"}
	const attempts = 12
	results := make(chan *RegistrationResponse, attempts)
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
	var first *RegistrationResponse
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
