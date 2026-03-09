package nginx

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type runCall struct {
	name string
	args []string
}

type fakeRunner struct {
	calls []runCall
	runFn func(name string, args ...string) ([]byte, error)
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, runCall{name: name, args: append([]string{}, args...)})
	if r.runFn != nil {
		return r.runFn(name, args...)
	}
	return nil, nil
}

func TestPathExecutorCommands(t *testing.T) {
	runner := &fakeRunner{}
	executor := &PathExecutor{
		Path:   "/opt/nginx/sbin/nginx",
		Runner: runner,
	}

	if err := executor.Test(context.Background()); err != nil {
		t.Fatalf("Test failed: %v", err)
	}
	if err := executor.Reload(context.Background()); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	expected := []runCall{
		{name: "/opt/nginx/sbin/nginx", args: []string{"-t"}},
		{name: "/opt/nginx/sbin/nginx", args: []string{"-s", "reload"}},
	}
	if !reflect.DeepEqual(runner.calls, expected) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}
}

func TestPathExecutorEnsureRuntimeNoop(t *testing.T) {
	executor := &PathExecutor{
		Path:   "/opt/nginx/sbin/nginx",
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
		ContainerName:  "atsflare-nginx",
		Image:          "nginx:stable-alpine",
		RouteConfigDir: filepath.Clean("/tmp/routes"),
		Runner:         runner,
	}

	if err := executor.Test(context.Background()); err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if len(runner.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(runner.calls))
	}
	if runner.calls[1].args[0] != "run" {
		t.Fatalf("expected docker run on second call, got %#v", runner.calls[1])
	}
	if runner.calls[2].args[0] != "exec" {
		t.Fatalf("expected docker exec on third call, got %#v", runner.calls[2])
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
		ContainerName:  "atsflare-nginx",
		Image:          "nginx:stable-alpine",
		RouteConfigDir: filepath.Clean("/tmp/routes"),
		Runner:         runner,
	}

	if err := executor.Reload(context.Background()); err != nil {
		t.Fatalf("Reload failed: %v", err)
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
	if runner.calls[3].args[0] != "exec" {
		t.Fatalf("expected docker exec on fourth call, got %#v", runner.calls[3])
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
		ContainerName:  "atsflare-nginx",
		Image:          "nginx:stable-alpine",
		RouteConfigDir: filepath.Clean("/tmp/routes"),
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
		ContainerName:   "atsflare-nginx",
		Image:           "nginx:stable-alpine",
		RouteConfigPath: "./data/etc/nginx/conf.d/atsflare_routes.conf",
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
