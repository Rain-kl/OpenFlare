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

func inspectOpenrestyProcesses(ctx context.Context, binary, configPath string) ([]runtimeProcess, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return nil, err
	}
	executable, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return inspectProcRuntime("/proc", executable, binary, configPath, os.Geteuid())
}

func signalOpenrestyProcess(ctx context.Context, binary, config string, process runtimeProcess, quit bool) error {
	processes, err := inspectOpenrestyProcesses(ctx, binary, config)
	if err != nil {
		return err
	}
	for _, current := range processes {
		if current.PID == process.PID && current.Master && current.StartTime == process.StartTime {
			signal := syscall.SIGHUP
			if quit {
				signal = syscall.SIGQUIT
			}
			return syscall.Kill(current.PID, signal)
		}
	}
	return fmt.Errorf("openresty master identity changed before signalling pid %d", process.PID)
}
