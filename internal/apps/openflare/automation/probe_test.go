// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"net/http"
	"slices"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/automationcontract"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/stretchr/testify/require"
)

func TestProbeAutomationOperationsRegistered(t *testing.T) {
	expected := map[string]struct {
		cli    string
		scope  string
		effect string
	}{
		"openflare_ping_task_list":    {cli: "ping list", scope: "ping:read", effect: "read"},
		"openflare_ping_task_create":  {cli: "ping create", scope: "ping:write", effect: "write"},
		"openflare_ping_task_update":  {cli: "ping update", scope: "ping:write", effect: "write"},
		"openflare_ping_task_delete":  {cli: "ping delete", scope: "ping:write", effect: "write"},
		"openflare_ping_task_reorder": {cli: "ping reorder", scope: "ping:write", effect: "write"},
		"openflare_node_ping_metrics": {cli: "node ping", scope: "metrics:read", effect: "read"},
		"openflare_exec_run":          {cli: "exec run", scope: "exec:write", effect: "write"},
		"openflare_exec_list":         {cli: "exec list", scope: "exec:read", effect: "read"},
		"openflare_exec_get":          {cli: "exec get", scope: "exec:read", effect: "read"},
	}

	byID := make(map[string]automationcontract.Operation)
	for _, op := range automationcontract.Operations() {
		byID[op.ID] = op
	}
	for id, want := range expected {
		op, ok := byID[id]
		require.True(t, ok, "missing operation %s in generated catalog", id)
		require.Equal(t, want.cli, op.CLI)
		require.Equal(t, want.scope, op.Scope)
		require.Equal(t, want.effect, op.Effect)
	}

	// Also verify bindings() wires every catalog operation without panicking.
	bound := bindings()
	require.GreaterOrEqual(t, len(bound), len(expected))
}

func TestExecRunRequiresWebShellWithWriteScope(t *testing.T) {
	previous := config.Config.EnableWebShell
	t.Cleanup(func() { config.Config.EnableWebShell = previous })
	config.Config.EnableWebShell = false

	operations := bindings()
	index := slices.IndexFunc(operations, func(operation binding) bool { return operation.ID == "openflare_exec_run" })
	require.NotEqual(t, -1, index)
	result, failure := operations[index].execute(t.Context(), identity{scope: "exec:write"}, map[string]string{
		"command": "true",
		"clients": `["test-edge"]`,
	})
	require.Nil(t, result)
	require.NotNil(t, failure)
	require.Equal(t, http.StatusForbidden, failure.Status)
	require.Equal(t, "WEBSHELL_DISABLED", failure.Code)
}
