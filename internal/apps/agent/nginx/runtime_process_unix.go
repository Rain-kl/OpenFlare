//go:build unix && !linux

// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nginx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

const (
	psCommandIndex      = 7
	psMasterBinaryIndex = 10
)

// Non-Linux Unix systems expose process ownership and start time through ps.
func inspectOpenrestyProcesses(ctx context.Context, binary, configPath string) ([]runtimeProcess, error) {
	output, err := exec.CommandContext(ctx, "ps", "-axo", "pid=,uid=,lstart=,command=").Output()
	if err != nil {
		return nil, fmt.Errorf("inspect openresty processes: %w", err)
	}
	var processes []runtimeProcess
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) <= psCommandIndex {
			continue
		}
		uid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		title := strings.Join(fields[psCommandIndex:], " ")
		if !strings.HasPrefix(title, "nginx: ") && !strings.HasPrefix(title, "openresty: ") {
			continue
		}
		master := uid == os.Geteuid() && matchesRuntimeMaster(title, configPath)
		if master && (len(fields) <= psMasterBinaryIndex || fields[psMasterBinaryIndex] != binary) {
			return nil, fmt.Errorf("cannot verify openresty master invocation %s", fields[0])
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, err
		}
		processes = append(processes, runtimeProcess{PID: pid, Master: master, StartTime: strings.Join(fields[2:psCommandIndex], " ")})
	}
	return processes, nil
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
