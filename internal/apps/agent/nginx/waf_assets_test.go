// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nginx

import (
	"path/filepath"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestWAFRuntime(t *testing.T) {
	state := lua.NewState()
	defer state.Close()
	preloadIPMatcher(t, state)

	runtimePath, err := filepath.Abs("waf_runtime.lua")
	if err != nil {
		t.Fatal(err)
	}
	specPath, err := filepath.Abs("waf_runtime_spec.lua")
	if err != nil {
		t.Fatal(err)
	}
	state.SetGlobal("WAF_RUNTIME_PATH", lua.LString(runtimePath))
	if err := state.DoFile(specPath); err != nil {
		t.Fatalf("WAF runtime specification failed: %v", err)
	}
}

func TestWAFIPGroupRefresh(t *testing.T) {
	state := lua.NewState()
	defer state.Close()
	preloadIPMatcher(t, state)

	modulePath, err := filepath.Abs("waf_ip_groups.lua")
	if err != nil {
		t.Fatal(err)
	}
	specPath, err := filepath.Abs("waf_ip_groups_spec.lua")
	if err != nil {
		t.Fatal(err)
	}
	state.SetGlobal("WAF_IP_GROUPS_PATH", lua.LString(modulePath))
	if err := state.DoFile(specPath); err != nil {
		t.Fatalf("WAF IP group refresh specification failed: %v", err)
	}
}

func preloadIPMatcher(t testing.TB, state *lua.LState) {
	t.Helper()
	if err := state.DoString("package.preload['resty.ipmatcher'] = function() return false end\npackage.preload['waf.ip_matcher'] = function()\n" + openRestyWAFIPMatcherLua + "\nend"); err != nil {
		t.Fatal(err)
	}
}

func TestIPMatcherDoesNotRetryBinaryMiss(t *testing.T) {
	state := lua.NewState()
	t.Cleanup(state.Close)
	preloadIPMatcher(t, state)
	if err := state.DoString(`
local binary_calls, text_calls = 0, 0
ngx = {shared = {}, log = function() end}
package.preload["resty.ipmatcher"] = function() return {
    new = function() return {
        match_bin = function(_, ip)
            binary_calls = binary_calls + 1
            if ip == "invalid" then return false, "invalid address" end
            return false
        end,
        match = function() text_calls = text_calls + 1; return false end,
    } end,
} end
local matcher = require("waf.ip_matcher").compile({"192.0.2.0/24"})
assert(matcher:match("198.51.100.1", string.char(198, 51, 100, 1)) == false)
assert(binary_calls == 1 and text_calls == 0, "valid binary miss must not repeat string matching")
assert(matcher:match("198.51.100.1", "invalid") == false)
assert(binary_calls == 2 and text_calls == 1, "invalid binary address must retain string fallback")
`); err != nil {
		t.Fatal(err)
	}
}

func TestWAFPreparesMatchersBeforeRequests(t *testing.T) {
	state := lua.NewState()
	t.Cleanup(state.Close)
	preloadIPMatcher(t, state)
	if err := state.DoString(`
local builds = 0
ngx = {var = {openflare_waf_site = "site", remote_addr = "198.51.100.1"}, ctx = {}, shared = {}}
package.preload["resty.ipmatcher"] = function() return {
    new = function()
        builds = builds + 1
        return {match = function() return false end}
    end,
} end
function matcher_builds() return builds end
`); err != nil {
		t.Fatal(err)
	}
	chunk, err := state.LoadString(openRestyWAFRuntimeLua)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.CallByParam(lua.P{Fn: chunk, NRet: 1, Protect: true}); err != nil {
		t.Fatal(err)
	}
	state.SetGlobal("waf_runtime", state.Get(-1))
	state.Pop(1)
	if err := state.DoString(`
local config = {rule_groups = {{id = 1, enabled = true, graph = {entry = "match", nodes = {
    match = {type = "ip_match", config = {cidrs = {"192.0.2.0/24"}}, next = {["false"] = "allow"}},
    allow = {type = "allow"},
}}}}, bindings = {{site_name = "site", rule_group_ids = {1}}}}
waf_runtime.init({config = config, ip_groups = {groups = {}}, pow = {}, geo_lookup = function() end})
assert(matcher_builds() == 1, "IP matcher must be compiled during initialization")
local first = waf_runtime.debug_active_rules("site")
assert(first == waf_runtime.debug_active_rules("site"), "rule selection must reuse the prepared array")
for _ = 1, 20 do assert(waf_runtime.check() == "ok") end
assert(matcher_builds() == 1, "requests must not compile new matchers")
`); err != nil {
		t.Fatal(err)
	}
}
