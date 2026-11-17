// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nginx

import (
	"fmt"
	"path/filepath"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// BenchmarkWAFRuleSelection measures Lua rule selection independently of HTTP IO.
func BenchmarkWAFRuleSelection(b *testing.B) {
	for _, count := range []int{10, 1000, 10000} {
		b.Run(fmt.Sprintf("sites=%d", count), func(b *testing.B) {
			state := lua.NewState()
			b.Cleanup(state.Close)
			preloadIPMatcher(b, state)
			path, err := filepath.Abs("waf_runtime.lua")
			if err != nil {
				b.Fatal(err)
			}
			state.SetGlobal("runtime_path", lua.LString(path))
			state.SetGlobal("site_count", lua.LNumber(count))
			if err := state.DoString(`
ngx = { var = {}, shared = {}, log = function() end }
local config = { rule_groups = {}, bindings = {} }
for i = 1, site_count do
    config.rule_groups[i] = { id = i, enabled = true, graph = { nodes = {} } }
    config.bindings[i] = { site_name = "site-" .. i, rule_group_ids = { i } }
end
local runtime = assert(loadfile(runtime_path))()
runtime.init({config = config, ip_groups = {groups = {}}, pow = {}, geo_lookup = function() end})
local site = "site-" .. site_count
function select_rules()
    return runtime.debug_active_rules(site)
end
assert(#select_rules() == 1)
`); err != nil {
				b.Fatal(err)
			}
			call := lua.P{Fn: state.GetGlobal("select_rules"), NRet: 1, Protect: true}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if err := state.CallByParam(call); err != nil {
					b.Fatal(err)
				}
				state.Pop(1)
			}
		})
	}
}
