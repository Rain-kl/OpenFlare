package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"openflare-agent/internal/nginx"
	"openflare-agent/internal/protocol"
	"openflare-agent/internal/state"
)

type fakeExecutor struct {
	testErr   error
	reloadErr error
}

type fakeClient struct {
	config     protocol.ActiveConfigResponse
	reports    []protocol.ApplyLogPayload
	fetchCalls int
}

type fakeManager struct {
	applyOutcome       nginx.ApplyOutcome
	currentChecksum    string
	currentChecksumErr error
	ensureErr          error
	fallbackErr        error
	ensureCalls        []bool
	fallbackReasons    []string
	applyMainContents  []string
	applyRouteContents []string
	applyFiles         [][]protocol.SupportFile
}

func testSourceConfigJSON(workerProcesses string, listen int) string {
	return fmt.Sprintf(`{"routes":[{"id":1,"site_name":"example","domain":"example.com","domains":["example.com"],"origin_url":"http://127.0.0.1:%d","upstreams":["http://127.0.0.1:%d"],"enabled":true}],"openresty_config":{"worker_processes":"%s","worker_connections":1024,"worker_rlimit_nofile":65535,"events_multi_accept_enabled":true,"keepalive_timeout":20,"keepalive_requests":1000,"client_header_timeout":15,"client_body_timeout":15,"client_max_body_size":"64m","large_client_header_buffers":"4 16k","send_timeout":30,"proxy_connect_timeout":3,"proxy_send_timeout":60,"proxy_read_timeout":60,"websocket_enabled":true,"proxy_request_buffering":false,"proxy_buffering_enabled":true,"proxy_buffers":"16 16k","proxy_buffer_size":"8k","proxy_busy_buffers_size":"64k","gzip_enabled":true,"gzip_min_length":1024,"gzip_comp_level":5,"cache_enabled":false,"cache_levels":"1:2","cache_inactive":"30m","cache_max_size":"1g","cache_key_template":"$scheme$host$request_uri","cache_lock_enabled":true,"cache_lock_timeout":"5s","cache_use_stale":"error timeout updating http_500 http_502 http_503 http_504","main_config_template":"worker_processes {{OpenRestyWorkerProcesses}};"},"waf":{"rule_groups":[],"bindings":[]}}`, listen, listen, workerProcesses)
}

func (f *fakeExecutor) Test(ctx context.Context) error {
	return f.testErr
}

func (f *fakeExecutor) Reload(ctx context.Context) error {
	return f.reloadErr
}

func (f *fakeExecutor) EnsureRuntime(ctx context.Context, recreate bool) error {
	return nil
}

func (f *fakeExecutor) CheckHealth(ctx context.Context) error {
	return f.testErr
}

func (f *fakeExecutor) Restart(ctx context.Context) error {
	return f.reloadErr
}

func (f *fakeClient) GetActiveConfig(ctx context.Context) (*protocol.ActiveConfigResponse, error) {
	f.fetchCalls++
	return &f.config, nil
}

func (f *fakeClient) ReportApplyLog(ctx context.Context, payload protocol.ApplyLogPayload) error {
	f.reports = append(f.reports, payload)
	return nil
}

func (f *fakeClient) SyncWAFIPGroups(ctx context.Context, payload protocol.WAFIPGroupSyncRequest) (*protocol.WAFIPGroupSyncResponse, error) {
	return &protocol.WAFIPGroupSyncResponse{}, nil
}

func (m *fakeManager) Apply(ctx context.Context, mainConfig string, routeConfig string, supportFiles []protocol.SupportFile) nginx.ApplyOutcome {
	m.applyMainContents = append(m.applyMainContents, mainConfig)
	m.applyRouteContents = append(m.applyRouteContents, routeConfig)
	m.applyFiles = append(m.applyFiles, append([]protocol.SupportFile(nil), supportFiles...))
	if m.applyOutcome.Status == "" {
		return nginx.ApplyOutcome{Status: nginx.ApplyStatusSuccess}
	}
	return m.applyOutcome
}

func (m *fakeManager) EnsureRuntime(ctx context.Context, recreate bool) error {
	m.ensureCalls = append(m.ensureCalls, recreate)
	return m.ensureErr
}

func (m *fakeManager) EnsureSafeFallbackRuntime(ctx context.Context, reason string) error {
	m.fallbackReasons = append(m.fallbackReasons, reason)
	return m.fallbackErr
}

func (m *fakeManager) CurrentChecksum() (string, error) {
	return m.currentChecksum, m.currentChecksumErr
}

func (m *fakeManager) WAFIPGroupChecksums() (map[string]string, error) {
	return map[string]string{}, nil
}

func (m *fakeManager) SyncWAFIPGroups(groups []protocol.WAFIPGroup) error {
	return nil
}

func TestSyncOnceSuccess(t *testing.T) {
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-001",
			Checksum:         "checksum-1",
			SourceConfigJSON: testSourceConfigJSON("auto", 80),
			SupportFiles:     []protocol.SupportFile{{Path: "1.crt", Content: "cert"}},
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
	}

	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID, err := stateStore.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	snapshot, _ := stateStore.Load()
	snapshot.NodeID = nodeID
	if err = stateStore.Save(snapshot); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	routePath := filepath.Join(t.TempDir(), "routes.conf")
	service := New(client, &nginx.Manager{
		MainConfigPath:  filepath.Join(filepath.Dir(routePath), "nginx.conf"),
		RouteConfigPath: routePath,
		Executor:        &fakeExecutor{},
	}, stateStore)

	if err = service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{
		Version:  client.config.Version,
		Checksum: client.config.Checksum,
	}); err != nil {
		t.Fatalf("SyncOnce failed: %v", err)
	}

	data, err := os.ReadFile(routePath)
	if err != nil {
		t.Fatalf("failed to read route config: %v", err)
	}
	if !strings.Contains(string(data), "listen 80;") || !strings.Contains(string(data), "server_name example.com;") {
		t.Fatal("expected rendered config to be written to route file")
	}
	mainData, err := os.ReadFile(filepath.Join(filepath.Dir(routePath), "nginx.conf"))
	if err != nil {
		t.Fatalf("failed to read main config: %v", err)
	}
	if string(mainData) != "worker_processes auto;" {
		t.Fatal("expected main config to be written")
	}
	snapshot, err = stateStore.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if snapshot.CurrentVersion != "20260309-001" || snapshot.CurrentChecksum != "checksum-1" {
		t.Fatal("expected state store to persist current version and checksum")
	}
	if len(client.reports) != 1 || client.reports[0].Result != ApplyResultSuccess {
		t.Fatal("expected successful apply report to be sent")
	}
	if client.reports[0].Checksum != "checksum-1" {
		t.Fatalf("expected config checksum to be reported, got %q", client.reports[0].Checksum)
	}
	if client.reports[0].MainConfigChecksum == "" || client.reports[0].RouteConfigChecksum == "" {
		t.Fatal("expected main and route config checksums to be reported")
	}
	if client.reports[0].SupportFileCount != 4 {
		t.Fatalf("expected support file count to be reported, got %d", client.reports[0].SupportFileCount)
	}
}

func TestSyncOnceRollbackOnNginxFailure(t *testing.T) {
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-002",
			Checksum:         "checksum-2",
			SourceConfigJSON: testSourceConfigJSON("2", 81),
			SupportFiles:     []protocol.SupportFile{{Path: "1.crt", Content: "cert"}},
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
	}

	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID, err := stateStore.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	if err = stateStore.Save(&state.Snapshot{
		NodeID:          nodeID,
		CurrentVersion:  "20260309-001",
		CurrentChecksum: "checksum-1",
	}); err != nil {
		t.Fatalf("failed to seed state: %v", err)
	}

	service := New(client, &fakeManager{
		applyOutcome: nginx.ApplyOutcome{
			Status:  nginx.ApplyStatusFatal,
			Message: "openresty failed after rollback",
		},
	}, stateStore)

	err = service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{
		Version:  client.config.Version,
		Checksum: client.config.Checksum,
	})
	if err == nil {
		t.Fatal("expected SyncOnce to fail when apply outcome is fatal")
	}
	snapshot, loadErr := stateStore.Load()
	if loadErr != nil {
		t.Fatalf("failed to load state: %v", loadErr)
	}
	if snapshot.CurrentVersion != "20260309-001" {
		t.Fatal("expected failed sync not to overwrite current version")
	}
	if snapshot.BlockedVersion != "20260309-002" || snapshot.BlockedChecksum != "checksum-2" {
		t.Fatalf("expected failed target version to be blocked, got %+v", snapshot)
	}
	if snapshot.OpenrestyStatus != protocol.OpenrestyStatusUnhealthy {
		t.Fatalf("expected unhealthy openresty status, got %q", snapshot.OpenrestyStatus)
	}
	if len(client.reports) != 1 || client.reports[0].Result != ApplyResultFailed {
		t.Fatal("expected failed apply report to be sent")
	}
	if client.reports[0].Checksum != "checksum-2" {
		t.Fatalf("expected failed report to retain target checksum, got %q", client.reports[0].Checksum)
	}
	if client.reports[0].MainConfigChecksum == "" || client.reports[0].RouteConfigChecksum == "" {
		t.Fatal("expected failed report to include main and route config checksums")
	}
	if client.reports[0].SupportFileCount != 4 {
		t.Fatalf("expected failed report to include support file count, got %d", client.reports[0].SupportFileCount)
	}
}

func TestSyncOnceReportsWarningWhenRollbackKeepsOpenrestyHealthy(t *testing.T) {
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-002",
			Checksum:         "checksum-2",
			SourceConfigJSON: testSourceConfigJSON("2", 81),
			SupportFiles:     []protocol.SupportFile{{Path: "1.crt", Content: "cert"}},
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
	}

	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID, err := stateStore.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	if err = stateStore.Save(&state.Snapshot{
		NodeID:          nodeID,
		CurrentVersion:  "20260309-001",
		CurrentChecksum: "checksum-1",
	}); err != nil {
		t.Fatalf("failed to seed state: %v", err)
	}

	service := New(client, &fakeManager{
		applyOutcome: nginx.ApplyOutcome{
			Status:  nginx.ApplyStatusWarning,
			Message: "apply failed, rolled back to previous config",
		},
	}, stateStore)

	if err = service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{
		Version:  client.config.Version,
		Checksum: client.config.Checksum,
	}); err != nil {
		t.Fatalf("expected warning outcome to keep sync successful, got %v", err)
	}

	snapshot, err := stateStore.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if snapshot.CurrentVersion != "20260309-001" || snapshot.CurrentChecksum != "checksum-1" {
		t.Fatal("expected warning apply to keep previous version state")
	}
	if snapshot.BlockedVersion != "20260309-002" || snapshot.BlockedChecksum != "checksum-2" {
		t.Fatalf("expected rolled-back target version to be blocked, got %+v", snapshot)
	}
	if snapshot.OpenrestyStatus != protocol.OpenrestyStatusHealthy {
		t.Fatalf("expected healthy openresty after rollback, got %q", snapshot.OpenrestyStatus)
	}
	if snapshot.LastError == "" {
		t.Fatal("expected rollback warning to be recorded")
	}
	if len(client.reports) != 1 || client.reports[0].Result != ApplyResultWarning {
		t.Fatal("expected warning apply report to be sent")
	}
}

func TestSyncOnStartupRecreatesRuntimeWhenChecksumMatches(t *testing.T) {
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-003",
			Checksum:         "checksum-3",
			SourceConfigJSON: testSourceConfigJSON("auto", 82),
			SupportFiles:     []protocol.SupportFile{{Path: "1.crt", Content: "cert"}},
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
	}
	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID, err := stateStore.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	if err = stateStore.Save(&state.Snapshot{NodeID: nodeID}); err != nil {
		t.Fatalf("failed to seed state: %v", err)
	}

	manager := &fakeManager{currentChecksum: "checksum-3"}
	service := New(client, manager, stateStore)
	if err = service.SyncOnStartup(context.Background(), &protocol.ActiveConfigMeta{
		Version:  client.config.Version,
		Checksum: client.config.Checksum,
	}); err != nil {
		t.Fatalf("SyncOnStartup failed: %v", err)
	}
	if len(manager.applyMainContents) != 1 {
		t.Fatal("expected startup sync to re-render and apply local config")
	}
	if len(client.reports) != 1 || client.reports[0].Result != ApplyResultSuccess {
		t.Fatal("expected startup sync to report apply success when state is refreshed")
	}
	snapshot, err := stateStore.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if snapshot.CurrentChecksum != "checksum-3" || snapshot.CurrentVersion != "20260309-003" {
		t.Fatal("expected snapshot to be refreshed from active config")
	}
	if snapshot.OpenrestyStatus != protocol.OpenrestyStatusHealthy || snapshot.OpenrestyMessage != "" {
		t.Fatal("expected startup sync to mark openresty healthy")
	}
}

func TestSyncOnceReportsNoopWhenVersionChangesButChecksumMatches(t *testing.T) {
	client := &fakeClient{}
	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID, err := stateStore.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	if err = stateStore.Save(&state.Snapshot{
		NodeID:          nodeID,
		CurrentVersion:  "20260309-002",
		CurrentChecksum: "checksum-3",
	}); err != nil {
		t.Fatalf("failed to seed state: %v", err)
	}

	manager := &fakeManager{currentChecksum: "checksum-3"}
	service := New(client, manager, stateStore)
	if err = service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{
		Version:  "20260309-003",
		Checksum: "checksum-3",
	}); err != nil {
		t.Fatalf("SyncOnce failed: %v", err)
	}
	if client.fetchCalls != 0 {
		t.Fatalf("expected checksum match to skip config fetch, got %d", client.fetchCalls)
	}
	if len(manager.applyMainContents) != 0 {
		t.Fatal("expected checksum match to skip apply")
	}
	if len(client.reports) != 1 || client.reports[0].Result != ApplyResultSuccess {
		t.Fatalf("expected noop apply success report, got %+v", client.reports)
	}
	if client.reports[0].Version != "20260309-003" || client.reports[0].Checksum != "checksum-3" {
		t.Fatalf("unexpected noop apply report: %+v", client.reports[0])
	}
	snapshot, err := stateStore.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if snapshot.CurrentVersion != "20260309-003" || snapshot.CurrentChecksum != "checksum-3" {
		t.Fatalf("expected state to refresh active version, got %+v", snapshot)
	}
}

func TestSyncOnceDoesNotRepeatNoopReportWhenStateAlreadyMatches(t *testing.T) {
	client := &fakeClient{}
	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID, err := stateStore.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	if err = stateStore.Save(&state.Snapshot{
		NodeID:          nodeID,
		CurrentVersion:  "20260309-003",
		CurrentChecksum: "checksum-3",
	}); err != nil {
		t.Fatalf("failed to seed state: %v", err)
	}

	manager := &fakeManager{currentChecksum: "checksum-3"}
	service := New(client, manager, stateStore)
	if err = service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{
		Version:  "20260309-003",
		Checksum: "checksum-3",
	}); err != nil {
		t.Fatalf("SyncOnce failed: %v", err)
	}
	if len(client.reports) != 0 {
		t.Fatalf("expected matching state to skip duplicate noop report, got %+v", client.reports)
	}
}

func TestSyncOnStartupRecordsRuntimeFailure(t *testing.T) {
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-004",
			Checksum:         "checksum-4",
			SourceConfigJSON: testSourceConfigJSON("4", 83),
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
	}
	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID, err := stateStore.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	if err = stateStore.Save(&state.Snapshot{NodeID: nodeID}); err != nil {
		t.Fatalf("failed to seed state: %v", err)
	}

	manager := &fakeManager{
		currentChecksum: "checksum-4",
		applyOutcome:    nginx.ApplyOutcome{Status: nginx.ApplyStatusFatal, Message: context.DeadlineExceeded.Error()},
	}
	service := New(client, manager, stateStore)
	if err = service.SyncOnStartup(context.Background(), &protocol.ActiveConfigMeta{
		Version:  client.config.Version,
		Checksum: client.config.Checksum,
	}); err == nil {
		t.Fatal("expected SyncOnStartup to fail when runtime recreation fails")
	}
	snapshot, err := stateStore.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if snapshot.OpenrestyStatus != protocol.OpenrestyStatusUnhealthy {
		t.Fatalf("expected unhealthy openresty status, got %q", snapshot.OpenrestyStatus)
	}
	if snapshot.OpenrestyMessage == "" {
		t.Fatal("expected runtime error message to be recorded")
	}
}

func TestSyncOnceSkipsPreviouslyBlockedVersion(t *testing.T) {
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-006",
			Checksum:         "checksum-6",
			SourceConfigJSON: testSourceConfigJSON("6", 86),
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
	}
	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID, err := stateStore.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	if err = stateStore.Save(&state.Snapshot{
		NodeID:          nodeID,
		CurrentVersion:  "20260309-005",
		CurrentChecksum: "checksum-5",
		BlockedVersion:  "20260309-006",
		BlockedChecksum: "checksum-6",
		BlockedReason:   "apply failed, rolled back to previous config",
		LastError:       "apply failed, rolled back to previous config",
	}); err != nil {
		t.Fatalf("failed to seed state: %v", err)
	}

	manager := &fakeManager{currentChecksum: "checksum-5"}
	service := New(client, manager, stateStore)
	if err = service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{
		Version:  "20260309-006",
		Checksum: "checksum-6",
	}); err != nil {
		t.Fatalf("expected blocked version to be skipped, got %v", err)
	}
	if client.fetchCalls != 0 {
		t.Fatalf("expected blocked version to skip fetch, got %d", client.fetchCalls)
	}
	if len(manager.applyMainContents) != 0 {
		t.Fatal("expected blocked version to skip apply")
	}
	if len(client.reports) != 0 {
		t.Fatal("expected blocked version to skip reporting duplicate apply result")
	}
}

func TestSyncOnStartupKeepsBlockedVersionSuppressedUntilNewTargetArrives(t *testing.T) {
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-007",
			Checksum:         "checksum-7",
			SourceConfigJSON: testSourceConfigJSON("7", 87),
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
	}
	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID, err := stateStore.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	if err = stateStore.Save(&state.Snapshot{
		NodeID:           nodeID,
		CurrentVersion:   "20260309-005",
		CurrentChecksum:  "checksum-5",
		BlockedVersion:   "20260309-007",
		BlockedChecksum:  "checksum-7",
		BlockedReason:    "apply failed, rolled back to previous config",
		OpenrestyStatus:  protocol.OpenrestyStatusUnhealthy,
		OpenrestyMessage: "apply failed, rolled back to previous config",
		LastError:        "apply failed, rolled back to previous config",
	}); err != nil {
		t.Fatalf("failed to seed state: %v", err)
	}

	manager := &fakeManager{currentChecksum: "checksum-5"}
	service := New(client, manager, stateStore)
	if err = service.SyncOnStartup(context.Background(), &protocol.ActiveConfigMeta{
		Version:  "20260309-007",
		Checksum: "checksum-7",
	}); err != nil {
		t.Fatalf("expected blocked startup target to be skipped, got %v", err)
	}
	if len(manager.ensureCalls) != 1 || !manager.ensureCalls[0] {
		t.Fatal("expected startup skip to ensure runtime with current local config")
	}
	if client.fetchCalls != 0 {
		t.Fatalf("expected blocked startup target to skip fetch, got %d", client.fetchCalls)
	}
	if len(client.reports) != 0 {
		t.Fatal("expected blocked startup target to skip duplicate apply report")
	}
	snapshot, err := stateStore.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if snapshot.BlockedVersion != "20260309-007" || snapshot.BlockedChecksum != "checksum-7" {
		t.Fatalf("expected blocked target to remain recorded, got %+v", snapshot)
	}
	if snapshot.OpenrestyStatus != protocol.OpenrestyStatusHealthy {
		t.Fatalf("expected startup runtime recovery to mark openresty healthy, got %q", snapshot.OpenrestyStatus)
	}
}

func TestSyncOnStartupStartsFallbackWhenBlockedVersionHasNoLocalConfig(t *testing.T) {
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-007",
			Checksum:         "checksum-7",
			SourceConfigJSON: testSourceConfigJSON("7", 87),
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
	}
	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID, err := stateStore.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	if err = stateStore.Save(&state.Snapshot{
		NodeID:           nodeID,
		BlockedVersion:   "20260309-007",
		BlockedChecksum:  "checksum-7",
		BlockedReason:    "apply failed, but fallback runtime started",
		OpenrestyStatus:  protocol.OpenrestyStatusUnhealthy,
		OpenrestyMessage: "apply failed, but fallback runtime started",
		LastError:        "apply failed, but fallback runtime started",
	}); err != nil {
		t.Fatalf("failed to seed state: %v", err)
	}

	manager := &fakeManager{}
	service := New(client, manager, stateStore)
	if err = service.SyncOnStartup(context.Background(), &protocol.ActiveConfigMeta{
		Version:  "20260309-007",
		Checksum: "checksum-7",
	}); err != nil {
		t.Fatalf("expected blocked startup target to start fallback, got %v", err)
	}
	if len(manager.fallbackReasons) != 1 {
		t.Fatalf("expected fallback runtime to be started once, got %d", len(manager.fallbackReasons))
	}
	if client.fetchCalls != 0 {
		t.Fatalf("expected blocked startup target to skip fetch, got %d", client.fetchCalls)
	}
	if len(client.reports) != 0 {
		t.Fatal("expected blocked startup target to skip duplicate apply report")
	}
	snapshot, err := stateStore.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if snapshot.BlockedVersion != "20260309-007" || snapshot.BlockedChecksum != "checksum-7" {
		t.Fatalf("expected blocked target to remain recorded, got %+v", snapshot)
	}
	if snapshot.OpenrestyStatus != protocol.OpenrestyStatusHealthy {
		t.Fatalf("expected fallback startup recovery to mark openresty healthy, got %q", snapshot.OpenrestyStatus)
	}
	if snapshot.OpenrestyMessage != "safe default fallback runtime started" {
		t.Fatalf("expected fallback status message, got %q", snapshot.OpenrestyMessage)
	}
}

func TestSyncOnStartupStartsFallbackWhenResidualConfigCannotRecover(t *testing.T) {
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-007",
			Checksum:         "checksum-7",
			SourceConfigJSON: testSourceConfigJSON("7", 87),
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
	}
	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID, err := stateStore.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	if err = stateStore.Save(&state.Snapshot{
		NodeID:          nodeID,
		BlockedVersion:  "20260309-007",
		BlockedChecksum: "checksum-7",
		BlockedReason:   "apply failed, but fallback runtime started",
	}); err != nil {
		t.Fatalf("failed to seed state: %v", err)
	}

	manager := &fakeManager{
		currentChecksum: "residual-checksum",
		ensureErr:       context.DeadlineExceeded,
	}
	service := New(client, manager, stateStore)
	if err = service.SyncOnStartup(context.Background(), &protocol.ActiveConfigMeta{
		Version:  "20260309-007",
		Checksum: "checksum-7",
	}); err != nil {
		t.Fatalf("expected residual config failure to start fallback, got %v", err)
	}
	if len(manager.ensureCalls) != 1 {
		t.Fatalf("expected residual config to be tested once, got %d", len(manager.ensureCalls))
	}
	if len(manager.fallbackReasons) != 1 {
		t.Fatalf("expected fallback runtime to be started once, got %d", len(manager.fallbackReasons))
	}
	snapshot, err := stateStore.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if snapshot.OpenrestyStatus != protocol.OpenrestyStatusHealthy {
		t.Fatalf("expected fallback startup recovery to mark openresty healthy, got %q", snapshot.OpenrestyStatus)
	}
	if snapshot.BlockedVersion != "20260309-007" || snapshot.BlockedChecksum != "checksum-7" {
		t.Fatalf("expected blocked target to remain recorded, got %+v", snapshot)
	}
}

func TestSyncOnceClearsBlockedTargetWhenNewVersionArrives(t *testing.T) {
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-008",
			Checksum:         "checksum-8",
			SourceConfigJSON: testSourceConfigJSON("8", 88),
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
	}
	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID, err := stateStore.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	if err = stateStore.Save(&state.Snapshot{
		NodeID:          nodeID,
		CurrentVersion:  "20260309-005",
		CurrentChecksum: "checksum-5",
		BlockedVersion:  "20260309-007",
		BlockedChecksum: "checksum-7",
		BlockedReason:   "apply failed, rolled back to previous config",
	}); err != nil {
		t.Fatalf("failed to seed state: %v", err)
	}

	manager := &fakeManager{}
	service := New(client, manager, stateStore)
	if err = service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{
		Version:  "20260309-008",
		Checksum: "checksum-8",
	}); err != nil {
		t.Fatalf("expected new target version to be applied, got %v", err)
	}
	if client.fetchCalls != 1 {
		t.Fatalf("expected new target to trigger fetch, got %d", client.fetchCalls)
	}
	if len(manager.applyMainContents) != 1 {
		t.Fatal("expected new target to trigger apply")
	}
	snapshot, err := stateStore.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if snapshot.BlockedVersion != "" || snapshot.BlockedChecksum != "" {
		t.Fatalf("expected blocked target to be cleared after new version succeeds, got %+v", snapshot)
	}
	if snapshot.CurrentVersion != "20260309-008" || snapshot.CurrentChecksum != "checksum-8" {
		t.Fatalf("expected current version to move to new target, got %+v", snapshot)
	}
}

func TestSyncOnceSkipsFetchWhenHeartbeatChecksumMatches(t *testing.T) {
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-005",
			Checksum:         "checksum-5",
			SourceConfigJSON: testSourceConfigJSON("auto", 84),
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
	}
	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID, err := stateStore.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	if err = stateStore.Save(&state.Snapshot{
		NodeID:          nodeID,
		CurrentVersion:  client.config.Version,
		CurrentChecksum: client.config.Checksum,
	}); err != nil {
		t.Fatalf("failed to seed state: %v", err)
	}

	manager := &fakeManager{currentChecksum: client.config.Checksum}
	service := New(client, manager, stateStore)
	if err = service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{
		Version:  client.config.Version,
		Checksum: client.config.Checksum,
	}); err != nil {
		t.Fatalf("SyncOnce failed: %v", err)
	}
	if client.fetchCalls != 0 {
		t.Fatalf("expected no active config fetch when heartbeat checksum matches, got %d", client.fetchCalls)
	}
	if len(client.reports) != 0 {
		t.Fatal("expected no apply log when no config change is needed")
	}
}
