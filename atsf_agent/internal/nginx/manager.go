package nginx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"atsflare-agent/internal/protocol"
)

const CertDirPlaceholder = "__ATSF_CERT_DIR__"
const RouteConfigPlaceholder = "__ATSF_ROUTE_CONFIG__"
const DockerMainConfigPath = "/usr/local/openresty/nginx/conf/nginx.conf"

const dockerRuntimeCommand = "openresty"

type Executor interface {
	Test(ctx context.Context) error
	Reload(ctx context.Context) error
	EnsureRuntime(ctx context.Context, recreate bool) error
	CheckHealth(ctx context.Context) error
	Restart(ctx context.Context) error
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
	log.Printf("running openresty test with binary: %s", e.Path)
	output, err := e.Runner.Run(ctx, e.Path, "-t")
	if err != nil {
		return fmt.Errorf("openresty -t failed: %w: %s", err, string(output))
	}
	log.Printf("openresty test succeeded with binary: %s", e.Path)
	return nil
}

func (e *PathExecutor) Reload(ctx context.Context) error {
	log.Printf("running openresty reload with binary: %s", e.Path)
	output, err := e.Runner.Run(ctx, e.Path, "-s", "reload")
	if err != nil {
		return fmt.Errorf("openresty reload failed: %w: %s", err, string(output))
	}
	log.Printf("openresty reload succeeded with binary: %s", e.Path)
	return nil
}

func (e *PathExecutor) EnsureRuntime(ctx context.Context, recreate bool) error {
	return nil
}

func (e *PathExecutor) CheckHealth(ctx context.Context) error {
	return e.Test(ctx)
}

func (e *PathExecutor) Restart(ctx context.Context) error {
	log.Printf("restarting openresty with binary: %s", e.Path)
	output, err := e.Runner.Run(ctx, e.Path, "-s", "quit")
	if err != nil {
		text := string(output)
		if !isIgnorableOpenrestyStopError(text) {
			return fmt.Errorf("openresty stop failed: %w: %s", err, text)
		}
	}
	output, err = e.Runner.Run(ctx, e.Path)
	if err != nil {
		return fmt.Errorf("openresty start failed: %w: %s", err, string(output))
	}
	log.Printf("openresty restart succeeded with binary: %s", e.Path)
	return nil
}

type DockerExecutor struct {
	DockerBinary   string
	ContainerName  string
	Image          string
	MainConfigPath string
	RouteConfigDir string
	CertDir        string
	NginxCertDir   string
	Runner         CommandRunner
}

func (e *DockerExecutor) Test(ctx context.Context) error {
	log.Printf("running docker openresty test: container=%s image=%s", e.ContainerName, e.Image)
	output, err := e.runEphemeralRuntimeCommand(ctx, "-t")
	if err != nil {
		return fmt.Errorf("docker %s -t failed: %w: %s", dockerRuntimeCommand, err, string(output))
	}
	log.Printf("docker openresty test succeeded: container=%s runtime=%s", e.ContainerName, dockerRuntimeCommand)
	return nil
}

func (e *DockerExecutor) Reload(ctx context.Context) error {
	return e.EnsureRuntime(ctx, true)
}

func (e *DockerExecutor) EnsureRuntime(ctx context.Context, recreate bool) error {
	log.Printf("ensuring docker openresty runtime: container=%s recreate=%t", e.ContainerName, recreate)
	output, err := e.Runner.Run(ctx, e.DockerBinary, "inspect", "-f", "{{.State.Running}}", e.ContainerName)
	if err == nil {
		if recreate {
			if err := e.removeContainer(ctx); err != nil {
				return err
			}
			return e.runContainer(ctx)
		}
		if strings.TrimSpace(string(output)) == "true" {
			log.Printf("docker openresty runtime already healthy: container=%s", e.ContainerName)
			return nil
		}
		if err := e.removeContainer(ctx); err != nil {
			return err
		}
		return e.runContainer(ctx)
	}
	return e.runContainer(ctx)
}

func (e *DockerExecutor) CheckHealth(ctx context.Context) error {
	log.Printf("checking docker openresty runtime health: container=%s", e.ContainerName)
	output, err := e.Runner.Run(ctx, e.DockerBinary, "inspect", "-f", "{{.State.Running}}", e.ContainerName)
	if err != nil {
		return fmt.Errorf("docker inspect openresty failed: %w: %s", err, string(output))
	}
	if strings.TrimSpace(string(output)) != "true" {
		return errors.New("docker openresty container is not running")
	}
	return nil
}

func (e *DockerExecutor) Restart(ctx context.Context) error {
	return e.EnsureRuntime(ctx, true)
}

func (e *DockerExecutor) removeContainer(ctx context.Context) error {
	log.Printf("removing docker openresty container: container=%s", e.ContainerName)
	output, err := e.Runner.Run(ctx, e.DockerBinary, "rm", "-f", e.ContainerName)
	if err != nil {
		text := string(output)
		if strings.Contains(text, "No such container") {
			return nil
		}
		return fmt.Errorf("docker rm openresty failed: %w: %s", err, text)
	}
	log.Printf("docker openresty container removed: container=%s", e.ContainerName)
	return nil
}

func (e *DockerExecutor) runContainer(ctx context.Context) error {
	log.Printf("starting docker openresty container: container=%s image=%s", e.ContainerName, e.Image)
	runArgs := []string{
		"run", "-d",
		"--name", e.ContainerName,
		"-p", "80:80",
		"-p", "443:443",
		"-v", fmt.Sprintf("%s:%s", e.MainConfigPath, DockerMainConfigPath),
		"-v", fmt.Sprintf("%s:/etc/nginx/conf.d", e.RouteConfigDir),
		"-v", fmt.Sprintf("%s:%s", e.CertDir, e.NginxCertDir),
		e.Image,
	}
	runOutput, runErr := e.Runner.Run(ctx, e.DockerBinary, runArgs...)
	if runErr != nil {
		return fmt.Errorf("docker run openresty failed: %w: %s", runErr, string(runOutput))
	}
	log.Printf("docker openresty container started: container=%s", e.ContainerName)
	return nil
}

type Manager struct {
	MainConfigPath  string
	RouteConfigPath string
	CertDir         string
	NginxCertDir    string
	Executor        Executor
}

func (m *Manager) Apply(ctx context.Context, mainConfig string, routeConfig string, supportFiles []protocol.SupportFile) error {
	log.Printf("openresty apply started: main_config=%s route_config=%s support_files=%d", m.MainConfigPath, m.RouteConfigPath, len(supportFiles))
	backup, err := m.backup()
	if err != nil {
		return err
	}
	if err = m.writeSupportFiles(supportFiles); err != nil {
		log.Printf("writing support files failed, restoring backup: error=%v", err)
		_ = m.restore(backup)
		return err
	}
	renderedMainConfig := m.renderMainConfig(mainConfig)
	if err = os.WriteFile(m.MainConfigPath, []byte(renderedMainConfig), 0o644); err != nil {
		log.Printf("writing openresty main config failed, restoring backup: error=%v", err)
		_ = m.restore(backup)
		return err
	}
	renderedRouteConfig := m.renderRouteConfig(routeConfig)
	if err = os.WriteFile(m.RouteConfigPath, []byte(renderedRouteConfig), 0o644); err != nil {
		log.Printf("writing openresty route config failed, restoring backup: error=%v", err)
		_ = m.restore(backup)
		return err
	}
	if err = m.Executor.Test(ctx); err != nil {
		log.Printf("openresty test failed after config write, restoring backup: error=%v", err)
		_ = m.restore(backup)
		return err
	}
	if err = m.Executor.Reload(ctx); err != nil {
		log.Printf("openresty reload failed after config write, restoring backup: error=%v", err)
		_ = m.restore(backup)
		return err
	}
	log.Printf("openresty apply completed successfully: main_config=%s route_config=%s", m.MainConfigPath, m.RouteConfigPath)
	return nil
}

func (m *Manager) EnsureRuntime(ctx context.Context, recreate bool) error {
	if m.Executor == nil {
		return errors.New("executor 未配置")
	}
	log.Printf("openresty ensure runtime requested: recreate=%t", recreate)
	return m.Executor.EnsureRuntime(ctx, recreate)
}

func (m *Manager) CheckHealth(ctx context.Context) error {
	if m.Executor == nil {
		return errors.New("executor 未配置")
	}
	return m.Executor.CheckHealth(ctx)
}

func (m *Manager) Restart(ctx context.Context) error {
	if m.Executor == nil {
		return errors.New("executor 未配置")
	}
	log.Printf("openresty restart requested")
	return m.Executor.Restart(ctx)
}

func (m *Manager) CurrentChecksum() (string, error) {
	if m.RouteConfigPath == "" {
		return "", errors.New("route config path 不能为空")
	}
	if m.MainConfigPath == "" {
		return "", errors.New("main config path 不能为空")
	}
	mainData, err := os.ReadFile(m.MainConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	data, err := os.ReadFile(m.RouteConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	normalizedMain := string(mainData)
	if m.RouteConfigPath != "" {
		normalizedMain = strings.ReplaceAll(normalizedMain, m.RouteConfigPath, RouteConfigPlaceholder)
	}
	normalizedRoute := string(data)
	if m.NginxCertDir != "" {
		normalizedRoute = strings.ReplaceAll(normalizedRoute, m.NginxCertDir, CertDirPlaceholder)
	}
	files, err := m.readSupportFiles()
	if err != nil {
		return "", err
	}
	result := bundleChecksum(normalizedMain, normalizedRoute, files)
	log.Printf("openresty current checksum calculated: main_config=%s route_config=%s checksum=%s support_files=%d", m.MainConfigPath, m.RouteConfigPath, result, len(files))
	return result, nil
}

type ExecutorOptions struct {
	NginxPath       string
	DockerBinary    string
	ContainerName   string
	Image           string
	MainConfigPath  string
	RouteConfigPath string
	CertDir         string
	NginxCertDir    string
}

func NewExecutor(options ExecutorOptions) Executor {
	runner := &OSCommandRunner{}
	if options.NginxPath != "" {
		return &PathExecutor{
			Path:   options.NginxPath,
			Runner: runner,
		}
	}
	routeConfigDir := filepath.Dir(options.RouteConfigPath)
	if absDir, err := filepath.Abs(routeConfigDir); err == nil {
		routeConfigDir = absDir
	}
	certDir := options.CertDir
	if absDir, err := filepath.Abs(certDir); err == nil {
		certDir = absDir
	}
	return &DockerExecutor{
		DockerBinary:   options.DockerBinary,
		ContainerName:  options.ContainerName,
		Image:          options.Image,
		MainConfigPath: options.MainConfigPath,
		RouteConfigDir: routeConfigDir,
		CertDir:        certDir,
		NginxCertDir:   options.NginxCertDir,
		Runner:         runner,
	}
}

func DetectVersion(ctx context.Context, options ExecutorOptions) string {
	version, err := detectVersion(ctx, options, &OSCommandRunner{})
	if err != nil {
		log.Printf("detect openresty version failed: %v", err)
		return ""
	}
	log.Printf("detected openresty version: %s", version)
	return version
}

func detectVersion(ctx context.Context, options ExecutorOptions, runner CommandRunner) (string, error) {
	if runner == nil {
		runner = &OSCommandRunner{}
	}
	if options.NginxPath != "" {
		output, err := runner.Run(ctx, options.NginxPath, "-v")
		if err != nil {
			return "", fmt.Errorf("run runtime -v failed: %w: %s", err, string(output))
		}
		version := parseNginxVersion(string(output))
		if version == "" {
			return "", errors.New("cannot parse runtime version from binary output")
		}
		return version, nil
	}
	output, err := runDockerVersionProbe(ctx, runner, options.DockerBinary, options.Image)
	if err != nil {
		return "", fmt.Errorf("run docker %s -v failed: %w: %s", dockerRuntimeCommand, err, string(output))
	}
	version := parseNginxVersion(string(output))
	if version == "" {
		return "", errors.New("cannot parse runtime version from docker output")
	}
	return version, nil
}

func parseNginxVersion(output string) string {
	matches := nginxVersionPattern.FindStringSubmatch(output)
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

var nginxVersionPattern = regexp.MustCompile(`(?im)(?:nginx|openresty) version:\s*(?:nginx|openresty)/([^\s]+)`)

func isIgnorableOpenrestyStopError(output string) bool {
	text := strings.ToLower(strings.TrimSpace(output))
	if text == "" {
		return false
	}
	return strings.Contains(text, "invalid pid") || strings.Contains(text, "no such process")
}

func (e *DockerExecutor) runEphemeralRuntimeCommand(ctx context.Context, args ...string) ([]byte, error) {
	return e.runEphemeralRuntimeCommandWithBinary(ctx, dockerRuntimeCommand, args...)
}

func (e *DockerExecutor) runEphemeralRuntimeCommandWithBinary(ctx context.Context, runtimeBinary string, args ...string) ([]byte, error) {
	runtimeArgs := []string{
		"run",
		"--rm",
		"-v",
		fmt.Sprintf("%s:%s", e.MainConfigPath, DockerMainConfigPath),
		"-v",
		fmt.Sprintf("%s:/etc/nginx/conf.d", e.RouteConfigDir),
		"-v",
		fmt.Sprintf("%s:%s", e.CertDir, e.NginxCertDir),
		e.Image,
		runtimeBinary,
	}
	runtimeArgs = append(runtimeArgs, args...)
	return e.Runner.Run(ctx, e.DockerBinary, runtimeArgs...)
}

func runDockerVersionProbe(ctx context.Context, runner CommandRunner, dockerBinary string, image string) ([]byte, error) {
	return runner.Run(ctx, dockerBinary, "run", "--rm", image, dockerRuntimeCommand, "-v")
}

type backupState struct {
	MainExisted  bool
	MainData     []byte
	RouteExisted bool
	RouteData    []byte
	Files        []protocol.SupportFile
}

func (m *Manager) backup() (*backupState, error) {
	if m.MainConfigPath == "" {
		return nil, errors.New("main config path 不能为空")
	}
	if m.RouteConfigPath == "" {
		return nil, errors.New("route config path 不能为空")
	}
	if err := os.MkdirAll(filepath.Dir(m.MainConfigPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(m.RouteConfigPath), 0o755); err != nil {
		return nil, err
	}
	if m.CertDir != "" {
		if err := os.MkdirAll(m.CertDir, 0o755); err != nil {
			return nil, err
		}
	}
	state := &backupState{}
	mainData, err := os.ReadFile(m.MainConfigPath)
	if err == nil {
		state.MainExisted = true
		state.MainData = mainData
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	data, err := os.ReadFile(m.RouteConfigPath)
	if err == nil {
		state.RouteExisted = true
		state.RouteData = data
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	files, err := m.readSupportFiles()
	if err != nil {
		return nil, err
	}
	state.Files = files
	log.Printf("backup captured: main_exists=%t route_exists=%t support_files=%d", state.MainExisted, state.RouteExisted, len(state.Files))
	return state, nil
}

func (m *Manager) restore(state *backupState) error {
	if state == nil {
		return nil
	}
	log.Printf("restoring nginx backup: main_existed=%t route_existed=%t support_files=%d", state.MainExisted, state.RouteExisted, len(state.Files))
	if state.MainExisted {
		if err := os.WriteFile(m.MainConfigPath, state.MainData, 0o644); err != nil {
			return err
		}
	} else if err := os.Remove(m.MainConfigPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if state.RouteExisted {
		if err := os.WriteFile(m.RouteConfigPath, state.RouteData, 0o644); err != nil {
			return err
		}
	} else if err := os.Remove(m.RouteConfigPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if m.CertDir == "" {
		return nil
	}
	if err := os.RemoveAll(m.CertDir); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(m.CertDir, 0o755); err != nil {
		return err
	}
	for _, file := range state.Files {
		targetPath := filepath.Join(m.CertDir, filepath.Clean(file.Path))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, []byte(file.Content), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) writeSupportFiles(supportFiles []protocol.SupportFile) error {
	if m.CertDir == "" {
		return nil
	}
	if err := os.RemoveAll(m.CertDir); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(m.CertDir, 0o755); err != nil {
		return err
	}
	for _, file := range supportFiles {
		targetPath := filepath.Join(m.CertDir, filepath.Clean(file.Path))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, []byte(file.Content), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) readSupportFiles() ([]protocol.SupportFile, error) {
	if m.CertDir == "" {
		return nil, nil
	}
	if _, err := os.Stat(m.CertDir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	files := make([]protocol.SupportFile, 0)
	err := filepath.Walk(m.CertDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(m.CertDir, path)
		if err != nil {
			return err
		}
		files = append(files, protocol.SupportFile{
			Path:    filepath.ToSlash(relativePath),
			Content: string(data),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i int, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func (m *Manager) renderRouteConfig(content string) string {
	if m.NginxCertDir == "" {
		return content
	}
	return strings.ReplaceAll(content, CertDirPlaceholder, m.NginxCertDir)
}

func (m *Manager) renderMainConfig(content string) string {
	if m.RouteConfigPath == "" {
		return content
	}
	return strings.ReplaceAll(content, RouteConfigPlaceholder, m.RouteConfigPath)
}

func checksum(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func bundleChecksum(mainConfig string, routeConfig string, supportFiles []protocol.SupportFile) string {
	files := append([]protocol.SupportFile(nil), supportFiles...)
	sort.Slice(files, func(i int, j int) bool {
		return files[i].Path < files[j].Path
	})
	var builder strings.Builder
	builder.WriteString(mainConfig)
	builder.WriteString("\n--route-config--\n")
	builder.WriteString(routeConfig)
	builder.WriteString("\n--support-files--\n")
	for _, file := range files {
		builder.WriteString(file.Path)
		builder.WriteString("\n")
		builder.WriteString(file.Content)
		builder.WriteString("\n")
	}
	return checksum(builder.String())
}
