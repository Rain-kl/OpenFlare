package frps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	service "github.com/Rain-kl/Wavelet/pkg/protocol"
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
	scriptPath := filepath.Join(dir, "dummy_frps")

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
func assertStatusEventually(t *testing.T, m *Manager, expectedStatus string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rt := m.GetRuntimeStatus()
		if rt.Status == expectedStatus {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	rt := m.GetRuntimeStatus()
	t.Fatalf("expected status eventually %s, got %s (err: %s)", expectedStatus, rt.Status, rt.LastError)
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

	m := NewManager(scriptPath, dir, "agent-token")
	defer m.Stop()

	cfg := &service.RelayConfig{
		BindPort:         7000,
		VhostHTTPPort:    8080,
		AuthToken:        "test-auth",
		WebServerEnabled: false,
	}

	m.UpdateConfig(cfg)

	assertStatusEventually(t, m, "healthy", 2*time.Second)

	rt := m.GetRuntimeStatus()
	if !rt.ProcessAlive {
		t.Error("expected process to be alive")
	}
}

func TestStartProcessFailureAndBackoff(t *testing.T) {
	dir := t.TempDir()
	invalidScriptPath := filepath.Join(dir, "non_existent_frps")

	m := NewManager(invalidScriptPath, dir, "agent-token")
	defer m.Stop()

	cfg := &service.RelayConfig{
		BindPort:         7000,
		VhostHTTPPort:    8080,
		AuthToken:        "test-auth",
		WebServerEnabled: false,
	}

	m.UpdateConfig(cfg)

	assertStatusEventually(t, m, "unhealthy", 2*time.Second)

	rt := m.GetRuntimeStatus()
	if !strings.Contains(rt.LastError, "failed to start") {
		t.Errorf("expected error message containing 'failed to start', got %s", rt.LastError)
	}

	// Correct the path to dummy script
	scriptPath, _ := setupDummyScript(t)
	writeControl(t, filepath.Dir(scriptPath), 0, 5)

	m.mu.Lock()
	m.frpsPath = scriptPath
	m.mu.Unlock()

	// Wait for backoff retry (1s backoff)
	assertStatusEventually(t, m, "healthy", 3*time.Second)

	rt = m.GetRuntimeStatus()
	if !rt.ProcessAlive {
		t.Error("expected process to be alive now")
	}
}

func TestUnexpectedExitAndAutorestart(t *testing.T) {
	scriptPath, dir := setupDummyScript(t)
	// Start with immediate exit code 1
	writeControl(t, dir, 1, 0)

	m := NewManager(scriptPath, dir, "agent-token")
	defer m.Stop()

	cfg := &service.RelayConfig{
		BindPort:         7000,
		VhostHTTPPort:    8080,
		AuthToken:        "test-auth",
		WebServerEnabled: false,
	}

	m.UpdateConfig(cfg)

	assertStatusEventually(t, m, "unhealthy", 2*time.Second)

	rt := m.GetRuntimeStatus()
	if !strings.Contains(rt.LastError, "exited with error") {
		t.Errorf("expected exit error, got %s", rt.LastError)
	}

	// Change control to be healthy (runs for 5s, exit 0)
	writeControl(t, dir, 0, 5)

	// Wait for the retry to fire (backoff was 1s)
	assertStatusEventually(t, m, "healthy", 3*time.Second)
}

func TestBackoffReset(t *testing.T) {
	scriptPath, dir := setupDummyScript(t)
	// Rapid exit to increase backoff
	writeControl(t, dir, 1, 0)

	m := NewManager(scriptPath, dir, "agent-token")
	defer m.Stop()

	cfg := &service.RelayConfig{
		BindPort:         7000,
		VhostHTTPPort:    8080,
		AuthToken:        "test-auth",
		WebServerEnabled: false,
	}

	m.UpdateConfig(cfg)

	// Crashed once, backoff is 2s
	assertStatusEventually(t, m, "unhealthy", 2*time.Second)

	// Now make it run successfully for 11 seconds (exit code 0, sleep 11s)
	writeControl(t, dir, 0, 11)

	// Wait for next retry to start running
	assertStatusEventually(t, m, "healthy", 4*time.Second)

	// Wait for process to run for 10.5 seconds to trigger backoff reset
	time.Sleep(10500 * time.Millisecond)

	// Now make it crash again (exit code 1, sleep 0s)
	writeControl(t, dir, 1, 0)

	// Wait for it to finish and crash
	assertStatusEventually(t, m, "unhealthy", 3*time.Second)

	// It crashed. Since it ran for > 10s, backoff should have been reset to 1s.
	// We make it healthy again (exit code 0, sleep 5)
	writeControl(t, dir, 0, 5)

	// Wait 1.5 seconds. If backoff was reset to 1s, it should be healthy now.
	assertStatusEventually(t, m, "healthy", 2*time.Second)
}

func TestImmediateRestartOnSameConfigDeadProcess(t *testing.T) {
	scriptPath, dir := setupDummyScript(t)
	// Crashes immediately
	writeControl(t, dir, 1, 0)

	m := NewManager(scriptPath, dir, "agent-token")
	defer m.Stop()

	cfg := &service.RelayConfig{
		BindPort:         7000,
		VhostHTTPPort:    8080,
		AuthToken:        "test-auth",
		WebServerEnabled: false,
	}

	m.UpdateConfig(cfg)

	// Let it crash
	assertStatusEventually(t, m, "unhealthy", 2*time.Second)

	// Make it start successfully
	writeControl(t, dir, 0, 5)

	// Send same config block to trigger immediate restart bypass of backoff sleep
	m.UpdateConfig(cfg)

	// Check if it started immediately
	assertStatusEventually(t, m, "healthy", 2*time.Second)
}

func TestSupervisorGenerationInterrupt(t *testing.T) {
	scriptPath, dir := setupDummyScript(t)
	writeControl(t, dir, 0, 10)

	m := NewManager(scriptPath, dir, "agent-token")
	defer m.Stop()

	cfg := &service.RelayConfig{
		BindPort:         7000,
		VhostHTTPPort:    8080,
		AuthToken:        "test-auth",
		WebServerEnabled: false,
	}

	m.UpdateConfig(cfg)

	assertStatusEventually(t, m, "healthy", 2*time.Second)

	m.mu.Lock()
	gen1 := m.generation
	cmd1 := m.cmd
	m.mu.Unlock()

	if cmd1 == nil {
		t.Fatal("expected active process")
	}

	// Update configuration with new bind port to trigger new generation
	cfg2 := &service.RelayConfig{
		BindPort:         7001,
		VhostHTTPPort:    8080,
		AuthToken:        "test-auth",
		WebServerEnabled: false,
	}
	m.UpdateConfig(cfg2)

	assertStatusEventually(t, m, "healthy", 2*time.Second)

	m.mu.Lock()
	gen2 := m.generation
	cmd2 := m.cmd
	m.mu.Unlock()

	if gen2 <= gen1 {
		t.Errorf("expected generation incremented, got gen1=%d gen2=%d", gen1, gen2)
	}
	if cmd2 == cmd1 {
		t.Error("expected old process killed and new command started")
	}

	// Verify old process is actually killed
	var cmd1Finished int32
	go func() {
		_ = cmd1.Wait()
		atomic.StoreInt32(&cmd1Finished, 1)
	}()

	time.Sleep(200 * time.Millisecond)
	if atomic.LoadInt32(&cmd1Finished) != 1 {
		t.Error("expected first process to be killed")
	}
}

func TestUpdateConfigKillsOrphanProcessBeforeRestart(t *testing.T) {
	scriptPath, dir := setupDummyScript(t)
	writeControl(t, dir, 0, 5)

	m := NewManager(scriptPath, dir, "agent-token")
	defer m.Stop()

	orphan := exec.Command("sh", "-c", "sleep 30")
	if err := orphan.Start(); err != nil {
		t.Fatalf("failed to start orphan process: %v", err)
	}
	t.Cleanup(func() {
		if orphan.Process != nil {
			_ = orphan.Process.Kill()
		}
	})

	if err := os.WriteFile(m.pidPath, []byte(fmt.Sprintf("%d", orphan.Process.Pid)), 0o644); err != nil {
		t.Fatalf("failed to seed orphan pid file: %v", err)
	}

	cfg := &service.RelayConfig{
		BindPort:         7000,
		VhostHTTPPort:    8080,
		AuthToken:        "test-auth",
		WebServerEnabled: false,
	}

	m.UpdateConfig(cfg)

	assertCommandExitedEventually(t, orphan, 2*time.Second)
	assertStatusEventually(t, m, "healthy", 2*time.Second)
}
