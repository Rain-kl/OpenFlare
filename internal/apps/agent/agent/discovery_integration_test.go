// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

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
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/config"
	agentheartbeat "github.com/Rain-kl/Wavelet/internal/apps/agent/heartbeat"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/httpclient"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/state"
	serveragent "github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

func newDiscoveryTestRunner(t *testing.T, configPath string) (*Runner, string) {
	t.Helper()
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	store := state.NewStore(cfg.StatePath)
	nodeID, err := store.EnsureNodeID()
	if err != nil {
		t.Fatal(err)
	}
	client := httpclient.New(cfg.ServerURL, cfg.InitialAuthToken(), time.Second)
	runner := withHeartbeatCycle(&Runner{Config: cfg, StateStore: store,
		HeartbeatService: agentheartbeat.New(client), SyncService: &fakeSyncService{}}, nil)
	return runner, nodeID
}

func setupDiscoveryIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()
	conn, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "discovery.db")), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	previous := db.DB(ctx)
	t.Cleanup(func() {
		if invalidateErr := repository.InvalidateAllSystemConfigCaches(ctx); invalidateErr != nil {
			t.Error(invalidateErr)
		}
		db.SetDB(previous)
		if closeErr := sqlDB.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	provider, err := goose.NewProvider(goose.DialectSQLite3, sqlDB, os.DirFS("../../../infra/persistence/migrator/goose/sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	db.SetDB(conn)
	return conn
}

func TestDiscoveryHTTPRecoveryAndRestart(t *testing.T) {
	ctx := context.Background()
	conn := setupDiscoveryIntegrationDB(t)
	var err error
	if err = repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyAgentDiscoveryToken, "discovery-test"); err != nil {
		t.Fatal(err)
	}
	if err = repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyGeoIPProvider, "disabled"); err != nil {
		t.Fatal(err)
	}
	var registrations, heartbeats atomic.Int32
	var dropResponse atomic.Bool
	dropResponse.Store(true)
	router := testhelper.NewTestGinEngine()
	router.POST("/api/v1/agent/nodes/register", serveragent.RegisterAuth(), func(c *gin.Context) {
		registrations.Add(1)
		serveragent.RegisterHandler(c)
	})
	router.POST("/api/v1/agent/nodes/heartbeat", serveragent.Auth(), func(c *gin.Context) {
		var payload map[string]json.RawMessage
		if decodeErr := c.ShouldBindJSON(&payload); decodeErr != nil {
			t.Error(decodeErr)
		}
		if _, exists := payload["registration_key"]; exists {
			t.Error("heartbeat exposed registration_key")
		}
		heartbeats.Add(1)
		c.JSON(http.StatusOK, response.OKNil())
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/nodes/register" && dropResponse.Swap(false) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, r)
			if recorder.Code != http.StatusOK {
				t.Errorf("First registration status = %d, want 200", recorder.Code)
			}
			w.WriteHeader(http.StatusBadGateway) // Enrollment committed, but its response was lost.
			return
		}
		router.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	configPath := filepath.Join(t.TempDir(), "agent.json")
	data, err := json.Marshal(map[string]string{"server_url": server.URL, "discovery_token": "discovery-test", "node_ip": "10.0.0.8", "node_name": "edge"})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	first, nodeID := newDiscoveryTestRunner(t, configPath)
	if err = first.tryRegister(ctx, &nodeID); err == nil {
		t.Fatal("tryRegister(lost response) succeeded, want transport failure")
	}
	key, err := first.StateStore.EnsureRegistrationKey()
	if err != nil {
		t.Fatal(err)
	}
	if first.hasAccessToken() {
		t.Fatal("tryRegister(lost response) activated token")
	}

	// Restart with the same private key, then fail the second durable identity write.
	second, secondID := newDiscoveryTestRunner(t, configPath)
	backup := configPath + ".original"
	if err = os.Rename(configPath, backup); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(configPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err = second.tryRegister(ctx, &secondID); err == nil {
		t.Fatal("tryRegister(config save failure) succeeded, want write error")
	}
	if second.hasAccessToken() || second.Config.DiscoveryToken == "" {
		t.Fatal("Config write failure activated partial in-memory credentials")
	}
	if err = os.Remove(configPath); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(backup, configPath); err != nil {
		t.Fatal(err)
	}

	third, thirdID := newDiscoveryTestRunner(t, configPath)
	if err = third.tryRegister(ctx, &thirdID); err != nil {
		t.Fatalf("tryRegister(after restart) error = %v", err)
	}
	if !third.hasAccessToken() || third.Config.DiscoveryToken != "" {
		t.Fatal("Successful enrollment did not activate node credentials")
	}
	persistedKey, err := third.StateStore.EnsureRegistrationKey()
	if err != nil || persistedKey != key {
		t.Errorf("Retry key changed across partial-write restarts: %v", err)
	}

	restarted, restartedID := newDiscoveryTestRunner(t, configPath)
	restarted.runStartupAuth(ctx, &restartedID)
	if restartedID != thirdID {
		t.Errorf("Restarted node ID = %q, want %q", restartedID, thirdID)
	}
	if registrations.Load() != 3 || heartbeats.Load() != 2 {
		t.Errorf("Enrollment/restart calls = %d registrations, %d heartbeats; want 3, 2", registrations.Load(), heartbeats.Load())
	}
	var count int64
	if err = conn.Model(&model.OpenFlareNode{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("Enrollment through lost response and partial write created %d nodes, want 1", count)
	}

	// Knowing the private retry key alone is insufficient after the global discovery token rotates.
	if err = repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyAgentDiscoveryToken, "rotated-discovery"); err != nil {
		t.Fatal(err)
	}
	request := protocol.RegisterNodeRequest{NodePayload: protocol.NodePayload{IP: "10.0.0.8", Version: "test"}, RegistrationKey: key}
	for _, bearer := range []string{"", "invalid", "discovery-test"} {
		client := httpclient.New(server.URL, bearer, time.Second)
		result, authErr := client.RegisterNode(ctx, request)
		if authErr == nil || result != nil {
			t.Errorf("RegisterNode(unauthorized bearer) = %+v, %v; want rejection", result, authErr)
		}
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/api/v1/agent/nodes/register", strings.NewReader(string(encoded))))
	if unauthorized.Code != http.StatusUnauthorized || strings.Contains(unauthorized.Body.String(), third.Config.AccessToken) {
		t.Errorf("Unauthorized retry response status = %d; want 401 without credential", unauthorized.Code)
	}
	client := httpclient.New(server.URL, "rotated-discovery", time.Second)
	result, err := client.RegisterNode(ctx, request)
	if err != nil || result.AccessToken != third.Config.AccessToken {
		t.Errorf("RegisterNode(current discovery bearer) error = %v, want same identity", err)
	}
}
