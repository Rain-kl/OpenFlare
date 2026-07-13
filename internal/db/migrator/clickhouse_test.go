// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package migrator

import (
	"testing"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/pressly/goose/v3"
)

func TestClickHouseMigrationFilesEmbedded(t *testing.T) {
	entries, err := clickhouseMigrationFS.ReadDir(clickhouseMigrationDir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", clickhouseMigrationDir, err)
	}
	if len(entries) == 0 {
		t.Fatal("expected embedded ClickHouse migrations, got none")
	}

	expected := map[string]bool{
		"202606190001_create_user_access_logs.sql": false,
		"202606200001_create_node_access_logs.sql": false,
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, ok := expected[entry.Name()]; ok {
			expected[entry.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Fatalf("expected %s in embedded migrations", name)
		}
	}
}

func TestClickHouseGooseDialect(t *testing.T) {
	if err := goose.SetDialect("clickhouse"); err != nil {
		t.Fatalf("SetDialect(clickhouse) error = %v", err)
	}
}

func TestMigrateClickHouseSkipsWhenDisabled(t *testing.T) {
	previousEnabled := config.Config.ClickHouse.Enabled
	config.Config.ClickHouse.Enabled = false
	t.Cleanup(func() {
		config.Config.ClickHouse.Enabled = previousEnabled
	})

	MigrateClickHouse()
}
