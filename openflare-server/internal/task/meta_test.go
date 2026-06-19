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
