// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package msg_gateway_test

import (
	"Wavelet/core"
	"Wavelet/plugins/domain/msg_gateway"
	"context"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMsgGatewayPluginUnit(t *testing.T) {
	ctx := core.NewContext(context.Background())
	p := msg_gateway.New()
	assert.Equal(t, "msg_gateway", p.Name())
	assert.Equal(t, "1.0.0", p.Manifest().Version)
	require.NoError(t, p.Apply(ctx))

	// Verify migrations
	entry, ok := ctx.Migrations().Get("msg_gateway")
	require.True(t, ok)
	entries, err := fs.ReadDir(entry.FS, entry.Dir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries)

	// Verify tasks
	task, ok := ctx.Tasks().Get("msg_gateway:push_notification")
	require.True(t, ok)
	assert.Equal(t, 3, task.Retry)

	// Verify schedules
	sched, ok := ctx.Schedules().Get("msg_gateway:cleanup_pairing_codes")
	require.True(t, ok)
	assert.Equal(t, "*/10 * * * *", sched.Spec)

	// Verify settings
	setting, ok := ctx.Settings().Get("msg_gateway.max_bindings_per_user")
	require.True(t, ok)
	assert.Equal(t, 5, setting.Default)
}

// TestEveryScheduleHasTaskHandler 回归：RegisterCron 仅登记调度；若同名任务从未
// Register，则每次触发都投递到无人处理的任务类型，清理逻辑静默失效。
func TestEveryScheduleHasTaskHandler(t *testing.T) {
	ctx := core.NewContext(context.Background())
	require.NoError(t, msg_gateway.New().Apply(ctx))

	schedules := ctx.Schedules().Schedules()
	require.NotEmpty(t, schedules)

	for _, sched := range schedules {
		_, ok := ctx.Tasks().Get(sched.TaskType)
		assert.Truef(t, ok, "schedule %q dispatches to task %q, which is never registered",
			sched.Spec, sched.TaskType)
	}
}
