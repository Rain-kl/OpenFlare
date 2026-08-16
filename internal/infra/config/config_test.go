// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config

import "testing"

func TestApplyEnvOverridesRedisMaintNotifications(t *testing.T) {
	t.Setenv("REDIS_MAINT_NOTIFICATIONS", "true")

	cfg := &configModel{}
	applyEnvOverrides(cfg)

	if !cfg.Redis.MaintNotifications {
		t.Fatal("REDIS_MAINT_NOTIFICATIONS=true was not applied")
	}
}

// TestApplyClickHouseDefaultsRespectsDisabledConfig 回归：配置未显式启用（缺省或
// enabled: false）时 ClickHouse 必须保持关闭，不得被 applyClickHouseDefaults 强制打开。
// CLICKHOUSE_ENABLED=true 仅用于绕过测试分支（isTest 默认强制关闭），以便测真实默认逻辑。
func TestApplyClickHouseDefaultsRespectsDisabledConfig(t *testing.T) {
	t.Setenv("CLICKHOUSE_ENABLED", "true")

	cfg := &configModel{ClickHouse: clickHouseConfig{Enabled: false}}
	applyClickHouseDefaults(cfg)

	if cfg.ClickHouse.Enabled {
		t.Fatal("ClickHouse must stay disabled when config does not enable it")
	}
}

// TestApplyClickHouseDefaultsEnablesWhenConfigured 显式 enabled: true 时保持启用并补齐默认连接参数。
func TestApplyClickHouseDefaultsEnablesWhenConfigured(t *testing.T) {
	t.Setenv("CLICKHOUSE_ENABLED", "true")

	cfg := &configModel{ClickHouse: clickHouseConfig{Enabled: true}}
	applyClickHouseDefaults(cfg)

	if !cfg.ClickHouse.Enabled {
		t.Fatal("ClickHouse must stay enabled when explicitly configured")
	}
	if cfg.ClickHouse.Database != "openflare" {
		t.Fatalf("database default not applied: %q", cfg.ClickHouse.Database)
	}
	if len(cfg.ClickHouse.Hosts) != 1 || cfg.ClickHouse.Hosts[0] != "127.0.0.1:9000" {
		t.Fatalf("hosts default not applied: %v", cfg.ClickHouse.Hosts)
	}
}
