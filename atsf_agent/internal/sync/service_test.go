package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"atsflare-agent/internal/nginx"
	"atsflare-agent/internal/protocol"
	"atsflare-agent/internal/state"
)

type fakeExecutor struct {
	testErr   error
	reloadErr error
}

type fakeClient struct {
	config  protocol.ActiveConfigResponse
	reports []protocol.ApplyLogPayload
}

type fakeManager struct {
	applyErr           error
	currentChecksum    string
	currentChecksumErr error
	ensureCalls        []bool
	applyContents      []string
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

func (f *fakeClient) GetActiveConfig(ctx context.Context) (*protocol.ActiveConfigResponse, error) {
	return &f.config, nil
}

func (f *fakeClient) ReportApplyLog(ctx context.Context, payload protocol.ApplyLogPayload) error {
	f.reports = append(f.reports, payload)
	return nil
}

func (m *fakeManager) Apply(ctx context.Context, content string) error {
	m.applyContents = append(m.applyContents, content)
	return m.applyErr
}

func (m *fakeManager) EnsureRuntime(ctx context.Context, recreate bool) error {
	m.ensureCalls = append(m.ensureCalls, recreate)
	return nil
}

func (m *fakeManager) CurrentChecksum() (string, error) {
	return m.currentChecksum, m.currentChecksumErr
}

func TestSyncOnceSuccess(t *testing.T) {
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:        "20260309-001",
			Checksum:       "checksum-1",
			RenderedConfig: "server { listen 80; }",
			CreatedAt:      time.Now().Format(time.RFC3339),
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
		RouteConfigPath: routePath,
		Executor:        &fakeExecutor{},
	}, stateStore)

	if err = service.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce failed: %v", err)
	}

	data, err := os.ReadFile(routePath)
	if err != nil {
		t.Fatalf("failed to read route config: %v", err)
	}
	if string(data) != "server { listen 80; }" {
		t.Fatal("expected rendered config to be written to route file")
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
}

func TestSyncOnceRollbackOnNginxFailure(t *testing.T) {
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:        "20260309-002",
			Checksum:       "checksum-2",
			RenderedConfig: "server { listen 81; }",
			CreatedAt:      time.Now().Format(time.RFC3339),
		},
	}

	tempDir := t.TempDir()
	routePath := filepath.Join(tempDir, "routes.conf")
	if err := os.WriteFile(routePath, []byte("server { listen 80; }"), 0o644); err != nil {
		t.Fatalf("failed to seed route file: %v", err)
	}

	stateStore := state.NewStore(filepath.Join(tempDir, "state.json"))
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

	service := New(client, &nginx.Manager{
		RouteConfigPath: routePath,
		Executor: &fakeExecutor{
			testErr: context.DeadlineExceeded,
		},
	}, stateStore)

	err = service.SyncOnce(context.Background())
	if err == nil {
		t.Fatal("expected SyncOnce to fail when nginx test fails")
	}

	data, readErr := os.ReadFile(routePath)
	if readErr != nil {
		t.Fatalf("failed to read route file after rollback: %v", readErr)
	}
	if string(data) != "server { listen 80; }" {
		t.Fatal("expected original route config to be restored after rollback")
	}
	snapshot, loadErr := stateStore.Load()
	if loadErr != nil {
		t.Fatalf("failed to load state: %v", loadErr)
	}
	if snapshot.CurrentVersion != "20260309-001" {
		t.Fatal("expected failed sync not to overwrite current version")
	}
	if len(client.reports) != 1 || client.reports[0].Result != ApplyResultFailed {
		t.Fatal("expected failed apply report to be sent")
	}
}

func TestSyncOnStartupRecreatesRuntimeWhenChecksumMatches(t *testing.T) {
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:        "20260309-003",
			Checksum:       "checksum-3",
			RenderedConfig: "server { listen 82; }",
			CreatedAt:      time.Now().Format(time.RFC3339),
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
	if err = service.SyncOnStartup(context.Background()); err != nil {
		t.Fatalf("SyncOnStartup failed: %v", err)
	}
	if len(manager.ensureCalls) != 1 || !manager.ensureCalls[0] {
		t.Fatal("expected startup sync to recreate runtime")
	}
	if len(client.reports) != 0 {
		t.Fatal("expected no apply report when checksum already matches")
	}
	snapshot, err := stateStore.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if snapshot.CurrentChecksum != "checksum-3" || snapshot.CurrentVersion != "20260309-003" {
		t.Fatal("expected snapshot to be refreshed from active config")
	}
}
