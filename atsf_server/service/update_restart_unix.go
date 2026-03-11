//go:build !windows

package service

import (
	"fmt"
	"os"
	"syscall"
)

func replaceAndRestartServer(execPath string, tmpPath string) error {
	backupPath := execPath + ".bak"
	_ = os.Remove(backupPath)
	if err := os.Rename(execPath, backupPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("备份当前服务端二进制失败: %w", err)
	}
	if err := os.Rename(tmpPath, execPath); err != nil {
		_ = os.Rename(backupPath, execPath)
		return fmt.Errorf("替换服务端二进制失败: %w", err)
	}
	_ = os.Remove(backupPath)
	if err := syscall.Exec(execPath, os.Args, os.Environ()); err != nil {
		return fmt.Errorf("重启服务失败: %w", err)
	}
	return fmt.Errorf("unreachable after exec")
}
