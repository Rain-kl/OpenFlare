// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nginx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestPoWChallengeRendersCustomPageSafely(t *testing.T) {
	state := lua.NewState()
	t.Cleanup(state.Close)
	if err := state.DoString(`
local custom_html = '<HTML><BODY><h1>{{host}}</h1>{{widget}}<p>{{title}}</p></BODY></HTML>'
package.preload["cjson.safe"]=function() return {
    decode=function(value)
        if value=="page" then return {html=custom_html,widget="turnstile"} end
        return {title="<unsafe>",status="Working"}
    end,
    encode=function(value)
        if value.data then stored_challenge=value end
        return '{"marker":"</script><script>unsafe</script>"}'
    end,
} end
io.open=function() return {read=function() return "page" end,close=function() end} end
ngx={var={host='<img src=x onerror=1>',openflare_waf_site="site",http_accept_language="en"},
    req={get_uri_args=function() return {redir="/admin?ok=1"} end},
    ctx={openflare_pow_config={difficulty=2,session_ttl=120,challenge_ttl=60}},
    shared={openflare_pow_config={},openflare_pow_challenges={set=function() return true end}},
    worker={pid=function() return 1 end},now=function() return 1 end,md5=function() return "random" end,
    header={},say=function(value) rendered_html=value end}
`); err != nil {
		t.Fatal(err)
	}
	if err := state.DoString("package.preload['pow.page'] = function()\n" + openRestyPowPageLua + "\nend\nrequire('pow.page')"); err != nil {
		t.Fatal(err)
	}
	if err := state.DoString(`io.open = function() error("challenge request must not read files") end`); err != nil {
		t.Fatal(err)
	}
	if err := state.DoString(openRestyPowChallengeLua); err != nil {
		t.Fatalf("challenge.lua(custom page) error = %v", err)
	}
	html := state.GetGlobal("rendered_html").String()
	for _, want := range []string{"&lt;img src=x onerror=1&gt;", "&lt;unsafe&gt;", `<openflare-pow appearance="turnstile">`, `\u003c/script>`, "openflare-pow-config"} {
		if !strings.Contains(html, want) {
			t.Errorf("challenge.lua(custom page) HTML missing %q: %s", want, html)
		}
	}
	if strings.Contains(html, "<img src=x") || strings.Contains(html, "<script>unsafe") {
		t.Errorf("challenge.lua(custom page) rendered unescaped request or JSON data: %s", html)
	}
	stored := state.GetGlobal("stored_challenge").(*lua.LTable)
	if stored.RawGetString("site").String() != "site" || stored.RawGetString("redir").String() != "/admin?ok=1" {
		t.Errorf("challenge.lua() stored challenge = %v, want site-bound original path", stored)
	}
}

func TestPoWVerificationBindsSiteAndPreventsReplay(t *testing.T) {
	data := "test-challenge"
	nonce := 0
	var hash string
	for {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s%d", data, nonce)))
		hash = hex.EncodeToString(sum[:])
		if strings.HasPrefix(hash, "0") {
			break
		}
		nonce++
	}
	for _, site := range []string{"site", "other-site"} {
		t.Run(site, func(t *testing.T) {
			state := lua.NewState()
			t.Cleanup(state.Close)
			state.SetGlobal("hash", state.NewFunction(func(l *lua.LState) int {
				sum := sha256.Sum256([]byte(l.CheckString(1)))
				l.Push(lua.LString(hex.EncodeToString(sum[:])))
				return 1
			}))
			state.SetGlobal("test_nonce", lua.LNumber(nonce))
			state.SetGlobal("test_response", lua.LString(hash))
			state.SetGlobal("test_site", lua.LString(site))
			if err := state.DoString(`
local consumed=false
package.preload["cjson.safe"]=function() return {
 decode=function() return {data="test-challenge",difficulty=1,host="example.com",site="site",redir="/original",session_ttl=600} end,
 encode=function(value) result_ok=value.ok;return "result" end,
} end
package.preload["resty.sha256"]=function() return {new=function() return {
 update=function(self,value) self.value=value end, final=function(self) return hash(self.value) end,
} end} end
package.preload["resty.string"]=function() return {to_hex=function(value) return value end} end
ngx={var={openflare_waf_site=test_site,host="example.com",scheme="https"},
 req={get_uri_args=function() return {id="challenge",response=test_response,nonce=tostring(test_nonce),redir="https://evil.example"} end},
 shared={openflare_pow_challenges={get=function() if not consumed then return "stored" end end,delete=function() consumed=true end},
 openflare_pow_sessions={set=function() session_created=true end}},
 worker={pid=function()return 1 end},now=function() return 1 end,sha1_bin=function()return "session" end,
 header={},say=function()end,redirect=function(target)redirect_target=target end}
`); err != nil {
				t.Fatal(err)
			}
			if err := state.DoString(openRestyPowVerifyLua); err != nil {
				t.Fatal(err)
			}
			if site != "site" {
				if state.GetGlobal("session_created") != lua.LNil {
					t.Error("verify(cross-site challenge) created a session")
				}
				return
			}
			if state.GetGlobal("session_created") != lua.LTrue {
				t.Error("verify(valid SHA256 proof) did not create a session")
			}
			if got := state.GetGlobal("redirect_target").String(); got != "/original" {
				t.Errorf("verify(untrusted redirect) target = %q, want /original", got)
			}
			if err := state.DoString(openRestyPowVerifyLua); err != nil {
				t.Fatal(err)
			}
			ngx := state.GetGlobal("ngx").(*lua.LTable)
			if got := ngx.RawGetString("status"); got != lua.LNumber(410) {
				t.Errorf("verify(replayed proof) status = %v, want 410", got)
			}
		})
	}
}

func TestPoWPathPolicy(t *testing.T) {
	state := lua.NewState()
	t.Cleanup(state.Close)
	preloadIPMatcher(t, state)
	ngx := state.NewTable()
	re := state.NewTable()
	re.RawSetString("find", state.NewFunction(func(l *lua.LState) int {
		pattern, err := regexp.Compile(l.CheckString(2))
		if err != nil {
			l.Push(lua.LNil)
			return 1
		}
		match := pattern.FindStringIndex(l.CheckString(1))
		if match == nil {
			l.Push(lua.LNil)
		} else {
			l.Push(lua.LNumber(match[0] + 1))
		}
		return 1
	}))
	ngx.RawSetString("re", re)
	state.SetGlobal("ngx", ngx)
	chunk, err := state.LoadString(openRestyPowPolicyLua)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.CallByParam(lua.P{Fn: chunk, NRet: 1, Protect: true}); err != nil {
		t.Fatal(err)
	}
	policy := state.Get(-1)
	state.Pop(1)
	tests := []struct {
		path, mode, matchType, value, allowType, allowValue string
		want                                                bool
	}{
		{path: "/", mode: "all", want: true},
		{path: "/admin", mode: "blacklist", matchType: "exact", value: "/admin", want: true},
		{path: "/administrator", mode: "blacklist", matchType: "exact", value: "/admin", want: false},
		{path: "/admin/a", mode: "blacklist", matchType: "prefix", value: "/admin/", want: true},
		{path: "/admin/a/b", mode: "blacklist", matchType: "wildcard", value: "/admin/*", want: true},
		{path: "/admin/", mode: "blacklist", matchType: "wildcard", value: "/admin/*", want: true},
		{path: "/fileXjson", mode: "blacklist", matchType: "wildcard", value: "/*.json", want: false},
		{path: "/FILE.json", mode: "blacklist", matchType: "wildcard", value: "/file.*", want: false},
		{path: "/admin/a", mode: "blacklist", matchType: "regex", value: `^/admin(?:/|$)`, want: true},
		{path: "/administrator", mode: "blacklist", matchType: "regex", value: `^/admin(?:/|$)`, want: false},
		{path: "/admin/health", mode: "blacklist", matchType: "prefix", value: "/admin/", allowType: "exact", allowValue: "/admin/health", want: false},
		{path: "/assets/a.js", mode: "all", allowType: "wildcard", allowValue: "/assets/*", want: false},
		{path: "/public", mode: "blacklist", matchType: "exact", value: "/admin", want: false},
		{path: "/empty", mode: "blacklist", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.path+tt.matchType+tt.allowType, func(t *testing.T) {
			rules := state.NewTable()
			rules.RawSetString("mode", lua.LString(tt.mode))
			for _, item := range []struct{ key, kind, value string }{{"blacklist", tt.matchType, tt.value}, {"whitelist", tt.allowType, tt.allowValue}} {
				list := state.NewTable()
				if item.kind != "" {
					rule := state.NewTable()
					rule.RawSetString("type", lua.LString(item.kind))
					rule.RawSetString("value", lua.LString(item.value))
					list.Append(rule)
				}
				rules.RawSetString(item.key, list)
			}
			if err := state.CallByParam(lua.P{Fn: state.GetField(policy, "should_challenge"), NRet: 1, Protect: true}, lua.LString(tt.path), rules); err != nil {
				t.Fatal(err)
			}
			got := lua.LVAsBool(state.Get(-1))
			state.Pop(1)
			if got != tt.want {
				t.Errorf("should_challenge(%q, %+v) = %t, want %t", tt.path, tt, got, tt.want)
			}
		})
	}
}

func TestPoWAddressAndLegacyPathPolicy(t *testing.T) {
	state := lua.NewState()
	t.Cleanup(state.Close)
	preloadIPMatcher(t, state)
	if err := state.DoString(`ngx = {var = {}, shared = {}, log = function() end}`); err != nil {
		t.Fatal(err)
	}
	chunk, err := state.LoadString(openRestyPowPolicyLua)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.CallByParam(lua.P{Fn: chunk, NRet: 1, Protect: true}); err != nil {
		t.Fatal(err)
	}
	policy := state.Get(-1)
	state.Pop(1)
	for _, tt := range []struct {
		name, key, entry, ip, path string
		want                       bool
	}{
		{name: "IPv4 CIDR", key: "ip_cidrs", entry: "192.0.2.0/24", ip: "192.0.2.1", want: true},
		{name: "IPv4 miss", key: "ip_cidrs", entry: "192.0.2.0/24", ip: "198.51.100.1"},
		{name: "IPv6 CIDR", key: "ip_cidrs", entry: "2001:db8::/32", ip: "2001:db8:1::1", want: true},
		{name: "IPv6 miss", key: "ip_cidrs", entry: "2001:db8::/32", ip: "2001:db9::1"},
		{name: "IPv6 exact canonical", key: "ips", entry: "2001:db8::1", ip: "2001:0db8:0:0:0:0:0:1", want: true},
		{name: "IPv4 exact", key: "ips", entry: "192.0.2.1", ip: "192.0.2.1", want: true},
		{name: "IPv4 zero prefix", key: "ip_cidrs", entry: "0.0.0.0/0", ip: "203.0.113.1", want: true},
		{name: "address family isolation", key: "ip_cidrs", entry: "0.0.0.0/0", ip: "2001:db8::1"},
		{name: "legacy wildcard", key: "paths", entry: "/admin/*", path: "/ADMIN/users", want: true},
		{name: "legacy path literal", key: "paths", entry: "/file.json", path: "/fileXjson"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			list, entries := state.NewTable(), state.NewTable()
			entries.Append(lua.LString(tt.entry))
			list.RawSetString(tt.key, entries)
			if err := state.CallByParam(lua.P{Fn: state.GetField(policy, "match_any"), NRet: 1, Protect: true},
				lua.LString(tt.ip), lua.LString("test"), lua.LString(tt.path), list); err != nil {
				t.Fatal(err)
			}
			got := lua.LVAsBool(state.Get(-1))
			state.Pop(1)
			if got != tt.want {
				t.Errorf("match_any(ip=%q, path=%q, %s=%q) = %t, want %t", tt.ip, tt.path, tt.key, tt.entry, got, tt.want)
			}
		})
	}
}

func TestPoWConfigReadsOnlyAtWorkerInitialization(t *testing.T) {
	state := lua.NewState()
	t.Cleanup(state.Close)
	state.SetGlobal("pow_config_source", lua.LString(openRestyPowConfigLua))
	if err := state.DoString(`
local reads, preparations = 0, 0
local disk = {site = {enabled = true, config = {difficulty = 4}}}
package.preload["cjson.safe"] = function() return {decode = function() return disk end} end
package.preload["pow.policy"] = function() return {prepare = function() preparations = preparations + 1 end} end
io.open = function()
    reads = reads + 1
    return {read = function() return "config" end, close = function() end}
end
local first = assert(loadstring(pow_config_source))()
assert(reads == 1 and preparations == 1, "worker must load and prepare policies once")
disk = {site = {enabled = false, config = {}}}
for _ = 1, 20 do assert(first.get("site").enabled == true) end
assert(reads == 1, "requests must not poll configuration files")
local second = assert(loadstring(pow_config_source))()
assert(second.get("site").enabled == false, "new worker must load the new published snapshot")
assert(first.get("site").enabled == true, "old worker must retain its immutable snapshot")
assert(reads == 2 and preparations == 2, "each worker must prepare its own snapshot")
io.open = function() return nil, "missing", 2 end
assert(assert(loadstring(pow_config_source))().get("site") == nil, "missing policy file must clear bindings in new worker")
`); err != nil {
		t.Fatal(err)
	}
}

func TestPoWConfigRejectsBrokenSnapshot(t *testing.T) {
	for _, setup := range []string{
		`io.open = function() return {read = function() return "invalid" end, close = function() end} end`,
		`io.open = function() return nil, "permission denied", 13 end`,
	} {
		t.Run(setup, func(t *testing.T) {
			state := lua.NewState()
			t.Cleanup(state.Close)
			if err := state.DoString(`
package.preload["cjson.safe"] = function() return {decode = function() return nil, "invalid JSON" end} end
package.preload["pow.policy"] = function() return {} end
` + setup); err != nil {
				t.Fatal(err)
			}
			if err := state.DoString(openRestyPowConfigLua); err == nil {
				t.Errorf("load pow_config.json (%s) succeeded, want initialization error", setup)
			}
		})
	}
}

func TestPoWStandalonePolicySkipsLegacyNode(t *testing.T) {
	state := lua.NewState()
	t.Cleanup(state.Close)
	if err := state.DoString(`
package.preload["pow.policy"]=function() return {} end
package.preload["cjson.safe"]=function() return {} end
ngx={shared={},ctx={openflare_pow_standalone={enabled=false}}}
`); err != nil {
		t.Fatal(err)
	}
	chunk, err := state.LoadString(openRestyPowRuntimeLua)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.CallByParam(lua.P{Fn: chunk, NRet: 1, Protect: true}); err != nil {
		t.Fatal(err)
	}
	runtime := state.Get(-1)
	state.Pop(1)
	if err := state.CallByParam(lua.P{Fn: state.GetField(runtime, "evaluate"), NRet: 1, Protect: true}, state.NewTable()); err != nil {
		t.Fatal(err)
	}
	if got := state.Get(-1); got != lua.LTrue {
		t.Errorf("evaluate(legacy node with standalone disabled policy) = %v, want true", got)
	}
}

func TestPowRuntimePassesConfigKeyToInternalChallenge(t *testing.T) {
	state := lua.NewState()
	defer state.Close()

	if err := state.DoString(`
package.preload["pow.policy"] = function()
    return {
        match_any = function() return false end,
        has_entries = function() return false end,
    }
end
package.preload["cjson.safe"] = function()
    return { encode = function() return "{}" end }
end

local config_values = {}
local config_dict = {}
function config_dict:set(key, value) config_values[key] = value return true end
function config_dict:get(key) return config_values[key] end

local sessions = {}
function sessions:get() return nil end
function sessions:set() return true end

ngx = {
    var = {
        host = "pow.example.com",
        uri = "/protected",
        scheme = "https",
        remote_addr = "192.0.2.1",
        http_user_agent = "test",
        request_id = "request-1",
    },
    ctx = {},
    header = {},
    shared = {
        openflare_pow_sessions = sessions,
        openflare_pow_config = config_dict,
    },
    req = {},
    now = function() return 1 end,
}
function ngx.req.set_uri_args(args) captured_uri_args = args end
function ngx.exec(uri, args)
    captured_exec_uri = uri
    captured_exec_args = args
end
`); err != nil {
		t.Fatalf("prepare Lua runtime: %v", err)
	}

	chunk, err := state.LoadString(openRestyPowRuntimeLua)
	if err != nil {
		t.Fatalf("load PoW runtime: %v", err)
	}
	if err := state.CallByParam(lua.P{Fn: chunk, NRet: 1, Protect: true}); err != nil {
		t.Fatalf("initialize PoW runtime: %v", err)
	}
	runtimeModule := state.Get(-1)
	state.Pop(1)

	evaluate := state.GetField(runtimeModule, "evaluate")
	config := state.NewTable()
	config.RawSetString("challenge_ttl", lua.LNumber(300))
	if err := state.CallByParam(lua.P{Fn: evaluate, NRet: 1, Protect: true}, config); err != nil {
		t.Fatalf("evaluate PoW node: %v", err)
	}
	state.Pop(1)

	if got := state.GetGlobal("captured_exec_uri").String(); got != "/.within.website/x/cmd/anubis/api/make-challenge" {
		t.Fatalf("unexpected internal challenge URI: %q", got)
	}
	execArgs, ok := state.GetGlobal("captured_exec_args").(*lua.LTable)
	if !ok {
		t.Fatal("expected ngx.exec to receive explicit challenge arguments")
	}
	if got := execArgs.RawGetString("openflare_pow_config_key").String(); got != "_request_config:request-1" {
		t.Fatalf("unexpected PoW config key: %q", got)
	}
	if state.GetGlobal("captured_uri_args") != execArgs {
		t.Fatal("expected URI arguments and internal redirect arguments to use the same table")
	}
}
