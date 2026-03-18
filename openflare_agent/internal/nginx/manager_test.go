package nginx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"openflare-agent/internal/protocol"
)

type runCall struct {
	name string
	args []string
}

type fakeRunner struct {
	calls []runCall
	runFn func(name string, args ...string) ([]byte, error)
}

type fakeExecutor struct {
	testErr   error
	reloadErr error
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, runCall{name: name, args: append([]string{}, args...)})
	if r.runFn != nil {
		return r.runFn(name, args...)
	}
	return nil, nil
}

func (e *fakeExecutor) Test(ctx context.Context) error {
	return e.testErr
}

func (e *fakeExecutor) Reload(ctx context.Context) error {
	return e.reloadErr
}

func (e *fakeExecutor) EnsureRuntime(ctx context.Context, recreate bool) error {
	return nil
}

func (e *fakeExecutor) CheckHealth(ctx context.Context) error {
	return e.testErr
}

func (e *fakeExecutor) Restart(ctx context.Context) error {
	return e.reloadErr
}

func TestPathExecutorCommands(t *testing.T) {
	runner := &fakeRunner{}
	executor := &PathExecutor{
		Path:   "/usr/local/openresty/nginx/sbin/openresty",
		Runner: runner,
	}

	if err := executor.Test(context.Background()); err != nil {
		t.Fatalf("Test failed: %v", err)
	}
	if err := executor.Reload(context.Background()); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	expected := []runCall{
		{name: "/usr/local/openresty/nginx/sbin/openresty", args: []string{"-t"}},
		{name: "/usr/local/openresty/nginx/sbin/openresty", args: []string{"-s", "reload"}},
	}
	if !reflect.DeepEqual(runner.calls, expected) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}
}

func TestPathExecutorEnsureRuntimeNoop(t *testing.T) {
	executor := &PathExecutor{
		Path:   "/usr/local/openresty/nginx/sbin/openresty",
		Runner: &fakeRunner{},
	}
	if err := executor.EnsureRuntime(context.Background(), true); err != nil {
		t.Fatalf("EnsureRuntime failed: %v", err)
	}
}

func TestPathExecutorRestartIgnoresMissingPID(t *testing.T) {
	runner := &fakeRunner{
		runFn: func(name string, args ...string) ([]byte, error) {
			if len(args) == 2 && args[0] == "-s" && args[1] == "quit" {
				return []byte("openresty: [error] invalid PID number \"\" in \"/usr/local/openresty/nginx/logs/nginx.pid\""), errors.New("exit status 1")
			}
			return []byte(""), nil
		},
	}
	executor := &PathExecutor{
		Path:   "/usr/local/openresty/nginx/sbin/openresty",
		Runner: runner,
	}
	if err := executor.Restart(context.Background()); err != nil {
		t.Fatalf("Restart failed: %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 restart calls, got %d", len(runner.calls))
	}
}

func TestDockerExecutorCheckHealthFailsWhenContainerStopped(t *testing.T) {
	runner := &fakeRunner{
		runFn: func(name string, args ...string) ([]byte, error) {
			if len(args) >= 4 && args[0] == "inspect" && args[2] == "{{.State.Running}}" {
				return []byte("false"), nil
			}
			if len(args) >= 4 && args[0] == "inspect" {
				return []byte("status=exited exit_code=1 error=\"\" oom_killed=false finished_at=2026-03-18T10:08:30Z"), nil
			}
			if len(args) >= 1 && args[0] == "logs" {
				return []byte("nginx: [emerg] host not found in upstream \"c1\" in /etc/nginx/conf.d/openflare_routes.conf:30"), nil
			}
			return []byte("false"), nil
		},
	}
	executor := &DockerExecutor{
		DockerBinary:   "docker",
		ContainerName:  "openflare-openresty",
		Image:          "openresty/openresty:alpine",
		MainConfigPath: filepath.Clean("/tmp/nginx.conf"),
		RouteConfigDir: filepath.Clean("/tmp/routes"),
		CertDir:        filepath.Clean("/tmp/certs"),
		NginxCertDir:   "/etc/nginx/openflare-certs",
		LuaDir:         filepath.Clean("/tmp/lua"),
		NginxLuaDir:    "/etc/nginx/openflare-lua",
		Runner:         runner,
	}
	if err := executor.CheckHealth(context.Background()); err == nil {
		t.Fatal("expected CheckHealth to fail when container is not running")
	} else {
		text := err.Error()
		if !strings.Contains(text, "exit_code=1") {
			t.Fatalf("expected exit code in health error, got %v", err)
		}
		if !strings.Contains(text, "host not found in upstream") {
			t.Fatalf("expected recent docker logs in health error, got %v", err)
		}
	}
}

func prepareDockerMountSources(t *testing.T) (string, string, string, string) {
	t.Helper()
	tempDir := t.TempDir()
	mainConfigPath := filepath.Join(tempDir, "nginx.conf")
	routeConfigDir := filepath.Join(tempDir, "conf.d")
	certDir := filepath.Join(tempDir, "certs")
	luaDir := filepath.Join(tempDir, "lua")

	if err := os.WriteFile(mainConfigPath, []byte("events {}\nhttp {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	for _, dir := range []string{routeConfigDir, certDir, luaDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}
	}
	return mainConfigPath, routeConfigDir, certDir, luaDir
}

func TestDockerExecutorStartsContainerWhenMissing(t *testing.T) {
	mainConfigPath, routeConfigDir, certDir, luaDir := prepareDockerMountSources(t)
	runner := &fakeRunner{
		runFn: func(name string, args ...string) ([]byte, error) {
			if len(args) >= 1 && args[0] == "inspect" {
				return []byte(""), errors.New("not found")
			}
			return []byte("ok"), nil
		},
	}
	executor := &DockerExecutor{
		DockerBinary:   "docker",
		ContainerName:  "openflare-openresty",
		Image:          "openresty/openresty:alpine",
		MainConfigPath: mainConfigPath,
		RouteConfigDir: routeConfigDir,
		CertDir:        certDir,
		NginxCertDir:   "/etc/nginx/openflare-certs",
		LuaDir:         luaDir,
		NginxLuaDir:    "/etc/nginx/openflare-lua",
		Runner:         runner,
	}

	if err := executor.Test(context.Background()); err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.calls))
	}
	if runner.calls[0].args[0] != "run" || runner.calls[0].args[1] != "--rm" {
		t.Fatalf("expected docker run --rm for test, got %#v", runner.calls[0])
	}
	if runner.calls[0].args[len(runner.calls[0].args)-2] != "openresty" {
		t.Fatalf("expected docker test command to invoke openresty, got %#v", runner.calls[0])
	}
}

func TestDockerExecutorStartsStoppedContainer(t *testing.T) {
	mainConfigPath, routeConfigDir, certDir, luaDir := prepareDockerMountSources(t)
	runner := &fakeRunner{
		runFn: func(name string, args ...string) ([]byte, error) {
			if len(args) >= 2 && args[0] == "inspect" {
				return []byte("false"), nil
			}
			return []byte("ok"), nil
		},
	}
	executor := &DockerExecutor{
		DockerBinary:   "docker",
		ContainerName:  "openflare-openresty",
		Image:          "openresty/openresty:alpine",
		MainConfigPath: mainConfigPath,
		RouteConfigDir: routeConfigDir,
		CertDir:        certDir,
		NginxCertDir:   "/etc/nginx/openflare-certs",
		LuaDir:         luaDir,
		NginxLuaDir:    "/etc/nginx/openflare-lua",
		Runner:         runner,
	}

	if err := executor.Reload(context.Background()); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if len(runner.calls) != 4 {
		t.Fatalf("expected 4 calls, got %d", len(runner.calls))
	}
	if runner.calls[0].args[0] != "inspect" {
		t.Fatalf("expected docker inspect on first call, got %#v", runner.calls[0])
	}
	if runner.calls[1].args[0] != "inspect" {
		t.Fatalf("expected docker inspect on second call, got %#v", runner.calls[1])
	}
	if runner.calls[2].args[0] != "rm" {
		t.Fatalf("expected docker rm on third call, got %#v", runner.calls[2])
	}
	if runner.calls[3].args[0] != "run" {
		t.Fatalf("expected docker run on fourth call, got %#v", runner.calls[3])
	}
}

func TestDockerExecutorReloadsRunningContainerInPlace(t *testing.T) {
	mainConfigPath, routeConfigDir, certDir, luaDir := prepareDockerMountSources(t)
	runner := &fakeRunner{
		runFn: func(name string, args ...string) ([]byte, error) {
			if len(args) >= 1 && args[0] == "inspect" {
				return []byte("true"), nil
			}
			return []byte("ok"), nil
		},
	}
	executor := &DockerExecutor{
		DockerBinary:   "docker",
		ContainerName:  "openflare-openresty",
		Image:          "openresty/openresty:alpine",
		MainConfigPath: mainConfigPath,
		RouteConfigDir: routeConfigDir,
		CertDir:        certDir,
		NginxCertDir:   "/etc/nginx/openflare-certs",
		LuaDir:         luaDir,
		NginxLuaDir:    "/etc/nginx/openflare-lua",
		Runner:         runner,
	}

	if err := executor.Reload(context.Background()); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	expected := []runCall{
		{name: "docker", args: []string{"inspect", "-f", "{{.State.Running}}", "openflare-openresty"}},
		{name: "docker", args: []string{"exec", "openflare-openresty", "openresty", "-s", "reload"}},
	}
	if !reflect.DeepEqual(runner.calls, expected) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}
}

func TestDockerExecutorRunContainerMountsManagedFiles(t *testing.T) {
	mainConfigPath, routeConfigDir, certDir, luaDir := prepareDockerMountSources(t)
	runner := &fakeRunner{}
	executor := &DockerExecutor{
		DockerBinary:               "docker",
		ContainerName:              "openflare-openresty",
		Image:                      "openresty/openresty:alpine",
		MainConfigPath:             mainConfigPath,
		RouteConfigDir:             routeConfigDir,
		CertDir:                    certDir,
		NginxCertDir:               "/etc/nginx/openflare-certs",
		LuaDir:                     luaDir,
		NginxLuaDir:                "/etc/nginx/openflare-lua",
		OpenrestyObservabilityPort: 18081,
		Runner:                     runner,
	}

	if err := executor.runContainer(context.Background()); err != nil {
		t.Fatalf("runContainer failed: %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected one docker run call, got %d", len(runner.calls))
	}

	expectedArgs := []string{
		"run", "-d",
		"--name", "openflare-openresty",
		"-p", "80:80",
		"-p", "443:443",
		"-p", "127.0.0.1:18081:18081",
		"-v", mainConfigPath + ":" + DockerMainConfigPath,
		"-v", routeConfigDir + ":/etc/nginx/conf.d",
		"-v", certDir + ":/etc/nginx/openflare-certs",
		"-v", luaDir + ":/etc/nginx/openflare-lua",
		"openresty/openresty:alpine",
	}
	if !reflect.DeepEqual(runner.calls[0].args, expectedArgs) {
		t.Fatalf("unexpected docker run args: %#v", runner.calls[0].args)
	}
}

func TestDockerExecutorRecreatesContainerOnStartup(t *testing.T) {
	mainConfigPath, routeConfigDir, certDir, luaDir := prepareDockerMountSources(t)
	runner := &fakeRunner{
		runFn: func(name string, args ...string) ([]byte, error) {
			if len(args) >= 1 && args[0] == "inspect" {
				return []byte("true"), nil
			}
			return []byte("ok"), nil
		},
	}
	executor := &DockerExecutor{
		DockerBinary:               "docker",
		ContainerName:              "openflare-openresty",
		Image:                      "openresty/openresty:alpine",
		MainConfigPath:             mainConfigPath,
		RouteConfigDir:             routeConfigDir,
		CertDir:                    certDir,
		NginxCertDir:               "/etc/nginx/openflare-certs",
		LuaDir:                     luaDir,
		NginxLuaDir:                "/etc/nginx/openflare-lua",
		OpenrestyObservabilityPort: 18081,
		Runner:                     runner,
	}

	if err := executor.EnsureRuntime(context.Background(), true); err != nil {
		t.Fatalf("EnsureRuntime failed: %v", err)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(runner.calls))
	}
	if runner.calls[1].args[0] != "rm" {
		t.Fatalf("expected docker rm on second call, got %#v", runner.calls[1])
	}
	if runner.calls[2].args[0] != "run" {
		t.Fatalf("expected docker run on third call, got %#v", runner.calls[2])
	}
}

func TestDockerExecutorRunContainerRejectsMissingMainConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	routeConfigDir := filepath.Join(tempDir, "conf.d")
	certDir := filepath.Join(tempDir, "certs")
	luaDir := filepath.Join(tempDir, "lua")
	for _, dir := range []string{routeConfigDir, certDir, luaDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}
	}

	executor := &DockerExecutor{
		DockerBinary:   "docker",
		ContainerName:  "openflare-openresty",
		Image:          "openresty/openresty:alpine",
		MainConfigPath: filepath.Join(tempDir, "nginx.conf"),
		RouteConfigDir: routeConfigDir,
		CertDir:        certDir,
		NginxCertDir:   "/etc/nginx/openflare-certs",
		LuaDir:         luaDir,
		NginxLuaDir:    "/etc/nginx/openflare-lua",
		Runner:         &fakeRunner{},
	}

	err := executor.runContainer(context.Background())
	if err == nil {
		t.Fatal("expected missing main config file to be rejected")
	}
	if !strings.Contains(err.Error(), "run a config apply first") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDockerExecutorRunContainerRejectsMainConfigDirectory(t *testing.T) {
	tempDir := t.TempDir()
	mainConfigPath := filepath.Join(tempDir, "nginx.conf")
	routeConfigDir := filepath.Join(tempDir, "conf.d")
	certDir := filepath.Join(tempDir, "certs")
	luaDir := filepath.Join(tempDir, "lua")
	for _, dir := range []string{mainConfigPath, routeConfigDir, certDir, luaDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}
	}

	executor := &DockerExecutor{
		DockerBinary:   "docker",
		ContainerName:  "openflare-openresty",
		Image:          "openresty/openresty:alpine",
		MainConfigPath: mainConfigPath,
		RouteConfigDir: routeConfigDir,
		CertDir:        certDir,
		NginxCertDir:   "/etc/nginx/openflare-certs",
		LuaDir:         luaDir,
		NginxLuaDir:    "/etc/nginx/openflare-lua",
		Runner:         &fakeRunner{},
	}

	err := executor.runContainer(context.Background())
	if err == nil {
		t.Fatal("expected main config directory to be rejected")
	}
	if !strings.Contains(err.Error(), "expected a file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewExecutorUsesAbsoluteDockerMountPath(t *testing.T) {
	executor := NewExecutor(ExecutorOptions{
		DockerBinary:               "docker",
		ContainerName:              "openflare-openresty",
		Image:                      "openresty/openresty:alpine",
		MainConfigPath:             "./data/etc/nginx/nginx.conf",
		RouteConfigPath:            "./data/etc/nginx/conf.d/openflare_routes.conf",
		CertDir:                    "./data/etc/nginx/certs",
		NginxCertDir:               "/etc/nginx/openflare-certs",
		LuaDir:                     "./data/etc/nginx/lua",
		NginxLuaDir:                "/etc/nginx/openflare-lua",
		OpenrestyObservabilityPort: 18081,
	})

	dockerExecutor, ok := executor.(*DockerExecutor)
	if !ok {
		t.Fatal("expected docker executor")
	}
	if !filepath.IsAbs(dockerExecutor.RouteConfigDir) {
		t.Fatalf("expected absolute route config dir, got %s", dockerExecutor.RouteConfigDir)
	}
	if !filepath.IsAbs(dockerExecutor.MainConfigPath) {
		t.Fatalf("expected absolute main config path, got %s", dockerExecutor.MainConfigPath)
	}
	if !strings.HasSuffix(dockerExecutor.RouteConfigDir, filepath.Clean("data/etc/nginx/conf.d")) {
		t.Fatalf("unexpected route config dir: %s", dockerExecutor.RouteConfigDir)
	}
	if !strings.HasSuffix(dockerExecutor.MainConfigPath, filepath.Clean("data/etc/nginx/nginx.conf")) {
		t.Fatalf("unexpected main config path: %s", dockerExecutor.MainConfigPath)
	}
}

func TestDetectVersionFromBinary(t *testing.T) {
	version, err := detectVersion(context.Background(), ExecutorOptions{
		NginxPath: "/usr/local/openresty/nginx/sbin/openresty",
	}, &fakeRunner{
		runFn: func(name string, args ...string) ([]byte, error) {
			return []byte("nginx version: openresty/1.27.1.2\n"), nil
		},
	})
	if err != nil {
		t.Fatalf("detectVersion failed: %v", err)
	}
	if version != "1.27.1.2" {
		t.Fatalf("unexpected version: %s", version)
	}
}

func TestManagerApplyAndChecksumIncludeMainConfig(t *testing.T) {
	tempDir := t.TempDir()
	mainPath := filepath.Join(tempDir, "nginx.conf")
	routePath := filepath.Join(tempDir, "conf.d", "openflare_routes.conf")
	certDir := filepath.Join(tempDir, "certs")
	manager := &Manager{
		MainConfigPath:  mainPath,
		RouteConfigPath: routePath,
		CertDir:         certDir,
		NginxCertDir:    "/etc/nginx/openflare-certs",
		LuaDir:          filepath.Join(tempDir, "lua"),
		NginxLuaDir:     "/etc/nginx/openflare-lua",
		Executor:        &fakeExecutor{},
	}

	err := manager.Apply(
		context.Background(),
		"include __OPENFLARE_ROUTE_CONFIG__;\naccess_log __OPENFLARE_ACCESS_LOG__ openflare_json;\n",
		"ssl_certificate __OPENFLARE_CERT_DIR__/1.crt;\n",
		[]protocol.SupportFile{{Path: "1.crt", Content: "cert"}},
	)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	mainData, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("failed to read main config: %v", err)
	}
	expectedMain := "include " + routePath + ";\naccess_log " + filepath.Join(filepath.Dir(routePath), "openflare_access.log") + " openflare_json;\n"
	if string(mainData) != expectedMain {
		t.Fatalf("unexpected main config: %s", string(mainData))
	}

	routeData, err := os.ReadFile(routePath)
	if err != nil {
		t.Fatalf("failed to read route config: %v", err)
	}
	if string(routeData) != "ssl_certificate /etc/nginx/openflare-certs/1.crt;\n" {
		t.Fatalf("unexpected route config: %s", string(routeData))
	}

	value, err := manager.CurrentChecksum()
	if err != nil {
		t.Fatalf("CurrentChecksum failed: %v", err)
	}
	expected := bundleChecksum(
		"include __OPENFLARE_ROUTE_CONFIG__;\naccess_log __OPENFLARE_ACCESS_LOG__ openflare_json;\n",
		"ssl_certificate __OPENFLARE_CERT_DIR__/1.crt;\n",
		[]protocol.SupportFile{{Path: "1.crt", Content: "cert"}},
	)
	if value != expected {
		t.Fatalf("unexpected checksum: got %s want %s", value, expected)
	}
}

func TestManagerApplyUsesRuntimeRouteConfigPath(t *testing.T) {
	tempDir := t.TempDir()
	mainPath := filepath.Join(tempDir, "nginx.conf")
	routePath := filepath.Join(tempDir, "conf.d", "openflare_routes.conf")
	manager := &Manager{
		MainConfigPath:         mainPath,
		RouteConfigPath:        routePath,
		RuntimeRouteConfigPath: DockerRouteConfigPath,
		CertDir:                filepath.Join(tempDir, "certs"),
		NginxCertDir:           "/etc/nginx/openflare-certs",
		LuaDir:                 filepath.Join(tempDir, "lua"),
		NginxLuaDir:            "/etc/nginx/openflare-lua",
		Executor:               &fakeExecutor{},
	}

	if err := manager.Apply(context.Background(), "include __OPENFLARE_ROUTE_CONFIG__;\naccess_log __OPENFLARE_ACCESS_LOG__ openflare_json;\n", "server { listen 80; }\n", nil); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	mainData, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("failed to read main config: %v", err)
	}
	expectedMain := "include " + DockerRouteConfigPath + ";\naccess_log " + DockerAccessLogPath + " openflare_json;\n"
	if string(mainData) != expectedMain {
		t.Fatalf("unexpected main config include path: %s", string(mainData))
	}

	value, err := manager.CurrentChecksum()
	if err != nil {
		t.Fatalf("CurrentChecksum failed: %v", err)
	}
	expected := bundleChecksum(
		"include __OPENFLARE_ROUTE_CONFIG__;\naccess_log __OPENFLARE_ACCESS_LOG__ openflare_json;\n",
		"server { listen 80; }\n",
		nil,
	)
	if value != expected {
		t.Fatalf("unexpected checksum: got %s want %s", value, expected)
	}
}

func TestDetectVersionFromDockerImage(t *testing.T) {
	runner := &fakeRunner{
		runFn: func(name string, args ...string) ([]byte, error) {
			return []byte("nginx version: openresty/1.27.1.2\n"), nil
		},
	}
	version, err := detectVersion(context.Background(), ExecutorOptions{
		DockerBinary: "docker",
		Image:        "openresty/openresty:alpine",
	}, runner)
	if err != nil {
		t.Fatalf("detectVersion failed: %v", err)
	}
	if version != "1.27.1.2" {
		t.Fatalf("unexpected version: %s", version)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected one command call, got %d", len(runner.calls))
	}
	expectedArgs := []string{"run", "--rm", "openresty/openresty:alpine", "openresty", "-v"}
	if !reflect.DeepEqual(runner.calls[0].args, expectedArgs) {
		t.Fatalf("unexpected docker args: %#v", runner.calls[0].args)
	}
}

func TestParseNginxVersionIgnoresDockerEntrypointPaths(t *testing.T) {
	output := strings.Join([]string{
		"/docker-entrypoint.sh: /docker-entrypoint.d/10-listen-on-ipv6-by-default.sh: info: can not modify /etc/nginx/conf.d/default.conf (read-only file system?)",
		"nginx version: openresty/1.27.1.2",
	}, "\n")

	version := parseNginxVersion(output)
	if version != "1.27.1.2" {
		t.Fatalf("unexpected version: %s", version)
	}
}

func TestManagerApplyWritesSupportFilesAndReplacesPlaceholder(t *testing.T) {
	tempDir := t.TempDir()
	manager := &Manager{
		MainConfigPath:               filepath.Join(tempDir, "nginx.conf"),
		RouteConfigPath:              filepath.Join(tempDir, "routes.conf"),
		CertDir:                      filepath.Join(tempDir, "certs"),
		NginxCertDir:                 "/etc/nginx/openflare-certs",
		LuaDir:                       filepath.Join(tempDir, "lua"),
		NginxLuaDir:                  "/etc/nginx/openflare-lua",
		OpenrestyObservabilityListen: "18081",
		OpenrestyResolverDirective:   "    resolver 127.0.0.11 valid=30s ipv6=off;\n    resolver_timeout 5s;\n",
		Executor:                     &fakeExecutor{},
	}

	err := manager.Apply(context.Background(), "include __OPENFLARE_ROUTE_CONFIG__;\n__OPENFLARE_RESOLVER_DIRECTIVE__server { listen __OPENFLARE_OBSERVABILITY_LISTEN__; }", "ssl_certificate __OPENFLARE_CERT_DIR__/1.crt;", []protocol.SupportFile{
		{Path: "1.crt", Content: "cert-data"},
		{Path: "1.key", Content: "key-data"},
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	routeData, err := os.ReadFile(manager.RouteConfigPath)
	if err != nil {
		t.Fatalf("failed to read route config: %v", err)
	}
	if !strings.Contains(string(routeData), "/etc/nginx/openflare-certs/1.crt") {
		t.Fatalf("expected placeholder replacement in route config, got %s", string(routeData))
	}
	mainData, err := os.ReadFile(manager.MainConfigPath)
	if err != nil {
		t.Fatalf("failed to read main config: %v", err)
	}
	if !strings.Contains(string(mainData), "listen 18081;") {
		t.Fatalf("expected observability listen placeholder replacement in main config, got %s", string(mainData))
	}
	if !strings.Contains(string(mainData), "resolver 127.0.0.11 valid=30s ipv6=off;") {
		t.Fatalf("expected resolver directive placeholder replacement in main config, got %s", string(mainData))
	}
	certData, err := os.ReadFile(filepath.Join(manager.CertDir, "1.crt"))
	if err != nil {
		t.Fatalf("failed to read cert file: %v", err)
	}
	if string(certData) != "cert-data" {
		t.Fatalf("unexpected cert file content: %s", string(certData))
	}
	luaInfo, err := os.Stat(filepath.Join(manager.LuaDir, "log.lua"))
	if err != nil {
		t.Fatalf("expected managed lua file to exist, stat err = %v", err)
	}
	if luaInfo.Mode().Perm() != 0o644 {
		t.Fatalf("unexpected lua mode: %o", luaInfo.Mode().Perm())
	}
}

func TestResolverDirectiveForDockerMode(t *testing.T) {
	got := ResolverDirective("")
	if !strings.Contains(got, "resolver 127.0.0.11") {
		t.Fatalf("expected docker resolver directive, got %q", got)
	}
}

func TestCertFileMode(t *testing.T) {
	testCases := []struct {
		path string
		want os.FileMode
	}{
		{path: "1.crt", want: 0o644},
		{path: "1.pem", want: 0o644},
		{path: "1.key", want: 0o600},
		{path: "misc.txt", want: 0o644},
	}

	for _, testCase := range testCases {
		if got := certFileMode(testCase.path); got != testCase.want {
			t.Fatalf("unexpected mode for %s: got %o want %o", testCase.path, got, testCase.want)
		}
	}
}

func TestManagerEnsureLuaAssetsWritesReadableFiles(t *testing.T) {
	tempDir := t.TempDir()
	manager := &Manager{
		LuaDir:      filepath.Join(tempDir, "lua"),
		NginxLuaDir: "/etc/nginx/openflare-lua",
	}

	err := manager.EnsureLuaAssets()
	if err != nil {
		t.Fatalf("EnsureLuaAssets failed: %v", err)
	}

	luaInfo, err := os.Stat(filepath.Join(manager.LuaDir, "log.lua"))
	if err != nil {
		t.Fatalf("failed to stat lua file: %v", err)
	}
	if luaInfo.Mode().Perm() != 0o644 {
		t.Fatalf("unexpected lua mode: %o", luaInfo.Mode().Perm())
	}
}

func TestManagerRollbackRestoresCertFiles(t *testing.T) {
	tempDir := t.TempDir()
	routePath := filepath.Join(tempDir, "routes.conf")
	mainPath := filepath.Join(tempDir, "nginx.conf")
	certDir := filepath.Join(tempDir, "certs")
	if err := os.MkdirAll(certDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(mainPath, []byte("old-main"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(routePath, []byte("old-route"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(certDir, "1.crt"), []byte("old-cert"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	manager := &Manager{
		MainConfigPath:  mainPath,
		RouteConfigPath: routePath,
		CertDir:         certDir,
		NginxCertDir:    "/etc/nginx/openflare-certs",
		LuaDir:          filepath.Join(tempDir, "lua"),
		NginxLuaDir:     "/etc/nginx/openflare-lua",
		Executor: &fakeExecutor{
			reloadErr: errors.New("openresty reload failed"),
		},
	}

	err := manager.Apply(context.Background(), "new-main", "new-route", []protocol.SupportFile{
		{Path: "1.crt", Content: "new-cert"},
	})
	if err == nil {
		t.Fatal("expected Apply to fail")
	}

	mainData, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("failed to read main config: %v", err)
	}
	if string(mainData) != "old-main" {
		t.Fatalf("expected main rollback, got %s", string(mainData))
	}
	routeData, err := os.ReadFile(routePath)
	if err != nil {
		t.Fatalf("failed to read route config: %v", err)
	}
	if string(routeData) != "old-route" {
		t.Fatalf("expected route rollback, got %s", string(routeData))
	}
	certData, err := os.ReadFile(filepath.Join(certDir, "1.crt"))
	if err != nil {
		t.Fatalf("failed to read cert file: %v", err)
	}
	if string(certData) != "old-cert" {
		t.Fatalf("expected cert rollback, got %s", string(certData))
	}
}

func TestManagerCertFileTargetPathRejectsEscapes(t *testing.T) {
	manager := &Manager{CertDir: filepath.Join(t.TempDir(), "certs")}
	if err := os.MkdirAll(manager.CertDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	absolutePath := "/tmp/evil.crt"
	if runtime.GOOS == "windows" {
		absolutePath = `C:/tmp/evil.crt`
	}

	testCases := []struct {
		path      string
		shouldErr bool
	}{
		{path: "nested/1.crt", shouldErr: false},
		{path: "../escape.crt", shouldErr: true},
		{path: "..\\escape.crt", shouldErr: true},
		{path: absolutePath, shouldErr: true},
		{path: "", shouldErr: true},
	}

	for _, testCase := range testCases {
		targetPath, err := manager.certFileTargetPath(testCase.path)
		if testCase.shouldErr {
			if err == nil {
				t.Fatalf("expected path %q to be rejected, got target %q", testCase.path, targetPath)
			}
			continue
		}
		if err != nil {
			t.Fatalf("expected path %q to be accepted: %v", testCase.path, err)
		}
		if !strings.HasPrefix(targetPath, manager.CertDir) {
			t.Fatalf("expected target path %q to stay under %q", targetPath, manager.CertDir)
		}
	}
}

func TestManagerApplyRejectsCertFilePathTraversal(t *testing.T) {
	tempDir := t.TempDir()
	manager := &Manager{
		MainConfigPath:  filepath.Join(tempDir, "nginx.conf"),
		RouteConfigPath: filepath.Join(tempDir, "routes.conf"),
		CertDir:         filepath.Join(tempDir, "certs"),
		NginxCertDir:    "/etc/nginx/openflare-certs",
		LuaDir:          filepath.Join(tempDir, "lua"),
		NginxLuaDir:     "/etc/nginx/openflare-lua",
		Executor:        &fakeExecutor{},
	}

	err := manager.Apply(context.Background(), "main", "route", []protocol.SupportFile{
		{Path: "../escape.crt", Content: "bad"},
	})
	if err == nil {
		t.Fatal("expected Apply to reject traversal path")
	}

	if _, statErr := os.Stat(filepath.Join(tempDir, "escape.crt")); !os.IsNotExist(statErr) {
		t.Fatalf("expected escaped file to not exist, stat err = %v", statErr)
	}
}

func TestObservabilityListenAddress(t *testing.T) {
	if got := ObservabilityListenAddress("", 18081); got != "18081" {
		t.Fatalf("unexpected docker observability listen address: %s", got)
	}
	if got := ObservabilityListenAddress("/usr/local/openresty/nginx/sbin/openresty", 18081); got != "127.0.0.1:18081" {
		t.Fatalf("unexpected path observability listen address: %s", got)
	}
}
