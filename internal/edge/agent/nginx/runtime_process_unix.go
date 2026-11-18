//go:build unix && !linux

// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nginx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Non-Linux Unix systems expose process ownership and start time through ps.
func inspectOpenrestyProcesses(ctx context.Context, binary, configPath string) ([]runtimeProcess, error) {
	output, err := exec.CommandContext(ctx, "ps", "-axo", "pid=,ppid=,uid=,lstart=,command=").Output()
	if err != nil {
		return nil, fmt.Errorf("inspect openresty processes: %w", err)
	}
	return parsePSRuntime(string(output), binary, configPath, os.Geteuid())
}

func signalOpenrestyProcess(ctx context.Context, binary, config string, process runtimeProcess, quit bool) error {
	processes, err := inspectOpenrestyProcesses(ctx, binary, config)
	if err != nil {
		return err
	}
	for _, current := range processes {
		if current == process && current.Master {
			signal := syscall.SIGHUP
			if quit {
				signal = syscall.SIGQUIT
			}
			return syscall.Kill(current.PID, signal)
		}
	}
	return fmt.Errorf("openresty master identity changed before signalling pid %d", process.PID)
}
