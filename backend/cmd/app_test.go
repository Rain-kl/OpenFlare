// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"path/filepath"
	"testing"

	"Wavelet/core"
)

func testSource(t *testing.T) core.ConfigSource {
	t.Helper()
	return core.NewMapSource(map[string]any{
		"app": map[string]any{
			"addr": "127.0.0.1:0",
			"env":  "testing",
		},
		"redis": map[string]any{
			"enabled": false,
		},
		"database": map[string]any{
			"enabled":     false,
			"sqlite_path": filepath.Join(t.TempDir(), "openflare-cmd.db"),
		},
	})
}

func TestNewOpenFlareAppRegistersServerAndWaveletUser(t *testing.T) {
	app := newOpenFlareApp(core.ProfileAPI, core.WithConfigSource(testSource(t)))
	if err := app.Prepare(); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, p := range app.Plugins() {
		names[p.Name()] = true
	}
	for _, n := range []string{"user", "auth", "admin", "server"} {
		if !names[n] {
			t.Errorf("missing plugin %s", n)
		}
	}

	if err := app.Reconcile(); err != nil {
		t.Fatal(err)
	}

	got := map[string]bool{}
	for _, rd := range app.Context().Router().Routes() {
		got[rd.Method+" "+rd.Path] = true
	}
	for _, want := range []string{
		"GET /api/healthz",
		"GET /api/v1/user/self",
		"GET /api/v1/d/nodes",
		"POST /api/v1/cap/challenge",
	} {
		if !got[want] {
			t.Errorf("missing route %s", want)
		}
	}
	for _, drop := range []string{
		"GET /api/health",
		"GET /healthz",
		"POST /api/cap/challenge",
	} {
		if got[drop] {
			t.Errorf("removed route still registered: %s", drop)
		}
	}
}

func TestFreshInstallSeedsOpenFlareDefaults(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fresh.db")
	app := cordisPrepare(t, cordisSQLiteSource(t, dbPath))
	t.Cleanup(func() { _ = app.Context().Dispose() })

	db := openInspectDB(t, dbPath, "")
	defer func() { _ = db.Close() }()

	var tables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name LIKE 'of_%'`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables == 0 {
		t.Fatal("fresh install created no of_* tables")
	}

	rows, err := db.Query(`SELECT task_type FROM w_schedules WHERE task_type LIKE 'of_%' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var got []string
	for rows.Next() {
		var taskType string
		if err := rows.Scan(&taskType); err != nil {
			t.Fatal(err)
		}
		got = append(got, taskType)
	}
	want := []string{
		"of_pages_source_scan",
		"of_ssl_renew",
		"of_uptime_kuma_sync",
		"of_waf_ip_group_sync",
	}
	if len(got) != len(want) {
		t.Fatalf("of_* schedules = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("of_* schedules = %v, want %v", got, want)
		}
	}

	var cleanup int
	if err := db.QueryRow(`SELECT COUNT(*) FROM w_schedules WHERE task_type = 'of_database_auto_cleanup'`).Scan(&cleanup); err != nil {
		t.Fatal(err)
	}
	if cleanup != 0 {
		t.Fatal("must not seed of_database_auto_cleanup")
	}

	var geoip string
	if err := db.QueryRow(`SELECT value FROM w_system_configs WHERE key = 'geoip_provider'`).Scan(&geoip); err != nil {
		t.Fatalf("geoip_provider: %v", err)
	}
	if geoip != "ipinfo" {
		t.Fatalf("geoip_provider = %q, want ipinfo", geoip)
	}
}
