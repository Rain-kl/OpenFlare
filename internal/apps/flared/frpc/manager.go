// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package frpc manages frpc child processes for tunnel relay connections.
package frpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/flared/config"
	service "github.com/Rain-kl/Wavelet/pkg/protocol"
)

const (
	dataDirPerm            = 0o750
	frpcConfigFilePerm     = 0o644
	orphanProcessKillDelay = 500 * time.Millisecond
)

// Manager supervises frpc processes for each active relay node.
type Manager struct {
	cfg       *config.Config
	processes map[string]*Process
	mu        sync.RWMutex

	currentVersion  string
	currentChecksum string
}

// Process tracks a single frpc child process and its runtime state.
type Process struct {
	RelayID   string
	Cmd       *exec.Cmd
	Cancel    context.CancelFunc
	Status    string
	StartTime time.Time
	LastError string
}

// NewManager creates a Manager using the given flared configuration.
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:       cfg,
		processes: make(map[string]*Process),
	}
}

// GetVersion returns the installed frpc binary version string.
func (m *Manager) GetVersion(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, m.cfg.FrpcPath, "-v") //nolint:gosec // FrpcPath is the configured trusted frpc binary location
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// GetConnectedRelays reports the relay nodes with active or managed frpc processes.
func (m *Manager) GetConnectedRelays() []service.FlaredConnectedRelay {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]service.FlaredConnectedRelay, 0, len(m.processes))
	for relayID, proc := range m.processes {
		result = append(result, service.FlaredConnectedRelay{
			RelayNodeID: relayID,
			Status:      proc.Status,
		})
	}
	return result
}

// GetCurrentConfigVersion returns the version of the applied tunnel configuration.
func (m *Manager) GetCurrentConfigVersion() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentVersion
}

// GetCurrentConfigChecksum returns the checksum of the applied tunnel configuration.
func (m *Manager) GetCurrentConfigChecksum() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentChecksum
}

// UpdateConfig reconciles running frpc processes with the latest tunnel configuration.
// The returned bool indicates whether the active config version or checksum changed.
func (m *Manager) UpdateConfig(ctx context.Context, newConfig *service.FlaredTunnelConfigResponse) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if newConfig == nil {
		return false, nil
	}

	versionChanged := newConfig.Version != m.currentVersion || newConfig.Checksum != m.currentChecksum
	if versionChanged {
		slog.Info("applying new tunnel config", "version", newConfig.Version)
	} else {
		slog.Debug("tunnel config version unchanged, ensuring processes are running", "version", newConfig.Version)
	}

	if err := os.MkdirAll(m.cfg.DataDir, dataDirPerm); err != nil {
		return false, fmt.Errorf("create data dir failed: %w", err)
	}

	activeRelays := make(map[string]struct{})

	for _, relay := range newConfig.Relays {
		activeRelays[relay.RelayNodeID] = struct{}{}
		tomlContent := buildFrpcToml(relay, newConfig.Proxies)
		configPath := filepath.Join(m.cfg.DataDir, fmt.Sprintf("frpc_%s.toml", relay.RelayNodeID))

		needsRestart := false
		existingData, err := os.ReadFile(configPath) //nolint:gosec // configPath is under managed DataDir
		if err != nil || string(existingData) != tomlContent {
			// 配置文件不存在或内容有变化，需要写入并重启
			needsRestart = true
		}

		if needsRestart {
			if err := os.WriteFile(configPath, []byte(tomlContent), frpcConfigFilePerm); err != nil {
				slog.Error("failed to write frpc config", "relay_id", relay.RelayNodeID, "error", err)
				continue
			}
			m.restartProcess(ctx, relay.RelayNodeID, configPath)
		} else if _, ok := m.processes[relay.RelayNodeID]; !ok {
			// 配置未变但进程不存在（如重启后），直接启动进程
			slog.Info("frpc process missing, starting", "relay_id", relay.RelayNodeID)
			m.restartProcess(ctx, relay.RelayNodeID, configPath)
		}
	}

	// Stop obsolete processes
	for relayID, proc := range m.processes {
		if _, ok := activeRelays[relayID]; !ok {
			slog.Info("stopping obsolete frpc process", "relay_id", relayID)
			proc.Cancel()
			pidPath := filepath.Join(m.cfg.DataDir, fmt.Sprintf("frpc_%s.pid", relayID))
			_ = os.Remove(pidPath)
			delete(m.processes, relayID)
		}
	}

	if versionChanged {
		m.currentVersion = newConfig.Version
		m.currentChecksum = newConfig.Checksum
		return true, m.saveState()
	}
	return false, nil
}

func (m *Manager) restartProcess(ctx context.Context, relayID string, configPath string) {
	pidPath := filepath.Join(m.cfg.DataDir, fmt.Sprintf("frpc_%s.pid", relayID))
	if proc, ok := m.processes[relayID]; ok {
		proc.Cancel()
		_ = os.Remove(pidPath)
	}

	procCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	proc := &Process{
		RelayID:   relayID,
		Cancel:    cancel,
		Status:    "starting",
		StartTime: time.Now(),
	}
	m.processes[relayID] = proc

	go func() {
		backoff := 1 * time.Second
		const maxBackoff = 60 * time.Second

		for {
			m.mu.Lock()
			if procCtx.Err() != nil {
				m.mu.Unlock()
				return
			}
			m.mu.Unlock()

			ensureNoOrphanProcess(pidPath)

			cmd := exec.CommandContext(procCtx, m.cfg.FrpcPath, "-c", configPath) //nolint:gosec // FrpcPath and configPath are managed trusted locations
			var stderrBuf bytes.Buffer
			cmd.Stderr = &stderrBuf

			// frpc 及其中间子进程必须整体随上下文终止：CommandContext 默认只杀
			// 直接子进程，孤儿孙进程会继续持有 stderr 管道导致 cmd.Wait 阻塞到其
			// 自然退出。这里为 frpc 单独建进程组并整组 SIGKILL。
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.Cancel = func() error {
				if cmd.Process == nil {
					return nil
				}
				return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}

			m.mu.Lock()
			proc.Cmd = cmd
			proc.Status = "running"
			m.mu.Unlock()

			startedAt := time.Now()
			err := cmd.Start()
			if err == nil {
				_ = os.WriteFile(pidPath, fmt.Appendf(nil, "%d", cmd.Process.Pid), frpcConfigFilePerm)
				err = cmd.Wait()
			}
			_ = os.Remove(pidPath)

			m.mu.Lock()
			if procCtx.Err() != nil {
				proc.Status = "stopped"
				m.mu.Unlock()
				return
			}

			stderrOutput := strings.TrimSpace(stderrBuf.String())
			if err != nil {
				proc.LastError = err.Error()
				proc.Status = "error"
				if stderrOutput != "" {
					slog.Error("frpc process exited unexpectedly", "relay_id", relayID, "error", err, "stderr", stderrOutput)
				} else {
					slog.Error("frpc process exited unexpectedly", "relay_id", relayID, "error", err)
				}
			} else {
				proc.Status = "stopped"
				proc.LastError = "exited unexpectedly with code 0"
				slog.Warn("frpc process exited unexpectedly with code 0", "relay_id", relayID)
			}
			m.mu.Unlock()

			if time.Since(startedAt) >= 10*time.Second {
				backoff = 1 * time.Second
			}

			t := time.NewTimer(backoff)
			select {
			case <-procCtx.Done():
				t.Stop()
				return
			case <-t.C:
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
			t.Stop()
		}
	}()
}

// Stop cancels and removes all managed frpc processes.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for relayID, proc := range m.processes {
		if proc != nil && proc.Cancel != nil {
			proc.Cancel()
		}
		pidPath := filepath.Join(m.cfg.DataDir, fmt.Sprintf("frpc_%s.pid", relayID))
		_ = os.Remove(pidPath)
		delete(m.processes, relayID)
	}
}

func buildFrpcToml(relay service.FlaredRelayInfo, proxies []service.FlaredProxyEntry) string {
	var buf bytes.Buffer

	host, port := parseAddr(relay.Address)

	fmt.Fprintf(&buf, "serverAddr = \"%s\"\nserverPort = %s\n", host, port)

	if relay.AuthToken != "" {
		fmt.Fprintf(&buf, "auth.method = \"token\"\nauth.token = \"%s\"\n", relay.AuthToken)
	}

	if relay.ProxyURL != "" {
		fmt.Fprintf(&buf, "transport.proxyURL = \"%s\"\n", relay.ProxyURL)
	}

	buf.WriteString("\n")

	for _, proxy := range proxies {
		fmt.Fprintf(&buf, "[[proxies]]\nname = \"%s\"\ntype = \"%s\"\nlocalIP = \"%s\"\nlocalPort = %d\n",
			proxy.Name, proxy.Type, proxy.LocalAddr, proxy.LocalPort)
		if len(proxy.CustomDomains) > 0 {
			fmt.Fprintf(&buf, "customDomains = [\"%s\"]\n", strings.Join(proxy.CustomDomains, "\", \""))
		}
		buf.WriteString("\n")
	}

	return buf.String()
}

func parseAddr(addr string) (string, string) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "127.0.0.1", "7000"
	}
	host, port, err := net.SplitHostPort(addr)
	if err == nil {
		return strings.Trim(host, "[]"), port
	}
	lastColon := strings.LastIndex(addr, ":")
	if lastColon > 0 && strings.Count(addr, ":") == 1 {
		return addr[:lastColon], addr[lastColon+1:]
	}
	return addr, "7000"
}

// ManagerState persists the last applied tunnel configuration version and checksum.
type ManagerState struct {
	Version  string
	Checksum string
}

func (m *Manager) saveState() error {
	state := ManagerState{
		Version:  m.currentVersion,
		Checksum: m.currentChecksum,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.cfg.StatePath, data, frpcConfigFilePerm)
}

// LoadState restores the last applied configuration version and checksum from disk.
func (m *Manager) LoadState() error {
	data, err := os.ReadFile(m.cfg.StatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var state ManagerState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	m.mu.Lock()
	m.currentVersion = state.Version
	m.currentChecksum = state.Checksum
	m.mu.Unlock()
	return nil
}

func ensureNoOrphanProcess(pidPath string) {
	data, err := os.ReadFile(pidPath) //nolint:gosec // pidPath is under managed DataDir
	if err != nil {
		return
	}
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return
	}
	if pid <= 0 {
		return
	}
	process, err := os.FindProcess(pid)
	if err == nil && process != nil {
		err = process.Signal(syscall.Signal(0))
		if err == nil || errors.Is(err, os.ErrPermission) {
			slog.Warn("attempting to kill potentially orphan process", "pid", pid, "pid_path", pidPath)
			_ = process.Kill()
			// Wait a little bit to ensure the OS has reclaimed ports
			time.Sleep(orphanProcessKillDelay)
		}
	}
	_ = os.Remove(pidPath)
}
