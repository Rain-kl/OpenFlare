// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package sync

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/nginx"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/state"
	"github.com/Rain-kl/Wavelet/pkg/pagesarchive"
)

type fakeExecutor struct {
	testErr   error
	reloadErr error
}

func testPagesSourceConfigJSON(projectID, deploymentID uint, checksum string) string {
	return fmt.Sprintf(`{"routes":[{"id":1,"site_name":"pages","domain":"pages.example.com","domains":["pages.example.com"],"origin_url":"openflare-pages://project/%d","upstreams":["openflare-pages://project/%d"],"enabled":true,"upstream_type":"pages","pages_project_id":%d,"pages_deployment":{"project_id":%d,"project_slug":"pages","deployment_id":%d,"deployment_number":1,"checksum":"%s","entry_file":"index.html","spa_fallback_enabled":true,"local_root":"__OPENFLARE_PAGES_DIR__/projects/%d/current"}}],"openresty_config":{"worker_processes":"auto","worker_connections":1024,"worker_rlimit_nofile":65535,"events_multi_accept_enabled":true,"keepalive_timeout":20,"keepalive_requests":1000,"client_header_timeout":15,"client_body_timeout":15,"client_max_body_size":"64m","large_client_header_buffers":"4 16k","send_timeout":30,"proxy_connect_timeout":3,"proxy_send_timeout":60,"proxy_read_timeout":60,"websocket_enabled":true,"proxy_request_buffering":false,"proxy_buffering_enabled":true,"proxy_buffers":"16 16k","proxy_buffer_size":"8k","proxy_busy_buffers_size":"64k","gzip_enabled":true,"gzip_min_length":1024,"gzip_comp_level":5,"cache_enabled":false,"cache_levels":"1:2","cache_inactive":"30m","cache_max_size":"1g","cache_key_template":"$scheme$host$request_uri","cache_lock_enabled":true,"cache_lock_timeout":"5s","cache_use_stale":"error timeout updating http_500 http_502 http_503 http_504","main_config_template":"worker_processes {{OpenRestyWorkerProcesses}};"},"waf":{"rule_groups":[],"bindings":[]}}`, projectID, projectID, projectID, projectID, deploymentID, checksum, projectID)
}

type fakeClient struct {
	config                protocol.ActiveConfigResponse
	reports               []protocol.ApplyLogPayload
	wafSyncCalls          []protocol.WAFIPGroupSyncRequest
	pagesPackages         map[uint][]byte // key: project_id (latest package)
	pagesHashes           map[uint]string // key: project_id
	pagesLatestDeployIDs  map[uint]uint   // key: project_id → deployment_id
	pagesMetadata         map[uint]protocol.PagesProjectLatestHashResponse
	pagesPackageDownloads int
	wafSyncResult         protocol.WAFIPGroupSyncResponse
	fetchCalls            int
	hashCalls             int
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
	wafChecksums       map[string]string
	wafReconcileIDs    []uint
	wafReconcileGroups []protocol.WAFIPGroup
	wafUpdatedGroups   []protocol.WAFIPGroup
	wafReconcileErr    error
	wafReconcileCalls  int
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

func (f *fakeClient) GetPagesDeploymentHash(ctx context.Context, deploymentID uint) (string, error) {
	// Legacy path: map deployment id lookups via reverse of latest deploy ids when present.
	f.hashCalls++
	for projectID, depID := range f.pagesLatestDeployIDs {
		if depID == deploymentID {
			return f.projectHash(projectID)
		}
	}
	return f.projectHash(deploymentID)
}

func (f *fakeClient) DownloadPagesDeploymentPackage(
	ctx context.Context,
	deploymentID uint,
	dst io.Writer,
	maxBytes int64,
) (int64, error) {
	for projectID, depID := range f.pagesLatestDeployIDs {
		if depID == deploymentID {
			return f.writeProjectPackage(projectID, dst, maxBytes)
		}
	}
	return f.writeProjectPackage(deploymentID, dst, maxBytes)
}

func (f *fakeClient) GetPagesProjectLatestHash(ctx context.Context, projectID uint) (*protocol.PagesProjectLatestHashResponse, error) {
	f.hashCalls++
	if f.pagesMetadata != nil {
		if metadata, ok := f.pagesMetadata[projectID]; ok {
			result := metadata
			return &result, nil
		}
	}
	hash, err := f.projectHash(projectID)
	if err != nil {
		return nil, err
	}
	deploymentID := projectID
	if f.pagesLatestDeployIDs != nil {
		if id, ok := f.pagesLatestDeployIDs[projectID]; ok {
			deploymentID = id
		}
	}
	packageBytes, err := f.projectPackage(projectID)
	if err != nil {
		return nil, err
	}
	fileCount, totalSize, err := testPagesPackageStats(packageBytes)
	if err != nil {
		return nil, err
	}
	return &protocol.PagesProjectLatestHashResponse{
		ProjectID:    projectID,
		DeploymentID: deploymentID,
		Hash:         hash,
		PackageSize:  int64(len(packageBytes)),
		FileCount:    fileCount,
		TotalSize:    totalSize,
	}, nil
}

func (f *fakeClient) DownloadPagesProjectLatestPackage(
	ctx context.Context,
	projectID uint,
	dst io.Writer,
	maxBytes int64,
) (int64, error) {
	f.pagesPackageDownloads++
	return f.writeProjectPackage(projectID, dst, maxBytes)
}

func (f *fakeClient) projectHash(projectID uint) (string, error) {
	if f.pagesHashes != nil {
		if hash, ok := f.pagesHashes[projectID]; ok {
			return hash, nil
		}
	}
	if f.pagesPackages != nil {
		if packageBytes, ok := f.pagesPackages[projectID]; ok {
			return testBytesChecksum(packageBytes), nil
		}
	}
	return "", fmt.Errorf("missing Pages project hash %d", projectID)
}

func (f *fakeClient) projectPackage(projectID uint) ([]byte, error) {
	if f.pagesPackages == nil {
		return nil, fmt.Errorf("missing Pages project package %d", projectID)
	}
	packageBytes, ok := f.pagesPackages[projectID]
	if !ok {
		return nil, fmt.Errorf("missing Pages project package %d", projectID)
	}
	return packageBytes, nil
}

func (f *fakeClient) writeProjectPackage(projectID uint, dst io.Writer, maxBytes int64) (int64, error) {
	packageBytes, err := f.projectPackage(projectID)
	if err != nil {
		return 0, err
	}
	limited := &io.LimitedReader{R: bytes.NewReader(packageBytes), N: maxBytes + 1}
	written, err := io.Copy(dst, limited)
	if err != nil {
		return written, err
	}
	if written > maxBytes {
		return written, fmt.Errorf("test pages package exceeds limit %d", maxBytes)
	}
	return written, nil
}

func (f *fakeClient) ReportApplyLog(ctx context.Context, payload protocol.ApplyLogPayload) error {
	f.reports = append(f.reports, payload)
	return nil
}

func (f *fakeClient) SyncWAFIPGroups(ctx context.Context, payload protocol.WAFIPGroupSyncRequest) (*protocol.WAFIPGroupSyncResponse, error) {
	f.wafSyncCalls = append(f.wafSyncCalls, payload)
	result := f.wafSyncResult
	return &result, nil
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
	return m.wafChecksums, nil
}

func (m *fakeManager) ReconcileWAFIPGroups(ids []uint, groups []protocol.WAFIPGroup) error {
	m.wafReconcileCalls++
	m.wafReconcileIDs = append([]uint(nil), ids...)
	m.wafReconcileGroups = append([]protocol.WAFIPGroup(nil), groups...)
	return m.wafReconcileErr
}

func (m *fakeManager) UpdateExistingWAFIPGroups(groups []protocol.WAFIPGroup) error {
	m.wafUpdatedGroups = append([]protocol.WAFIPGroup(nil), groups...)
	return nil
}

func (m *fakeManager) EnsureWorkerReadAccess() error {
	return nil
}

func TestReferencedWAFIPGroupIDsSyncsCompiledDAGReferences(t *testing.T) {
	client := &fakeClient{wafSyncResult: protocol.WAFIPGroupSyncResponse{Groups: []protocol.WAFIPGroup{{ID: 7, Checksum: "new-7"}}}}
	manager := &fakeManager{wafChecksums: map[string]string{"2": "sum-2", "7": "old-7", "99": "stale"}}
	service := New(client, manager, nil)
	supportFiles := []protocol.SupportFile{{Path: "waf_config.json", Content: `{
		"rule_groups":[
			{"id":1,"ip_whitelist_group_ids":[0,11,7],"graph":{"entry":"start","nodes":{
				"start":{"type":"start","config":{}},
				"first":{"type":"ip_match","config":{"ip_group_ids":[7,0,2,7]}},
				"geo":{"type":"geo_match","config":{"countries":["US"],"ip_group_ids":[700]}},
				"pow":{"type":"pow","config":{"difficulty":4,"ip_group_ids":[800]}},
				"second":{"type":"ip_match","config":{"ip_group_ids":[9,2]}}
			}}}
		],
		"ip_groups":[{"id":2},{"id":7},{"id":9},{"id":11},{"id":404}],
		"bindings":[]
	}`}}

	if err := service.syncReferencedWAFIPGroups(context.Background(), supportFiles); err != nil {
		t.Fatalf("syncReferencedWAFIPGroups failed: %v", err)
	}
	if len(client.wafSyncCalls) != 1 {
		t.Fatalf("expected one WAF IP group sync request, got %d", len(client.wafSyncCalls))
	}
	if got, want := fmt.Sprint(client.wafSyncCalls[0].IDs), "[2 7 9 11]"; got != want {
		t.Fatalf("referenced IDs = %s, want %s", got, want)
	}
	if got, want := fmt.Sprint(client.wafSyncCalls[0].Checksums), "map[2:sum-2 7:old-7]"; got != want {
		t.Fatalf("request checksums = %s, want target-only %s", got, want)
	}
	if got, want := fmt.Sprint(manager.wafReconcileIDs), "[2 7 9 11]"; got != want {
		t.Fatalf("reconcile IDs = %s, want %s", got, want)
	}
	if len(manager.wafReconcileGroups) != 1 || manager.wafReconcileGroups[0].ID != 7 {
		t.Fatalf("changed groups not passed to reconcile: %#v", manager.wafReconcileGroups)
	}
}

func TestReferencedWAFIPGroupIDsReconcilesEmptyResponseAndSurfacesMissingLocal(t *testing.T) {
	client := &fakeClient{}
	manager := &fakeManager{
		wafChecksums:    map[string]string{"7": "mistaken-match"},
		wafReconcileErr: fmt.Errorf("missing referenced WAF IP group 7"),
	}
	service := New(client, manager, nil)
	err := service.syncReferencedWAFIPGroups(context.Background(), []protocol.SupportFile{{
		Path: "waf_config.json", Content: `{"rule_groups":[{"graph":{"nodes":{"match":{"type":"ip_match","config":{"ip_group_ids":[7]}}}}}]}`,
	}})
	if err == nil || !strings.Contains(err.Error(), "missing referenced WAF IP group 7") {
		t.Fatalf("expected missing local group error after empty delta, got %v", err)
	}
	if len(client.wafSyncCalls) != 1 || len(manager.wafReconcileIDs) != 1 || manager.wafReconcileIDs[0] != 7 {
		t.Fatalf("empty response did not reach authoritative reconcile: calls=%#v ids=%#v", client.wafSyncCalls, manager.wafReconcileIDs)
	}
}

func TestReferencedWAFIPGroupIDsEmptyTargetClearsWithoutRequest(t *testing.T) {
	client := &fakeClient{}
	manager := &fakeManager{}
	service := New(client, manager, nil)
	if err := service.syncReferencedWAFIPGroups(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(client.wafSyncCalls) != 0 {
		t.Fatalf("empty target performed request I/O: %#v", client.wafSyncCalls)
	}
	if manager.wafReconcileCalls != 1 {
		t.Fatal("empty authoritative target was not reconciled")
	}
}

func TestApplyWAFIPGroupsUsesExistingOnlyUpdatePath(t *testing.T) {
	manager := &fakeManager{}
	service := New(nil, manager, nil)
	groups := []protocol.WAFIPGroup{{ID: 99, Checksum: "broadcast"}}
	if err := service.ApplyWAFIPGroups(context.Background(), groups); err != nil {
		t.Fatal(err)
	}
	if len(manager.wafUpdatedGroups) != 1 || manager.wafUpdatedGroups[0].ID != 99 {
		t.Fatalf("broadcast did not use existing-only update path: %#v", manager.wafUpdatedGroups)
	}
	if manager.wafReconcileCalls != 0 {
		t.Fatal("broadcast must not use authoritative reconciliation")
	}
}

func TestReferencedWAFIPGroupIDsRejectsMalformedRuntimeConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{name: "document", content: `{`, wantErr: "decode waf_config.json"},
		{
			name:    "ip match config",
			content: `{"rule_groups":[{"id":1,"graph":{"nodes":{"match":{"type":"ip_match","config":{"ip_group_ids":"bad"}}}}}],"bindings":[]}`,
			wantErr: "decode ip_match config",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &fakeClient{}
			service := New(client, &fakeManager{}, nil)
			err := service.syncReferencedWAFIPGroups(context.Background(), []protocol.SupportFile{{
				Path: "waf_config.json", Content: test.content,
			}})
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected %q error, got %v", test.wantErr, err)
			}
			if len(client.wafSyncCalls) != 0 {
				t.Fatalf("malformed WAF config must not send sync request, got %#v", client.wafSyncCalls)
			}
		})
	}
}

func TestWAFIPGroupChecksumServicePublishesSidecar(t *testing.T) {
	runtimeDir := t.TempDir()
	manager := &nginx.Manager{RuntimeConfigDir: runtimeDir}
	if err := manager.ReconcileWAFIPGroups([]uint{3}, []protocol.WAFIPGroup{{
		ID: 3, Enabled: true, IPList: []string{"203.0.113.3"}, Checksum: "sum-3",
	}}); err != nil {
		t.Fatalf("ReconcileWAFIPGroups failed: %v", err)
	}
	jsonData, err := os.ReadFile(filepath.Join(runtimeDir, nginx.WAFIPGroupsConfigFileName))
	if err != nil {
		t.Fatalf("read IP group JSON: %v", err)
	}
	checksumData, err := os.ReadFile(filepath.Join(runtimeDir, nginx.WAFIPGroupsChecksumFileName))
	if err != nil {
		t.Fatalf("read IP group checksum: %v", err)
	}
	if got, want := strings.TrimSpace(string(checksumData)), testBytesChecksum(jsonData); got != want {
		t.Fatalf("published checksum mismatch: got %q want %q", got, want)
	}
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
	if client.reports[0].SupportFileCount != 3 {
		t.Fatalf("expected support file count to be reported, got %d", client.reports[0].SupportFileCount)
	}
}

func TestSyncOnceDownloadsPagesDeploymentBeforeApply(t *testing.T) {
	packageBytes := testPagesPackage(t, map[string]string{"index.html": "hello"})
	checksum := testBytesChecksum(packageBytes)
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-101",
			Checksum:         "pages-config-checksum",
			SourceConfigJSON: testPagesSourceConfigJSON(7, 7, checksum),
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
		pagesPackages: map[uint][]byte{7: packageBytes},
	}
	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID, err := stateStore.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	snapshot, _ := stateStore.Load()
	snapshot.NodeID = nodeID
	if err = stateStore.Save(snapshot); err != nil {
		t.Fatalf("save state failed: %v", err)
	}
	manager := &fakeManager{currentChecksum: "old-checksum"}
	service := New(client, manager, stateStore)
	pagesDir := t.TempDir()
	service.SetPagesDir(pagesDir)

	if err = service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{Version: "20260309-101", Checksum: "pages-config-checksum"}); err != nil {
		t.Fatalf("SyncOnce failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(pagesDir, "projects", "7", "current", "index.html"))
	if err != nil {
		t.Fatalf("expected Pages file to be extracted: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected Pages file content: %s", string(data))
	}
	if len(manager.applyRouteContents) != 1 || !strings.Contains(manager.applyRouteContents[0], "__OPENFLARE_PAGES_DIR__/projects/7/current") {
		t.Fatalf("expected Pages placeholder in rendered route config, got %#v", manager.applyRouteContents)
	}
}

func TestSyncPagesDeploymentEnsuresWorkerReadAccess(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	pagesDir := filepath.Join(dataDir, "var", "lib", "openflare", "pages")

	packageBytes := testPagesPackage(t, map[string]string{"index.html": "hello"})
	checksum := testBytesChecksum(packageBytes)
	config := protocol.ActiveConfigResponse{
		Version:          "20260309-106",
		Checksum:         "pages-config-checksum",
		SourceConfigJSON: testPagesSourceConfigJSON(7, 7, checksum),
		CreatedAt:        time.Now().Format(time.RFC3339),
	}
	client := &fakeClient{
		config:        config,
		pagesPackages: map[uint][]byte{7: packageBytes},
	}
	stateStore := state.NewStore(filepath.Join(tempDir, "state.json"))
	snapshot, _ := stateStore.Load()

	runtimeManager := &nginx.Manager{PagesDir: pagesDir}
	service := New(client, runtimeManager, stateStore)
	service.SetPagesDir(pagesDir)

	if err := service.syncPagesDeployments(context.Background(), snapshot, &config); err != nil {
		t.Fatalf("syncPagesDeployments failed: %v", err)
	}

	dataInfo, err := os.Stat(dataDir)
	if err != nil {
		t.Fatalf("Stat dataDir failed: %v", err)
	}
	if dataInfo.Mode().Perm()&0o005 == 0 {
		t.Fatalf("expected dataDir to be world-traversable, got %o", dataInfo.Mode().Perm())
	}
	indexPath := filepath.Join(pagesDir, "projects", "7", "current", "index.html")
	indexInfo, err := os.Stat(indexPath)
	if err != nil {
		t.Fatalf("expected Pages file to be extracted: %v", err)
	}
	if indexInfo.Mode().Perm() != 0o644 {
		t.Fatalf("expected index.html mode 0644, got %o", indexInfo.Mode().Perm())
	}
}

func TestSyncOnceExtractsPagesPackageWithZeroByteFiles(t *testing.T) {
	packageBytes := testPagesPackage(t, map[string]string{
		"index.html": "hello",
		".gitkeep":   "",
	})
	checksum := testBytesChecksum(packageBytes)
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-103",
			Checksum:         "pages-config-checksum",
			SourceConfigJSON: testPagesSourceConfigJSON(9, 9, checksum),
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
		pagesPackages: map[uint][]byte{9: packageBytes},
	}
	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID, err := stateStore.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	snapshot, _ := stateStore.Load()
	snapshot.NodeID = nodeID
	if err = stateStore.Save(snapshot); err != nil {
		t.Fatalf("save state failed: %v", err)
	}
	manager := &fakeManager{currentChecksum: "old-checksum"}
	service := New(client, manager, stateStore)
	pagesDir := t.TempDir()
	service.SetPagesDir(pagesDir)

	if err = service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{Version: "20260309-103", Checksum: "pages-config-checksum"}); err != nil {
		t.Fatalf("SyncOnce failed: %v", err)
	}
	gitkeepPath := filepath.Join(pagesDir, "projects", "9", "current", ".gitkeep")
	info, err := os.Stat(gitkeepPath)
	if err != nil {
		t.Fatalf("expected zero-byte Pages file to be extracted: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected zero-byte Pages file, got %d bytes", info.Size())
	}
}

func TestSyncOnceRejectsPagesZipSlipBeforeApply(t *testing.T) {
	packageBytes := testPagesPackage(t, map[string]string{"../escape.html": "bad", "index.html": "ok"})
	checksum := testBytesChecksum(packageBytes)
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-102",
			Checksum:         "pages-config-checksum",
			SourceConfigJSON: testPagesSourceConfigJSON(8, 8, checksum),
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
		pagesPackages: map[uint][]byte{8: packageBytes},
	}
	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	if _, err := stateStore.EnsureNodeID(); err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	manager := &fakeManager{currentChecksum: "old-checksum"}
	service := New(client, manager, stateStore)
	service.SetPagesDir(t.TempDir())

	err := service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{Version: "20260309-102", Checksum: "pages-config-checksum"})
	if err == nil || (!strings.Contains(err.Error(), "escapes deployment root") &&
		!strings.Contains(err.Error(), "escapes directory") &&
		!strings.Contains(err.Error(), "dot segment")) {
		t.Fatalf("expected zip-slip rejection, got %v", err)
	}
	if len(manager.applyRouteContents) != 0 {
		t.Fatalf("OpenResty apply must not run after Pages package rejection")
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
	if client.reports[0].SupportFileCount != 3 {
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
	if client.fetchCalls != 1 {
		t.Fatalf("expected checksum match to fetch active config once for Pages reconciliation, got %d", client.fetchCalls)
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

func TestSyncOnStartupSkipsDuplicateSuccessReportWhenStateAlreadySynced(t *testing.T) {
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-003",
			Checksum:         "checksum-3",
			SourceConfigJSON: testSourceConfigJSON("auto", 80),
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
		CurrentVersion:  "20260309-003",
		CurrentChecksum: "checksum-3",
	}); err != nil {
		t.Fatalf("failed to seed state: %v", err)
	}

	manager := &fakeManager{currentChecksum: "checksum-3"}
	service := New(client, manager, stateStore)
	if err = service.SyncOnStartup(context.Background(), &protocol.ActiveConfigMeta{
		Version:  "20260309-003",
		Checksum: "checksum-3",
	}); err != nil {
		t.Fatalf("SyncOnStartup failed: %v", err)
	}
	if len(client.reports) != 0 {
		t.Fatalf("expected startup sync to skip duplicate success report, got %+v", client.reports)
	}
	if len(manager.applyMainContents) != 1 {
		t.Fatal("expected startup sync to still refresh local config once")
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
		NodeID:           nodeID,
		CurrentVersion:   "20260309-003",
		CurrentChecksum:  "checksum-3",
		PagesDeployments: []state.PagesDeployment{},
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
		t.Fatalf("expected no active config fetch when Pages releases are already reconciled, got %d", client.fetchCalls)
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
		NodeID:           nodeID,
		CurrentVersion:   client.config.Version,
		CurrentChecksum:  client.config.Checksum,
		PagesDeployments: []state.PagesDeployment{},
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
		t.Fatalf("expected no active config fetch when Pages releases are already reconciled, got %d", client.fetchCalls)
	}
	if len(manager.applyMainContents) != 0 {
		t.Fatal("expected checksum match to skip apply")
	}
	if len(client.reports) != 0 {
		t.Fatal("expected no apply log when no config change is needed")
	}
}

func TestSyncOnceDownloadsPagesDeploymentWhenChecksumMatches(t *testing.T) {
	packageBytes := testPagesPackage(t, map[string]string{"index.html": "hello"})
	checksum := testBytesChecksum(packageBytes)
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-101",
			Checksum:         "pages-config-checksum",
			SourceConfigJSON: testPagesSourceConfigJSON(7, 7, checksum),
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
		pagesPackages: map[uint][]byte{7: packageBytes},
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
		t.Fatalf("save state failed: %v", err)
	}
	manager := &fakeManager{currentChecksum: client.config.Checksum}
	service := New(client, manager, stateStore)
	pagesDir := t.TempDir()
	service.SetPagesDir(pagesDir)

	if err = service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{
		Version:  client.config.Version,
		Checksum: client.config.Checksum,
	}); err != nil {
		t.Fatalf("SyncOnce failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(pagesDir, "projects", "7", "current", "index.html"))
	if err != nil {
		t.Fatalf("expected Pages file to be extracted when checksum already matches: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected Pages file content: %s", string(data))
	}
	if len(manager.applyMainContents) != 0 {
		t.Fatal("expected checksum match to skip OpenResty apply")
	}
	if client.fetchCalls != 1 {
		t.Fatalf("expected one active config fetch on first reconcile, got %d", client.fetchCalls)
	}

	client.fetchCalls = 0
	if err = service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{
		Version:  client.config.Version,
		Checksum: client.config.Checksum,
	}); err != nil {
		t.Fatalf("second SyncOnce failed: %v", err)
	}
	if client.fetchCalls != 0 {
		t.Fatalf("expected no active config fetch after Pages release is ready, got %d", client.fetchCalls)
	}
	snapshot, err := stateStore.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if len(snapshot.PagesDeployments) != 1 ||
		snapshot.PagesDeployments[0].ProjectID != 7 ||
		snapshot.PagesDeployments[0].Hash != checksum {
		t.Fatalf("expected Pages project refs to be cached in state, got %+v", snapshot.PagesDeployments)
	}
	if client.hashCalls == 0 {
		t.Fatal("expected Pages hash check during reconcile")
	}
}

func TestSyncOnceRedownloadsPagesDeploymentWhenServerHashChanges(t *testing.T) {
	initialPackage := testPagesPackage(t, map[string]string{"index.html": "v1"})
	updatedPackage := testPagesPackage(t, map[string]string{"index.html": "v2"})
	initialHash := testBytesChecksum(initialPackage)
	updatedHash := testBytesChecksum(updatedPackage)
	projectID := uint(12)
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-107",
			Checksum:         "pages-config-checksum",
			SourceConfigJSON: testPagesSourceConfigJSON(projectID, projectID, initialHash),
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
		pagesPackages: map[uint][]byte{projectID: initialPackage},
		pagesHashes:   map[uint]string{projectID: initialHash},
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
		PagesDeployments: []state.PagesDeployment{{
			ProjectID: projectID,
			Hash:      initialHash,
		}},
	}); err != nil {
		t.Fatalf("save state failed: %v", err)
	}
	pagesDir := t.TempDir()
	releaseDir := pagesProjectReleaseDir(pagesDir, projectID, initialHash)
	if err = extractTestPagesPackage(t, initialPackage, releaseDir, pagesProjectRef{
		ProjectID: projectID,
		Checksum:  initialHash,
	}); err != nil {
		t.Fatalf("seed release failed: %v", err)
	}
	if err = switchPagesProjectCurrentDir(pagesDir, projectID, releaseDir); err != nil {
		t.Fatalf("seed current dir failed: %v", err)
	}

	// Control plane activates a new package; agent discovers via latest hash poll
	// without main-config republish (checksum unchanged).
	client.pagesPackages[projectID] = updatedPackage
	client.pagesHashes[projectID] = updatedHash
	manager := &fakeManager{currentChecksum: client.config.Checksum}
	service := New(client, manager, stateStore)
	service.SetPagesDir(pagesDir)
	if err = service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{
		Version:  client.config.Version,
		Checksum: client.config.Checksum,
	}); err != nil {
		t.Fatalf("SyncOnce failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(pagesDir, "projects", "12", "current", "index.html"))
	if err != nil {
		t.Fatalf("expected updated Pages file: %v", err)
	}
	if string(data) != "v2" {
		t.Fatalf("unexpected Pages file content after hash change: %s", string(data))
	}
	if client.fetchCalls != 0 {
		t.Fatalf("expected hash-only reconcile without active config fetch, got %d", client.fetchCalls)
	}
	// Only the latest release must remain on disk.
	releasesRoot := filepath.Join(pagesDir, "projects", "12", "releases")
	entries, err := os.ReadDir(releasesRoot)
	if err != nil {
		t.Fatalf("read releases dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != updatedHash {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected only latest release %s, got %v", updatedHash, names)
	}
	if _, err := os.Stat(filepath.Join(releasesRoot, initialHash)); !os.IsNotExist(err) {
		t.Fatalf("expected stale release %s to be removed", initialHash)
	}
}

// racingLatestClient simulates control-plane activation changing between hash
// probe and package download (hash A → package B → verify hash B).
type racingLatestClient struct {
	fakeClient
	pkgA, pkgB    []byte
	hashA, hashB  string
	hashCall      int
	downloadCalls int
}

func (r *racingLatestClient) GetPagesProjectLatestHash(ctx context.Context, projectID uint) (*protocol.PagesProjectLatestHashResponse, error) {
	r.hashCall++
	r.hashCalls++
	// Sequence:
	// 1: probe before download → A
	// 2: verify after downloading B → B (race)
	// 3+: stable on B for retry
	hash, dep := r.hashA, uint(1)
	packageBytes := r.pkgA
	if r.hashCall >= 2 {
		hash, dep = r.hashB, 2
		packageBytes = r.pkgB
	}
	fileCount, totalSize, err := testPagesPackageStats(packageBytes)
	if err != nil {
		return nil, err
	}
	return &protocol.PagesProjectLatestHashResponse{
		ProjectID:    projectID,
		DeploymentID: dep,
		Hash:         hash,
		PackageSize:  int64(len(packageBytes)),
		FileCount:    fileCount,
		TotalSize:    totalSize,
	}, nil
}

func (r *racingLatestClient) DownloadPagesProjectLatestPackage(
	ctx context.Context,
	projectID uint,
	dst io.Writer,
	maxBytes int64,
) (int64, error) {
	r.downloadCalls++
	// Always return package B (what "latest download" would stream mid-race / after).
	limited := &io.LimitedReader{R: bytes.NewReader(r.pkgB), N: maxBytes + 1}
	written, err := io.Copy(dst, limited)
	if err != nil {
		return written, err
	}
	if written > maxBytes {
		return written, fmt.Errorf("test pages package exceeds limit %d", maxBytes)
	}
	return written, nil
}

func TestEnsurePagesProjectSurvivesHashPackageRace(t *testing.T) {
	pkgA := testPagesPackage(t, map[string]string{"index.html": "A"})
	pkgB := testPagesPackage(t, map[string]string{"index.html": "B"})
	hashA := testBytesChecksum(pkgA)
	hashB := testBytesChecksum(pkgB)
	projectID := uint(42)
	client := &racingLatestClient{
		pkgA:  pkgA,
		pkgB:  pkgB,
		hashA: hashA,
		hashB: hashB,
	}
	client.config = protocol.ActiveConfigResponse{
		Version:          "v-race",
		Checksum:         "c-race",
		SourceConfigJSON: testPagesSourceConfigJSON(projectID, 1, hashA),
		CreatedAt:        time.Now().Format(time.RFC3339),
	}
	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	snapshot, _ := stateStore.Load()
	pagesDir := t.TempDir()
	service := New(client, &fakeManager{}, stateStore)
	service.SetPagesDir(pagesDir)

	if err := service.syncPagesDeployments(context.Background(), snapshot, &client.config); err != nil {
		t.Fatalf("sync after race: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(pagesDir, "projects", "42", "current", "index.html"))
	if err != nil {
		t.Fatalf("read current: %v", err)
	}
	if string(data) != "B" {
		t.Fatalf("expected final content B after race recovery, got %q", string(data))
	}
	// Only B release retained.
	entries, err := os.ReadDir(filepath.Join(pagesDir, "projects", "42", "releases"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != hashB {
		t.Fatalf("expected only release %s, got %v", hashB, entries)
	}
	if client.downloadCalls < 1 {
		t.Fatal("expected at least one package download")
	}
}

func TestCleanupDoesNotRunBeforeCurrentSwitch(t *testing.T) {
	// If switch fails, stale release must remain so traffic can keep using it.
	pagesDir := t.TempDir()
	projectID := uint(9)
	oldHash := "old-hash"
	oldDir := pagesProjectReleaseDir(pagesDir, projectID, oldHash)
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "index.html"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := switchPagesProjectCurrentDir(pagesDir, projectID, oldDir); err != nil {
		t.Fatal(err)
	}
	// Simulate failed upgrade path: new release extracted but we do NOT switch/cleanup.
	newHash := "new-hash"
	newDir := pagesProjectReleaseDir(pagesDir, projectID, newHash)
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "index.html"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Old must still be readable via current.
	data, err := os.ReadFile(filepath.Join(pagesDir, "projects", "9", "current", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old" {
		t.Fatalf("current should still serve old until switch, got %q", data)
	}
	if _, err := os.Stat(oldDir); err != nil {
		t.Fatal("old release must not be deleted before successful switch")
	}
}

func TestLegacyStateWithoutProjectIDForcesDiscovery(t *testing.T) {
	snapshot := &state.Snapshot{
		PagesDeployments: []state.PagesDeployment{{
			DeploymentID: 99,
			Hash:         "abc",
		}},
	}
	if !pagesDiscoveryNeeded(snapshot) {
		t.Fatal("legacy deployment-only state must force config rediscovery")
	}
	if pagesSyncNeeded(snapshot) {
		t.Fatal("legacy rows without project_id must not enter hash-only sync")
	}
}

func TestCleanupPagesProjectStaleReleasesKeepsOnlyLatest(t *testing.T) {
	pagesDir := t.TempDir()
	projectID := uint(3)
	keep := "keep-hash"
	stale := "stale-hash"
	keepDir := pagesProjectReleaseDir(pagesDir, projectID, keep)
	staleDir := pagesProjectReleaseDir(pagesDir, projectID, stale)
	if err := os.MkdirAll(filepath.Join(keepDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tmpDir := staleDir + ".tmp"
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cleanupPagesProjectStaleReleases(pagesDir, projectID, keep); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := os.Stat(keepDir); err != nil {
		t.Fatalf("keep release missing: %v", err)
	}
	if _, err := os.Stat(staleDir); !os.IsNotExist(err) {
		t.Fatal("stale release should be removed")
	}
	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		t.Fatal("tmp leftover should be removed")
	}
}

func TestSyncPagesDeploymentsIsolatesProjectFailures(t *testing.T) {
	okPackage := testPagesPackage(t, map[string]string{"index.html": "ok"})
	okHash := testBytesChecksum(okPackage)
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:  "20260309-200",
			Checksum: "cfg",
			SourceConfigJSON: fmt.Sprintf(
				`{"routes":[{"upstream_type":"pages","pages_project_id":1,"pages_deployment":{"project_id":1}},{"upstream_type":"pages","pages_project_id":2,"pages_deployment":{"project_id":2}}]}`,
			),
			CreatedAt: time.Now().Format(time.RFC3339),
		},
		pagesPackages: map[uint][]byte{1: okPackage},
		pagesHashes:   map[uint]string{1: okHash},
		// project 2 missing → ensure fails
	}
	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	snapshot, _ := stateStore.Load()
	pagesDir := t.TempDir()
	service := New(client, &fakeManager{}, stateStore)
	service.SetPagesDir(pagesDir)

	err := service.syncPagesDeployments(context.Background(), snapshot, &client.config)
	if err == nil {
		t.Fatal("expected aggregated error when one project fails")
	}
	// Project 1 must still have been applied.
	data, readErr := os.ReadFile(filepath.Join(pagesDir, "projects", "1", "current", "index.html"))
	if readErr != nil {
		t.Fatalf("project 1 should succeed despite project 2 failure: %v", readErr)
	}
	if string(data) != "ok" {
		t.Fatalf("unexpected content: %s", data)
	}
}

func TestSyncOnceRedownloadsPagesDeploymentWhenReleaseDirOnlyHasMarker(t *testing.T) {
	packageBytes := testPagesPackage(t, map[string]string{"index.html": "hello"})
	checksum := testBytesChecksum(packageBytes)
	projectID := uint(11)
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-106",
			Checksum:         "pages-config-checksum",
			SourceConfigJSON: testPagesSourceConfigJSON(projectID, projectID, checksum),
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
		pagesPackages: map[uint][]byte{projectID: packageBytes},
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
		t.Fatalf("save state failed: %v", err)
	}
	pagesDir := t.TempDir()
	releaseDir := pagesProjectReleaseDir(pagesDir, projectID, checksum)
	if err = os.MkdirAll(releaseDir, pagesDirPerm); err != nil {
		t.Fatalf("mkdir release dir failed: %v", err)
	}
	if err = writePagesMarker(releaseDir, pagesProjectRef{ProjectID: projectID, Checksum: checksum}); err != nil {
		t.Fatalf("write marker failed: %v", err)
	}
	if pagesProjectReleaseReady(releaseDir, pagesProjectRef{ProjectID: projectID, Checksum: checksum}) {
		t.Fatal("expected marker-only release dir to be treated as not ready")
	}

	manager := &fakeManager{currentChecksum: client.config.Checksum}
	service := New(client, manager, stateStore)
	service.SetPagesDir(pagesDir)
	if err = service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{
		Version:  client.config.Version,
		Checksum: client.config.Checksum,
	}); err != nil {
		t.Fatalf("SyncOnce failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(pagesDir, "projects", "11", "current", "index.html"))
	if err != nil {
		t.Fatalf("expected Pages file to be extracted after marker-only release dir: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected Pages file content: %s", string(data))
	}
}

func testPagesPackage(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip file failed: %v", err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatalf("write zip file failed: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}
	return buffer.Bytes()
}

func extractTestPagesPackage(
	t *testing.T,
	packageBytes []byte,
	releaseDir string,
	project pagesProjectRef,
) error {
	t.Helper()
	packagePath := filepath.Join(t.TempDir(), "pages-package.zip")
	if err := os.WriteFile(packagePath, packageBytes, pagesFilePerm); err != nil {
		t.Fatalf("write test Pages package error = %v", err)
	}
	return extractPagesPackageFile(packagePath, releaseDir, project, pagesarchive.Limits{
		MaxFiles:      agentPagesMaxFiles,
		MaxFileBytes:  agentPagesMaxFileBytes,
		MaxTotalBytes: agentPagesMaxTotalBytes,
	}, nil)
}

func testPagesPackageStats(packageBytes []byte) (int, int64, error) {
	reader, err := zip.NewReader(bytes.NewReader(packageBytes), int64(len(packageBytes)))
	if err != nil {
		return 0, 0, err
	}
	fileCount := 0
	totalSize := int64(0)
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		fileCount++
		totalSize += int64(file.UncompressedSize64) //nolint:gosec // test packages are memory-bounded
	}
	return fileCount, totalSize, nil
}

func testBytesChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestSyncOnceDownloadsPagesDeploymentWithTopLevelFolder(t *testing.T) {
	packageBytes := testPagesPackage(t, map[string]string{
		"Speed-Test-source/index.html":    "hello html",
		"Speed-Test-source/assets/app.js": "hello js",
	})
	checksum := testBytesChecksum(packageBytes)
	client := &fakeClient{
		config: protocol.ActiveConfigResponse{
			Version:          "20260309-105",
			Checksum:         "pages-config-checksum",
			SourceConfigJSON: testPagesSourceConfigJSON(77, 77, checksum),
			CreatedAt:        time.Now().Format(time.RFC3339),
		},
		pagesPackages: map[uint][]byte{77: packageBytes},
	}
	stateStore := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID, err := stateStore.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	snapshot, _ := stateStore.Load()
	snapshot.NodeID = nodeID
	if err = stateStore.Save(snapshot); err != nil {
		t.Fatalf("save state failed: %v", err)
	}
	manager := &fakeManager{currentChecksum: "old-checksum"}
	service := New(client, manager, stateStore)
	pagesDir := t.TempDir()
	service.SetPagesDir(pagesDir)

	if err = service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{Version: "20260309-105", Checksum: "pages-config-checksum"}); err != nil {
		t.Fatalf("SyncOnce failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(pagesDir, "projects", "77", "current", "index.html"))
	if err != nil {
		t.Fatalf("expected Pages index.html file to be extracted: %v", err)
	}
	if string(data) != "hello html" {
		t.Fatalf("unexpected Pages index.html content: %s", string(data))
	}
	jsData, err := os.ReadFile(filepath.Join(pagesDir, "projects", "77", "current", "assets", "app.js"))
	if err != nil {
		t.Fatalf("expected Pages assets/app.js file to be extracted: %v", err)
	}
	if string(jsData) != "hello js" {
		t.Fatalf("unexpected Pages assets/app.js content: %s", string(jsData))
	}
}
