// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/credential"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/migrator"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRotateCredentialsCommand(t *testing.T) {
	dir := t.TempDir()
	database, err := gorm.Open(sqlite.Open(filepath.Join(dir, "openflare.db")), &gorm.Config{})
	require.NoError(t, err)
	previous, previousDatabase := config.Config.App.SessionSecret, db.DB(t.Context())
	const oldSecret = "fixture-old-secret"
	newSecret := strings.Repeat("n", credential.MinSessionSecretBytes)
	config.Config.App.SessionSecret = oldSecret
	db.SetDB(database)
	t.Cleanup(func() {
		config.Config.App.SessionSecret = previous
		db.SetDB(previousDatabase)
		conn, err := database.DB()
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	})
	migrator.Migrate()

	const plaintext = "fixture-credential"
	sealed, err := credential.Seal(plaintext)
	require.NoError(t, err)
	records := []struct {
		model  any
		column string
	}{
		{&model.TLSCertificate{ID: 10, Name: "fixture-tls", KeyPEM: sealed}, "key_pem"},
		{&model.DNSAccount{ID: 10, Name: "fixture-dns", Authorization: sealed}, "authorization"},
		{&model.AcmeAccount{ID: 10, Email: "acme@example.test", PrivateKey: sealed}, "private_key"},
		{&model.CFConnection{ID: 10, Authorization: sealed}, "authorization"},
	}
	for _, record := range records {
		require.NoError(t, database.Create(record.model).Error)
	}
	legacy := model.DNSAccount{ID: 11, Name: "legacy", Authorization: "fixture-plaintext"}
	require.NoError(t, database.Create(&legacy).Error)
	secretFile := filepath.Join(dir, "new-session-secret")
	require.NoError(t, os.WriteFile(secretFile, []byte(newSecret+"\n"), 0o600))

	run := func(args ...string) (string, error) {
		cmd := newRotateCredentialsCommand()
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(append([]string{"--new-secret-file", secretFile}, args...))
		err := cmd.ExecuteContext(t.Context())
		return output.String(), err
	}
	read := func(record any, column string) string {
		var value string
		require.NoError(t, database.Model(record).Where("id = ?", 10).Pluck(column, &value).Error)
		return value
	}
	assertUnchanged := func() {
		t.Helper()
		for _, record := range records {
			require.Equal(t, sealed, read(record.model, record.column))
		}
	}

	output, err := run()
	require.NoError(t, err)
	require.Contains(t, output, "of_cf_connections.authorization: Would migrate 1")
	assertUnchanged()

	config.Config.App.SessionSecret = "incorrect-secret"
	_, err = run("--apply")
	require.ErrorContains(t, err, "unable to decrypt")
	assertUnchanged()
	config.Config.App.SessionSecret = oldSecret

	require.NoError(t, database.Exec(`CREATE TRIGGER reject_rotation BEFORE UPDATE OF authorization
		ON of_cf_connections BEGIN SELECT RAISE(ABORT, 'fixture write failure'); END`).Error)
	_, err = run("--apply")
	require.ErrorContains(t, err, "could not write of_cf_connections.authorization")
	assertUnchanged()
	require.NoError(t, database.Exec("DROP TRIGGER reject_rotation").Error)

	output, err = run("--apply")
	require.NoError(t, err)
	require.Contains(t, output, "of_tls_certificates.key_pem: Migrated 1")
	require.NotContains(t, output, oldSecret)
	require.NotContains(t, output, newSecret)
	require.NotContains(t, output, plaintext)

	output, err = run()
	require.NoError(t, err)
	require.Contains(t, output, "No credentials need re-encryption")
	require.Contains(t, output, "of_tls_certificates.key_pem: Would migrate 0, already using the new secret 1")

	config.Config.App.SessionSecret = newSecret
	for _, record := range records {
		value, err := credential.Open(read(record.model, record.column))
		require.NoError(t, err)
		require.Equal(t, plaintext, value)
	}
	var persisted model.DNSAccount
	require.NoError(t, database.First(&persisted, legacy.ID).Error)
	require.Equal(t, legacy.Authorization, persisted.Authorization)
}
