package nginx

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

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
