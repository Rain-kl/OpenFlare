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

func TestRunnerHeartbeatPayloadIncludesObservabilityExtensions(t *testing.T) {
	tempDir := t.TempDir()
	stateStore := state.NewStore(filepath.Join(tempDir, "state.json"))
	if err := stateStore.Save(&state.Snapshot{
		NodeID:           "node-observe",
		CurrentVersion:   "20260314-001",
		LastError:        "sync failed",
		OpenrestyStatus:  protocol.OpenrestyStatusUnhealthy,
		OpenrestyMessage: "reload failed",
	}); err != nil {
		t.Fatalf("failed to seed state: %v", err)
	}

	runner := &Runner{
		Config: &config.Config{
			NodeName:          "edge-observe-1",
			NodeIP:            "10.0.0.51",
			AgentVersion:      config.AgentVersion,
			NginxVersion:      "1.27.1.2",
			DataDir:           tempDir,
			RouteConfigPath:   filepath.Join(tempDir, "conf.d", "atsflare_routes.conf"),
			HeartbeatInterval: config.MillisecondDuration(10 * time.Millisecond),
		},
		StateStore: stateStore,
	}
	if err := os.MkdirAll(filepath.Dir(runner.Config.RouteConfigPath), 0o755); err != nil {
		t.Fatalf("failed to prepare route config dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(filepath.Dir(runner.Config.RouteConfigPath), "atsflare_access.log"),
		[]byte("{\"ts\":\""+time.Now().UTC().Format(time.RFC3339)+"\",\"host\":\"edge.example.com\",\"remote_addr\":\"10.0.0.8\",\"status\":200}\n"),
		0o644,
	); err != nil {
		t.Fatalf("failed to prepare access log: %v", err)
	}

	firstPayload := runner.nodePayload("node-observe")
	if firstPayload.Profile == nil {
		t.Fatal("expected first heartbeat payload to include system profile")
	}
	if firstPayload.Snapshot == nil {
		t.Fatal("expected first heartbeat payload to include metric snapshot")
	}
	if firstPayload.TrafficReport == nil || firstPayload.TrafficReport.RequestCount != 1 {
		t.Fatalf("expected first heartbeat payload to include traffic report, got %+v", firstPayload.TrafficReport)
	}
	if len(firstPayload.HealthEvents) != 2 {
		t.Fatalf("expected health events for openresty and sync error, got %+v", firstPayload.HealthEvents)
	}

	secondPayload := runner.nodePayload("node-observe")
	if secondPayload.Profile != nil {
		t.Fatal("expected unchanged profile to be omitted on subsequent heartbeat")
	}
	if secondPayload.Snapshot == nil {
		t.Fatal("expected metric snapshot to continue reporting on subsequent heartbeat")
	}
	if secondPayload.TrafficReport != nil {
		t.Fatalf("expected unchanged traffic window to be omitted on subsequent heartbeat, got %+v", secondPayload.TrafficReport)
	}
}

func TestRunnerReplaysBufferedObservabilityAfterHeartbeatRecovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	stateStore := state.NewStore(filepath.Join(tempDir, "state.json"))
	bufferStore := state.NewObservabilityBufferStore(filepath.Join(tempDir, "observability-buffer.json"))
	nowUnix := time.Now().UTC().Unix()
	bufferWindow := nowUnix - (nowUnix % 60) - 60
	if err := bufferStore.Upsert(state.ObservabilityBufferRecord{
		WindowStartedAtUnix: bufferWindow,
		Snapshot:            &protocol.NodeMetricSnapshot{CapturedAtUnix: bufferWindow + 5, CPUUsagePercent: 30},
		TrafficReport:       &protocol.NodeTrafficReport{WindowStartedAtUnix: bufferWindow, WindowEndedAtUnix: bufferWindow + 60, RequestCount: 8},
		QueuedAtUnix:        bufferWindow + 60,
	}, 0); err != nil {
		t.Fatalf("failed to seed observability buffer: %v", err)
	}
	heartbeatService := &fakeHeartbeatService{
		heartbeatErrs:    []error{errors.New("server offline"), nil},
		heartbeatResults: []*protocol.HeartbeatResult{{}, {}},
		onHeartbeat: func(callCount int) {
			if callCount >= 2 {
				cancel()
			}
		},
	}
	runner := &Runner{
		Config: &config.Config{
			AgentToken:                 "agent-token",
			NodeName:                   "edge-buffer-01",
			NodeIP:                     "10.0.0.52",
			AgentVersion:               config.AgentVersion,
			NginxVersion:               "1.27.1.2",
			DataDir:                    tempDir,
			RouteConfigPath:            filepath.Join(tempDir, "conf.d", "atsflare_routes.conf"),
			HeartbeatInterval:          config.MillisecondDuration(10 * time.Millisecond),
			ObservabilityReplayMinutes: 15,
		},
		StateStore:          stateStore,
		ObservabilityBuffer: bufferStore,
		HeartbeatService:    heartbeatService,
		SyncService:         &fakeSyncService{},
	}
	if err := os.MkdirAll(filepath.Dir(runner.Config.RouteConfigPath), 0o755); err != nil {
		t.Fatalf("failed to prepare route config dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(filepath.Dir(runner.Config.RouteConfigPath), "atsflare_access.log"),
		[]byte("{\"ts\":\""+time.Now().UTC().Format(time.RFC3339)+"\",\"host\":\"edge.example.com\",\"remote_addr\":\"10.0.0.8\",\"status\":200}\n"),
		0o644,
	); err != nil {
		t.Fatalf("failed to prepare access log: %v", err)
	}

	runErr := runner.Run(ctx)
	if runErr != context.Canceled {
		t.Fatalf("expected run to stop by context cancellation, got %v", runErr)
	}
	if len(heartbeatService.heartbeatPayloads) != 2 {
		t.Fatalf("expected two heartbeat payloads, got %d", len(heartbeatService.heartbeatPayloads))
	}
	secondPayload := heartbeatService.heartbeatPayloads[1]
	if len(secondPayload.BufferedObservability) != 1 {
		t.Fatalf("expected second heartbeat to replay one buffered observation, got %+v", secondPayload.BufferedObservability)
	}

	replayable, err := bufferStore.Replayable(0, 0)
	if err != nil {
		t.Fatalf("Replayable after recovery failed: %v", err)
	}
	if len(replayable) != 0 {
		t.Fatalf("expected buffer to be acked after successful heartbeat, got %+v", replayable)
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
