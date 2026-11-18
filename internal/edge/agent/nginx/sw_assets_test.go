// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nginx

import (
	"os"
	"path/filepath"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestSWRuntimeAndChallenge(t *testing.T) {
	state := lua.NewState()
	defer state.Close()

	runtimePath := filepath.Join(t.TempDir(), "runtime.lua")
	if err := os.WriteFile(runtimePath, []byte(openRestySWRuntimeLua), 0o644); err != nil {
		t.Fatal(err)
	}
	challengePath := filepath.Join(t.TempDir(), "challenge.lua")
	if err := os.WriteFile(challengePath, []byte(openRestySWChallengeLua), 0o644); err != nil {
		t.Fatal(err)
	}
	specPath, err := filepath.Abs("sw_runtime_spec.lua")
	if err != nil {
		t.Fatal(err)
	}
	state.SetGlobal("SW_RUNTIME_PATH", lua.LString(runtimePath))
	state.SetGlobal("SW_CHALLENGE_PATH", lua.LString(challengePath))
	if err := state.DoFile(specPath); err != nil {
		t.Fatalf("SW runtime/challenge specification failed: %v", err)
	}
}
