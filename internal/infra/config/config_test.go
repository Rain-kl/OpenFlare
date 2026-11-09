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

// TestOTLPProtocolPrecedence covers signal-specific, general, YAML and default settings.
func TestOTLPProtocolPrecedence(t *testing.T) {
	for _, tt := range []struct{ name, signal, general, configured, want string }{
		{name: "default"},
		{name: "yaml", configured: "http/protobuf", want: "http/protobuf"},
		{name: "general", general: "http/protobuf", configured: "grpc", want: "http/protobuf"},
		{name: "signal", signal: " HTTP/PROTOBUF ", general: "grpc", want: "http/protobuf"},
		{name: "empty_signal", signal: " ", general: "grpc", configured: "http/protobuf", want: "grpc"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", tt.signal)
			t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", tt.general)
			cfg := &configModel{Otel: otelConfig{ExportProtocol: tt.configured}}
			applyEnvOverrides(cfg)
			if cfg.Otel.ExportProtocol != tt.want {
				t.Errorf("OTLP protocol = %q, want %q", cfg.Otel.ExportProtocol, tt.want)
			}
		})
	}
}
