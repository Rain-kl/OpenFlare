// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package proxy_route

import "testing"

func TestNormalizeProxyRouteLimitConnValue(t *testing.T) {
	t.Parallel()
	got, err := normalizeProxyRouteLimitConnValue(-1, "limit_conn_per_server")
	if err != nil || got != -1 {
		t.Fatalf("want -1, got %d err %v", got, err)
	}
	if _, err := normalizeProxyRouteLimitConnValue(-2, "limit_conn_per_server"); err == nil {
		t.Fatal("expected error for -2")
	}
}

func TestNormalizeProxyRouteLimitRate(t *testing.T) {
	t.Parallel()
	got, err := normalizeProxyRouteLimitRate("-1")
	if err != nil || got != "-1" {
		t.Fatalf("want -1, got %q err %v", got, err)
	}
	got, err = normalizeProxyRouteLimitRate("0")
	if err != nil || got != "" {
		t.Fatalf("want empty inherit, got %q err %v", got, err)
	}
}
