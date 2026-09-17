// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package frps manages the lifecycle of the frps reverse-proxy process:
// rendering its TOML config, supervising the child process with exponential-
// backoff restarts, and exposing runtime status to the heartbeat subsystem.
package frps

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	service "Wavelet/openflare/share/protocol"
	"Wavelet/pkg/util"
)

const (
	statusUnhealthy = "unhealthy"

	defaultFrpsWebServerPort      = 17500
	frpsSupervisorPollInterval    = 100 * time.Millisecond
	frpsOrphanProcessCleanupDelay = 500 * time.Millisecond
	frpsDataDirPerm               = 0o750
	frpsConfigFilePerm            = 0o600
	frpsPidFilePerm               = 0o600
)

// Manager controls a single frps child process. It renders TOML configuration,
// supervises the process with automatic restarts, and reports runtime status.
type Manager struct {
	frpsPath   string
	dataDir    string
	configPath string
	pidPath    string
	agentToken string

	mu           sync.RWMutex
	activeConfig *service.RelayConfig
	cmd          *exec.Cmd
	status       string
	lastError    string
	generation   uint64
	stopping     bool
}

// RuntimeStatus is a snapshot of the frps process state at the time of the call.
type RuntimeStatus struct {
	Status       string
	LastError    string
	Connections  int
	ProxyCount   int
	ClientCount  int
	Proxies      []service.RelayProxyStat
	ProcessAlive bool
}

// NewManager constructs a Manager that will run frpsPath and store its PID file
// and generated configuration under dataDir.
func NewManager(frpsPath string, dataDir string, agentToken string) *Manager {
	return &Manager{
		frpsPath:   frpsPath,
		dataDir:    dataDir,
		configPath: filepath.Join(dataDir, "frps.toml"),
		pidPath:    filepath.Join(dataDir, "frps.pid"),
		status:     "unknown", // 启动阶段尚未获取配置，状态未知；避免首次 heartbeat 误报 frps_unhealthy
		agentToken: agentToken,
	}
}

// GetVersion executes frps -v and returns the trimmed version string.
func (m *Manager) GetVersion(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, m.frpsPath, "-v") //nolint:gosec // frpsPath is the configured trusted frps binary location
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("failed to get frps version", "error", err)
		return ""
	}
	return strings.TrimSpace(string(out))
}

// GetStatus returns the current status string (e.g. "healthy", "unhealthy") under
// a read lock.
func (m *Manager) GetStatus() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// GetRuntimeStatus returns a RuntimeStatus snapshot without blocking the
// supervisor goroutine for more than the duration of a read lock.
func (m *Manager) GetRuntimeStatus() RuntimeStatus {
	m.mu.RLock()
	status := m.status
	lastError := m.lastError
	cmd := m.cmd
	m.mu.RUnlock()

	return RuntimeStatus{
		Status:       status,
		LastError:    lastError,
		Connections:  0,
		ProxyCount:   0,
		ClientCount:  0,
		Proxies:      nil,
		ProcessAlive: cmd != nil && cmd.Process != nil,
	}
}

// UpdateConfig applies a new RelayConfig. If the configuration has not changed
// and frps is already running, this is a no-op. Otherwise the existing process
// is killed and a new supervisor goroutine is started.
func (m *Manager) UpdateConfig(ctx context.Context, cfg *service.RelayConfig) {
	if cfg == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if config changed
	if m.activeConfig != nil &&
		m.activeConfig.BindPort == cfg.BindPort &&
		m.activeConfig.VhostHTTPPort == cfg.VhostHTTPPort &&
		m.activeConfig.AuthToken == cfg.AuthToken &&
		m.activeConfig.WebServerEnabled == cfg.WebServerEnabled &&
		m.activeConfig.WebServerPort == cfg.WebServerPort {
		if m.cmd == nil && !m.stopping {
			slog.Warn("frps config unchanged but process is not running, restarting")
			m.stopping = false
			m.generation++
			generation := m.generation
			if err := m.renderConfig(cfg); err != nil {
				slog.Error("failed to render frps config", "error", err)
				m.status = statusUnhealthy
				m.lastError = err.Error()
				return
			}
			util.Go(func() { m.supervise(ctx, generation) })
		}
		return
	}

	m.activeConfig = cfg
	m.stopping = false
	m.generation++
	generation := m.generation
	slog.Info("relay config updated, reloading frps")

	if m.cmd != nil && m.cmd.Process != nil {
		slog.Debug("stopping existing frps process")
		_ = m.cmd.Process.Kill()
		m.cmd = nil
	}

	if err := m.renderConfig(cfg); err != nil {
		slog.Error("failed to render frps config", "error", err)
		m.status = statusUnhealthy
		m.lastError = err.Error()
		return
	}

	util.Go(func() { m.supervise(ctx, generation) })
}

func (m *Manager) renderConfig(cfg *service.RelayConfig) error {
	if err := os.MkdirAll(m.dataDir, frpsDataDirPerm); err != nil {
		return err
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "bindPort = %d\n", cfg.BindPort)
	if cfg.VhostHTTPPort > 0 {
		fmt.Fprintf(&buf, "vhostHTTPPort = %d\n", cfg.VhostHTTPPort)
	}
	if cfg.AuthToken != "" {
		buf.WriteString("[auth]\n")
		buf.WriteString("method = \"token\"\n")
		buf.WriteString("token = " + service.TOMLQuote(cfg.AuthToken) + "\n")
	}

	// WebServer configuration
	buf.WriteString("\n[webServer]\n")
	if cfg.WebServerEnabled {
		buf.WriteString("addr = \"0.0.0.0\"\n")
	} else {
		buf.WriteString("addr = \"127.0.0.1\"\n")
	}
	port := cfg.WebServerPort
	if port <= 0 {
		port = defaultFrpsWebServerPort
	}
	fmt.Fprintf(&buf, "port = %d\n", port)
	buf.WriteString("user = \"admin\"\n")

	password := m.agentToken
	if password == "" {
		password = "admin"
	}
	fmt.Fprintf(&buf, "password = %s\n", service.TOMLQuote(password))
	return os.WriteFile(m.configPath, buf.Bytes(), frpsConfigFilePerm)
}

func (m *Manager) supervise(ctx context.Context, generation uint64) {
	procCtx := context.WithoutCancel(ctx)
	backoff := 1 * time.Second
	const maxBackoff = 60 * time.Second

	for {
		m.mu.Lock()
		if m.stopping || m.generation != generation {
			m.mu.Unlock()
			return
		}

		ensureNoOrphanProcess(m.pidPath)

		cmd := exec.CommandContext(procCtx, m.frpsPath, "-c", m.configPath) //nolint:gosec // frpsPath and configPath are managed trusted locations
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Start()
		if err != nil {
			m.status = statusUnhealthy
			m.lastError = fmt.Sprintf("failed to start: %v", err)
			slog.Error("failed to start frps", "error", err, "generation", generation)
			m.mu.Unlock()

			if !m.sleepOrInterrupt(generation, backoff) {
				return
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		_ = os.WriteFile(m.pidPath, fmt.Appendf(nil, "%d", cmd.Process.Pid), frpsPidFilePerm)

		m.cmd = cmd
		m.status = "healthy"
		m.lastError = ""
		m.mu.Unlock()

		startedAt := time.Now()
		waitErr := cmd.Wait()
		_ = os.Remove(m.pidPath)

		m.mu.Lock()
		if m.cmd == cmd {
			m.cmd = nil
			m.status = statusUnhealthy
			if waitErr != nil {
				m.lastError = fmt.Sprintf("exited with error: %v", waitErr)
			} else {
				m.lastError = "exited unexpectedly"
			}
			slog.Warn("frps process exited unexpectedly", "error", waitErr, "generation", generation)
		}
		shouldContinue := !m.stopping && m.generation == generation
		m.mu.Unlock()

		if !shouldContinue {
			return
		}

		if time.Since(startedAt) >= 10*time.Second {
			backoff = 1 * time.Second
		}

		if !m.sleepOrInterrupt(generation, backoff) {
			return
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (m *Manager) sleepOrInterrupt(generation uint64, d time.Duration) bool {
	ticker := time.NewTicker(frpsSupervisorPollInterval)
	defer ticker.Stop()

	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		<-ticker.C
		m.mu.RLock()
		interrupted := m.stopping || m.generation != generation
		m.mu.RUnlock()
		if interrupted {
			return false
		}
	}
	return true
}

// Stop signals the supervisor to cease restarting frps and kills the running
// process if one exists.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopping = true
	m.generation++
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
		m.cmd = nil
	}
	_ = os.Remove(m.pidPath)
	m.status = statusUnhealthy
}

func ensureNoOrphanProcess(pidPath string) {
	data, err := os.ReadFile(pidPath) //nolint:gosec // pidPath is a managed internal path, not user input
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
			time.Sleep(frpsOrphanProcessCleanupDelay)
		}
	}
	_ = os.Remove(pidPath)
}
