// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"Wavelet/core"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sample services for testing
type SampleService interface {
	Greet(name string) string
}

type sampleServiceImpl struct {
	prefix string
}

func (s *sampleServiceImpl) Greet(name string) string {
	if s.prefix != "" {
		return s.prefix + " " + name
	}
	return "Hello, " + name
}

type LogService interface {
	Log(msg string)
}

type logServiceImpl struct {
	logs []string
}

func (l *logServiceImpl) Log(msg string) {
	l.logs = append(l.logs, msg)
}

type ConfigService interface {
	Get(key string) string
}

type configServiceImpl struct {
	data map[string]string
}

func (c *configServiceImpl) Get(key string) string {
	return c.data[key]
}

// Sample plugin for testing
type samplePlugin struct {
	name string
}

func (p *samplePlugin) Name() string {
	return p.name
}

func (p *samplePlugin) Apply(ctx *core.Context) error {
	core.Provide[SampleService](ctx, &sampleServiceImpl{prefix: "Plugin:"})
	return nil
}

func (p *samplePlugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        p.name,
		Version:     "1.0.0",
		Description: "Sample plugin",
	}
}

// Sample driver for testing
type mockDriver struct {
	driverType core.DriverType
	started    bool
	stopped    bool
}

func (m *mockDriver) Type() core.DriverType {
	return m.driverType
}

func (m *mockDriver) Start(ctx context.Context) error {
	m.started = true
	return nil
}

func (m *mockDriver) Stop(ctx context.Context) error {
	m.stopped = true
	return nil
}

func TestContextProvideAndInject(t *testing.T) {
	ctx := core.NewContext(context.Background())

	// Before providing, Inject should fail
	_, err := core.Inject[SampleService](ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, core.ErrServiceNotFound))
	assert.False(t, core.Has[SampleService](ctx))

	// MustInject should panic
	assert.Panics(t, func() {
		core.MustInject[SampleService](ctx)
	})

	// Provide service
	svcImpl := &sampleServiceImpl{prefix: "Hello,"}
	core.Provide[SampleService](ctx, svcImpl)

	// Inject should succeed
	assert.True(t, core.Has[SampleService](ctx))
	svc, err := core.Inject[SampleService](ctx)
	require.NoError(t, err)
	assert.Equal(t, "Hello, Wavelet", svc.Greet("Wavelet"))

	// MustInject should succeed
	mustSvc := core.MustInject[SampleService](ctx)
	assert.Equal(t, "Hello, Cordis", mustSvc.Greet("Cordis"))
}

func TestContextProvideNilPanics(t *testing.T) {
	ctx := core.NewContext(context.Background())

	assert.Panics(t, func() {
		core.Provide[SampleService](nil, &sampleServiceImpl{})
	})

	assert.Panics(t, func() {
		var nilSvc SampleService
		core.Provide[SampleService](ctx, nilSvc)
	})

	assert.Panics(t, func() {
		var nilImpl *sampleServiceImpl
		core.Provide[*sampleServiceImpl](ctx, nilImpl)
	})

	// Inject with nil context
	var nilCtx *core.Context
	_, err := core.Inject[SampleService](nilCtx)
	assert.ErrorIs(t, err, core.ErrNilContext)
}

func TestContextUsing(t *testing.T) {
	ctx := core.NewContext(context.Background())
	var called bool

	// Using when service not ready should return ErrServiceNotReady
	err := core.Using(ctx, func(s SampleService) {
		called = true
		assert.Equal(t, "Hello, Cordis", s.Greet("Cordis"))
	})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, core.ErrServiceNotReady))
	assert.False(t, called)

	// Provide service and try Using again
	core.Provide[SampleService](ctx, &sampleServiceImpl{})
	err = core.Using(ctx, func(s SampleService) {
		called = true
		assert.Equal(t, "Hello, Cordis", s.Greet("Cordis"))
	})
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestContextUsingMultiple(t *testing.T) {
	ctx := core.NewContext(context.Background())

	// Using2 with missing dependencies
	var called2 bool
	err := core.Using2(ctx, func(s SampleService, l LogService) {
		called2 = true
	})
	assert.Error(t, err)
	assert.False(t, called2)

	// Provide 1 of 2
	core.Provide[SampleService](ctx, &sampleServiceImpl{})
	err = core.Using2(ctx, func(s SampleService, l LogService) {
		called2 = true
	})
	assert.Error(t, err)
	assert.False(t, called2)

	// Provide 2 of 2
	logSvc := &logServiceImpl{}
	core.Provide[LogService](ctx, logSvc)
	err = core.Using2(ctx, func(s SampleService, l LogService) {
		called2 = true
		l.Log(s.Greet("World"))
	})
	assert.NoError(t, err)
	assert.True(t, called2)
	assert.Equal(t, []string{"Hello, World"}, logSvc.logs)

	// Using3 test - error condition
	err = core.Using3(ctx, func(s SampleService, l LogService, c ConfigService) {})
	assert.Error(t, err)

	// Using3 test - success condition
	var called3 bool
	cfgSvc := &configServiceImpl{data: map[string]string{"env": "test"}}
	core.Provide[ConfigService](ctx, cfgSvc)

	err = core.Using3(ctx, func(s SampleService, l LogService, c ConfigService) {
		called3 = true
		assert.Equal(t, "test", c.Get("env"))
	})
	assert.NoError(t, err)
	assert.True(t, called3)
}

// UsingN must keep every dependency failure reachable through the error chain,
// not just report that something went wrong.
func TestContextUsingMultipleErrorChain(t *testing.T) {
	ctx := core.NewContext(context.Background())

	err := core.Using2(ctx, func(s SampleService, l LogService) {
		t.Fatal("callback must not run when dependencies are missing")
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, core.ErrServiceNotReady)
	assert.ErrorIs(t, err, core.ErrServiceNotFound)

	// Only LogService is missing now, so exactly one joined cause must be present.
	core.Provide[SampleService](ctx, &sampleServiceImpl{})
	err = core.Using2(ctx, func(s SampleService, l LogService) {
		t.Fatal("callback must not run when a dependency is missing")
	})
	assert.ErrorIs(t, err, core.ErrServiceNotReady)
	assert.ErrorIs(t, err, core.ErrServiceNotFound)

	err = core.Using3(ctx, func(s SampleService, l LogService, c ConfigService) {
		t.Fatal("callback must not run when a dependency is missing")
	})
	assert.ErrorIs(t, err, core.ErrServiceNotReady)
	assert.ErrorIs(t, err, core.ErrServiceNotFound)
}

func TestContextHierarchyAndFork(t *testing.T) {
	parent := core.NewContext(nil) // nil base context test
	core.Provide[SampleService](parent, &sampleServiceImpl{prefix: "Parent:"})

	child := parent.ForkWithContext(nil) // nil child context test
	require.NotNil(t, child)
	assert.Equal(t, parent, child.Parent())

	// Child can resolve service from parent
	svc, err := core.Inject[SampleService](child)
	require.NoError(t, err)
	assert.Equal(t, "Parent: Ryan", svc.Greet("Ryan"))

	// Child provides LogService
	childLog := &logServiceImpl{}
	core.ProvideScoped[LogService](child, childLog)

	// Child has LogService, parent does not
	assert.True(t, core.Has[LogService](child))
	assert.False(t, core.Has[LogService](parent))

	// Child overrides SampleService locally
	core.ProvideScoped[SampleService](child, &sampleServiceImpl{prefix: "Child:"})
	childSvc, err := core.Inject[SampleService](child)
	require.NoError(t, err)
	assert.Equal(t, "Child: Ryan", childSvc.Greet("Ryan"))

	parentSvc, err := core.Inject[SampleService](parent)
	require.NoError(t, err)
	assert.Equal(t, "Parent: Ryan", parentSvc.Greet("Ryan"))
}

func TestContextReactiveWhen(t *testing.T) {
	ctx := core.NewContext(context.Background())

	assert.Panics(t, func() {
		core.When[SampleService](nil, func(s SampleService) {})
	})

	var whenCalled atomic.Bool
	var greeted string

	// Register When before service is provided
	core.When[SampleService](ctx, func(s SampleService) {
		whenCalled.Store(true)
		greeted = s.Greet("Reactive")
	})

	assert.False(t, whenCalled.Load())

	// Now Provide the service - listener should trigger
	core.Provide[SampleService](ctx, &sampleServiceImpl{})

	assert.True(t, whenCalled.Load())
	assert.Equal(t, "Hello, Reactive", greeted)

	// Register another When after service is already provided - should trigger immediately
	var immediateCalled bool
	core.When[SampleService](ctx, func(s SampleService) {
		immediateCalled = true
	})
	assert.True(t, immediateCalled)
}

func TestWhenObservesProvideFromForkedFiberContext(t *testing.T) {
	root := core.NewContext(context.Background())
	adminFiber := root.Fork()
	lateFiber := root.Fork()

	var got atomic.Bool
	core.When[SampleService](adminFiber, func(s SampleService) {
		if s != nil {
			got.Store(true)
		}
	})
	assert.False(t, got.Load())

	core.Provide[SampleService](lateFiber, &sampleServiceImpl{})
	assert.True(t, got.Load(), "When on a Fiber child must observe Provide on the root")
}

func TestBindIsWhen(t *testing.T) {
	ctx := core.NewContext(context.Background())
	var called atomic.Bool
	core.Bind[SampleService](ctx, func(s SampleService) {
		called.Store(true)
	})
	core.Provide[SampleService](ctx, &sampleServiceImpl{})
	assert.True(t, called.Load())
}

func TestInjectFromAppContext(t *testing.T) {
	app := core.NewContext(context.Background())
	core.Provide[SampleService](app, &sampleServiceImpl{prefix: "Hi:"})

	req := core.WithAppContext(context.Background(), app)
	svc, err := core.InjectFrom[SampleService](req)
	require.NoError(t, err)
	assert.Equal(t, "Hi: Ada", svc.Greet("Ada"))

	_, err = core.InjectFrom[SampleService](context.Background())
	assert.ErrorIs(t, err, core.ErrNilContext)
}

func TestContextDisposerLifecycle(t *testing.T) {
	parent := core.NewContext(context.Background())
	child := parent.Fork()

	var order []string

	// Test nil disposer
	parent.OnDispose(nil)

	// Test Disposer type
	var customDisposer core.Disposer = func() error {
		order = append(order, "parent-custom")
		return nil
	}
	parent.OnDispose(customDisposer)

	parent.OnDispose(func() error {
		order = append(order, "parent-1")
		return nil
	})
	parent.OnDispose(func() {
		order = append(order, "parent-2")
	})

	child.OnDispose(func() error {
		order = append(order, "child-1")
		return errors.New("child-1 error")
	})
	child.OnDispose(func() {
		order = append(order, "child-2")
	})

	assert.Panics(t, func() {
		parent.OnDispose("invalid-func")
	})

	assert.False(t, parent.IsDisposed())
	assert.False(t, child.IsDisposed())

	// Disposing parent should cascade to children first, and execute disposers in LIFO order
	err := parent.Dispose()
	assert.Error(t, err) // child-1 error should be joined
	assert.Contains(t, err.Error(), "child-1 error")

	assert.True(t, parent.IsDisposed())
	assert.True(t, child.IsDisposed())

	// Child disposers run in LIFO: child-2, child-1
	// Parent disposers run in LIFO: parent-2, parent-1, parent-custom
	expected := []string{"child-2", "child-1", "parent-2", "parent-1", "parent-custom"}
	assert.Equal(t, expected, order)

	// Disposing again should be idempotent and return nil
	err = parent.Dispose()
	assert.NoError(t, err)
}

func TestContextStandardGoContext(t *testing.T) {
	baseCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	defer cancel()

	parentCtx := core.NewContext(baseCtx)
	parentCtx.Set("parent_key", "parent_val")

	childCtx := parentCtx.Fork()

	// Deadline
	dl, ok := childCtx.Deadline()
	assert.True(t, ok)
	assert.False(t, dl.IsZero())

	// Value fallback: child has no key, falls back to parentCtx
	assert.Equal(t, "parent_val", childCtx.Value("parent_key"))

	// GoContext getter
	assert.NotNil(t, childCtx.GoContext())

	// Value not found in either
	assert.Nil(t, childCtx.Value("non_existent_key"))

	// Cancellation propagation
	select {
	case <-childCtx.Done():
		t.Fatal("ctx should not be done yet")
	default:
	}

	cancel()

	select {
	case <-childCtx.Done():
		assert.Equal(t, context.Canceled, childCtx.Err())
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ctx should be cancelled")
	}
}

func TestManifestValidation(t *testing.T) {
	mValid := core.Manifest{
		Name:        "auth",
		Version:     "1.0.0",
		Description: "Auth plugin",
	}
	assert.NoError(t, mValid.Validate())

	mInvalid := core.Manifest{
		Version: "1.0.0",
	}
	assert.Error(t, mInvalid.Validate())
}

func TestDriverRegistration(t *testing.T) {
	ctx := core.NewContext(context.Background())

	// Register nil driver returns error
	assert.ErrorIs(t, ctx.RegisterDriver(nil), core.ErrNilService)

	dHTTP := &mockDriver{driverType: core.DriverTypeHTTP}
	dWorker := &mockDriver{driverType: core.DriverTypeWorker}

	require.NoError(t, ctx.RegisterDriver(dHTTP))
	require.NoError(t, ctx.RegisterDriver(dWorker))

	drivers := ctx.Drivers()
	assert.Len(t, drivers, 2)

	foundHTTP, ok := ctx.Driver(core.DriverTypeHTTP)
	assert.True(t, ok)
	assert.Equal(t, dHTTP, foundHTTP)

	foundWorker, ok := ctx.Driver(core.DriverTypeWorker)
	assert.True(t, ok)
	assert.Equal(t, dWorker, foundWorker)

	_, ok = ctx.Driver(core.DriverTypeScheduler)
	assert.False(t, ok)
}

func TestPluginInterfaces(t *testing.T) {
	ctx := core.NewContext(context.Background())
	var p core.Plugin = &samplePlugin{name: "sample"}
	assert.Equal(t, "sample", p.Name())
	require.NoError(t, p.Apply(ctx))

	svc, err := core.Inject[SampleService](ctx)
	require.NoError(t, err)
	assert.Equal(t, "Plugin: Ryan", svc.Greet("Ryan"))

	var pwm core.PluginWithManifest = &samplePlugin{name: "sample"}
	manifest := pwm.Manifest()
	assert.Equal(t, "sample", manifest.Name)
	assert.Equal(t, "1.0.0", manifest.Version)
}

func TestConcurrentAccess(t *testing.T) {
	ctx := core.NewContext(context.Background())
	var wg sync.WaitGroup

	// Concurrently provide, inject, fork, set, and get
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx.Set(fmt.Sprintf("key-%d", idx), idx)
			_, _ = ctx.Get(fmt.Sprintf("key-%d", idx))

			child := ctx.Fork()
			child.Set("child_key", idx)
		}(i)
	}

	core.Provide[SampleService](ctx, &sampleServiceImpl{})

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc, err := core.Inject[SampleService](ctx)
			if err == nil {
				_ = svc.Greet("Concurrency")
			}
			_ = core.Using(ctx, func(s SampleService) {
				_ = s.Greet("Safe")
			})
		}()
	}

	wg.Wait()
}

func TestContextExtensionPointsAccessors(t *testing.T) {
	ctx := core.NewContext(nil)
	assert.NotNil(t, ctx.Events())
	assert.NotNil(t, ctx.Router())
	assert.NotNil(t, ctx.Migrations())
	assert.NotNil(t, ctx.Tasks())
	assert.NotNil(t, ctx.Task())
	assert.NotNil(t, ctx.Schedules())
	assert.NotNil(t, ctx.Schedule())
	assert.NotNil(t, ctx.Settings())
	assert.NotNil(t, ctx.Setting())

	child := ctx.Fork()
	assert.Equal(t, ctx.Events(), child.Events())
	assert.Equal(t, ctx.Migrations(), child.Migrations())
	assert.NotNil(t, child.Router())
	assert.NotNil(t, child.Tasks())
	assert.NotNil(t, child.Task())
	assert.NotNil(t, child.Schedules())
	assert.NotNil(t, child.Schedule())
	assert.NotNil(t, child.Settings())
	assert.NotNil(t, child.Setting())
}

func TestContext_ScopedExtpoints_RevertibleEffects(t *testing.T) {
	root := core.NewContext(context.Background())
	child := root.Fork()

	// Register route, task, schedule, setting, event, middleware, whitelist on child
	child.Router().GET("/test-route", func() {})
	assert.Equal(t, 1, len(root.Router().Routes()))

	child.Router().Use("scoped_middleware")
	assert.Equal(t, 1, len(root.Router().Middlewares()))

	child.Router().RegisterWhitelist("/api/v1/scoped/*")
	assert.True(t, root.Router().IsWhitelisted("/api/v1/scoped/test"))

	child.Tasks().Register("test:task", func() {})
	assert.Equal(t, 1, len(root.Tasks().Tasks()))

	child.Schedules().RegisterCron("@hourly", "test:cron", nil)
	assert.Equal(t, 1, len(root.Schedules().Schedules()))

	child.Settings().Register(core.SettingSchema{Key: "test.key", Default: "val"})
	assert.Equal(t, 1, len(root.Settings().Schemas()))

	child.On("test:event", func() {})
	assert.Equal(t, 1, root.Events().Listeners("test:event"))

	// Dispose child
	err := child.Dispose()
	assert.NoError(t, err)

	// All child effects should be cleanly revoked in LIFO order
	assert.Equal(t, 0, len(root.Router().Routes()))
	assert.Equal(t, 0, len(root.Router().Middlewares()))
	assert.False(t, root.Router().IsWhitelisted("/api/v1/scoped/test"))
	assert.Equal(t, 0, len(root.Tasks().Tasks()))
	assert.Equal(t, 0, len(root.Schedules().Schedules()))
	assert.Equal(t, 0, len(root.Settings().Schemas()))
	assert.Equal(t, 0, root.Events().Listeners("test:event"))
}

func TestContainer_InterfaceResolutionCache(t *testing.T) {
	ctx := core.NewContext(context.Background())
	svc := &sampleServiceImpl{prefix: "Cached:"}

	core.Provide[SampleService](ctx, svc)

	// 1. Initial resolution populates interfaceCache
	res1, err := core.Inject[SampleService](ctx)
	require.NoError(t, err)
	assert.Equal(t, "Cached: Alice", res1.Greet("Alice"))

	// 2. Subsequent resolutions hit interfaceCache
	res2, err := core.Inject[SampleService](ctx)
	require.NoError(t, err)
	assert.Same(t, res1, res2)

	// 3. Concurrent lookups
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := core.Inject[SampleService](ctx)
			assert.NoError(t, e)
			assert.Equal(t, "Cached: Bob", r.Greet("Bob"))
		}()
	}
	wg.Wait()

	// 4. Overriding/providing another service invalidates cache
	svc2 := &sampleServiceImpl{prefix: "Updated:"}
	core.Provide[SampleService](ctx, svc2)

	res3, err := core.Inject[SampleService](ctx)
	require.NoError(t, err)
	assert.Equal(t, "Updated: Alice", res3.Greet("Alice"))
}
