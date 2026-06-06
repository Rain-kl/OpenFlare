package frpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rain-kl/openflare/openflare-server/service"
	"github.com/rain-kl/openflare/openflared/internal/config"
)

type Manager struct {
	cfg       *config.Config
	processes map[string]*Process
	mu        sync.RWMutex

	currentVersion  string
	currentChecksum string
}

type Process struct {
	RelayID   string
	Cmd       *exec.Cmd
	Cancel    context.CancelFunc
	Status    string
	StartTime time.Time
	LastError string
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:       cfg,
		processes: make(map[string]*Process),
	}
}

func (m *Manager) GetVersion() string {
	cmd := exec.Command(m.cfg.FrpcPath, "-v")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

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

func (m *Manager) GetCurrentConfigVersion() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentVersion
}

func (m *Manager) GetCurrentConfigChecksum() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentChecksum
}

func (m *Manager) UpdateConfig(ctx context.Context, newConfig *service.FlaredTunnelConfigResponse) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if newConfig == nil {
		return nil
	}

	versionChanged := newConfig.Version != m.currentVersion || newConfig.Checksum != m.currentChecksum
	if versionChanged {
		slog.Info("applying new tunnel config", "version", newConfig.Version)
	} else {
		slog.Debug("tunnel config version unchanged, ensuring processes are running", "version", newConfig.Version)
	}

	if err := os.MkdirAll(m.cfg.DataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir failed: %w", err)
	}

	activeRelays := make(map[string]struct{})

	for _, relay := range newConfig.Relays {
		activeRelays[relay.RelayNodeID] = struct{}{}
		tomlContent := buildFrpcToml(relay, newConfig.Proxies)
		configPath := filepath.Join(m.cfg.DataDir, fmt.Sprintf("frpc_%s.toml", relay.RelayNodeID))

		needsRestart := false
		existingData, err := os.ReadFile(configPath)
		if err != nil || string(existingData) != tomlContent {
			// 配置文件不存在或内容有变化，需要写入并重启
			needsRestart = true
		}

		if needsRestart {
			if err := os.WriteFile(configPath, []byte(tomlContent), 0o644); err != nil {
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
		return m.saveState()
	}
	return nil
}

func (m *Manager) restartProcess(ctx context.Context, relayID string, configPath string) {
	pidPath := filepath.Join(m.cfg.DataDir, fmt.Sprintf("frpc_%s.pid", relayID))
	if proc, ok := m.processes[relayID]; ok {
		proc.Cancel()
		_ = os.Remove(pidPath)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	procCtx, cancel := context.WithCancel(ctx)
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

			cmd := exec.CommandContext(procCtx, m.cfg.FrpcPath, "-c", configPath)

			m.mu.Lock()
			proc.Cmd = cmd
			proc.Status = "running"
			m.mu.Unlock()

			startedAt := time.Now()
			err := cmd.Start()
			if err == nil {
				_ = os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0o644)
				err = cmd.Wait()
			}
			_ = os.Remove(pidPath)

			m.mu.Lock()
			if procCtx.Err() != nil {
				proc.Status = "stopped"
				m.mu.Unlock()
				return
			}

			if err != nil {
				proc.LastError = err.Error()
				proc.Status = "error"
				slog.Error("frpc process exited unexpectedly", "relay_id", relayID, "error", err)
			} else {
				proc.Status = "stopped"
				proc.LastError = "exited unexpectedly with code 0"
				slog.Warn("frpc process exited unexpectedly with code 0", "relay_id", relayID)
			}
			m.mu.Unlock()

			if time.Since(startedAt) >= 10*time.Second {
				backoff = 1 * time.Second
			}

			select {
			case <-procCtx.Done():
				return
			case <-time.After(backoff):
				backoff = backoff * 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
		}
	}()
}

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

	buf.WriteString(fmt.Sprintf(`serverAddr = "%s"
serverPort = %s
`, host, port))

	if relay.AuthToken != "" {
		buf.WriteString(fmt.Sprintf(`auth.method = "token"
auth.token = "%s"
`, relay.AuthToken))
	}

	if relay.ProxyURL != "" {
		buf.WriteString(fmt.Sprintf(`transport.proxyURL = "%s"
`, relay.ProxyURL))
	}

	buf.WriteString("\n")

	for _, proxy := range proxies {
		buf.WriteString(fmt.Sprintf("[[proxies]]\nname = \"%s\"\ntype = \"%s\"\nlocalIP = \"%s\"\nlocalPort = %d\n",
			proxy.Name, proxy.Type, proxy.LocalAddr, proxy.LocalPort))
		if len(proxy.CustomDomains) > 0 {
			buf.WriteString(fmt.Sprintf("customDomains = [\"%s\"]\n", strings.Join(proxy.CustomDomains, "\", \"")))
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

// State persistence
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
	return os.WriteFile(m.cfg.StatePath, data, 0o644)
}

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
