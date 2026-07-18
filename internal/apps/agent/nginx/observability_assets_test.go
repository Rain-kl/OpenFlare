package nginx

import (
	"strings"
	"testing"
)

func TestManagedObservabilityLuaIsHealthOnly(t *testing.T) {
	t.Parallel()

	files := ManagedObservabilityLuaFiles()
	var logLua, readLua string
	for _, file := range files {
		switch file.Path {
		case "log.lua":
			logLua = file.Content
		case "read.lua":
			readLua = file.Content
		}
	}
	if logLua == "" || readLua == "" {
		t.Fatal("expected log.lua and read.lua")
	}
	// Business counters must not be written in log phase.
	if strings.Contains(logLua, "openresty_rx_bytes") ||
		strings.Contains(logLua, "request_count") {
		t.Fatal("log.lua must not accumulate business counters")
	}
	if !strings.Contains(readLua, "connections") || !strings.Contains(readLua, "ok") {
		t.Fatal("read.lua must expose ok + connections health snapshot")
	}
	if strings.Contains(readLua, "top_domains") || strings.Contains(readLua, "request_count") {
		t.Fatal("read.lua must not expose business traffic aggregates")
	}
}
