// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task_test

import (
	"testing"

	"github.com/Rain-kl/Wavelet/internal/task"
	taskhandlers "github.com/Rain-kl/Wavelet/internal/task/handlers"
)

func TestDuplicateTaskMeta(t *testing.T) {
	// Call Register twice to simulate being imported by multiple packages (routers, worker, etc.)
	taskhandlers.Register()
	taskhandlers.Register()

	metas := task.GetDispatchableTasks()

	// Check if we have duplicates by checking if a Type appears more than once
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
