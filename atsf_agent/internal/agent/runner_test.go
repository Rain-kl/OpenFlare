package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"atsflare-agent/internal/config"
	"atsflare-agent/internal/protocol"
	"atsflare-agent/internal/state"
)

type fakeHeartbeatService struct {
	mu                sync.Mutex
	registerCalls     int
	heartbeatCalls    int
	registerErr       error
	registerResp      *protocol.RegisterNodeResponse
	heartbeatErrs     []error
	heartbeatResults  []*protocol.HeartbeatResult
	heartbeatPayloads []protocol.NodePayload
	onHeartbeat       func(int)
	lastToken         string
}

func (f *fakeHeartbeatService) Register(ctx context.Context, payload protocol.NodePayload) (*protocol.RegisterNodeResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registerCalls++
	return f.registerResp, f.registerErr
}

func (f *fakeHeartbeatService) Heartbeat(ctx context.Context, payload protocol.NodePayload) (*protocol.HeartbeatResult, error) {
	f.mu.Lock()
	f.heartbeatCalls++
	callIndex := f.heartbeatCalls
	f.heartbeatPayloads = append(f.heartbeatPayloads, payload)
	var err error
	if len(f.heartbeatErrs) >= callIndex {
		err = f.heartbeatErrs[callIndex-1]
	}
	var result *protocol.HeartbeatResult
	if len(f.heartbeatResults) >= callIndex {
		result = f.heartbeatResults[callIndex-1]
	}
	onHeartbeat := f.onHeartbeat
	f.mu.Unlock()
	if onHeartbeat != nil {
		onHeartbeat(callIndex)
	}
	return result, err
}

func (f *fakeHeartbeatService) SetToken(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastToken = token
}

type fakeSyncService struct {
	mu             sync.Mutex
	startupErr     error
	syncOnceErr    error
	startupCalls   int
	syncOnceCalls  int
	onSyncOnceCall func(int)
}

type fakeRuntimeManager struct {
	mu                   sync.Mutex
	healthErr            error
	restartErr           error
	restartCalls         int
	clearHealthOnRestart bool
}

func (f *fakeRuntimeManager) CheckHealth(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.healthErr
}

func (f *fakeRuntimeManager) Restart(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.restartCalls++
	if f.clearHealthOnRestart && f.restartErr == nil {
		f.healthErr = nil
	}
	return f.restartErr
}

func (f *fakeSyncService) SyncOnStartup(ctx context.Context, target *protocol.ActiveConfigMeta) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.startupCalls++
	return f.startupErr
}

func (f *fakeSyncService) SyncOnce(ctx context.Context, target *protocol.ActiveConfigMeta) error {
	f.mu.Lock()
	f.syncOnceCalls++
	callIndex := f.syncOnceCalls
	callback := f.onSyncOnceCall
	f.mu.Unlock()
	if callback != nil {
		callback(callIndex)
	}
	return f.syncOnceErr
}

func TestRunnerKeepsHeartbeatWhenStartupSyncFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	heartbeatService := &fakeHeartbeatService{
		heartbeatResults: []*protocol.HeartbeatResult{{}},
		onHeartbeat: func(callCount int) {
			if callCount >= 2 {
				cancel()
			}
		},
	}
	syncService := &fakeSyncService{
		startupErr: errors.New("当前没有激活版本，保持当前 OpenResty 配置"),
	}
	runner := &Runner{
		Config: &config.Config{
			AgentToken:        "agent-token",
			NodeName:          "edge-01",
			NodeIP:            "10.0.0.8",
			AgentVersion:      config.AgentVersion,
			NginxVersion:      "1.27.1.2",
			HeartbeatInterval: config.MillisecondDuration(10 * time.Millisecond),
		},
		StateStore:       stateStore,
		HeartbeatService: heartbeatService,
		SyncService:      syncService,
	}

	err := runner.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if heartbeatService.registerCalls != 0 {
		t.Fatalf("expected no discovery register call, got %d", heartbeatService.registerCalls)
	}
	if heartbeatService.heartbeatCalls < 2 {
		t.Fatalf("expected heartbeat loop to continue, got %d heartbeat calls", heartbeatService.heartbeatCalls)
	}
	snapshot, loadErr := stateStore.Load()
	if loadErr != nil {
		t.Fatalf("failed to load state: %v", loadErr)
	}
	if snapshot.LastError != "当前没有激活版本，保持当前 OpenResty 配置" {
		t.Fatalf("expected startup sync error to be recorded, got %q", snapshot.LastError)
	}
}

func TestRunnerDoesNotExitOnHeartbeatOrSyncError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	heartbeatService := &fakeHeartbeatService{
		registerErr:   errors.New("register timeout"),
		heartbeatErrs: []error{errors.New("heartbeat timeout")},
		heartbeatResults: []*protocol.HeartbeatResult{
			{},
		},
	}
	syncService := &fakeSyncService{
		syncOnceErr: errors.New("openresty reload failed"),
		onSyncOnceCall: func(callCount int) {
			if callCount >= 1 {
				cancel()
			}
		},
	}
	runner := &Runner{
		Config: &config.Config{
			AgentToken:        "agent-token",
			NodeName:          "edge-01",
			NodeIP:            "10.0.0.8",
			AgentVersion:      config.AgentVersion,
			NginxVersion:      "1.27.1.2",
			HeartbeatInterval: config.MillisecondDuration(10 * time.Millisecond),
		},
		StateStore:       stateStore,
		HeartbeatService: heartbeatService,
		SyncService:      syncService,
	}

	err := runner.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if heartbeatService.registerCalls != 0 {
		t.Fatalf("expected no register attempt, got %d", heartbeatService.registerCalls)
	}
	if syncService.syncOnceCalls == 0 {
		t.Fatal("expected sync loop to continue after heartbeat/register errors")
	}
	snapshot, loadErr := stateStore.Load()
	if loadErr != nil {
		t.Fatalf("failed to load state: %v", loadErr)
	}
	if snapshot.LastError != "openresty reload failed" {
		t.Fatalf("expected sync error to be recorded, got %q", snapshot.LastError)
	}
}

func TestRunnerReportsOpenrestyHealthAndExecutesRestart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	if err := stateStore.Save(&state.Snapshot{
		OpenrestyStatus:  protocol.OpenrestyStatusUnhealthy,
		OpenrestyMessage: "docker run openresty failed: bind 80 already allocated",
	}); err != nil {
		t.Fatalf("failed to seed state: %v", err)
	}
	heartbeatService := &fakeHeartbeatService{
		heartbeatResults: []*protocol.HeartbeatResult{{
			AgentSettings: &protocol.AgentSettings{RestartOpenrestyNow: true},
		}},
		onHeartbeat: func(callCount int) {
			if callCount >= 1 {
				cancel()
			}
		},
	}
	runtimeManager := &fakeRuntimeManager{
		healthErr:            errors.New("docker openresty container is not running"),
		clearHealthOnRestart: true,
	}
	runner := &Runner{
		Config: &config.Config{
			AgentToken:        "agent-token",
			NodeName:          "edge-01",
			NodeIP:            "10.0.0.8",
			AgentVersion:      config.AgentVersion,
			NginxVersion:      "1.27.1.2",
			HeartbeatInterval: config.MillisecondDuration(10 * time.Millisecond),
		},
		StateStore:       stateStore,
		HeartbeatService: heartbeatService,
		SyncService:      &fakeSyncService{},
		RuntimeManager:   runtimeManager,
	}

	err := runner.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if len(heartbeatService.heartbeatPayloads) == 0 {
		t.Fatal("expected at least one heartbeat payload")
	}
	payload := heartbeatService.heartbeatPayloads[0]
	if payload.OpenrestyStatus != protocol.OpenrestyStatusUnhealthy {
		t.Fatalf("expected unhealthy openresty status in heartbeat payload, got %q", payload.OpenrestyStatus)
	}
	if payload.OpenrestyMessage != "docker run openresty failed: bind 80 already allocated" {
		t.Fatalf("unexpected openresty message: %q", payload.OpenrestyMessage)
	}
	if runtimeManager.restartCalls != 1 {
		t.Fatalf("expected one openresty restart attempt, got %d", runtimeManager.restartCalls)
	}
	snapshot, loadErr := stateStore.Load()
	if loadErr != nil {
		t.Fatalf("failed to load state: %v", loadErr)
	}
	if snapshot.OpenrestyStatus != protocol.OpenrestyStatusHealthy || snapshot.OpenrestyMessage != "" {
		t.Fatal("expected restart success to mark openresty healthy")
	}
}

func TestRunnerDiscoveryRegisterUpdatesTokenAndNodeID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	heartbeatService := &fakeHeartbeatService{
		registerResp: &protocol.RegisterNodeResponse{
			NodeID:     "node-server-assigned",
			AgentToken: "agent-token-issued",
			Name:       "edge-01",
		},
		heartbeatResults: []*protocol.HeartbeatResult{{}},
		onHeartbeat: func(callCount int) {
			if callCount >= 1 {
				cancel()
			}
		},
	}
	syncService := &fakeSyncService{}
	configPath := filepath.Join(t.TempDir(), "agent.json")
	if err := os.WriteFile(configPath, []byte(`{"server_url":"http://127.0.0.1:3000","discovery_token":"discovery-token","node_name":"edge-01","node_ip":"10.0.0.8"}`), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	runner := &Runner{
		Config: &config.Config{
			ServerURL:         cfg.ServerURL,
			DiscoveryToken:    cfg.DiscoveryToken,
			NodeName:          cfg.NodeName,
			NodeIP:            cfg.NodeIP,
			AgentVersion:      config.AgentVersion,
			NginxVersion:      "1.27.1.2",
			HeartbeatInterval: config.MillisecondDuration(10 * time.Millisecond),
		},
		StateStore:       stateStore,
		HeartbeatService: heartbeatService,
		SyncService:      syncService,
	}
	runner.Config = cfg
	runner.Config.AgentVersion = config.AgentVersion
	runner.Config.NginxVersion = "1.27.1.2"
	runner.Config.HeartbeatInterval = config.MillisecondDuration(10 * time.Millisecond)

	err = runner.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if heartbeatService.registerCalls == 0 {
		t.Fatal("expected discovery register to be attempted")
	}
	if heartbeatService.lastToken != "agent-token-issued" {
		t.Fatalf("expected client token to be updated, got %q", heartbeatService.lastToken)
	}
	snapshot, loadErr := stateStore.Load()
	if loadErr != nil {
		t.Fatalf("failed to load state: %v", loadErr)
	}
	if snapshot.NodeID != "node-server-assigned" {
		t.Fatalf("expected node id to be replaced, got %q", snapshot.NodeID)
	}
	if runner.Config.AgentToken != "agent-token-issued" || runner.Config.DiscoveryToken != "" {
		t.Fatal("expected config token rotation to complete")
	}
}
