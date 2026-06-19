package frps

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	service "github.com/Rain-kl/Wavelet/pkg/protocol"
)

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

type RuntimeStatus struct {
	Status       string
	LastError    string
	Connections  int
	ProxyCount   int
	ClientCount  int
	Proxies      []service.RelayProxyStat
	ProcessAlive bool
}

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

func (m *Manager) GetVersion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, m.frpsPath, "-v")
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("failed to get frps version", "error", err)
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (m *Manager) GetStatus() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

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

func (m *Manager) UpdateConfig(cfg *service.RelayConfig) {
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
		m.activeConfig.WebServerEnabled == cfg.WebServerEnabled {
		if m.cmd == nil && !m.stopping {
			slog.Warn("frps config unchanged but process is not running, restarting")
			m.stopping = false
			m.generation++
			generation := m.generation
			if err := m.renderConfig(cfg); err != nil {
				slog.Error("failed to render frps config", "error", err)
				m.status = "unhealthy"
				m.lastError = err.Error()
				return
			}
			go m.supervise(generation)
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
		m.status = "unhealthy"
		m.lastError = err.Error()
		return
	}

	go m.supervise(generation)
}

func (m *Manager) renderConfig(cfg *service.RelayConfig) error {
	if err := os.MkdirAll(m.dataDir, 0755); err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("bindPort = %d\n", cfg.BindPort))
	if cfg.VhostHTTPPort > 0 {
		buf.WriteString(fmt.Sprintf("vhostHTTPPort = %d\n", cfg.VhostHTTPPort))
	}
	if cfg.AuthToken != "" {
		buf.WriteString("[auth]\n")
		buf.WriteString("method = \"token\"\n")
		buf.WriteString(fmt.Sprintf("token = \"%s\"\n", cfg.AuthToken))
	}

	// WebServer configuration
	buf.WriteString("\n[webServer]\n")
	if cfg.WebServerEnabled {
		buf.WriteString("addr = \"0.0.0.0\"\n")
	} else {
		buf.WriteString("addr = \"127.0.0.1\"\n")
	}
	buf.WriteString(fmt.Sprintf("port = %d\n", 17500))
	buf.WriteString("user = \"admin\"\n")

	password := m.agentToken
	if password == "" {
		password = "admin"
	}
	buf.WriteString(fmt.Sprintf("password = \"%s\"\n", password))

	return os.WriteFile(m.configPath, buf.Bytes(), 0644)
}

func (m *Manager) supervise(generation uint64) {
	backoff := 1 * time.Second
	const maxBackoff = 60 * time.Second

	for {
		m.mu.Lock()
		if m.stopping || m.generation != generation {
			m.mu.Unlock()
			return
		}

		ensureNoOrphanProcess(m.pidPath)

		cmd := exec.Command(m.frpsPath, "-c", m.configPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Start()
		if err != nil {
			m.status = "unhealthy"
			m.lastError = fmt.Sprintf("failed to start: %v", err)
			slog.Error("failed to start frps", "error", err, "generation", generation)
			m.mu.Unlock()

			if !m.sleepOrInterrupt(generation, backoff) {
				return
			}
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		_ = os.WriteFile(m.pidPath, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644)

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
			m.status = "unhealthy"
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
		backoff = backoff * 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (m *Manager) sleepOrInterrupt(generation uint64, d time.Duration) bool {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		select {
		case <-ticker.C:
			m.mu.RLock()
			interrupted := m.stopping || m.generation != generation
			m.mu.RUnlock()
			if interrupted {
				return false
			}
		}
	}
	return true
}

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
	m.status = "unhealthy"
}

func ensureNoOrphanProcess(pidPath string) {
	data, err := os.ReadFile(pidPath)
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
		slog.Warn("attempting to kill potentially orphan process", "pid", pid, "pid_path", pidPath)
		_ = process.Kill()
		// Wait a little bit to ensure the OS has reclaimed ports
		time.Sleep(500 * time.Millisecond)
	}
	_ = os.Remove(pidPath)
}
