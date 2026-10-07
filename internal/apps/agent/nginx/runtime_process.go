// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nginx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	runtimeShutdownTimeout     = 10 * time.Second
	runtimeProcessPollInterval = 50 * time.Millisecond
)

// runtimeProcess records an inspected process identity, including its start time
// to reject a PID that has since been reused.
type runtimeProcess struct {
	PID       int
	Master    bool
	StartTime string
}

func (e *PathExecutor) runtimeProcesses(ctx context.Context) ([]runtimeProcess, error) {
	if e.inspectProcesses != nil {
		return e.inspectProcesses()
	}
	return inspectOpenrestyProcesses(ctx, e.Path, e.ConfigPath)
}

func (e *PathExecutor) signalRuntime(ctx context.Context, process runtimeProcess, quit bool) error {
	if e.signalProcess != nil {
		return e.signalProcess(process, quit)
	}
	return signalOpenrestyProcess(ctx, e.Path, e.ConfigPath, process, quit)
}

func findRuntimeMaster(processes []runtimeProcess) (runtimeProcess, error) {
	var master runtimeProcess
	for _, process := range processes {
		if !process.Master {
			continue
		}
		if master.PID != 0 {
			return runtimeProcess{}, errors.New("multiple matching openresty masters; refusing ambiguous runtime recovery")
		}
		master = process
	}
	if master.PID == 0 && len(processes) != 0 {
		return runtimeProcess{}, errors.New("openresty processes remain without a matching master; refusing to start another instance")
	}
	return master, nil
}

func (e *PathExecutor) recoverRuntime(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	processes, err := e.runtimeProcesses(ctx)
	if err != nil {
		return fmt.Errorf("inspect openresty runtime: %w", err)
	}
	master, err := findRuntimeMaster(processes)
	if err != nil {
		return err
	}
	if master.PID != 0 {
		repaired, err := e.repairPIDFile(master.PID)
		if err != nil {
			return err
		}
		if err := e.signalRuntime(ctx, master, false); err != nil {
			return fmt.Errorf("reload recovered openresty master: %w", err)
		}
		if repaired {
			slog.WarnContext(ctx, "recovered openresty master with invalid pid file", "pid", master.PID, "config", e.ConfigPath)
		}
		return nil
	}
	return e.startRuntime(ctx)
}

func (e *PathExecutor) startRuntime(ctx context.Context) error {
	output, err := e.Runner.Run(ctx, e.Path, "-c", e.ConfigPath)
	if err != nil {
		return fmt.Errorf("openresty start failed: %w: %s", err, output)
	}
	return nil
}

var pidDirectivePattern = regexp.MustCompile(`(?m)^\s*pid\s+([^;\n]+);`)

func (e *PathExecutor) repairPIDFile(pid int) (bool, error) {
	config, err := os.ReadFile(e.ConfigPath)
	if err != nil {
		return false, fmt.Errorf("read config for pid recovery: %w", err)
	}
	matches := pidDirectivePattern.FindAllSubmatch(config, -1)
	if len(matches) != 1 {
		return false, errors.New("pid recovery requires one explicit pid directive")
	}
	path := strings.Trim(string(matches[0][1]), " \t\"'")
	if !filepath.IsAbs(path) {
		return false, errors.New("pid recovery requires an absolute pid path")
	}
	expected := strconv.Itoa(pid) + "\n"
	//nolint:gosec // PID path comes from the validated, Agent-managed main configuration.
	if data, err := os.ReadFile(path); err == nil && string(data) == expected {
		return false, nil
	}
	if err := writeAtomicFile(path, []byte(strconv.Itoa(pid)+"\n"), nginxConfigFilePerm); err != nil {
		return false, fmt.Errorf("repair openresty pid file: %w", err)
	}
	return true, nil
}

func (e *PathExecutor) restartRuntime(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	processes, err := e.runtimeProcesses(ctx)
	if err != nil {
		return err
	}
	master, err := findRuntimeMaster(processes)
	if err != nil {
		return err
	}
	if master.PID == 0 {
		return e.startRuntime(ctx)
	}
	if err := e.signalRuntime(ctx, master, true); err != nil {
		return fmt.Errorf("stop openresty master: %w", err)
	}
	// A successful signal only acknowledges delivery, not process termination.
	// Never create a second master while the old master or workers remain.
	waitCtx, cancel := context.WithTimeout(ctx, runtimeShutdownTimeout)
	defer cancel()
	ticker := time.NewTicker(runtimeProcessPollInterval)
	defer ticker.Stop()
	for {
		remaining, err := e.runtimeProcesses(waitCtx)
		if err != nil {
			return err
		}
		if len(remaining) == 0 {
			break
		}
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("waiting for openresty processes to exit before restart: %w", waitCtx.Err())
		case <-ticker.C:
		}
	}
	return e.startRuntime(ctx)
}

func matchesRuntimeMaster(command, configPath string) bool {
	command = strings.ReplaceAll(command, "\x00", " ")
	if !strings.HasPrefix(command, "nginx: master process ") && !strings.HasPrefix(command, "openresty: master process ") {
		return false
	}
	fields := strings.Fields(command)
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "-c" {
			return filepath.IsAbs(fields[i+1]) && filepath.Clean(fields[i+1]) == filepath.Clean(configPath)
		}
	}
	return false
}
