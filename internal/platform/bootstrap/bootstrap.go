// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package bootstrap wires cross-module integrations and process-level subsystem initialization.
// All registrations use sync.Once so entry points can call them safely without import-order side effects.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	admin_push "github.com/Rain-kl/Wavelet/internal/apps/admin/push"
	"github.com/Rain-kl/Wavelet/internal/apps/admin/push/custom_events"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/chwriter"
	ofgeoip "github.com/Rain-kl/Wavelet/internal/apps/openflare/geoip"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	taskhandlers "github.com/Rain-kl/Wavelet/internal/infra/task/handlers"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/platform/lifecycle"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
	"github.com/Rain-kl/Wavelet/pkg/cache/ram"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
)

// Options selects role-specific runtime bootstrap steps for the current process.
type Options struct {
	// API enables HTTP-only subsystems such as the ClickHouse access-log writer.
	API bool
}

// CacheRegistry holds settings for a registered cache type.
type CacheRegistry struct {
	Loader ram.Loader
}

var (
	registerTasksOnce            sync.Once
	registerPushDomainEventsOnce sync.Once
	registerTaskListenersOnce    sync.Once
	initRuntimeOnce              sync.Once

	cacheRegistries   = make(map[string]CacheRegistry)
	cacheRegistriesMu sync.RWMutex

	refreshLocks   = make(map[string]*sync.Mutex)
	refreshLocksMu sync.Mutex
)

// RegisterCache registers a cache type with its Loader for unified preheating and refreshing.
func RegisterCache(configType string, reg CacheRegistry) {
	cacheRegistriesMu.Lock()
	defer cacheRegistriesMu.Unlock()
	cacheRegistries[configType] = reg
}

func getRefreshLock(configType string) *sync.Mutex {
	refreshLocksMu.Lock()
	defer refreshLocksMu.Unlock()
	lock, found := refreshLocks[configType]
	if !found {
		lock = &sync.Mutex{}
		refreshLocks[configType] = lock
	}
	return lock
}

// PreheatAllCaches preheats all registered caches.
func PreheatAllCaches(ctx context.Context) error {
	cacheRegistriesMu.RLock()
	defer cacheRegistriesMu.RUnlock()

	for configType, reg := range cacheRegistries {
		lock := getRefreshLock(configType)
		lock.Lock()
		err := ram.Refresh(ctx, configType, "", reg.Loader)
		lock.Unlock()
		if err != nil {
			logger.ErrorF(ctx, "[Bootstrap] preheating cache type %s failed: %v", configType, err)
		}
	}
	return nil
}

// RegisterTasks registers all built-in task handlers and metadata.
func RegisterTasks() {
	registerTasksOnce.Do(func() {
		taskhandlers.Register()
	})
}

// RegisterPushDomainEvents wires push notification handlers for domain events.
func RegisterPushDomainEvents() {
	registerPushDomainEventsOnce.Do(func() {
		custom_events.Register()
	})
}

// RegisterTaskListeners wires operational listeners to task framework hooks.
func RegisterTaskListeners() {
	registerTaskListenersOnce.Do(func() {
		admin_push.RegisterTaskListeners()
	})
}

// RegisterAPI wires integrations required by the HTTP API process.
func RegisterAPI() {
	RegisterTasks()
	RegisterPushDomainEvents()
}

// RegisterWorker wires integrations required by the task worker process.
func RegisterWorker() {
	RegisterTasks()
	RegisterTaskListeners()
}

// RegisterScheduler wires integrations required by the task scheduler process.
func RegisterScheduler() {
	RegisterTasks()
}

// RegisterAll wires integrations for fused mode (API + Worker + Scheduler).
func RegisterAll() {
	RegisterTasks()
	RegisterPushDomainEvents()
	RegisterTaskListeners()
}

// Init runs shared runtime bootstrap exactly once per process.
// Call from cmd entry points after wiring registration and database migration, not from router.
func Init(ctx context.Context, opts Options) {
	initRuntimeOnce.Do(func() {
		if err := validateAndSeedLogDatabase(ctx); err != nil {
			logger.ErrorF(ctx, "[Bootstrap] 日志主库配置校验失败: %v", err)
			log.Fatalf("[Bootstrap] 日志主库配置校验失败: %v", err)
		}

		// 注入 logstore 配置读取（避免 logstore ↔ repository 循环依赖），并预热激活 store。
		logstore.SetConfigReader(func(ctx context.Context, key string) (string, error) {
			cfg, err := repository.GetSystemConfigByKey(ctx, key)
			if err != nil {
				return "", err
			}
			return cfg.Value, nil
		})
		logstore.Init(ctx)

		// Register config cache loader
		RegisterCache(repository.ConfigCacheType, CacheRegistry{
			Loader: repository.ConfigLoader{},
		})

		// Preheat config cache initially (using PreheatAllCaches)
		if err := PreheatAllCaches(ctx); err != nil {
			logger.ErrorF(ctx, "[Bootstrap] preheating all caches failed: %v", err)
		}

		if err := ofgeoip.EnsureRuntimeProvider(ctx); err != nil {
			logger.ErrorF(ctx, "[Bootstrap] init GeoIP provider failed: %v", err)
		}
		if err := admin_push.SyncEvents(ctx); err != nil {
			logger.ErrorF(ctx, "[Bootstrap] sync push events failed: %v", err)
		}
		if opts.API {
			chwriter.Init(ctx)
		}
	})
}

// validateAndSeedLogDatabase 校验日志主库标记与运行配置的一致性，首次启动 seed。
func validateAndSeedLogDatabase(ctx context.Context) error {
	cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyLogDatabase)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("读取日志主库配置失败: %w", err)
	}
	current := cfg.Value
	if current == "" {
		// 首次启动 seed：CH 启用 → clickhouse；否则随主库。
		current = "sqlite"
		if config.Config.Database.Enabled {
			current = "postgres"
		}
		if config.Config.ClickHouse.Enabled {
			current = "clickhouse"
		}
		// 行缺失时 UpdateSystemConfigFields 仅为 UPDATE 无法插入，改用可创建可更新的 SaveOrUpdateSystemConfig。
		if err := repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyLogDatabase, current); err != nil {
			return fmt.Errorf("初始化日志主库配置失败: %w", err)
		}
		return nil
	}
	switch current {
	case "clickhouse":
		if !config.Config.ClickHouse.Enabled {
			return errors.New("当前日志主库为 ClickHouse 但 ClickHouse 未启用。请先重新启用 ClickHouse 配置并启动，在任务管理运行『切换日志数据库』迁移到 PostgreSQL/SQLite 后再禁用 ClickHouse")
		}
	case "postgres":
		if !config.Config.Database.Enabled {
			return errors.New("当前日志主库为 PostgreSQL 但 PostgreSQL 未启用（当前为 SQLite 主库）。请运行『切换日志数据库』迁回 SQLite 或启用 PostgreSQL")
		}
	case "sqlite":
		if config.Config.Database.Enabled {
			return errors.New("当前日志主库为 SQLite 但当前主库为 PostgreSQL。请运行『切换日志数据库』迁移到 PostgreSQL")
		}
	default:
		return fmt.Errorf("未知的日志主库配置: %s", current)
	}
	return nil
}

// Stop stops all batch writers and background resources.
func Stop(ctx context.Context) {
	lifecycle.Stop(ctx)
}

// ResetInitRuntimeOnceForTest clears initRuntimeOnce so Init can run again in unit tests.
func ResetInitRuntimeOnceForTest() {
	initRuntimeOnce = sync.Once{}
}
