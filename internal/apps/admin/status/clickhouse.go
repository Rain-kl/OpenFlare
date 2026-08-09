// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"errors"
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/chwriter"
	"github.com/Rain-kl/Wavelet/internal/apps/risk_control"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/batchwriter"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 日志库名取值（与 model 配置值、logstore provider 分支保持一致）。
const (
	logDBNamePostgres   = "postgres"
	logDBNameSQLite     = "sqlite"
	logDBNameClickHouse = "clickhouse"
)

// defaultLogRetentionDays 日志保留天数配置缺失时的兜底值（与 seed 默认一致）。
const defaultLogRetentionDays = 90

// LogDatabaseStatus 日志库状态。
type LogDatabaseStatus struct {
	ActiveDatabase   string                                     `json:"active_database"`
	Migration        string                                     `json:"migration"` // idle | migrating
	RetentionDays    map[string]int                             `json:"retention_days"`
	AvailableTargets []string                                   `json:"available_targets"`
	ClickHouse       *analyticsmodel.ClickHouseOperationalStats `json:"clickhouse,omitempty"`
}

// GetLogDatabaseStatus 返回当前日志库状态。
// @Summary 获取日志数据库状态
// @Description 返回当前日志主库、迁移状态、各库保留天数与合法迁移目标，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=status.LogDatabaseStatus} "获取成功"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/status/log-database [get]
func GetLogDatabaseStatus(c *gin.Context) {
	ctx := c.Request.Context()
	store, err := logstore.Active(ctx)
	if err != nil {
		logger.ErrorF(ctx, "获取日志存储实例失败: %v", err)
		response.AbortInternal(c, "日志存储初始化失败")
		return
	}

	// 分支判定复用同一 store 实例的 ActiveDatabase，避免与 Active 解析之间出现 TOCTOU。
	activeDB, err := store.Status.ActiveDatabase(ctx)
	if err != nil {
		logger.ErrorF(ctx, "获取日志库状态失败: %v", err)
		response.AbortInternal(c, "获取日志库状态失败")
		return
	}
	migration := "idle"
	if logstore.Migrating(ctx) {
		migration = "migrating"
	}

	out := LogDatabaseStatus{
		ActiveDatabase: activeDB,
		Migration:      migration,
		RetentionDays: map[string]int{
			logDBNamePostgres:   retentionOr(ctx, model.ConfigKeyLogRetentionDaysPostgres),
			logDBNameSQLite:     retentionOr(ctx, model.ConfigKeyLogRetentionDaysSQLite),
			logDBNameClickHouse: retentionOr(ctx, model.ConfigKeyLogRetentionDaysClickHouse),
		},
		AvailableTargets: availableTargets(activeDB),
	}

	if activeDB == logDBNameClickHouse {
		stats, err := store.Status.ClickHouseOperationalStats(ctx)
		if err != nil {
			logger.ErrorF(ctx, "获取 ClickHouse 运行指标失败: %v", err)
		} else {
			stats.BatchWriters = collectBatchWriterStats()
			out.ClickHouse = stats
		}
	}

	c.JSON(http.StatusOK, response.OK(out))
}

// retentionOr 读取保留天数配置，缺失或非法时返回默认值。
func retentionOr(ctx context.Context, key string) int {
	v, err := repository.GetIntByKey(ctx, key)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.ErrorF(ctx, "读取日志保留天数配置失败 key=%s: %v", key, err)
		}
		return defaultLogRetentionDays
	}
	if v < 1 {
		return defaultLogRetentionDays
	}
	return v
}

// availableTargets 返回当前日志主库的合法迁移目标（复用调用方已解析的 active）：
// 当前为 clickhouse 时目标为主库（postgres/sqlite 按启动配置）；当前为主库时目标为 clickhouse（仅 CH 启用时）。
func availableTargets(active string) []string {
	if active == logDBNameClickHouse {
		if config.Config.Database.Enabled {
			return []string{logDBNamePostgres}
		}
		return []string{logDBNameSQLite}
	}
	if config.Config.ClickHouse.Enabled {
		return []string{logDBNameClickHouse}
	}
	return []string{}
}

func collectBatchWriterStats() []batchwriter.Stats {
	out := chwriter.WriterStats()
	if out == nil {
		out = make([]batchwriter.Stats, 0, 1)
	}
	out = append(out, risk_control.LogWriterStats())
	return out
}
