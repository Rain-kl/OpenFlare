// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task_test

import (
	"testing"

	"github.com/Rain-kl/Wavelet/internal/infra/task"
)

func TestDuplicateTaskMeta(t *testing.T) {
	// Registering the same task type repeatedly must remain idempotent.
	meta := task.TaskMeta{
		Type:      "test_duplicate_meta",
		AsynqTask: "test:duplicate_meta",
		Name:      "Duplicate test task",
	}
	task.RegisterTaskMeta(meta)
	task.RegisterTaskMeta(meta)

	metas := task.GetDispatchableTasks()

	seen := make(map[string]int)
	for _, m := range metas {
		seen[m.Type]++
	}

	for taskType, count := range seen {
		if count > 1 {
			t.Errorf("Task type %q registered %d times, expected at most 1", taskType, count)
		}
	}
}

func TestInternalOnlyTaskMetaIsHiddenFromDispatchableTasks(t *testing.T) {
	const taskType = "test_internal_only_meta"
	meta := task.TaskMeta{
		Type:         taskType,
		AsynqTask:    "test:internal_only_meta",
		Name:         "内部测试任务",
		InternalOnly: true,
	}
	task.RegisterTaskMeta(meta)

	registered := task.GetTaskMeta(taskType)
	if registered == nil {
		t.Fatal("GetTaskMeta() did not return internal-only metadata")
	}
	if !registered.InternalOnly {
		t.Fatal("GetTaskMeta() lost InternalOnly flag")
	}

	for _, dispatchable := range task.GetDispatchableTasks() {
		if dispatchable.Type == taskType {
			t.Fatalf("GetDispatchableTasks() exposed internal-only task %q", taskType)
		}
	}
}
