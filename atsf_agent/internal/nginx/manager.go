package nginx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Executor interface {
	Test(ctx context.Context) error
	Reload(ctx context.Context) error
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type OSCommandRunner struct{}

func (r *OSCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	return output, err
}

type PathExecutor struct {
	Path   string
	Runner CommandRunner
}

func (e *PathExecutor) Test(ctx context.Context) error {
	output, err := e.Runner.Run(ctx, e.Path, "-t")
	if err != nil {
		return fmt.Errorf("nginx -t failed: %w: %s", err, string(output))
	}
	return nil
}

func (e *PathExecutor) Reload(ctx context.Context) error {
	output, err := e.Runner.Run(ctx, e.Path, "-s", "reload")
	if err != nil {
		return fmt.Errorf("nginx reload failed: %w: %s", err, string(output))
	}
	return nil
}

type DockerExecutor struct {
	DockerBinary   string
	ContainerName  string
	Image          string
	RouteConfigDir string
	Runner         CommandRunner
}

func (e *DockerExecutor) Test(ctx context.Context) error {
	if err := e.ensureContainer(ctx); err != nil {
		return err
	}
	output, err := e.Runner.Run(ctx, e.DockerBinary, "exec", e.ContainerName, "nginx", "-t")
	if err != nil {
		return fmt.Errorf("docker nginx -t failed: %w: %s", err, string(output))
	}
	return nil
}

func (e *DockerExecutor) Reload(ctx context.Context) error {
	if err := e.ensureContainer(ctx); err != nil {
		return err
	}
	output, err := e.Runner.Run(ctx, e.DockerBinary, "exec", e.ContainerName, "nginx", "-s", "reload")
	if err != nil {
		return fmt.Errorf("docker nginx reload failed: %w: %s", err, string(output))
	}
	return nil
}

func (e *DockerExecutor) ensureContainer(ctx context.Context) error {
	output, err := e.Runner.Run(ctx, e.DockerBinary, "inspect", "-f", "{{.State.Running}}", e.ContainerName)
	if err == nil {
		if strings.TrimSpace(string(output)) == "true" {
			return nil
		}
		startOutput, startErr := e.Runner.Run(ctx, e.DockerBinary, "start", e.ContainerName)
		if startErr != nil {
			return fmt.Errorf("docker start nginx failed: %w: %s", startErr, string(startOutput))
		}
		return nil
	}

	runArgs := []string{
		"run", "-d",
		"--name", e.ContainerName,
		"-p", "80:80",
		"-p", "443:443",
		"-v", fmt.Sprintf("%s:/etc/nginx/conf.d", e.RouteConfigDir),
		e.Image,
	}
	runOutput, runErr := e.Runner.Run(ctx, e.DockerBinary, runArgs...)
	if runErr != nil {
		return fmt.Errorf("docker run nginx failed: %w: %s", runErr, string(runOutput))
	}
	return nil
}

type Manager struct {
	RouteConfigPath string
	Executor        Executor
}

func (m *Manager) Apply(ctx context.Context, content string) error {
	backupPath, hadExisting, err := m.backup()
	if err != nil {
		return err
	}
	if err = os.WriteFile(m.RouteConfigPath, []byte(content), 0o644); err != nil {
		return err
	}
	if err = m.Executor.Test(ctx); err != nil {
		_ = m.restore(backupPath, hadExisting)
		return err
	}
	if err = m.Executor.Reload(ctx); err != nil {
		_ = m.restore(backupPath, hadExisting)
		return err
	}
	if backupPath != "" {
		_ = os.Remove(backupPath)
	}
	return nil
}

type ExecutorOptions struct {
	NginxPath       string
	DockerBinary    string
	ContainerName   string
	Image           string
	RouteConfigPath string
}

func NewExecutor(options ExecutorOptions) Executor {
	runner := &OSCommandRunner{}
	if options.NginxPath != "" {
		return &PathExecutor{
			Path:   options.NginxPath,
			Runner: runner,
		}
	}
	return &DockerExecutor{
		DockerBinary:   options.DockerBinary,
		ContainerName:  options.ContainerName,
		Image:          options.Image,
		RouteConfigDir: filepath.Dir(options.RouteConfigPath),
		Runner:         runner,
	}
}

func (m *Manager) backup() (string, bool, error) {
	if m.RouteConfigPath == "" {
		return "", false, errors.New("route config path 不能为空")
	}
	if err := os.MkdirAll(filepath.Dir(m.RouteConfigPath), 0o755); err != nil {
		return "", false, err
	}
	data, err := os.ReadFile(m.RouteConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	backupPath := m.RouteConfigPath + ".bak"
	if err = os.WriteFile(backupPath, data, 0o644); err != nil {
		return "", false, err
	}
	return backupPath, true, nil
}

func (m *Manager) restore(backupPath string, hadExisting bool) error {
	if hadExisting {
		data, err := os.ReadFile(backupPath)
		if err != nil {
			return err
		}
		return os.WriteFile(m.RouteConfigPath, data, 0o644)
	}
	if err := os.Remove(m.RouteConfigPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
