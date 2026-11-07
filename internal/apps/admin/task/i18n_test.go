// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"testing"

	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestLocalizeTaskMetaDoesNotMutateSharedParams(t *testing.T) {
	source := task.TaskMeta{
		Name: "Switch log database", AsynqTask: "openflare:log_db_switch",
		Params: []task.TaskParam{{Name: "target", Label: "Target log database", Placeholder: "postgres|sqlite|clickhouse"}},
	}
	localized := localizeTaskMeta("zh-CN", source)
	assert.Equal(t, "切换日志数据库", localized.Name)
	assert.Equal(t, "目标日志库", localized.Params[0].Label)
	assert.Equal(t, "openflare:log_db_switch", localized.AsynqTask)
	assert.Equal(t, "target", localized.Params[0].Name)
	assert.Equal(t, "Target log database", source.Params[0].Label)
}

func TestLocalizeTaskExecutionsPreservesSourceAndPayload(t *testing.T) {
	source := []model.TaskExecution{{
		TaskName: "OpenFlare SSL 自动续期", Log: "[12:34:56] 开始扫描待续期证书\n",
		Payload: `{"name":"开始扫描待续期证书"}`, TriggeredBy: "用户甲",
	}}
	localized := localizeTaskExecutions("en", source)
	assert.Equal(t, "OpenFlare automatic SSL renewal", localized[0].TaskName)
	assert.Equal(t, "[12:34:56] Scanning certificates due for renewal\n", localized[0].Log)
	assert.Equal(t, source[0].Payload, localized[0].Payload)
	assert.Equal(t, source[0].TriggeredBy, localized[0].TriggeredBy)
	assert.Equal(t, "OpenFlare SSL 自动续期", source[0].TaskName)
}

func TestLocalizeScheduleNamePreservesCustomNames(t *testing.T) {
	meta := &task.TaskMeta{Type: "system_cleanup", Name: "System cleanup"}
	assert.Equal(t, "Scheduled system cleanup", localizeScheduleName("en", "系统定期垃圾清理", meta))
	assert.Equal(t, "系统定期垃圾清理", localizeScheduleName("zh-CN", "Scheduled system cleanup", meta))
	assert.Equal(t, "自定义每周清理", localizeScheduleName("en", "自定义每周清理", meta))
}
