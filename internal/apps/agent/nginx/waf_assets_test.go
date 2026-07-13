package nginx

import (
	"path/filepath"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestWAFRuntime(t *testing.T) {
	state := lua.NewState()
	defer state.Close()

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
