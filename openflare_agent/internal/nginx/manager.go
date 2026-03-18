package nginx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"openflare-agent/internal/protocol"
)

const CertDirPlaceholder = "__OPENFLARE_CERT_DIR__"
const RouteConfigPlaceholder = "__OPENFLARE_ROUTE_CONFIG__"
const AccessLogPlaceholder = "__OPENFLARE_ACCESS_LOG__"
const LuaDirPlaceholder = "__OPENFLARE_LUA_DIR__"
const ObservabilityListenPlaceholder = "__OPENFLARE_OBSERVABILITY_LISTEN__"
const ObservabilityPortPlaceholder = "__OPENFLARE_OBSERVABILITY_PORT__"
const ResolverDirectivePlaceholder = "__OPENFLARE_RESOLVER_DIRECTIVE__"
const DockerMainConfigPath = "/usr/local/openresty/nginx/conf/nginx.conf"
const DockerRouteConfigPath = "/etc/nginx/conf.d/openflare_routes.conf"
const DockerAccessLogPath = "/etc/nginx/conf.d/openflare_access.log"

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
	slog.Debug("running openresty test with binary", "path", e.Path)
	output, err := e.Runner.Run(ctx, e.Path, "-t")
	if err != nil {
		return fmt.Errorf("openresty -t failed: %w: %s", err, string(output))
	}
	slog.Debug("openresty test succeeded with binary", "path", e.Path)
	return nil
}

func (e *PathExecutor) Reload(ctx context.Context) error {
	slog.Debug("running openresty reload with binary", "path", e.Path)
	output, err := e.Runner.Run(ctx, e.Path, "-s", "reload")
	if err != nil {
		return fmt.Errorf("openresty reload failed: %w: %s", err, string(output))
	}
	slog.Debug("openresty reload succeeded with binary", "path", e.Path)
	return nil
}

func (e *PathExecutor) EnsureRuntime(ctx context.Context, recreate bool) error {
	return nil
}

func (e *PathExecutor) CheckHealth(ctx context.Context) error {
	return e.Test(ctx)
}

func (e *PathExecutor) Restart(ctx context.Context) error {
	slog.Info("restarting openresty with binary", "path", e.Path)
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
	slog.Info("openresty restart succeeded with binary", "path", e.Path)
	return nil
}

type DockerExecutor struct {
	DockerBinary               string
	ContainerName              string
	Image                      string
	MainConfigPath             string
	RouteConfigDir             string
	CertDir                    string
	NginxCertDir               string
	LuaDir                     string
	NginxLuaDir                string
	OpenrestyObservabilityPort int
	Runner                     CommandRunner
}

func (e *DockerExecutor) Test(ctx context.Context) error {
	slog.Debug("running docker openresty test", "container", e.ContainerName, "image", e.Image)
	if err := e.validateMountSources(); err != nil {
		return err
	}
	output, err := e.runEphemeralRuntimeCommand(ctx, "-t")
	if err != nil {
		return fmt.Errorf("docker %s -t failed: %w: %s", dockerRuntimeCommand, err, string(output))
	}
	slog.Debug("docker openresty test succeeded", "container", e.ContainerName, "runtime", dockerRuntimeCommand)
	return nil
}

func (e *DockerExecutor) Reload(ctx context.Context) error {
	if err := e.validateMountSources(); err != nil {
		return err
	}
	output, err := e.Runner.Run(ctx, e.DockerBinary, "inspect", "-f", "{{.State.Running}}", e.ContainerName)
	if err != nil || strings.TrimSpace(string(output)) != "true" {
		return e.EnsureRuntime(ctx, false)
	}
	output, err = e.Runner.Run(ctx, e.DockerBinary, "exec", e.ContainerName, dockerRuntimeCommand, "-s", "reload")
	if err != nil {
		if e.shouldRecreateAfterReloadFailure(string(output)) {
			slog.Warn("docker openresty reload failed due to missing mounted files, recreating container", "container", e.ContainerName)
			if recreateErr := e.EnsureRuntime(ctx, true); recreateErr != nil {
				return fmt.Errorf("docker exec %s reload failed: %w: %s; recreate failed: %v", dockerRuntimeCommand, err, string(output), recreateErr)
			}
			return nil
		}
		return fmt.Errorf("docker exec %s reload failed: %w: %s", dockerRuntimeCommand, err, string(output))
	}
	return nil
}

func (e *DockerExecutor) EnsureRuntime(ctx context.Context, recreate bool) error {
	slog.Info("ensuring docker openresty runtime", "container", e.ContainerName, "recreate", recreate)
	output, err := e.Runner.Run(ctx, e.DockerBinary, "inspect", "-f", "{{.State.Running}}", e.ContainerName)
	if err == nil {
		if recreate {
			if err := e.removeContainer(ctx); err != nil {
				return err
			}
			return e.runContainer(ctx)
		}
		if strings.TrimSpace(string(output)) == "true" {
			slog.Debug("docker openresty runtime already healthy", "container", e.ContainerName)
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
	slog.Debug("checking docker openresty runtime health", "container", e.ContainerName)
	output, err := e.Runner.Run(ctx, e.DockerBinary, "inspect", "-f", "{{.State.Running}}", e.ContainerName)
	if err != nil {
		return fmt.Errorf("docker inspect openresty failed: %w: %s", err, string(output))
	}
	if strings.TrimSpace(string(output)) != "true" {
		return e.containerNotRunningError(ctx)
	}
	return nil
}

func (e *DockerExecutor) Restart(ctx context.Context) error {
	return e.EnsureRuntime(ctx, true)
}

func (e *DockerExecutor) removeContainer(ctx context.Context) error {
	slog.Info("removing docker openresty container", "container", e.ContainerName)
	output, err := e.Runner.Run(ctx, e.DockerBinary, "rm", "-f", e.ContainerName)
	if err != nil {
		text := string(output)
		if strings.Contains(text, "No such container") {
			return nil
		}
		return fmt.Errorf("docker rm openresty failed: %w: %s", err, text)
	}
	slog.Info("docker openresty container removed", "container", e.ContainerName)
	return nil
}

func (e *DockerExecutor) runContainer(ctx context.Context) error {
	slog.Info("starting docker openresty container", "container", e.ContainerName, "image", e.Image)
	if err := e.validateMountSources(); err != nil {
		return err
	}
	runArgs := []string{
		"run", "-d",
		"--name", e.ContainerName,
		"-p", "80:80",
		"-p", "443:443",
		"-p", fmt.Sprintf("127.0.0.1:%d:%d", e.OpenrestyObservabilityPort, e.OpenrestyObservabilityPort),
		"-v", fmt.Sprintf("%s:%s", e.MainConfigPath, DockerMainConfigPath),
		"-v", fmt.Sprintf("%s:/etc/nginx/conf.d", e.RouteConfigDir),
		"-v", fmt.Sprintf("%s:%s", e.CertDir, e.NginxCertDir),
		"-v", fmt.Sprintf("%s:%s", e.LuaDir, e.NginxLuaDir),
		e.Image,
	}
	runOutput, runErr := e.Runner.Run(ctx, e.DockerBinary, runArgs...)
	if runErr != nil {
		return fmt.Errorf("docker run openresty failed: %w: %s", runErr, string(runOutput))
	}
	if err := e.CheckHealth(ctx); err != nil {
		return err
	}
	slog.Info("docker openresty container started", "container", e.ContainerName)
	return nil
}

func (e *DockerExecutor) validateMountSources() error {
	if err := ensureRegularFile(e.MainConfigPath, "openresty main config"); err != nil {
		return err
	}
	if err := ensureDirectory(e.RouteConfigDir, "openresty route config dir"); err != nil {
		return err
	}
	if err := ensureDirectory(e.CertDir, "openresty cert dir"); err != nil {
		return err
	}
	if err := ensureDirectory(e.LuaDir, "openresty lua dir"); err != nil {
		return err
	}
	return nil
}

func (e *DockerExecutor) shouldRecreateAfterReloadFailure(output string) bool {
	text := strings.ToLower(strings.TrimSpace(output))
	if text == "" {
		return false
	}
	if !strings.Contains(text, "no such file") && !strings.Contains(text, "cannot load certificate") {
		return false
	}
	paths := []string{
		strings.ToLower(e.NginxCertDir),
		strings.ToLower(e.NginxLuaDir),
		strings.ToLower(DockerMainConfigPath),
		strings.ToLower("/etc/nginx/conf.d"),
	}
	for _, path := range paths {
		if strings.TrimSpace(path) != "" && strings.Contains(text, path) {
			return true
		}
	}
	return false
}

func (e *DockerExecutor) containerNotRunningError(ctx context.Context) error {
	inspectSummary := ""
	inspectOutput, inspectErr := e.Runner.Run(ctx, e.DockerBinary, "inspect", "-f", "status={{.State.Status}} exit_code={{.State.ExitCode}} error={{printf \"%q\" .State.Error}} oom_killed={{.State.OOMKilled}} finished_at={{.State.FinishedAt}}", e.ContainerName)
	if inspectErr == nil {
		inspectSummary = strings.TrimSpace(string(inspectOutput))
	}

	logTail := ""
	logOutput, logErr := e.Runner.Run(ctx, e.DockerBinary, "logs", "--tail", "50", e.ContainerName)
	if logErr == nil {
		logTail = strings.TrimSpace(string(logOutput))
	}

	message := "docker openresty container is not running"
	if inspectSummary != "" {
		message += ": " + inspectSummary
	}
	if logTail != "" {
		message += "; recent logs: " + compactDiagnosticText(logTail)
	}
	return errors.New(message)
}

func compactDiagnosticText(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) > 8 {
		lines = lines[len(lines)-8:]
	}
	joined := strings.Join(lines, " | ")
	joined = strings.Join(strings.Fields(joined), " ")
	if len(joined) > 800 {
		return joined[len(joined)-800:]
	}
	return joined
}

func ensureRegularFile(path string, label string) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return fmt.Errorf("%s path is empty", label)
	}
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s %q does not exist; run a config apply first so Docker does not create a directory mount source", label, cleanPath)
		}
		return fmt.Errorf("stat %s %q failed: %w", label, cleanPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s %q is a directory; expected a file for Docker bind mount", label, cleanPath)
	}
	return nil
}

func ensureDirectory(path string, label string) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return fmt.Errorf("%s path is empty", label)
	}
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s %q does not exist; expected a directory for Docker bind mount", label, cleanPath)
		}
		return fmt.Errorf("stat %s %q failed: %w", label, cleanPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s %q is not a directory; expected a directory for Docker bind mount", label, cleanPath)
	}
	return nil
}

type Manager struct {
	MainConfigPath               string
	RouteConfigPath              string
	RuntimeRouteConfigPath       string
	CertDir                      string
	NginxCertDir                 string
	LuaDir                       string
	NginxLuaDir                  string
	OpenrestyObservabilityListen string
	OpenrestyObservabilityPort   int
	OpenrestyResolverDirective   string
	Executor                     Executor
}

type ApplyStatus string

const (
	ApplyStatusSuccess ApplyStatus = "success"
	ApplyStatusWarning ApplyStatus = "warning"
	ApplyStatusFatal   ApplyStatus = "fatal"
)

type ApplyOutcome struct {
	Status  ApplyStatus
	Message string
}

func (m *Manager) Apply(ctx context.Context, mainConfig string, routeConfig string, supportFiles []protocol.SupportFile) ApplyOutcome {
	slog.Info("openresty apply started", "main_config", m.MainConfigPath, "route_config", m.RouteConfigPath, "cert_files", len(supportFiles))
	backup, err := m.backup()
	if err != nil {
		return fatalApplyOutcome(fmt.Errorf("backup openresty config failed: %w", err))
	}
	if err = m.writeTargetFiles(mainConfig, routeConfig, supportFiles); err != nil {
		return m.rollbackAfterFailedApply(ctx, backup, fmt.Errorf("write openresty config failed: %w", err))
	}
	if err = m.activateConfig(ctx); err != nil {
		return m.rollbackAfterFailedApply(ctx, backup, fmt.Errorf("activate openresty runtime failed: %w", err))
	}
	slog.Info("openresty apply completed successfully", "main_config", m.MainConfigPath, "route_config", m.RouteConfigPath)
	return ApplyOutcome{Status: ApplyStatusSuccess}
}

func (m *Manager) writeTargetFiles(mainConfig string, routeConfig string, supportFiles []protocol.SupportFile) error {
	if err := m.EnsureLuaAssets(); err != nil {
		return err
	}
	if err := m.writeCertFiles(supportFiles); err != nil {
		return err
	}
	if strings.TrimSpace(m.OpenrestyResolverDirective) == "" && strings.Contains(routeConfig, "set $openflare_upstream ") {
		slog.Warn("runtime-resolved hostname upstreams detected without available resolvers; hostname origin requests may fail until resolvers are configured")
	}
	renderedMainConfig := m.renderMainConfig(mainConfig)
	if err := os.WriteFile(m.MainConfigPath, []byte(renderedMainConfig), 0o644); err != nil {
		return err
	}
	renderedRouteConfig := m.renderRouteConfig(routeConfig)
	if err := os.WriteFile(m.RouteConfigPath, []byte(renderedRouteConfig), 0o644); err != nil {
		return err
	}
	return nil
}

func (m *Manager) activateConfig(ctx context.Context) error {
	if m.Executor == nil {
		return errors.New("executor 未配置")
	}
	if _, ok := m.Executor.(*DockerExecutor); ok {
		return m.Executor.EnsureRuntime(ctx, true)
	}
	return m.Executor.Reload(ctx)
}

func (m *Manager) rollbackAfterFailedApply(ctx context.Context, backup *backupState, applyErr error) ApplyOutcome {
	slog.Warn("openresty apply failed, restoring previous config", "error", applyErr)
	if err := m.restore(backup); err != nil {
		return fatalApplyOutcome(fmt.Errorf("restore openresty backup failed after apply error %v: %w", applyErr, err))
	}
	if err := m.activateConfig(ctx); err != nil {
		return fatalApplyOutcome(fmt.Errorf("apply failed: %v; rollback recovery failed: %w", applyErr, err))
	}
	message := fmt.Sprintf("apply failed, rolled back to previous config: %v", applyErr)
	slog.Warn("openresty apply rolled back successfully", "message", message)
	return ApplyOutcome{
		Status:  ApplyStatusWarning,
		Message: message,
	}
}

func fatalApplyOutcome(err error) ApplyOutcome {
	if err == nil {
		return ApplyOutcome{Status: ApplyStatusFatal}
	}
	return ApplyOutcome{
		Status:  ApplyStatusFatal,
		Message: strings.TrimSpace(err.Error()),
	}
}

func (m *Manager) EnsureLuaAssets() error {
	if strings.TrimSpace(m.LuaDir) == "" {
		return nil
	}
	files := make([]managedFile, 0, len(ManagedObservabilityLuaFiles()))
	for _, file := range ManagedObservabilityLuaFiles() {
		targetPath, err := luaFileTargetPath(m.LuaDir, file.Path)
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(m.LuaDir, targetPath)
		if err != nil {
			return err
		}
		files = append(files, managedFile{
			Path:    filepath.ToSlash(relativePath),
			Content: []byte(file.Content),
			Mode:    0o644,
		})
	}
	return syncManagedFiles(m.LuaDir, files)
}

func (m *Manager) EnsureRuntime(ctx context.Context, recreate bool) error {
	if m.Executor == nil {
		return errors.New("executor 未配置")
	}
	slog.Info("openresty ensure runtime requested", "recreate", recreate)
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
	slog.Info("openresty restart requested")
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
	if includePath := m.routeConfigIncludePath(); includePath != "" {
		normalizedMain = strings.ReplaceAll(normalizedMain, includePath, RouteConfigPlaceholder)
	}
	if accessLogPath := m.accessLogRuntimePath(); accessLogPath != "" {
		normalizedMain = strings.ReplaceAll(normalizedMain, accessLogPath, AccessLogPlaceholder)
	}
	if luaDir := m.luaRuntimePath(); luaDir != "" {
		normalizedMain = strings.ReplaceAll(normalizedMain, luaDir, LuaDirPlaceholder)
	}
	if listen := strings.TrimSpace(m.OpenrestyObservabilityListen); listen != "" {
		normalizedMain = strings.ReplaceAll(normalizedMain, listen, ObservabilityListenPlaceholder)
	}
	if m.OpenrestyObservabilityPort > 0 {
		normalizedMain = strings.ReplaceAll(normalizedMain, fmt.Sprintf("%d", m.OpenrestyObservabilityPort), ObservabilityPortPlaceholder)
	}
	if resolverDirective := strings.TrimSpace(m.OpenrestyResolverDirective); resolverDirective != "" {
		normalizedMain = strings.ReplaceAll(normalizedMain, resolverDirective, ResolverDirectivePlaceholder)
	}
	normalizedRoute := string(data)
	if m.NginxCertDir != "" {
		normalizedRoute = strings.ReplaceAll(normalizedRoute, m.NginxCertDir, CertDirPlaceholder)
	}
	files, err := m.readCertFiles()
	if err != nil {
		return "", err
	}
	result := bundleChecksum(normalizedMain, normalizedRoute, files)
	slog.Debug("openresty current checksum calculated", "main_config", m.MainConfigPath, "route_config", m.RouteConfigPath, "checksum", result, "cert_files", len(files))
	return result, nil
}

type ExecutorOptions struct {
	NginxPath                  string
	DockerBinary               string
	ContainerName              string
	Image                      string
	MainConfigPath             string
	RouteConfigPath            string
	CertDir                    string
	NginxCertDir               string
	LuaDir                     string
	NginxLuaDir                string
	OpenrestyObservabilityPort int
}

func NewExecutor(options ExecutorOptions) Executor {
	runner := &OSCommandRunner{}
	if options.NginxPath != "" {
		return &PathExecutor{
			Path:   options.NginxPath,
			Runner: runner,
		}
	}
	mainConfigPath := options.MainConfigPath
	if mainConfigPath != "" {
		if absPath, err := filepath.Abs(mainConfigPath); err == nil {
			mainConfigPath = absPath
		}
	}
	routeConfigDir := filepath.Dir(options.RouteConfigPath)
	if options.RouteConfigPath != "" {
		if absDir, err := filepath.Abs(routeConfigDir); err == nil {
			routeConfigDir = absDir
		}
	}
	certDir := options.CertDir
	if certDir != "" {
		if absDir, err := filepath.Abs(certDir); err == nil {
			certDir = absDir
		}
	}
	luaDir := options.LuaDir
	if luaDir != "" {
		if absDir, err := filepath.Abs(luaDir); err == nil {
			luaDir = absDir
		}
	}
	return &DockerExecutor{
		DockerBinary:               options.DockerBinary,
		ContainerName:              options.ContainerName,
		Image:                      options.Image,
		MainConfigPath:             mainConfigPath,
		RouteConfigDir:             routeConfigDir,
		CertDir:                    certDir,
		NginxCertDir:               options.NginxCertDir,
		LuaDir:                     luaDir,
		NginxLuaDir:                options.NginxLuaDir,
		OpenrestyObservabilityPort: options.OpenrestyObservabilityPort,
		Runner:                     runner,
	}
}

func DetectVersion(ctx context.Context, options ExecutorOptions) string {
	version, err := detectVersion(ctx, options, &OSCommandRunner{})
	if err != nil {
		slog.Error("detect openresty version failed", "error", err)
		return ""
	}
	slog.Info("detected openresty version", "version", version)
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
		"-v",
		fmt.Sprintf("%s:%s", e.LuaDir, e.NginxLuaDir),
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

type managedFile struct {
	Path    string
	Content []byte
	Mode    fs.FileMode
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
	files, err := m.readCertFiles()
	if err != nil {
		return nil, err
	}
	state.Files = files
	slog.Debug("backup captured", "main_exists", state.MainExisted, "route_exists", state.RouteExisted, "cert_files", len(state.Files))
	return state, nil
}

func (m *Manager) restore(state *backupState) error {
	if state == nil {
		return nil
	}
	slog.Warn("restoring nginx backup", "main_existed", state.MainExisted, "route_existed", state.RouteExisted, "cert_files", len(state.Files))
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
	return m.writeManagedCertFiles(state.Files)
}

func (m *Manager) writeCertFiles(certFiles []protocol.SupportFile) error {
	if m.CertDir == "" {
		return nil
	}
	return m.writeManagedCertFiles(certFiles)
}

func (m *Manager) writeManagedCertFiles(certFiles []protocol.SupportFile) error {
	files := make([]managedFile, 0, len(certFiles))
	for _, file := range certFiles {
		targetPath, err := m.certFileTargetPath(file.Path)
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(m.CertDir, targetPath)
		if err != nil {
			return err
		}
		files = append(files, managedFile{
			Path:    filepath.ToSlash(relativePath),
			Content: []byte(file.Content),
			Mode:    certFileMode(file.Path),
		})
	}
	return syncManagedFiles(m.CertDir, files)
}

func (m *Manager) readCertFiles() ([]protocol.SupportFile, error) {
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

func (m *Manager) certFileTargetPath(relativePath string) (string, error) {
	if strings.TrimSpace(m.CertDir) == "" {
		return "", errors.New("cert dir 不能为空")
	}
	candidate := strings.TrimSpace(relativePath)
	if strings.Contains(candidate, `\`) {
		candidate = strings.ReplaceAll(candidate, `\`, "/")
	}
	normalizedPath := filepath.Clean(filepath.FromSlash(candidate))
	if normalizedPath == "." || normalizedPath == "" {
		return "", errors.New("cert file path 不能为空")
	}
	if filepath.IsAbs(normalizedPath) || filepath.VolumeName(normalizedPath) != "" {
		return "", fmt.Errorf("cert file path %q must be relative", relativePath)
	}
	targetPath := filepath.Join(m.CertDir, normalizedPath)
	relativeToBase, err := filepath.Rel(m.CertDir, targetPath)
	if err != nil {
		return "", err
	}
	if relativeToBase == ".." || strings.HasPrefix(relativeToBase, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("cert file path %q escapes cert dir", relativePath)
	}
	return targetPath, nil
}

func certFileMode(relativePath string) fs.FileMode {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(relativePath))) {
	case ".crt", ".pem":
		return 0o644
	case ".key":
		return 0o600
	default:
		return 0o644
	}
}

func luaFileTargetPath(baseDir string, relativePath string) (string, error) {
	if strings.TrimSpace(baseDir) == "" {
		return "", errors.New("lua dir 不能为空")
	}
	candidate := strings.TrimSpace(relativePath)
	if strings.Contains(candidate, `\`) {
		candidate = strings.ReplaceAll(candidate, `\`, "/")
	}
	normalizedPath := filepath.Clean(filepath.FromSlash(candidate))
	if normalizedPath == "." || normalizedPath == "" {
		return "", errors.New("lua file path 不能为空")
	}
	if filepath.IsAbs(normalizedPath) || filepath.VolumeName(normalizedPath) != "" {
		return "", fmt.Errorf("lua file path %q must be relative", relativePath)
	}
	targetPath := filepath.Join(baseDir, normalizedPath)
	relativeToBase, err := filepath.Rel(baseDir, targetPath)
	if err != nil {
		return "", err
	}
	if relativeToBase == ".." || strings.HasPrefix(relativeToBase, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("lua file path %q escapes lua dir", relativePath)
	}
	return targetPath, nil
}

func syncManagedFiles(baseDir string, files []managedFile) error {
	if strings.TrimSpace(baseDir) == "" {
		return errors.New("managed dir cannot be empty")
	}
	if info, err := os.Stat(baseDir); err == nil && !info.IsDir() {
		return fmt.Errorf("managed dir %q is not a directory", baseDir)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}

	desired := make(map[string]managedFile, len(files))
	for _, file := range files {
		cleanPath := filepath.Clean(filepath.FromSlash(strings.TrimSpace(file.Path)))
		if cleanPath == "." || cleanPath == "" {
			return errors.New("managed file path cannot be empty")
		}
		desired[cleanPath] = managedFile{
			Path:    cleanPath,
			Content: file.Content,
			Mode:    file.Mode,
		}
	}

	if err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		if _, ok := desired[filepath.Clean(relativePath)]; ok {
			return nil
		}
		return os.Remove(path)
	}); err != nil {
		return err
	}

	for _, file := range desired {
		targetPath := filepath.Join(baseDir, file.Path)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, file.Content, file.Mode); err != nil {
			return err
		}
	}

	return removeEmptyManagedDirs(baseDir)
}

func removeEmptyManagedDirs(baseDir string) error {
	dirs := make([]string, 0)
	if err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path != baseDir {
			dirs = append(dirs, path)
		}
		return nil
	}); err != nil {
		return err
	}
	sort.Slice(dirs, func(i int, j int) bool {
		return len(dirs[i]) > len(dirs[j])
	})
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) renderRouteConfig(content string) string {
	if m.NginxCertDir == "" {
		return content
	}
	return strings.ReplaceAll(content, CertDirPlaceholder, m.NginxCertDir)
}

func (m *Manager) renderMainConfig(content string) string {
	rendered := content
	if includePath := m.routeConfigIncludePath(); includePath != "" {
		rendered = strings.ReplaceAll(rendered, RouteConfigPlaceholder, includePath)
	}
	if accessLogPath := m.accessLogRuntimePath(); accessLogPath != "" {
		rendered = strings.ReplaceAll(rendered, AccessLogPlaceholder, accessLogPath)
	}
	if luaDir := m.luaRuntimePath(); luaDir != "" {
		rendered = strings.ReplaceAll(rendered, LuaDirPlaceholder, luaDir)
	}
	if listen := strings.TrimSpace(m.OpenrestyObservabilityListen); listen != "" {
		rendered = strings.ReplaceAll(rendered, ObservabilityListenPlaceholder, listen)
	}
	if m.OpenrestyObservabilityPort > 0 {
		rendered = strings.ReplaceAll(rendered, ObservabilityPortPlaceholder, fmt.Sprintf("%d", m.OpenrestyObservabilityPort))
	}
	if resolverDirective := strings.TrimSpace(m.OpenrestyResolverDirective); resolverDirective != "" {
		rendered = strings.ReplaceAll(rendered, ResolverDirectivePlaceholder, resolverDirective)
	}
	return rendered
}

func ObservabilityListenAddress(openrestyPath string, port int) string {
	if port <= 0 {
		return ""
	}
	if strings.TrimSpace(openrestyPath) != "" {
		return fmt.Sprintf("127.0.0.1:%d", port)
	}
	return fmt.Sprintf("%d", port)
}

func ResolverDirective(openrestyPath string, explicitResolvers []string) string {
	resolvers := resolverAddresses(openrestyPath, explicitResolvers)
	if len(resolvers) == 0 {
		return ""
	}
	return fmt.Sprintf("    resolver %s valid=30s ipv6=off;\n    resolver_timeout 5s;\n", strings.Join(resolvers, " "))
}

func resolverAddresses(openrestyPath string, explicitResolvers []string) []string {
	if resolvers := normalizeResolverAddresses(explicitResolvers); len(resolvers) > 0 {
		return resolvers
	}
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	return parseResolverAddresses(string(data), strings.TrimSpace(openrestyPath) == "")
}

func parseResolverAddresses(content string, dockerMode bool) []string {
	lines := strings.Split(content, "\n")
	resolvers := make([]string, 0, 2)
	seen := make(map[string]struct{})
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 || fields[0] != "nameserver" {
			continue
		}
		addr := strings.TrimSpace(fields[1])
		if addr == "" {
			continue
		}
		if dockerMode && !isUsableDockerResolver(addr) {
			continue
		}
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		resolvers = append(resolvers, addr)
	}
	return resolvers
}

func isUsableDockerResolver(addr string) bool {
	ip := net.ParseIP(addr)
	if ip == nil {
		return false
	}
	return !ip.IsLoopback() && !ip.IsUnspecified()
}

func normalizeResolverAddresses(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	resolvers := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		addr := strings.TrimSpace(value)
		if addr == "" {
			continue
		}
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		resolvers = append(resolvers, addr)
	}
	if len(resolvers) == 0 {
		return nil
	}
	return resolvers
}

func RequiresRuntimeResolver(originURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(originURL))
	if err != nil || parsed.Hostname() == "" {
		return false
	}
	return net.ParseIP(parsed.Hostname()) == nil
}

func (m *Manager) routeConfigIncludePath() string {
	if strings.TrimSpace(m.RuntimeRouteConfigPath) != "" {
		return strings.TrimSpace(m.RuntimeRouteConfigPath)
	}
	return strings.TrimSpace(m.RouteConfigPath)
}

func (m *Manager) accessLogRuntimePath() string {
	includePath := m.routeConfigIncludePath()
	if strings.TrimSpace(includePath) == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join(filepath.Dir(includePath), "openflare_access.log"))
}

func (m *Manager) luaRuntimePath() string {
	if strings.TrimSpace(m.NginxLuaDir) == "" {
		return ""
	}
	return filepath.ToSlash(m.NginxLuaDir)
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
