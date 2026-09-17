// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints

import "testing"

// TestHandleRawPreservesTrailingSlash 验证 HandleRaw 能表达 /x 与 /x/ 两条不同路由，
// 而 Handle 会归一化掉尾部斜杠（server 插件的 list 端点历史行为依赖这一点）。
func TestHandleRawPreservesTrailingSlash(t *testing.T) {
	r := &RouterRegistry{}
	g := r.Group("/api/v1/nodes")

	if got := g.BasePath(); got != "/api/v1/nodes" {
		t.Fatalf("BasePath() = %q, want %q", got, "/api/v1/nodes")
	}
	slashless := g.Handle("GET", "")
	slashed := g.HandleRaw("GET", "/")

	if slashless.Path != "/api/v1/nodes" {
		t.Errorf("Handle(\"\") path = %q, want %q", slashless.Path, "/api/v1/nodes")
	}
	if slashed.Path != "/api/v1/nodes/" {
		t.Errorf("HandleRaw(\"/\") path = %q, want %q", slashed.Path, "/api/v1/nodes/")
	}
	if slashed.ID == slashless.ID {
		t.Error("HandleRaw must allocate its own route ID so scoped teardown can unregister both")
	}
	if got := len(r.Routes()); got != 2 {
		t.Errorf("registry routes = %d, want 2", got)
	}
	if !r.UnregisterByID(slashed.ID) {
		t.Error("UnregisterByID(HandleRaw route) = false, want true")
	}
	if got := len(r.Routes()); got != 1 {
		t.Errorf("routes after unregister = %d, want 1", got)
	}
}

// TestRegistryHandleRawKeepsAbsolutePath 根注册表上 HandleRaw 只做绝对化处理。
func TestRegistryHandleRawKeepsAbsolutePath(t *testing.T) {
	r := &RouterRegistry{}
	if got := r.HandleRaw("GET", "/health/").Path; got != "/health/" {
		t.Errorf("path = %q, want %q", got, "/health/")
	}
	if got := r.HandleRaw("POST", "submit").Path; got != "/submit" {
		t.Errorf("path = %q, want %q", got, "/submit")
	}
	if got := r.BasePath(); got != "" {
		t.Errorf("registry BasePath() = %q, want empty", got)
	}
}
