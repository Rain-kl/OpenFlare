//go:build !windows

package updater

import (
	"fmt"
	"os"
	"syscall"
)

func replaceAndRestart(execPath string, tmpPath string) error {
	backupPath := execPath + ".bak"
	os.Remove(backupPath)
	if err := os.Rename(execPath, backupPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("backup current binary: %w", err)
	}
	if err := os.Rename(tmpPath, execPath); err != nil {
		os.Rename(backupPath, execPath)
		return fmt.Errorf("replace binary: %w", err)
	}
	os.Remove(backupPath)
	if err := syscall.Exec(execPath, os.Args, os.Environ()); err != nil {
		return fmt.Errorf("exec restart: %w", err)
	}
	return fmt.Errorf("unreachable after exec")
}