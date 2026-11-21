// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/credential"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/spf13/cobra"
)

func newRotateCredentialsCommand() *cobra.Command {
	var secretPath string
	var apply bool
	command := &cobra.Command{
		Use:   "rotate-credentials",
		Short: "Re-encrypt stored credentials before changing the session secret",
		Long: `Read the old session secret from the current server configuration.
Supply the new secret through a private file. The default mode checks the database
without writing. After stopping all OpenFlare Server processes and backing up the
database and old secret, pass --apply to update the database in one transaction.
After success, set app.session_secret or APP_SESSION_SECRET to the new value
before restarting the service.`,
		Args: cobra.NoArgs,
		// Keep credential queries out of the service's telemetry pipeline.
		PersistentPreRun:  func(_ *cobra.Command, _ []string) {},
		PersistentPostRun: func(_ *cobra.Command, _ []string) {},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCredentialRotation(cmd, secretPath, apply)
		},
	}
	command.Flags().StringVar(&secretPath, "new-secret-file", "", "Path to a private file containing the new session secret")
	command.Flags().BoolVar(&apply, "apply", false, "Commit the changes after verifying all credentials")
	if err := command.MarkFlagRequired("new-secret-file"); err != nil {
		panic(err)
	}
	return command
}

func runCredentialRotation(cmd *cobra.Command, secretPath string, apply bool) error {
	secret, err := readRotationSecret(secretPath)
	if err != nil {
		return err
	}
	counts, err := credential.Rotate(cmd.Context(), config.Config.App.SessionSecret, secret, apply)
	if err != nil {
		return err
	}

	var report []string
	label := "Would migrate"
	if apply {
		label = "Migrated"
	}
	total := 0
	for _, count := range counts {
		total += count.Rotated
		report = append(report, fmt.Sprintf("%s: %s %d, already using the new secret %d", count.Field, label, count.Rotated, count.AlreadyNew))
	}
	switch {
	case apply:
		report = append(report, "Set the server configuration to the new secret before starting OpenFlare. Existing browser sessions must sign in again.")
	case total > 0:
		report = append(report, "Check complete. Stop all OpenFlare Server processes, back up the database and old secret, then rerun with --apply.")
	default:
		report = append(report, "Check complete. No credentials need re-encryption. Set the server configuration to the new secret and restart OpenFlare.")
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(report, "\n"))
	if err != nil && apply {
		return fmt.Errorf("migration committed, but could not print the report: %w", err)
	}
	return err
}

func readRotationSecret(path string) (string, error) {
	const maxFileBytes = 4096
	file, err := os.Open(path) //nolint:gosec // local file explicitly supplied by the operator
	if err != nil {
		return "", fmt.Errorf("open the new secret file: %w", err)
	}
	defer func() {
		// This file is opened for reading only.
		_ = file.Close()
	}()

	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("inspect the new secret file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("the new secret file must be a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return "", errors.New("the new secret file must be private; run chmod 600 on the file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		return "", fmt.Errorf("read the new secret file: %w", err)
	}
	if len(data) > maxFileBytes {
		return "", errors.New("the new secret file must be at most 4096 bytes")
	}
	secret := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if strings.ContainsAny(secret, "\r\n\x00") || strings.TrimSpace(secret) != secret {
		return "", errors.New("the new secret file must contain a single value without surrounding whitespace")
	}
	return secret, nil
}
