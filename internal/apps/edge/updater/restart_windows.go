//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func replaceAndRestart(execPath string, tmpPath string) error {
	backupPath := execPath + ".bak"
	scriptPath := execPath + ".update.cmd"
	script := fmt.Sprintf(`@echo off
setlocal
:waitloop
move /Y "%s" "%s" >nul 2>nul
if errorlevel 1 (
  ping 127.0.0.1 -n 2 >nul
  goto waitloop
)
move /Y "%s" "%s" >nul 2>nul
if errorlevel 1 exit /b 1
start "" %s
del /Q "%s" >nul 2>nul
del /Q "%%~f0" >nul 2>nul
`, execPath, backupPath, tmpPath, execPath, buildWindowsCommandLine(execPath, os.Args[1:]), backupPath)
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("write restart script: %w", err)
	}
	cmd := exec.Command("cmd", "/C", "start", "", scriptPath)
	if err := cmd.Start(); err != nil {
		os.Remove(scriptPath)
		os.Remove(tmpPath)
		return fmt.Errorf("schedule restart: %w", err)
	}
	os.Exit(0)
	return nil
}

func buildWindowsCommandLine(execPath string, args []string) string {
	parts := []string{quoteWindowsArg(execPath)}
	for _, arg := range args {
		parts = append(parts, quoteWindowsArg(arg))
	}
	return strings.Join(parts, " ")
}

func quoteWindowsArg(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}