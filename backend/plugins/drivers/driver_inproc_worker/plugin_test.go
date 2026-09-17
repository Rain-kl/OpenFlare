// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_inproc_worker_test

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/pkg/idgen"
	"Wavelet/pkg/testhelper"
	"Wavelet/plugins/drivers/driver_inproc_worker"
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type testDBService struct {
	db *gorm.DB
}

func (m *testDBService) GORM() *gorm.DB { return m.db }

func (m *testDBService) DB(ctx context.Context) *gorm.DB { return m.db.WithContext(ctx) }

func (m *testDBService) Named(_ string) *gorm.DB { return m.db }

func TestInprocWorkerPlugin(t *testing.T) {
	require.NoError(t, idgen.Init(1))
	ctx := core.NewContext(context.Background())
	p := driver_inproc_worker.New(
		driver_inproc_worker.WithConcurrency(2),
		driver_inproc_worker.WithShutdownTimeout(time.Second),
	)

	assert.Equal(t, "driver_inproc_worker", p.Name())
	assert.Equal(t, core.DriverTypeWorker, p.Type())
	require.NoError(t, p.Apply(ctx))

	var executedCount atomic.Int32
	ctx.Tasks().Register("test:task", func(ctx context.Context, payload []byte) error {
		executedCount.Add(1)
		return nil
	}, extpoints.WithTaskTimeout(2*time.Second))

	// Start worker driver
	require.NoError(t, p.Start(context.Background()))

	// Enqueue tasks
	taskID, err := driver_inproc_worker.DispatchTask(context.Background(), "test:task", []byte("hello"), "test")
	require.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// Wait for execution
	require.Eventually(t, func() bool {
		return executedCount.Load() == 1
	}, 2*time.Second, 20*time.Millisecond)

	// Stop driver
	require.NoError(t, p.Stop(context.Background()))
}

func TestInprocWorkerDispatchByTypeTracksExecution(t *testing.T) {
	require.NoError(t, idgen.Init(1))
	testDB, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	ctx := core.NewContext(context.Background())
	core.Provide[contracts.DBService](ctx, &testDBService{db: testDB})

	p := driver_inproc_worker.New(
		driver_inproc_worker.WithConcurrency(2),
		driver_inproc_worker.WithShutdownTimeout(time.Second),
	)
	require.NoError(t, p.Apply(ctx))

	var executedCount atomic.Int32
	ctx.Tasks().Register("system:cleanup", func(_ context.Context, _ []byte) (*contracts.TaskResultDTO, error) {
		executedCount.Add(1)
		return &contracts.TaskResultDTO{Message: "cleaned 3 files"}, nil
	},
		extpoints.WithTaskType("system_cleanup"),
		extpoints.WithTaskName("系统垃圾清理"),
		extpoints.WithTaskRetry(1),
		extpoints.WithTaskRetryable(true),
	)

	require.NoError(t, p.Start(context.Background()))
	t.Cleanup(func() {
		_ = p.Stop(context.Background())
	})

	taskID, err := driver_inproc_worker.DispatchTask(context.Background(), "system_cleanup", []byte("payload"), "manual")
	require.NoError(t, err)
	assert.NotEmpty(t, taskID)

	require.Eventually(t, func() bool {
		return executedCount.Load() == 1
	}, 2*time.Second, 20*time.Millisecond, "inproc worker should execute task dispatched by admin type")

	taskSvc, err := core.Inject[contracts.TaskService](ctx)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		execs, total, listErr := taskSvc.ListExecutions(context.Background(), "", "", 1, 10)
		if listErr != nil || total == 0 || len(execs) == 0 {
			return false
		}
		return execs[0].TaskID == taskID && execs[0].Status == "succeeded" && execs[0].TaskName == "系统垃圾清理" && execs[0].Result == "cleaned 3 files"
	}, 2*time.Second, 20*time.Millisecond, "inproc worker should persist a succeeded execution record")
}
