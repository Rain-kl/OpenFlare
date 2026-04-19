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

type scriptedExecutor struct {
	reloadErrors []error
	reloadCalls  int
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

func (e *scriptedExecutor) Test(ctx context.Context) error {
	return nil
}

func (e *scriptedExecutor) Reload(ctx context.Context) error {
	index := e.reloadCalls
	e.reloadCalls++
	if index >= len(e.reloadErrors) {
		return nil
	}
	return e.reloadErrors[index]
}

func (e *scriptedExecutor) EnsureRuntime(ctx context.Context, recreate bool) error {
	return nil
}

func (e *scriptedExecutor) CheckHealth(ctx context.Context) error {
	return nil
}

func (e *scriptedExecutor) Restart(ctx context.Context) error {
	return nil
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
	inspectCalls := 0
	runner := &fakeRunner{
		runFn: func(name string, args ...string) ([]byte, error) {
			if len(args) >= 2 && args[0] == "inspect" {
				inspectCalls++
				if inspectCalls < 3 {
					return []byte("false"), nil
				}
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

	if len(runner.calls) != 5 {
		t.Fatalf("expected 5 calls, got %d", len(runner.calls))
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
	if runner.calls[4].args[0] != "inspect" {
		t.Fatalf("expected docker inspect after run, got %#v", runner.calls[4])
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

func TestDockerExecutorReloadRecreatesContainerWhenMountedCertMissing(t *testing.T) {
	mainConfigPath, routeConfigDir, certDir, luaDir := prepareDockerMountSources(t)
	runner := &fakeRunner{
		runFn: func(name string, args ...string) ([]byte, error) {
			if len(args) >= 1 && args[0] == "inspect" {
				return []byte("true"), nil
			}
			if len(args) >= 2 && args[0] == "exec" {
				return []byte(`nginx: [emerg] cannot load certificate "/etc/nginx/openflare-certs/1.crt": BIO_new_file() failed (SSL: error:80000002:system library::No such file or directory)`), errors.New("exit status 1")
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

	if err := executor.Reload(context.Background()); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if len(runner.calls) != 6 {
		t.Fatalf("expected 6 calls, got %d", len(runner.calls))
	}
	if runner.calls[2].args[0] != "inspect" || runner.calls[3].args[0] != "rm" || runner.calls[4].args[0] != "run" || runner.calls[5].args[0] != "inspect" {
		t.Fatalf("expected recreate after reload failure, got %#v", runner.calls)
	}
}

func TestDockerExecutorRunContainerMountsManagedFiles(t *testing.T) {
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

	if err := executor.runContainer(context.Background()); err != nil {
		t.Fatalf("runContainer failed: %v", err)
	}

	if len(runner.calls) != 2 {
		t.Fatalf("expected docker run plus health check, got %d calls", len(runner.calls))
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
	if !reflect.DeepEqual(runner.calls[1].args, []string{"inspect", "-f", "{{.State.Running}}", "openflare-openresty"}) {
		t.Fatalf("unexpected docker health check args: %#v", runner.calls[1].args)
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
	if len(runner.calls) != 4 {
		t.Fatalf("expected 4 calls, got %d", len(runner.calls))
	}
	if runner.calls[1].args[0] != "rm" {
		t.Fatalf("expected docker rm on second call, got %#v", runner.calls[1])
	}
	if runner.calls[2].args[0] != "run" {
		t.Fatalf("expected docker run on third call, got %#v", runner.calls[2])
	}
	if runner.calls[3].args[0] != "inspect" {
		t.Fatalf("expected docker inspect after run, got %#v", runner.calls[3])
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

	outcome := manager.Apply(
		context.Background(),
		"include __OPENFLARE_ROUTE_CONFIG__;\naccess_log __OPENFLARE_ACCESS_LOG__ openflare_json;\n",
		"ssl_certificate __OPENFLARE_CERT_DIR__/1.crt;\n",
		[]protocol.SupportFile{{Path: "1.crt", Content: "cert"}},
	)
	if outcome.Status != ApplyStatusSuccess {
		t.Fatalf("Apply failed: %#v", outcome)
	}

	mainData, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("failed to read main config: %v", err)
	}
	expectedMain := "include " + routePath + ";\naccess_log " + filepath.ToSlash(filepath.Join(filepath.Dir(routePath), "openflare_access.log")) + " openflare_json;\n"
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

	if outcome := manager.Apply(context.Background(), "include __OPENFLARE_ROUTE_CONFIG__;\naccess_log __OPENFLARE_ACCESS_LOG__ openflare_json;\n", "server { listen 80; }\n", nil); outcome.Status != ApplyStatusSuccess {
		t.Fatalf("Apply failed: %#v", outcome)
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

	outcome := manager.Apply(context.Background(), "include __OPENFLARE_ROUTE_CONFIG__;\n__OPENFLARE_RESOLVER_DIRECTIVE__server { listen __OPENFLARE_OBSERVABILITY_LISTEN__; }", "ssl_certificate __OPENFLARE_CERT_DIR__/1.crt;", []protocol.SupportFile{
		{Path: "1.crt", Content: "cert-data"},
		{Path: "1.key", Content: "key-data"},
	})
	if outcome.Status != ApplyStatusSuccess {
		t.Fatalf("Apply failed: %#v", outcome)
	}

	routeData, err := os.ReadFile(manager.RouteConfigPath)
	if err != nil {
		t.Fatalf("failed to read route config: %v", err)
	}
	if !strings.Contains(string(routeData), "/etc/nginx/openflare-certs/1.crt") {
		t.Fatalf("expected placeholder replacement in route config, got %s", string(routeData))
	}
	renderedRoute := manager.renderRouteConfig("access_by_lua_file __OPENFLARE_LUA_DIR__/pow/check.lua;\nlocation /.within.website/x/cmd/anubis/static/ { alias __OPENFLARE_POW_STATIC_DIR__/; }\n")
	if !strings.Contains(renderedRoute, "access_by_lua_file /etc/nginx/openflare-lua/pow/check.lua;") {
		t.Fatalf("expected lua dir placeholder replacement in route config, got %s", renderedRoute)
	}
	if !strings.Contains(renderedRoute, "alias /etc/nginx/openflare-lua/pow/static/;") {
		t.Fatalf("expected pow static dir placeholder replacement in route config, got %s", renderedRoute)
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
	if runtime.GOOS != "windows" && luaInfo.Mode().Perm() != 0o644 {
		t.Fatalf("unexpected lua mode: %o", luaInfo.Mode().Perm())
	}
}

func TestResolverDirectiveUsesExplicitResolvers(t *testing.T) {
	got := ResolverDirective("", []string{"10.0.0.2", "1.1.1.1"})
	if !strings.Contains(got, "resolver 10.0.0.2 1.1.1.1") {
		t.Fatalf("expected explicit resolver directive, got %q", got)
	}
}

func TestParseResolverAddressesFiltersLoopbackForDocker(t *testing.T) {
	content := strings.Join([]string{
		"nameserver 127.0.0.53",
		"nameserver 10.0.0.2",
		"nameserver ::1",
		"nameserver 1.1.1.1",
	}, "\n")
	got := parseResolverAddresses(content, true)
	expected := []string{"10.0.0.2", "1.1.1.1"}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("unexpected docker resolvers: got %#v want %#v", got, expected)
	}
}

func TestParseResolverAddressesKeepsLoopbackForLocalBinary(t *testing.T) {
	content := strings.Join([]string{
		"nameserver 127.0.0.53",
		"nameserver 10.0.0.2",
	}, "\n")
	got := parseResolverAddresses(content, false)
	expected := []string{"127.0.0.53", "10.0.0.2"}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("unexpected local resolvers: got %#v want %#v", got, expected)
	}
}

func TestRequiresRuntimeResolver(t *testing.T) {
	testCases := []struct {
		name      string
		originURL string
		want      bool
	}{
		{name: "hostname", originURL: "https://origin.internal", want: true},
		{name: "ipv4", originURL: "https://10.0.0.8", want: false},
		{name: "ipv6", originURL: "https://[2001:db8::1]", want: false},
		{name: "invalid", originURL: "://bad", want: false},
	}

	for _, testCase := range testCases {
		if got := RequiresRuntimeResolver(testCase.originURL); got != testCase.want {
			t.Fatalf("%s: got %v want %v", testCase.name, got, testCase.want)
		}
	}
}

func TestWriteCertFilesKeepsBaseDirAndRemovesStaleFiles(t *testing.T) {
	tempDir := t.TempDir()
	certDir := filepath.Join(tempDir, "certs")
	if err := os.MkdirAll(filepath.Join(certDir, "stale"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(certDir, "stale", "old.crt"), []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	manager := &Manager{CertDir: certDir}

	if err := manager.writeCertFiles([]protocol.SupportFile{
		{Path: "1.crt", Content: "cert"},
		{Path: "1.key", Content: "key"},
	}); err != nil {
		t.Fatalf("writeCertFiles failed: %v", err)
	}

	if _, err := os.Stat(certDir); err != nil {
		t.Fatalf("expected cert dir to persist, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(certDir, "stale", "old.crt")); !os.IsNotExist(err) {
		t.Fatalf("expected stale cert file to be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(certDir, "1.crt")); err != nil {
		t.Fatalf("expected new cert file to exist, stat err = %v", err)
	}
}

func TestEnsureLuaAssetsKeepsBaseDirAndRemovesStaleFiles(t *testing.T) {
	tempDir := t.TempDir()
	luaDir := filepath.Join(tempDir, "lua")
	if err := os.MkdirAll(filepath.Join(luaDir, "stale"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(luaDir, "stale", "old.lua"), []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	manager := &Manager{LuaDir: luaDir}

	if err := manager.EnsureLuaAssets(); err != nil {
		t.Fatalf("EnsureLuaAssets failed: %v", err)
	}

	if _, err := os.Stat(luaDir); err != nil {
		t.Fatalf("expected lua dir to persist, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(luaDir, "stale", "old.lua")); !os.IsNotExist(err) {
		t.Fatalf("expected stale lua file to be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(luaDir, "log.lua")); err != nil {
		t.Fatalf("expected managed lua file to exist, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(luaDir, "pow", "check.lua")); err != nil {
		t.Fatalf("expected managed pow lua file to exist, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(luaDir, "pow", "static", "js", "main.mjs")); err != nil {
		t.Fatalf("expected managed pow static asset to exist, stat err = %v", err)
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
	if _, err := os.Stat(filepath.Join(manager.LuaDir, "pow", "check.lua")); err != nil {
		t.Fatalf("failed to stat pow lua file: %v", err)
	}
}

func TestEnsureLuaAssetsPreservesPowConfig(t *testing.T) {
	tempDir := t.TempDir()
	luaDir := filepath.Join(tempDir, "lua")
	if err := os.MkdirAll(luaDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	powConfigPath := filepath.Join(luaDir, "pow_config.json")
	want := `[{"domains":["pow.example.com"],"enabled":true}]`
	if err := os.WriteFile(powConfigPath, []byte(want), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	manager := &Manager{LuaDir: luaDir}

	if err := manager.EnsureLuaAssets(); err != nil {
		t.Fatalf("EnsureLuaAssets failed: %v", err)
	}

	got, err := os.ReadFile(powConfigPath)
	if err != nil {
		t.Fatalf("expected pow_config.json to remain after EnsureLuaAssets: %v", err)
	}
	if string(got) != want {
		t.Fatalf("unexpected pow_config.json content: got %s want %s", string(got), want)
	}
}

func TestManagerCurrentChecksumIncludesPowConfig(t *testing.T) {
	tempDir := t.TempDir()
	mainPath := filepath.Join(tempDir, "nginx.conf")
	routePath := filepath.Join(tempDir, "routes.conf")
	luaDir := filepath.Join(tempDir, "lua")
	manager := &Manager{
		MainConfigPath:  mainPath,
		RouteConfigPath: routePath,
		LuaDir:          luaDir,
		NginxLuaDir:     "/etc/nginx/openflare-lua",
		Executor:        &fakeExecutor{},
	}

	outcome := manager.Apply(
		context.Background(),
		"access_log __OPENFLARE_ACCESS_LOG__ openflare_json;\n",
		"location /.within.website/x/cmd/anubis/static/ { alias __OPENFLARE_POW_STATIC_DIR__/; }\n",
		[]protocol.SupportFile{{Path: "pow_config.json", Content: `[{"domains":["pow.example.com"],"enabled":true}]`}},
	)
	if outcome.Status != ApplyStatusSuccess {
		t.Fatalf("Apply failed: %#v", outcome)
	}

	value, err := manager.CurrentChecksum()
	if err != nil {
		t.Fatalf("CurrentChecksum failed: %v", err)
	}
	expected := bundleChecksum(
		"access_log __OPENFLARE_ACCESS_LOG__ openflare_json;\n",
		"location /.within.website/x/cmd/anubis/static/ { alias __OPENFLARE_POW_STATIC_DIR__/; }\n",
		[]protocol.SupportFile{{Path: "pow_config.json", Content: `[{"domains":["pow.example.com"],"enabled":true}]`}},
	)
	if value != expected {
		t.Fatalf("unexpected checksum with pow config: got %s want %s", value, expected)
	}
}

func TestManagedPowLuaFilesUseInternalChallengeFlow(t *testing.T) {
	if !strings.Contains(openRestyPowCheckLua, `return ngx.exec("/.within.website/x/cmd/anubis/api/make-challenge")`) {
		t.Fatal("expected check.lua to internally execute make-challenge instead of issuing a 302 redirect")
	}
	if strings.Contains(openRestyPowCheckLua, "ngx.redirect(") {
		t.Fatal("expected check.lua to avoid external redirects for challenge rendering")
	}
	if !strings.Contains(openRestyPowChallengeLua, `<h1 id="title" class="centered-div">`) {
		t.Fatal("expected challenge html to include Anubis-compatible title node")
	}
	if !strings.Contains(openRestyPowChallengeLua, `<div id="progress" role="progressbar" aria-labelledby="status"><div class="bar-inner"></div></div>`) {
		t.Fatal("expected challenge html to include Anubis-compatible progress markup")
	}
	if !strings.Contains(openRestyPowChallengeLua, `<script id="anubis_public_url" type="application/json">"__openflare_internal__"</script>`) {
		t.Fatal("expected challenge html to force Anubis frontend to reuse the current URL as redir target")
	}
	if !strings.Contains(openRestyPowVerifyLua, `if ngx.var.scheme == "https" then`) {
		t.Fatal("expected verify.lua to only mark the session cookie as Secure for HTTPS requests")
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

	outcome := manager.Apply(context.Background(), "new-main", "new-route", []protocol.SupportFile{
		{Path: "1.crt", Content: "new-cert"},
	})
	if outcome.Status != ApplyStatusFatal {
		t.Fatalf("expected fatal apply outcome, got %#v", outcome)
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

func TestManagerApplyReturnsWarningWhenRollbackRecoversRuntime(t *testing.T) {
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
		Executor: &scriptedExecutor{
			reloadErrors: []error{errors.New("target config failed"), nil},
		},
	}

	outcome := manager.Apply(context.Background(), "new-main", "new-route", []protocol.SupportFile{
		{Path: "1.crt", Content: "new-cert"},
	})
	if outcome.Status != ApplyStatusWarning {
		t.Fatalf("expected warning apply outcome, got %#v", outcome)
	}

	mainData, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("failed to read main config: %v", err)
	}
	if string(mainData) != "old-main" {
		t.Fatalf("expected main rollback, got %s", string(mainData))
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

	outcome := manager.Apply(context.Background(), "main", "route", []protocol.SupportFile{
		{Path: "../escape.crt", Content: "bad"},
	})
	if outcome.Status != ApplyStatusWarning {
		t.Fatalf("expected warning apply outcome, got %#v", outcome)
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
