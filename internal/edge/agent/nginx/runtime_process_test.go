// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nginx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReloadRecoversLiveMasterWithEmptyPID(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "nginx.conf")
	pidPath := filepath.Join(dir, "nginx.pid")
	if err := os.WriteFile(config, []byte("pid "+pidPath+";\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{runFn: func(_ string, _ ...string) ([]byte, error) {
		return []byte(`invalid PID number ""`), errors.New("exit status 1")
	}}
	master := runtimeProcess{PID: 42, Master: true, StartTime: "123"}
	var signalled runtimeProcess
	executor := &PathExecutor{Path: "openresty", ConfigPath: config, Runner: runner,
		inspectProcesses: func() ([]runtimeProcess, error) { return []runtimeProcess{master}, nil },
		signalProcess: func(process runtimeProcess, quit bool) error {
			if quit {
				t.Error("Reload requested quit, want reload")
			}
			signalled = process
			return nil
		},
	}
	if err := executor.Reload(context.Background()); err != nil {
		t.Fatalf("Reload(empty PID, live master) = %v, want nil", err)
	}
	if signalled != master {
		t.Errorf("Reload signalled %v, want %v", signalled, master)
	}
	if len(runner.calls) != 0 {
		t.Errorf("Reload commands = %v, want direct master signal and no start", runner.calls)
	}
	data, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "42\n" {
		t.Errorf("recovered PID = %q, want 42 newline", data)
	}
}

func TestReloadStartsWithUnrelatedWorkers(t *testing.T) {
	runner := &fakeRunner{runFn: func(_ string, _ ...string) ([]byte, error) { return []byte("invalid pid"), errors.New("failed") }}
	executor := &PathExecutor{Path: "openresty", Runner: runner,
		inspectProcesses: func() ([]runtimeProcess, error) { return []runtimeProcess{{PID: 99}}, nil },
	}
	if err := executor.Reload(context.Background()); err == nil || !strings.Contains(err.Error(), "start failed") {
		t.Errorf("Reload(unrelated workers) = %v, want attempted start", err)
	}
	if len(runner.calls) != 1 {
		t.Errorf("Reload commands = %v, want one start", runner.calls)
	}
}

func TestRestartWaitsForOldWorkersBeforeStart(t *testing.T) {
	inspections := 0
	quit := false
	runner := &fakeRunner{runFn: func(_ string, _ ...string) ([]byte, error) {
		if !quit || inspections < 3 {
			t.Errorf("start before shutdown completed: quit=%v inspections=%d", quit, inspections)
		}
		return nil, nil
	}}
	executor := &PathExecutor{Path: "openresty", Runner: runner,
		inspectProcesses: func() ([]runtimeProcess, error) {
			inspections++
			if inspections == 1 {
				return []runtimeProcess{{PID: 42, Master: true}, {PID: 43, ParentPID: 42}}, nil
			}
			if inspections == 2 {
				return []runtimeProcess{{PID: 43, ParentPID: 1}}, nil
			}
			return nil, nil
		},
		signalProcess: func(_ runtimeProcess, gracefulQuit bool) error { quit = gracefulQuit; return nil },
	}
	if err := executor.Restart(context.Background()); err != nil {
		t.Fatalf("Restart(draining workers) = %v, want nil", err)
	}
	if len(runner.calls) != 1 {
		t.Errorf("Restart commands=%v, want one start", runner.calls)
	}
}

func TestRestartCancellationNeverStartsSecondMaster(t *testing.T) {
	runner := &fakeRunner{}
	executor := &PathExecutor{Path: "openresty", Runner: runner,
		inspectProcesses: func() ([]runtimeProcess, error) { return []runtimeProcess{{PID: 42, Master: true}}, nil },
		signalProcess:    func(_ runtimeProcess, _ bool) error { return nil },
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := executor.Restart(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Restart(cancelled)=%v, want context.Canceled", err)
	}
	if len(runner.calls) != 0 {
		t.Errorf("Restart(cancelled) commands=%v, want no start", runner.calls)
	}
}

func TestRestartTimeoutNeverStartsWhileWorkersRemain(t *testing.T) {
	runner := &fakeRunner{}
	quits := 0
	executor := &PathExecutor{Path: "openresty", Runner: runner,
		inspectProcesses: func() ([]runtimeProcess, error) {
			if quits == 0 {
				return []runtimeProcess{{PID: 42, Master: true}, {PID: 43, ParentPID: 42}}, nil
			}
			return []runtimeProcess{{PID: 43, ParentPID: 1}}, nil
		},
		signalProcess: func(_ runtimeProcess, quit bool) error {
			if quit {
				quits++
			}
			return nil
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := executor.Restart(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Restart(draining worker, deadline) = %v, want deadline exceeded", err)
	}
	if quits != 1 || len(runner.calls) != 0 {
		t.Errorf("Restart quit count=%d commands=%v, want one quit and no start", quits, runner.calls)
	}
}

func TestReloadRefusesAmbiguousMasters(t *testing.T) {
	runner := &fakeRunner{}
	executor := &PathExecutor{Runner: runner,
		inspectProcesses: func() ([]runtimeProcess, error) {
			return []runtimeProcess{{PID: 42, Master: true}, {PID: 43, Master: true}}, nil
		},
	}
	if err := executor.Reload(context.Background()); err == nil {
		t.Error("Reload(two masters) = nil, want refusal")
	}
	if len(runner.calls) != 0 {
		t.Errorf("Reload commands = %v, want no start", runner.calls)
	}
}

type blockingRuntimeExecutor struct {
	fakeExecutor
	entered   chan struct{}
	release   chan struct{}
	restarted chan struct{}
}

func (e *blockingRuntimeExecutor) Reload(context.Context) error {
	close(e.entered)
	<-e.release
	return nil
}
func (e *blockingRuntimeExecutor) Restart(context.Context) error { close(e.restarted); return nil }

func TestManagerApplySerializesRestart(t *testing.T) {
	dir := t.TempDir()
	executor := &blockingRuntimeExecutor{entered: make(chan struct{}), release: make(chan struct{}), restarted: make(chan struct{})}
	manager := &Manager{MainConfigPath: filepath.Join(dir, "etc", "nginx.conf"), RouteConfigPath: filepath.Join(dir, "etc", "routes.conf"), Executor: executor}
	applied := make(chan ApplyOutcome, 1)
	go func() { applied <- manager.Apply(context.Background(), "events {}\nhttp {}", "", nil) }()
	select {
	case <-executor.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("Apply did not reach reload")
	}
	restarted := make(chan error, 1)
	go func() { restarted <- manager.Restart(context.Background()) }()
	select {
	case <-executor.restarted:
		t.Error("Restart overlapped Apply, want serialized lifecycle")
	case <-time.After(30 * time.Millisecond):
	}
	close(executor.release)
	if result := <-applied; result.Status != ApplyStatusSuccess {
		t.Errorf("Apply status=%v message=%s, want success", result.Status, result.Message)
	}
	if err := <-restarted; err != nil {
		t.Errorf("Restart(after Apply)=%v, want nil", err)
	}
}

func TestRepairPIDRefusesRelativeOrMissingPath(t *testing.T) {
	config := filepath.Join(t.TempDir(), "nginx.conf")
	if err := os.WriteFile(config, []byte("pid logs/nginx.pid;"), 0o644); err != nil {
		t.Fatal(err)
	}
	executor := &PathExecutor{ConfigPath: config}
	if _, err := executor.repairPIDFile(42); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Errorf("repairPIDFile(relative path)=%v, want absolute path error", err)
	}
}

func TestRuntimeDescendantsTracksOnlyInstanceIdentities(t *testing.T) {
	processes := []runtimeProcess{
		{PID: 40, ParentPID: 43, StartTime: "140"}, // Descendant precedes its parent.
		{PID: 42, Master: true, StartTime: "142"},
		{PID: 43, ParentPID: 42, StartTime: "143"},
		{PID: 50, StartTime: "150"}, // Unrelated master and worker.
		{PID: 51, ParentPID: 50, StartTime: "151"},
	}
	identities := runtimeMasterIdentities(processes)
	if got := runtimeDescendants(processes, identities); len(got) != 3 {
		t.Fatalf("instance processes = %v, want master and two descendants", got)
	}
	remaining := []runtimeProcess{
		{PID: 43, ParentPID: 1, StartTime: "143"}, // Reparented old worker.
		{PID: 40, ParentPID: 1, StartTime: "240"}, // Reused PID.
		{PID: 50, StartTime: "150"},
		{PID: 51, ParentPID: 50, StartTime: "151"},
	}
	got := runtimeDescendants(remaining, identities)
	if len(got) != 1 || got[0].PID != 43 {
		t.Fatalf("remaining instance processes = %v, want old worker 43", got)
	}
}

func TestRestartIgnoresUnrelatedProcessesAfterShutdown(t *testing.T) {
	inspections := 0
	runner := &fakeRunner{}
	executor := &PathExecutor{Path: "openresty", Runner: runner,
		inspectProcesses: func() ([]runtimeProcess, error) {
			inspections++
			processes := []runtimeProcess{{PID: 50}, {PID: 51, ParentPID: 50}}
			if inspections == 1 {
				processes = append(processes, runtimeProcess{PID: 42, Master: true})
			}
			return processes, nil
		},
		signalProcess: func(_ runtimeProcess, _ bool) error { return nil },
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := executor.Restart(ctx); err != nil {
		t.Fatalf("Restart(other nginx instance) = %v, want nil", err)
	}
	if len(runner.calls) != 1 {
		t.Errorf("Restart commands = %v, want one start", runner.calls)
	}
}
