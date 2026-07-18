package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/config"
	agentheartbeat "github.com/Rain-kl/Wavelet/internal/apps/agent/heartbeat"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/state"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/updater"
)

func withHeartbeatCycle(runner *Runner, observabilityBuffer *state.ObservabilityBufferStore) *Runner {
	runner.HeartbeatCycle = &agentheartbeat.Cycle{
		Config:              runner.Config,
		StateStore:          runner.StateStore,
		ObservabilityBuffer: observabilityBuffer,
		Heartbeat:           runner.HeartbeatService,
		Sync:                runner.SyncService,
		Updater:             updater.New(),
	}
	return runner
}

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
	lastTarget     *protocol.ActiveConfigMeta
	onSyncOnceCall func(int)
	wafChecksums   map[string]string
	wafGroups      []protocol.WAFIPGroup
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
	if target != nil {
		copied := *target
		f.lastTarget = &copied
	}
	callIndex := f.syncOnceCalls
	callback := f.onSyncOnceCall
	f.mu.Unlock()
	if callback != nil {
		callback(callIndex)
	}
	return f.syncOnceErr
}

func (f *fakeSyncService) ForceSyncOnce(ctx context.Context, target *protocol.ActiveConfigMeta) error {
	f.mu.Lock()
	f.syncOnceCalls++
	if target != nil {
		copied := *target
		f.lastTarget = &copied
	}
	callIndex := f.syncOnceCalls
	callback := f.onSyncOnceCall
	f.mu.Unlock()
	if callback != nil {
		callback(callIndex)
	}
	return f.syncOnceErr
}

func (f *fakeSyncService) WAFIPGroupChecksums() (map[string]string, error) {
	if f.wafChecksums == nil {
		return map[string]string{}, nil
	}
	return f.wafChecksums, nil
}

func (f *fakeSyncService) ApplyWAFIPGroups(ctx context.Context, groups []protocol.WAFIPGroup) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.wafGroups = append(f.wafGroups, groups...)
	return nil
}

type fakeWebSocketConnection struct {
	pongCalls int
}

func (f *fakeWebSocketConnection) URL() string {
	return "ws://127.0.0.1/api/v1/agent/ws"
}

func (f *fakeWebSocketConnection) SendStatus(payload protocol.NodePayload) error {
	return nil
}

func (f *fakeWebSocketConnection) SendPong() error {
	f.pongCalls++
	return nil
}

func (f *fakeWebSocketConnection) Receive() (protocol.WSMessage, error) {
	return protocol.WSMessage{}, errors.New("not implemented")
}

func (f *fakeWebSocketConnection) Close() error {
	return nil
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
	runner := withHeartbeatCycle(&Runner{
		Config: &config.Config{
			AccessToken:       "agent-token",
			NodeName:          "edge-01",
			NodeIP:            "10.0.0.8",
			NodeIPConfigured:  true,
			Version:           config.Version,
			ExtVersion:        "1.27.1.2",
			HeartbeatInterval: config.MillisecondDuration(10 * time.Millisecond),
		},
		StateStore:       stateStore,
		HeartbeatService: heartbeatService,
		SyncService:      syncService,
	}, nil)

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
	runner := withHeartbeatCycle(&Runner{
		Config: &config.Config{
			AccessToken:       "agent-token",
			NodeName:          "edge-01",
			NodeIP:            "10.0.0.8",
			NodeIPConfigured:  true,
			Version:           config.Version,
			ExtVersion:        "1.27.1.2",
			HeartbeatInterval: config.MillisecondDuration(10 * time.Millisecond),
		},
		StateStore:       stateStore,
		HeartbeatService: heartbeatService,
		SyncService:      syncService,
	}, nil)

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
	runner := withHeartbeatCycle(&Runner{
		Config: &config.Config{
			AccessToken:       "agent-token",
			NodeName:          "edge-01",
			NodeIP:            "10.0.0.8",
			NodeIPConfigured:  true,
			Version:           config.Version,
			ExtVersion:        "1.27.1.2",
			HeartbeatInterval: config.MillisecondDuration(10 * time.Millisecond),
		},
		StateStore:       stateStore,
		HeartbeatService: heartbeatService,
		SyncService:      &fakeSyncService{},
		RuntimeManager:   runtimeManager,
	}, nil)

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

	runner := withHeartbeatCycle(&Runner{
		Config: &config.Config{
			NodeName:          "edge-observe-1",
			NodeIP:            "10.0.0.51",
			NodeIPConfigured:  true,
			Version:           config.Version,
			ExtVersion:        "1.27.1.2",
			DataDir:           tempDir,
			RouteConfigPath:   filepath.Join(tempDir, "conf.d", "openflare_routes.conf"),
			AccessLogPath:     filepath.Join(tempDir, "var", "log", "openflare", "access.log"),
			HeartbeatInterval: config.MillisecondDuration(10 * time.Millisecond),
		},
		StateStore: stateStore,
	}, nil)
	if err := os.MkdirAll(filepath.Dir(runner.Config.AccessLogPath), 0o755); err != nil {
		t.Fatalf("failed to prepare access log dir: %v", err)
	}
	if err := os.WriteFile(
		runner.Config.AccessLogPath,
		[]byte("{\"ts\":\""+time.Now().UTC().Format(time.RFC3339)+"\",\"host\":\"edge.example.com\",\"path\":\"/\",\"remote_addr\":\"10.0.0.8\",\"status\":200}\n"),
		0o644,
	); err != nil {
		t.Fatalf("failed to prepare access log: %v", err)
	}

	firstPayload := runner.HeartbeatCycle.NodePayload(context.Background(), "node-observe")
	if firstPayload.Profile == nil {
		t.Fatal("expected first heartbeat payload to include system profile")
	}
	if firstPayload.HostMetrics == nil {
		t.Fatal("expected first heartbeat payload to include host metrics")
	}
	if firstPayload.SchemaVersion != 2 {
		t.Fatalf("expected schema_version 2, got %d", firstPayload.SchemaVersion)
	}
	if len(firstPayload.AccessLogs) != 1 || firstPayload.AccessLogs[0].Path != "/" {
		t.Fatalf("expected first heartbeat payload to include access logs, got %+v", firstPayload.AccessLogs)
	}
	if len(firstPayload.HealthEvents) != 2 {
		t.Fatalf("expected health events for openresty and sync error, got %+v", firstPayload.HealthEvents)
	}

	secondPayload := runner.HeartbeatCycle.NodePayload(context.Background(), "node-observe")
	if secondPayload.Profile != nil {
		t.Fatal("expected unchanged profile to be omitted on subsequent heartbeat")
	}
	if secondPayload.HostMetrics == nil {
		t.Fatal("expected host metrics to continue reporting on subsequent heartbeat")
	}
	if len(secondPayload.AccessLogs) != 0 {
		t.Fatalf("expected unchanged access log delta to be omitted on subsequent heartbeat, got %+v", secondPayload.AccessLogs)
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
		HostMetrics:         &protocol.NodeMetricSnapshot{CapturedAtUnix: bufferWindow + 5, CPUUsagePercent: 30},
		EdgeHealth:          &protocol.NodeEdgeHealth{CapturedAtUnix: bufferWindow + 5, Connections: 3, Status: "healthy"},
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
	runner := withHeartbeatCycle(&Runner{
		Config: &config.Config{
			AccessToken:                "agent-token",
			NodeName:                   "edge-buffer-01",
			NodeIP:                     "10.0.0.52",
			NodeIPConfigured:           true,
			Version:                    config.Version,
			ExtVersion:                 "1.27.1.2",
			DataDir:                    tempDir,
			RouteConfigPath:            filepath.Join(tempDir, "conf.d", "openflare_routes.conf"),
			HeartbeatInterval:          config.MillisecondDuration(10 * time.Millisecond),
			ObservabilityReplayMinutes: 15,
		},
		StateStore:       stateStore,
		HeartbeatService: heartbeatService,
		SyncService:      &fakeSyncService{},
	}, bufferStore)
	if err := os.MkdirAll(filepath.Dir(runner.Config.RouteConfigPath), 0o755); err != nil {
		t.Fatalf("failed to prepare route config dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(filepath.Dir(runner.Config.RouteConfigPath), "openflare_access.log"),
		[]byte("{\"ts\":\""+time.Now().UTC().Format(time.RFC3339)+"\",\"host\":\"edge.example.com\",\"path\":\"/\",\"remote_addr\":\"10.0.0.8\",\"status\":200}\n"),
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
	if len(secondPayload.Buffered) != 1 {
		t.Fatalf("expected second heartbeat to replay one buffered observation, got %+v", secondPayload.Buffered)
	}
	if len(secondPayload.Buffered[0].AccessLogs) != 0 {
		t.Fatalf("expected seeded buffered observation to keep empty access logs, got %+v", secondPayload.Buffered[0].AccessLogs)
	}
	if secondPayload.Buffered[0].EdgeHealth == nil || secondPayload.Buffered[0].EdgeHealth.Connections != 3 {
		t.Fatalf("expected buffered edge health, got %+v", secondPayload.Buffered[0].EdgeHealth)
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
			NodeID:      "node-server-assigned",
			AccessToken: "agent-token-issued",
			Name:        "edge-01",
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
	runner := withHeartbeatCycle(&Runner{
		Config: &config.Config{
			ServerURL:         cfg.ServerURL,
			DiscoveryToken:    cfg.DiscoveryToken,
			NodeName:          cfg.NodeName,
			NodeIP:            cfg.NodeIP,
			NodeIPConfigured:  cfg.NodeIPConfigured,
			Version:           config.Version,
			ExtVersion:        "1.27.1.2",
			HeartbeatInterval: config.MillisecondDuration(10 * time.Millisecond),
		},
		StateStore:       stateStore,
		HeartbeatService: heartbeatService,
		SyncService:      syncService,
	}, nil)
	runner.Config = cfg
	runner.Config.Version = config.Version
	runner.Config.ExtVersion = "1.27.1.2"
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
	if runner.Config.AccessToken != "agent-token-issued" || runner.Config.DiscoveryToken != "" {
		t.Fatal("expected config token rotation to complete")
	}
}

func TestRunnerHandlesWebSocketActiveConfigMessage(t *testing.T) {
	syncService := &fakeSyncService{}
	runner := withHeartbeatCycle(&Runner{SyncService: syncService}, nil)
	payload, err := json.Marshal(protocol.ActiveConfigMeta{
		Version:  "20260529-001",
		Checksum: "checksum-ws",
	})
	if err != nil {
		t.Fatalf("marshal active config: %v", err)
	}

	changed, err := runner.handleWebSocketMessage(context.Background(), protocol.WSMessage{
		Type:    protocol.WSMessageTypeActiveConfig,
		Payload: payload,
	}, &fakeWebSocketConnection{})
	if err != nil {
		t.Fatalf("handle websocket active config: %v", err)
	}
	if changed {
		t.Fatal("active config message should not change heartbeat interval")
	}
	if syncService.syncOnceCalls != 1 {
		t.Fatalf("expected one sync call, got %d", syncService.syncOnceCalls)
	}
	if syncService.lastTarget == nil || syncService.lastTarget.Version != "20260529-001" || syncService.lastTarget.Checksum != "checksum-ws" {
		t.Fatalf("unexpected sync target: %+v", syncService.lastTarget)
	}
}

func TestRunnerHandlesWebSocketSettingsDisabled(t *testing.T) {
	runner := withHeartbeatCycle(&Runner{
		Config: &config.Config{
			HeartbeatInterval: config.MillisecondDuration(10 * time.Second),
		},
		websocketUpgradeEnabled: true,
	}, nil)
	payload, err := json.Marshal(protocol.AgentSettings{
		HeartbeatInterval:       15000,
		WebsocketUpgradeEnabled: false,
	})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}

	changed, err := runner.handleWebSocketMessage(context.Background(), protocol.WSMessage{
		Type:    protocol.WSMessageTypeSettings,
		Payload: payload,
	}, &fakeWebSocketConnection{})
	if err == nil {
		t.Fatal("expected disabled websocket setting to request fallback")
	}
	if !changed {
		t.Fatal("expected heartbeat interval change to be reported")
	}
	if runner.websocketUpgradeEnabled {
		t.Fatal("expected websocket upgrade to be disabled")
	}
}

func TestWebSocketBackoffSequence(t *testing.T) {
	backoff := newWebSocketBackoff()
	expected := []time.Duration{
		time.Second,
		2 * time.Second,
		5 * time.Second,
		10 * time.Second,
		30 * time.Second,
		30 * time.Second,
	}
	for _, want := range expected {
		if got := backoff.Next(); got != want {
			t.Fatalf("unexpected backoff: got %s want %s", got, want)
		}
	}
	backoff.Reset()
	if got := backoff.Next(); got != time.Second {
		t.Fatalf("expected reset backoff to return 1s, got %s", got)
	}
}
