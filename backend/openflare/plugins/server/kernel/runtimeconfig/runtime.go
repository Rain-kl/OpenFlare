// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package runtimeconfig holds process-level config values bound from core.Context.
package runtimeconfig

import (
	"sync"
)

// ClickHouseConfig represents ClickHouse connection parameters.
type ClickHouseConfig struct {
	Enabled         bool     `config:"enabled" env:"CLICKHOUSE_ENABLED" default:"false" autoEnable:"CLICKHOUSE_HOST"`
	Hosts           []string `config:"hosts" env:"CLICKHOUSE_HOST"`
	Username        string   `config:"username" env:"CLICKHOUSE_USERNAME"`
	Password        string   `config:"password" env:"CLICKHOUSE_PASSWORD" secret:"true"`
	Database        string   `config:"database" env:"CLICKHOUSE_NAME" default:"wavelet"`
	MaxIdleConn     int      `config:"max_idle_conn" env:"CLICKHOUSE_MAX_IDLE_CONN" default:"10"`
	MaxOpenConn     int      `config:"max_open_conn" env:"CLICKHOUSE_MAX_OPEN_CONN" default:"50"`
	ConnMaxLifetime int      `config:"conn_max_lifetime" env:"CLICKHOUSE_CONN_MAX_LIFETIME" default:"3600"`
	DialTimeout     int      `config:"dial_timeout" env:"CLICKHOUSE_DIAL_TIMEOUT" default:"10"`
	BlockBufferSize uint8    `config:"block_buffer_size" env:"CLICKHOUSE_BLOCK_BUFFER_SIZE" default:"10"`
}

// Snapshot is the subset of host config remaining OF packages still need.
type Snapshot struct {
	SessionSecret   string
	DatabaseEnabled bool
	ClickHouse      ClickHouseConfig
}

var (
	mu      sync.RWMutex
	current Snapshot
)

// Set replaces the process snapshot.
func Set(s Snapshot) {
	mu.Lock()
	defer mu.Unlock()
	current = s
}

// Get returns the process snapshot.
func Get() Snapshot {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// SessionSecret returns the bound app.session_secret.
func SessionSecret() string {
	return Get().SessionSecret
}

// SetSessionSecret updates the bound session secret.
func SetSessionSecret(secret string) {
	s := Get()
	s.SessionSecret = secret
	Set(s)
}

// DatabaseEnabled reports whether PostgreSQL is the primary store.
func DatabaseEnabled() bool {
	return Get().DatabaseEnabled
}

// ClickHouseEnabled reports whether ClickHouse is enabled.
func ClickHouseEnabled() bool {
	return Get().ClickHouse.Enabled
}

// SetDatabaseEnabled updates the PostgreSQL enabled flag.
func SetDatabaseEnabled(enabled bool) {
	s := Get()
	s.DatabaseEnabled = enabled
	Set(s)
}

// SetClickHouseEnabled updates the ClickHouse enabled flag.
func SetClickHouseEnabled(enabled bool) {
	s := Get()
	s.ClickHouse.Enabled = enabled
	Set(s)
}

// Override replaces DB/CH enablement and returns a restore function.
func Override(databaseEnabled, clickHouseEnabled bool) func() {
	previous := Get()
	SetDatabaseEnabled(databaseEnabled)
	SetClickHouseEnabled(clickHouseEnabled)
	return func() { Set(previous) }
}
