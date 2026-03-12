package nginx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"atsflare-agent/internal/protocol"
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

func TestDockerExecutorStartsContainerWhenMissing(t *testing.T) {
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
		ContainerName:  "atsflare-openresty",
		Image:          "openresty/openresty:alpine",
		RouteConfigDir: filepath.Clean("/tmp/routes"),
		CertDir:        filepath.Clean("/tmp/certs"),
		NginxCertDir:   "/etc/nginx/atsflare-certs",
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
		ContainerName:  "atsflare-openresty",
		Image:          "openresty/openresty:alpine",
		RouteConfigDir: filepath.Clean("/tmp/routes"),
		CertDir:        filepath.Clean("/tmp/certs"),
		NginxCertDir:   "/etc/nginx/atsflare-certs",
		Runner:         runner,
	}

	if err := executor.Reload(context.Background()); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if len(runner.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(runner.calls))
	}
	if runner.calls[0].args[0] != "inspect" {
		t.Fatalf("expected docker inspect on first call, got %#v", runner.calls[0])
	}
	if runner.calls[1].args[0] != "rm" {
		t.Fatalf("expected docker rm on second call, got %#v", runner.calls[1])
	}
	if runner.calls[2].args[0] != "run" {
		t.Fatalf("expected docker run on third call, got %#v", runner.calls[2])
	}
}

func TestDockerExecutorRecreatesContainerOnStartup(t *testing.T) {
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
		ContainerName:  "atsflare-openresty",
		Image:          "openresty/openresty:alpine",
		RouteConfigDir: filepath.Clean("/tmp/routes"),
		CertDir:        filepath.Clean("/tmp/certs"),
		NginxCertDir:   "/etc/nginx/atsflare-certs",
		Runner:         runner,
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

func TestNewExecutorUsesAbsoluteDockerMountPath(t *testing.T) {
	executor := NewExecutor(ExecutorOptions{
		DockerBinary:    "docker",
		ContainerName:   "atsflare-openresty",
		Image:           "openresty/openresty:alpine",
		RouteConfigPath: "./data/etc/nginx/conf.d/atsflare_routes.conf",
		CertDir:         "./data/etc/nginx/certs",
		NginxCertDir:    "/etc/nginx/atsflare-certs",
	})

	dockerExecutor, ok := executor.(*DockerExecutor)
	if !ok {
		t.Fatal("expected docker executor")
	}
	if !filepath.IsAbs(dockerExecutor.RouteConfigDir) {
		t.Fatalf("expected absolute route config dir, got %s", dockerExecutor.RouteConfigDir)
	}
	if !strings.HasSuffix(dockerExecutor.RouteConfigDir, filepath.Clean("data/etc/nginx/conf.d")) {
		t.Fatalf("unexpected route config dir: %s", dockerExecutor.RouteConfigDir)
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
		RouteConfigPath: filepath.Join(tempDir, "routes.conf"),
		CertDir:         filepath.Join(tempDir, "certs"),
		NginxCertDir:    "/etc/nginx/atsflare-certs",
		Executor:        &fakeExecutor{},
	}

	err := manager.Apply(context.Background(), "ssl_certificate __ATSF_CERT_DIR__/1.crt;", []protocol.SupportFile{
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
	if !strings.Contains(string(routeData), "/etc/nginx/atsflare-certs/1.crt") {
		t.Fatalf("expected placeholder replacement in route config, got %s", string(routeData))
	}
	certData, err := os.ReadFile(filepath.Join(manager.CertDir, "1.crt"))
	if err != nil {
		t.Fatalf("failed to read cert file: %v", err)
	}
	if string(certData) != "cert-data" {
		t.Fatalf("unexpected cert file content: %s", string(certData))
	}
}

func TestManagerRollbackRestoresSupportFiles(t *testing.T) {
	tempDir := t.TempDir()
	routePath := filepath.Join(tempDir, "routes.conf")
	certDir := filepath.Join(tempDir, "certs")
	if err := os.MkdirAll(certDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(routePath, []byte("old-route"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(certDir, "1.crt"), []byte("old-cert"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	manager := &Manager{
		RouteConfigPath: routePath,
		CertDir:         certDir,
		NginxCertDir:    "/etc/nginx/atsflare-certs",
		Executor: &fakeExecutor{
			testErr: errors.New("openresty test failed"),
		},
	}

	err := manager.Apply(context.Background(), "new-route", []protocol.SupportFile{
		{Path: "1.crt", Content: "new-cert"},
	})
	if err == nil {
		t.Fatal("expected Apply to fail")
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
