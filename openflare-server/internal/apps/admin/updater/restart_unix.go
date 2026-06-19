//go:build !windows

// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const installedBinaryMode = 0o755

func replaceAndRestart(executable, stagedBinary string) error {
	ctx := context.Background()
	logger.InfoF(ctx, "[Updater] Swapping executable: %s -> %s", executable, stagedBinary)
	backup := executable + ".old"

	if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除旧备份失败: %w", err)
	}

	if err := os.Rename(executable, backup); err != nil {
		return fmt.Errorf("备份当前程序失败: %w", err)
	}

	if err := os.Rename(stagedBinary, executable); err != nil {
		_ = os.Rename(backup, executable)
		return fmt.Errorf("替换当前程序失败: %w", err)
	}

	if err := os.Chmod(executable, installedBinaryMode); err != nil { //nolint:gosec // the installed application binary must be executable.
		_ = os.Remove(executable)
		_ = os.Rename(backup, executable)
		return fmt.Errorf("设置程序执行权限失败: %w", err)
	}

	stagingDir := filepath.Dir(stagedBinary)
	// Cleanup is best effort; a leftover staging directory must not block restart.
	_ = os.RemoveAll(stagingDir)

	logger.InfoF(ctx, "[Updater] Executing syscall.Exec to restart service: %s %v", executable, os.Args)
	return syscall.Exec(executable, os.Args, os.Environ()) //nolint:gosec // executable is resolved from os.Executable and never supplied by a request.
}
