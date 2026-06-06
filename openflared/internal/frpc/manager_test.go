package frpc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rain-kl/openflare/openflare-server/service"
	"github.com/rain-kl/openflare/openflared/internal/config"
)

// Helper to write control file for the dummy script
func writeControl(t *testing.T, dir string, exitCode int, delaySeconds int) {
	controlPath := filepath.Join(dir, "control.txt")
	content := fmt.Sprintf("%d %d\n", exitCode, delaySeconds)
	err := os.WriteFile(controlPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write control file: %v", err)
	}
}

// Setup a dummy executable script that reads control.txt to decide exit code and sleep duration
func setupDummyScript(t *testing.T) (string, string) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "dummy_frpc")

	// On macOS/Linux, we write a shell script
	scriptContent := fmt.Sprintf(`#!/bin/sh
control_file="%s/control.txt"
EXIT_CODE=0
DELAY=0
if [ -f "$control_file" ]; then
    read -r EXIT_CODE DELAY < "$control_file"
fi
if [ -n "$DELAY" ] && [ "$DELAY" -gt 0 ] 2>/dev/null; then
    sleep "$DELAY"
fi
exit "${EXIT_CODE:-0}"
`, dir)

	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		t.Fatalf("failed to write dummy script: %v", err)
	}

	return scriptPath, dir
}

// Helper to poll for status to eliminate timing flakiness in tests
func assertStatusEventually(t *testing.T, m *Manager, relayID string, expectedStatus string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		m.mu.RLock()
		proc, ok := m.processes[relayID]
		m.mu.RUnlock()
		if ok && proc.Status == expectedStatus {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	m.mu.RLock()
	proc, ok := m.processes[relayID]
	var got string
	var errStr string
	if ok {
		got = proc.Status
		errStr = proc.LastError
	} else {
		got = "not_found"
	}
	m.mu.RUnlock()
	t.Fatalf("expected status eventually %s, got %s (err: %s)", expectedStatus, got, errStr)
}

func assertCommandExitedEventually(t *testing.T, cmd *exec.Cmd, timeout time.Duration) {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(timeout):
		t.Fatalf("expected process pid=%d to exit within %s", cmd.Process.Pid, timeout)
	case <-done:
	}
}

func TestStartProcessSuccess(t *testing.T) {
	scriptPath, dir := setupDummyScript(t)
	writeControl(t, dir, 0, 5) // exit code 0, sleep 5s

	cfg := &config.Config{
		ServerURL:   "http://localhost:8080",
		TunnelToken: "test-token",
		FrpcPath:    scriptPath,
		DataDir:     dir,
		StatePath:   filepath.Join(dir, "flared-state.json"),
	}

	m := NewManager(cfg)

	newConfig := &service.FlaredTunnelConfigResponse{
		Version:  "1",
		Checksum: "sum1",
		Relays: []service.FlaredRelayInfo{
			{
				RelayNodeID: "relay-1",
				Address:     "127.0.0.1:7000",
				AuthToken:   "auth-1",
			},
		},
		Proxies: nil,
	}

	err := m.UpdateConfig(context.Background(), newConfig)
	if err != nil {
		t.Fatalf("failed to UpdateConfig: %v", err)
	}

	assertStatusEventually(t, m, "relay-1", "running", 4*time.Second)

	m.mu.RLock()
	proc := m.processes["relay-1"]
	m.mu.RUnlock()

	proc.Cancel()
	assertStatusEventually(t, m, "relay-1", "stopped", 4*time.Second) // wait for clean stop
}

func TestStartProcessFailureAndBackoff(t *testing.T) {
	dir := t.TempDir()
	invalidScriptPath := filepath.Join(dir, "non_existent_frpc")

	cfg := &config.Config{
		ServerURL:   "http://localhost:8080",
		TunnelToken: "test-token",
		FrpcPath:    invalidScriptPath,
		DataDir:     dir,
		StatePath:   filepath.Join(dir, "flared-state.json"),
	}

	m := NewManager(cfg)
	newConfig := &service.FlaredTunnelConfigResponse{
		Version:  "1",
		Checksum: "sum1",
		Relays: []service.FlaredRelayInfo{
			{
				RelayNodeID: "relay-1",
				Address:     "127.0.0.1:7000",
				AuthToken:   "auth-1",
			},
		},
		Proxies: nil,
	}

	_ = m.UpdateConfig(context.Background(), newConfig)

	assertStatusEventually(t, m, "relay-1", "error", 4*time.Second)

	// Correct the path to dummy script
	scriptPath, _ := setupDummyScript(t)
	writeControl(t, filepath.Dir(scriptPath), 0, 5)

	m.mu.Lock()
	m.cfg.FrpcPath = scriptPath
	m.mu.Unlock()

	// Wait for backoff retry (1s backoff)
	assertStatusEventually(t, m, "relay-1", "running", 4*time.Second)

	m.mu.RLock()
	proc := m.processes["relay-1"]
	m.mu.RUnlock()
	proc.Cancel()
}

func TestUnexpectedExit0CPUProtection(t *testing.T) {
	scriptPath, dir := setupDummyScript(t)
	// Start with immediate exit code 0
	writeControl(t, dir, 0, 0)

	cfg := &config.Config{
		ServerURL:   "http://localhost:8080",
		TunnelToken: "test-token",
		FrpcPath:    scriptPath,
		DataDir:     dir,
		StatePath:   filepath.Join(dir, "flared-state.json"),
	}

	m := NewManager(cfg)
	newConfig := &service.FlaredTunnelConfigResponse{
		Version:  "1",
		Checksum: "sum1",
		Relays: []service.FlaredRelayInfo{
			{
				RelayNodeID: "relay-1",
				Address:     "127.0.0.1:7000",
				AuthToken:   "auth-1",
			},
		},
		Proxies: nil,
	}

	_ = m.UpdateConfig(context.Background(), newConfig)

	assertStatusEventually(t, m, "relay-1", "stopped", 4*time.Second)

	m.mu.RLock()
	proc := m.processes["relay-1"]
	if !strings.Contains(proc.LastError, "exited unexpectedly with code 0") {
		t.Errorf("expected LastError to record exit status 0 warning, got %s", proc.LastError)
	}
	m.mu.RUnlock()

	proc.Cancel()
}

func TestBackoffReset(t *testing.T) {
	scriptPath, dir := setupDummyScript(t)
	// Rapid exit code 1 to increase backoff
	writeControl(t, dir, 1, 0)

	cfg := &config.Config{
		ServerURL:   "http://localhost:8080",
		TunnelToken: "test-token",
		FrpcPath:    scriptPath,
		DataDir:     dir,
		StatePath:   filepath.Join(dir, "flared-state.json"),
	}

	m := NewManager(cfg)
	newConfig := &service.FlaredTunnelConfigResponse{
		Version:  "1",
		Checksum: "sum1",
		Relays: []service.FlaredRelayInfo{
			{
				RelayNodeID: "relay-1",
				Address:     "127.0.0.1:7000",
				AuthToken:   "auth-1",
			},
		},
		Proxies: nil,
	}

	_ = m.UpdateConfig(context.Background(), newConfig)

	// Wait to crash
	assertStatusEventually(t, m, "relay-1", "error", 4*time.Second)

	// Now make it run successfully for 11 seconds (exit code 0, sleep 11s)
	writeControl(t, dir, 0, 11)

	// Wait for next retry to start running
	assertStatusEventually(t, m, "relay-1", "running", 4*time.Second)

	// Wait for process to run for 10.5 seconds to trigger backoff reset
	time.Sleep(10500 * time.Millisecond)

	// Now make it crash again (exit code 1, sleep 0s)
	writeControl(t, dir, 1, 0)

	// Wait for it to finish and crash
	assertStatusEventually(t, m, "relay-1", "error", 4*time.Second)

	// It crashed. Since it ran for > 10s, backoff should have been reset to 1s.
	// We make it healthy again (exit code 0, sleep 5)
	writeControl(t, dir, 0, 5)

	// Wait 1.5 seconds. If backoff was reset to 1s, it should be running now.
	assertStatusEventually(t, m, "relay-1", "running", 4*time.Second)

	m.mu.RLock()
	proc := m.processes["relay-1"]
	m.mu.RUnlock()
	proc.Cancel()
}

func TestUpdateConfigKillsOrphanProcessBeforeRestart(t *testing.T) {
	scriptPath, dir := setupDummyScript(t)
	writeControl(t, dir, 0, 5)

	cfg := &config.Config{
		ServerURL:   "http://localhost:8080",
		TunnelToken: "test-token",
		FrpcPath:    scriptPath,
		DataDir:     dir,
		StatePath:   filepath.Join(dir, "flared-state.json"),
	}

	m := NewManager(cfg)

	orphan := exec.Command("sh", "-c", "sleep 30")
	if err := orphan.Start(); err != nil {
		t.Fatalf("failed to start orphan process: %v", err)
	}
	t.Cleanup(func() {
		if orphan.Process != nil {
			_ = orphan.Process.Kill()
		}
	})

	pidPath := filepath.Join(dir, "frpc_relay-1.pid")
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", orphan.Process.Pid)), 0o644); err != nil {
		t.Fatalf("failed to seed orphan pid file: %v", err)
	}

	newConfig := &service.FlaredTunnelConfigResponse{
		Version:  "1",
		Checksum: "sum1",
		Relays: []service.FlaredRelayInfo{
			{
				RelayNodeID: "relay-1",
				Address:     "127.0.0.1:7000",
				AuthToken:   "auth-1",
			},
		},
	}

	if err := m.UpdateConfig(context.Background(), newConfig); err != nil {
		t.Fatalf("failed to UpdateConfig: %v", err)
	}

	assertCommandExitedEventually(t, orphan, 2*time.Second)
	assertStatusEventually(t, m, "relay-1", "running", 4*time.Second)

	m.mu.RLock()
	proc := m.processes["relay-1"]
	m.mu.RUnlock()
	proc.Cancel()
}

func TestStopCancelsRunningProcesses(t *testing.T) {
	scriptPath, dir := setupDummyScript(t)
	writeControl(t, dir, 0, 30)

	cfg := &config.Config{
		ServerURL:   "http://localhost:8080",
		TunnelToken: "test-token",
		FrpcPath:    scriptPath,
		DataDir:     dir,
		StatePath:   filepath.Join(dir, "flared-state.json"),
	}

	m := NewManager(cfg)
	newConfig := &service.FlaredTunnelConfigResponse{
		Version:  "1",
		Checksum: "sum1",
		Relays: []service.FlaredRelayInfo{
			{
				RelayNodeID: "relay-1",
				Address:     "127.0.0.1:7000",
				AuthToken:   "auth-1",
			},
		},
	}

	if err := m.UpdateConfig(context.Background(), newConfig); err != nil {
		t.Fatalf("failed to UpdateConfig: %v", err)
	}

	assertStatusEventually(t, m, "relay-1", "running", 4*time.Second)

	m.mu.RLock()
	proc := m.processes["relay-1"]
	if proc == nil || proc.Cmd == nil {
		m.mu.RUnlock()
		t.Fatal("expected running process to have a command handle")
	}
	cmd := proc.Cmd
	m.mu.RUnlock()

	m.Stop()
	assertCommandExitedEventually(t, cmd, 2*time.Second)

	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.processes) != 0 {
		t.Fatalf("expected no managed processes after stop, got %d", len(m.processes))
	}
}
