// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package drivers_test

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/pkg/idgen"
	"Wavelet/pkg/testhelper"
	"Wavelet/plugins/drivers/driver_asynq_cron"
	"Wavelet/plugins/drivers/driver_asynq_worker"
	"Wavelet/plugins/drivers/driver_http"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
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

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHTTPDriverLifecycle(t *testing.T) {
	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(nil))
	require.NoError(t, ctx.Config().Resolve())

	var globalMiddlewareCalled atomic.Bool
	var groupMiddlewareCalled atomic.Bool

	// Register global middleware
	ctx.Router().Use(func(c *gin.Context) {
		globalMiddlewareCalled.Store(true)
		c.Next()
	})

	// Register standard gin handler
	ctx.Router().GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	// Register route group with middleware
	v1 := ctx.Router().Group("/api/v1", func(c *gin.Context) {
		groupMiddlewareCalled.Store(true)
		c.Next()
	})

	v1.POST("/echo", func(c *gin.Context) {
		var req map[string]any
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, req)
	})

	// Register standard http.HandlerFunc
	v1.GET("/legacy", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("legacy response"))
	}))

	// Create and apply HTTP driver with dynamic port
	httpPlugin := driver_http.New(driver_http.WithAddr("127.0.0.1:0"))
	require.Equal(t, "driver_http", httpPlugin.Name())

	err := httpPlugin.Apply(ctx)
	require.NoError(t, err)

	// Verify driver registered in context
	d, ok := ctx.Driver(core.DriverTypeHTTP)
	require.True(t, ok)
	require.Equal(t, core.DriverTypeHTTP, d.Type())

	// Start HTTP Server
	err = d.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, httpPlugin.IsRunning())

	addr := httpPlugin.Addr()
	require.NotEmpty(t, addr)

	// Verify GET /ping
	resp, err := http.Get(fmt.Sprintf("http://%s/ping", addr))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "pong")
	assert.True(t, globalMiddlewareCalled.Load())

	// Verify POST /api/v1/echo
	echoPayload := []byte(`{"title":"test-echo","value":42}`)
	respEcho, err := http.Post(fmt.Sprintf("http://%s/api/v1/echo", addr), "application/json", bytes.NewReader(echoPayload))
	require.NoError(t, err)
	defer respEcho.Body.Close()

	assert.Equal(t, http.StatusOK, respEcho.StatusCode)
	var echoResult map[string]any
	err = json.NewDecoder(respEcho.Body).Decode(&echoResult)
	require.NoError(t, err)
	assert.Equal(t, "test-echo", echoResult["title"])
	assert.Equal(t, float64(42), echoResult["value"])
	assert.True(t, groupMiddlewareCalled.Load())

	// Verify GET /api/v1/legacy
	respLegacy, err := http.Get(fmt.Sprintf("http://%s/api/v1/legacy", addr))
	require.NoError(t, err)
	defer respLegacy.Body.Close()

	assert.Equal(t, http.StatusOK, respLegacy.StatusCode)
	bodyLegacy, err := io.ReadAll(respLegacy.Body)
	require.NoError(t, err)
	assert.Equal(t, "legacy response", string(bodyLegacy))

	// Stop HTTP Server
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = d.Stop(stopCtx)
	require.NoError(t, err)
	assert.False(t, httpPlugin.IsRunning())

	// Idempotent Stop
	err = d.Stop(stopCtx)
	require.NoError(t, err)
}

func TestHTTPDriverAppliesGlobalMiddlewareRegisteredAfterRoutes(t *testing.T) {
	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(nil))
	require.NoError(t, ctx.Config().Resolve())

	ctx.Router().GET("/early", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	var called atomic.Bool
	ctx.Router().Use(func(c *gin.Context) {
		called.Store(true)
		c.Next()
	})

	httpPlugin := driver_http.New(driver_http.WithAddr("127.0.0.1:0"))
	require.NoError(t, httpPlugin.Apply(ctx))
	d, ok := ctx.Driver(core.DriverTypeHTTP)
	require.True(t, ok)
	require.NoError(t, d.Start(context.Background()))
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = d.Stop(stopCtx)
	})

	resp, err := http.Get(fmt.Sprintf("http://%s/early", httpPlugin.Addr()))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	if !called.Load() {
		t.Fatal("Router.Use middleware registered after the route must still run at HTTP Start")
	}
}

func TestAsynqWorkerDriverLifecycle(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	ctx := core.NewContext(context.Background())

	var emailTaskProcessed atomic.Bool
	var simpleTaskProcessed atomic.Bool

	// Register tasks with various handler signatures
	ctx.Tasks().Register("email:send", func(c context.Context, payload []byte) error {
		var data map[string]string
		if err := json.Unmarshal(payload, &data); err != nil {
			return err
		}
		if data["to"] == "user@example.com" {
			emailTaskProcessed.Store(true)
		}
		return nil
	}, extpoints.WithTaskConcurrency(2))

	ctx.Tasks().Register("maintenance:cleanup", func() error {
		simpleTaskProcessed.Store(true)
		return nil
	})

	workerPlugin := driver_asynq_worker.New(
		driver_asynq_worker.WithRedisOpt(asynq.RedisClientOpt{Addr: mr.Addr()}),
		driver_asynq_worker.WithConcurrency(2),
		driver_asynq_worker.WithShutdownTimeout(1*time.Second),
	)
	require.Equal(t, "driver_asynq_worker", workerPlugin.Name())

	err = workerPlugin.Apply(ctx)
	require.NoError(t, err)

	// Verify driver registered
	d, ok := ctx.Driver(core.DriverTypeWorker)
	require.True(t, ok)
	require.Equal(t, core.DriverTypeWorker, d.Type())

	// Start Worker Driver
	err = d.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, workerPlugin.IsRunning())

	// Enqueue tasks using asynq.Client
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: mr.Addr()})
	defer client.Close()

	emailPayload, _ := json.Marshal(map[string]string{"to": "user@example.com"})
	_, err = client.Enqueue(asynq.NewTask("email:send", emailPayload))
	require.NoError(t, err)

	_, err = client.Enqueue(asynq.NewTask("maintenance:cleanup", nil))
	require.NoError(t, err)

	// Wait for worker to consume tasks
	require.Eventually(t, func() bool {
		return emailTaskProcessed.Load() && simpleTaskProcessed.Load()
	}, 3*time.Second, 50*time.Millisecond)

	// Stop Worker Driver
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = d.Stop(stopCtx)
	require.NoError(t, err)
	assert.False(t, workerPlugin.IsRunning())

	// Idempotent Stop
	err = d.Stop(stopCtx)
	require.NoError(t, err)
}

func TestAsynqWorkerDispatchTracksExecution(t *testing.T) {
	_ = idgen.Init(1)
	testDB, mr, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(nil))
	require.NoError(t, ctx.Config().Resolve())
	core.Provide[contracts.DBService](ctx, &testDBService{db: testDB})

	var processed atomic.Bool
	ctx.Tasks().Register("system:cleanup", func(_ context.Context, _ []byte) (*contracts.TaskResultDTO, error) {
		processed.Store(true)
		return &contracts.TaskResultDTO{Message: "cleaned 3 files"}, nil
	},
		extpoints.WithTaskType("system_cleanup"),
		extpoints.WithTaskName("系统垃圾清理"),
		extpoints.WithTaskQueue("default"),
		extpoints.WithTaskRetry(1),
		extpoints.WithTaskRetryable(true),
	)

	workerPlugin := driver_asynq_worker.New(
		driver_asynq_worker.WithRedisOpt(asynq.RedisClientOpt{Addr: mr.Addr()}),
		driver_asynq_worker.WithConcurrency(2),
		driver_asynq_worker.WithShutdownTimeout(2*time.Second),
	)
	require.NoError(t, workerPlugin.Apply(ctx))
	require.NoError(t, workerPlugin.Start(context.Background()))
	t.Cleanup(func() {
		_ = workerPlugin.Stop(context.Background())
	})

	taskSvc, err := core.Inject[contracts.TaskService](ctx)
	require.NoError(t, err)

	taskID, err := taskSvc.Dispatch(context.Background(), "system_cleanup", []byte(`{}`), "manual")
	require.NoError(t, err)
	require.NotEmpty(t, taskID)

	require.Eventually(t, func() bool {
		return processed.Load()
	}, 5*time.Second, 50*time.Millisecond, "asynq worker should execute dispatched func handler")

	require.Eventually(t, func() bool {
		execs, _, listErr := taskSvc.ListExecutions(context.Background(), "", "", 1, 10)
		if listErr != nil || len(execs) == 0 {
			return false
		}
		return execs[0].TaskID == taskID && execs[0].Status == "succeeded" && execs[0].Result == "cleaned 3 files"
	}, 5*time.Second, 50*time.Millisecond, "task execution should become succeeded after worker runs")
}

func TestAsynqCronDriverLifecycle(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	ctx := core.NewContext(context.Background())

	// Register cron schedules
	ctx.Schedules().RegisterCron("@every 1s", "sync:stats", map[string]string{"type": "daily"},
		extpoints.WithScheduleOption("queue", "default"),
		extpoints.WithScheduleOption("retry", 3),
	)
	ctx.Schedules().RegisterCron("0 0 * * *", "report:generate", "raw_payload")

	cronPlugin := driver_asynq_cron.New(
		driver_asynq_cron.WithRedisOpt(asynq.RedisClientOpt{Addr: mr.Addr()}),
		driver_asynq_cron.WithLocation(time.UTC),
	)
	require.Equal(t, "driver_asynq_cron", cronPlugin.Name())

	err = cronPlugin.Apply(ctx)
	require.NoError(t, err)

	// Verify driver registered
	d, ok := ctx.Driver(core.DriverTypeScheduler)
	require.True(t, ok)
	require.Equal(t, core.DriverTypeScheduler, d.Type())

	// Start Cron Scheduler Driver
	err = d.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, cronPlugin.IsRunning())

	// Stop Cron Scheduler Driver
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = d.Stop(stopCtx)
	require.NoError(t, err)
	assert.False(t, cronPlugin.IsRunning())

	// Idempotent Stop
	err = d.Stop(stopCtx)
	require.NoError(t, err)
}

func TestMultipleDriversInContext(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(nil))
	require.NoError(t, ctx.Config().Resolve())

	httpPlugin := driver_http.New(driver_http.WithAddr("127.0.0.1:0"))
	workerPlugin := driver_asynq_worker.New(driver_asynq_worker.WithRedisOpt(asynq.RedisClientOpt{Addr: mr.Addr()}))
	cronPlugin := driver_asynq_cron.New(driver_asynq_cron.WithRedisOpt(asynq.RedisClientOpt{Addr: mr.Addr()}))

	require.NoError(t, httpPlugin.Apply(ctx))
	require.NoError(t, workerPlugin.Apply(ctx))
	require.NoError(t, cronPlugin.Apply(ctx))

	drivers := ctx.Drivers()
	assert.Len(t, drivers, 3)

	_, ok := ctx.Driver(core.DriverTypeHTTP)
	assert.True(t, ok)

	_, ok = ctx.Driver(core.DriverTypeWorker)
	assert.True(t, ok)

	_, ok = ctx.Driver(core.DriverTypeScheduler)
	assert.True(t, ok)

	_, ok = ctx.Driver("non_existent")
	assert.False(t, ok)
}
