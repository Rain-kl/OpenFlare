// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/repository"
	"context"
	"fmt"
	"math"
	"runtime"
	"time"
)

var startTime = time.Now()

const (
	hoursInDay      = 24
	minutesInHour   = 60
	secondsInMinute = 60
	nanosPerSecond  = 1e9

	logDBNamePostgres       = "postgres"
	logDBNameSQLite         = "sqlite"
	logDBNameClickHouse     = "clickhouse"
	defaultLogRetentionDays = 30

	logMigrationIdle       = "idle"
	logMigrationInProgress = "migrating"

	unknownGCLabel = "未知"
	noGCLabel      = "无"
)

// CollectSystemStatus samples the Go runtime counters for the console status page.
func CollectSystemStatus() model.SystemStatusResponse {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptime := formatDuration(time.Since(startTime))
	numGoroutine := runtime.NumGoroutine()

	var lastGCTime string
	switch {
	case m.LastGC > 0 && m.LastGC <= math.MaxInt64:
		lastGCTime = formatDuration(time.Since(time.Unix(0, int64(m.LastGC))))
	case m.LastGC > 0:
		lastGCTime = unknownGCLabel
	default:
		lastGCTime = noGCLabel
	}

	var lastPause string
	if m.NumGC > 0 {
		lastPause = fmt.Sprintf("%.3fs", float64(m.PauseNs[(m.NumGC-1)%256])/nanosPerSecond)
	} else {
		lastPause = "0.000s"
	}

	return model.SystemStatusResponse{
		Uptime:       uptime,
		NumGoroutine: numGoroutine,
		Alloc:        model.FormatBytes(m.Alloc),
		TotalAlloc:   model.FormatBytes(m.TotalAlloc),
		Sys:          model.FormatBytes(m.Sys),
		Lookups:      m.Lookups,
		Mallocs:      m.Mallocs,
		Frees:        m.Frees,
		HeapAlloc:    model.FormatBytes(m.HeapAlloc),
		HeapSys:      model.FormatBytes(m.HeapSys),
		HeapIdle:     model.FormatBytes(m.HeapIdle),
		HeapInuse:    model.FormatBytes(m.HeapInuse),
		HeapReleased: model.FormatBytes(m.HeapReleased),
		HeapObjects:  m.HeapObjects,
		StackInuse:   model.FormatBytes(m.StackInuse),
		StackSys:     model.FormatBytes(m.StackSys),
		MSpanInuse:   model.FormatBytes(m.MSpanInuse),
		MSpanSys:     model.FormatBytes(m.MSpanSys),
		MCacheInuse:  model.FormatBytes(m.MCacheInuse),
		MCacheSys:    model.FormatBytes(m.MCacheSys),
		BuckHashSys:  model.FormatBytes(m.BuckHashSys),
		GCSys:        model.FormatBytes(m.GCSys),
		OtherSys:     model.FormatBytes(m.OtherSys),
		NextGC:       model.FormatBytes(m.NextGC),
		LastGCTime:   lastGCTime,
		PauseTotalNs: fmt.Sprintf("%.1fs", float64(m.PauseTotalNs)/nanosPerSecond),
		LastPause:    lastPause,
		NumGC:        m.NumGC,
	}
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / hoursInDay
	hours := int(d.Hours()) % hoursInDay
	minutes := int(d.Minutes()) % minutesInHour
	seconds := int(d.Seconds()) % secondsInMinute

	var res string
	if days > 0 {
		res += fmt.Sprintf("%d天", days)
	}
	if hours > 0 {
		res += fmt.Sprintf("%d小时", hours)
	}
	if minutes > 0 {
		res += fmt.Sprintf("%d分钟", minutes)
	}
	if seconds > 0 || res == "" {
		res += fmt.Sprintf("%d秒钟", seconds)
	}
	return res
}

// LogDatabaseStatus reports the active log engine, migration freeze state and retention.
func LogDatabaseStatus(ctx context.Context) model.LogDatabaseStatus {
	activeDB := logDBNameSQLite
	migration := logMigrationIdle
	if rc := GetRiskControlService(ctx); rc != nil {
		activeDB = rc.ActiveLogEngine(ctx)
		if rc.IsLogEngineMigrating(ctx) {
			migration = logMigrationInProgress
		}
	}
	return model.LogDatabaseStatus{
		ActiveDatabase: activeDB,
		Migration:      migration,
		RetentionDays: map[string]int{
			logDBNamePostgres:   retentionOr(ctx, model.ConfigKeyLogRetentionDaysPostgres),
			logDBNameSQLite:     retentionOr(ctx, model.ConfigKeyLogRetentionDaysSQLite),
			logDBNameClickHouse: retentionOr(ctx, model.ConfigKeyLogRetentionDaysClickHouse),
		},
		AvailableTargets: availableLogTargets(activeDB),
	}
}

func retentionOr(ctx context.Context, key string) int {
	v, err := repository.GetIntByKey(ctx, key)
	if err != nil {
		if !isRecordMissing(err) {
			logger.ErrorF(ctx, "读取日志保留天数配置失败 key=%s: %v", key, err)
		}
		return defaultLogRetentionDays
	}
	if v < 1 {
		return defaultLogRetentionDays
	}
	return v
}

func availableLogTargets(active string) []string {
	if active == logDBNameClickHouse {
		if GetDBConfig().Enabled {
			return []string{logDBNamePostgres}
		}
		return []string{logDBNameSQLite}
	}
	if GetClickHouseConfig().Enabled {
		return []string{logDBNameClickHouse}
	}
	return []string{}
}
