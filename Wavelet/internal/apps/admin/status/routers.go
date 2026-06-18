// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package status 提供系统状态查询接口
package status

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/gin-gonic/gin"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

// startTime 记录服务启动时间
var startTime = time.Now()

const (
	hoursInDay      = 24
	minutesInHour   = 60
	secondsInMinute = 60
	nanosPerSecond  = 1e9
	binaryKB        = 0
	binaryMB        = 1
	binaryGB        = 2
	valueThreshold  = 10 // 格式化时区分整数显示的阈值
)

// SystemStatusResponse 系统状态响应结构体
type SystemStatusResponse struct {
	Uptime       string `json:"uptime"`
	NumGoroutine int    `json:"num_goroutine"`
	Alloc        string `json:"alloc"`
	TotalAlloc   string `json:"total_alloc"`
	Sys          string `json:"sys"`
	Lookups      uint64 `json:"lookups"`
	Mallocs      uint64 `json:"mallocs"`
	Frees        uint64 `json:"frees"`
	HeapAlloc    string `json:"heap_alloc"`
	HeapSys      string `json:"heap_sys"`
	HeapIdle     string `json:"heap_idle"`
	HeapInuse    string `json:"heap_inuse"`
	HeapReleased string `json:"heap_released"`
	HeapObjects  uint64 `json:"heap_objects"`
	StackInuse   string `json:"stack_inuse"`
	StackSys     string `json:"stack_sys"`
	MSpanInuse   string `json:"mspan_inuse"`
	MSpanSys     string `json:"mspan_sys"`
	MCacheInuse  string `json:"mcache_inuse"`
	MCacheSys    string `json:"mcache_sys"`
	BuckHashSys  string `json:"buck_hash_sys"`
	GCSys        string `json:"gc_sys"`
	OtherSys     string `json:"other_sys"`
	NextGC       string `json:"next_gc"`
	LastGCTime   string `json:"last_gc_time"`
	PauseTotalNs string `json:"pause_total_ns"`
	LastPause    string `json:"last_pause"`
	NumGC        uint32 `json:"num_gc"`
}

// formatBytes 格式化字节大小
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	value := float64(bytes) / float64(div)
	var suffix string
	switch exp {
	case binaryKB:
		suffix = "KiB"
	case binaryMB:
		suffix = "MiB"
	case binaryGB:
		suffix = "GiB"
	default:
		suffix = "TiB"
	}

	// 格式化规则：
	// - 如果是整数（如 16, 73, 105, 986, 112）：
	//   - 如果 >= 10，则格式化为 "%.0f" (e.g. "16 KiB")
	//   - 如果 < 10，则格式化为 "%.1f" (e.g. "9.0 KiB")
	// - 如果不是整数（如 5.8, 9.1, 7.6, 4.8）：格式化为 "%.1f"
	if value == math.Trunc(value) {
		if value >= valueThreshold {
			return fmt.Sprintf("%.0f %s", value, suffix)
		}
		return fmt.Sprintf("%.1f %s", value, suffix)
	}
	return fmt.Sprintf("%.1f %s", value, suffix)
}

// formatDuration 格式化时间持续时间
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

// GetSystemStatus 获取系统状态信息
// @Summary 获取系统状态信息
// @Description 获取后端服务运行状态、Goroutine、内存指标等详细统计数据，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=status.SystemStatusResponse} "获取成功"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/status [get]
func GetSystemStatus(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptime := formatDuration(time.Since(startTime))
	numGoroutine := runtime.NumGoroutine()

	var lastGCTime string
	switch {
	case m.LastGC > 0 && m.LastGC <= math.MaxInt64:
		lastGCTime = formatDuration(time.Since(time.Unix(0, int64(m.LastGC))))
	case m.LastGC > 0:
		lastGCTime = "未知"
	default:
		lastGCTime = "无"
	}

	var lastPause string
	if m.NumGC > 0 {
		lastPause = fmt.Sprintf("%.3fs", float64(m.PauseNs[(m.NumGC-1)%256])/nanosPerSecond)
	} else {
		lastPause = "0.000s"
	}

	res := SystemStatusResponse{
		Uptime:       uptime,
		NumGoroutine: numGoroutine,
		Alloc:        formatBytes(m.Alloc),
		TotalAlloc:   formatBytes(m.TotalAlloc),
		Sys:          formatBytes(m.Sys),
		Lookups:      m.Lookups,
		Mallocs:      m.Mallocs,
		Frees:        m.Frees,
		HeapAlloc:    formatBytes(m.HeapAlloc),
		HeapSys:      formatBytes(m.HeapSys),
		HeapIdle:     formatBytes(m.HeapIdle),
		HeapInuse:    formatBytes(m.HeapInuse),
		HeapReleased: formatBytes(m.HeapReleased),
		HeapObjects:  m.HeapObjects,
		StackInuse:   formatBytes(m.StackInuse),
		StackSys:     formatBytes(m.StackSys),
		MSpanInuse:   formatBytes(m.MSpanInuse),
		MSpanSys:     formatBytes(m.MSpanSys),
		MCacheInuse:  formatBytes(m.MCacheInuse),
		MCacheSys:    formatBytes(m.MCacheSys),
		BuckHashSys:  formatBytes(m.BuckHashSys),
		GCSys:        formatBytes(m.GCSys),
		OtherSys:     formatBytes(m.OtherSys),
		NextGC:       formatBytes(m.NextGC),
		LastGCTime:   lastGCTime,
		PauseTotalNs: fmt.Sprintf("%.1fs", float64(m.PauseTotalNs)/nanosPerSecond),
		LastPause:    lastPause,
		NumGC:        m.NumGC,
	}

	c.JSON(http.StatusOK, response.OK(res))
}

// DatabaseInfoResponse 数据库信息响应结构体
type DatabaseInfoResponse struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// getSQLiteInfo 返回 SQLite 数据库信息
func getSQLiteInfo(ctx context.Context) DatabaseInfoResponse {
	info := DatabaseInfoResponse{
		Type:    "sqlite",
		Name:    config.Config.Database.SQLitePath,
		Version: "SQLite",
	}
	if info.Name == "" {
		info.Name = "./data/openflare.db"
	}
	gormDB := db.DB(ctx)
	if gormDB == nil {
		return info
	}
	var ver string
	if err := gormDB.Raw("SELECT sqlite_version()").Scan(&ver).Error; err == nil && ver != "" {
		info.Version = "SQLite " + ver
	}
	return info
}

// getPostgresInfo 返回 PostgreSQL 数据库信息
func getPostgresInfo(ctx context.Context) DatabaseInfoResponse {
	info := DatabaseInfoResponse{
		Type:    "postgres",
		Name:    config.Config.Database.Database,
		Version: "PostgreSQL",
	}
	gormDB := db.DB(ctx)
	if gormDB == nil {
		return info
	}
	var ver string
	if err := gormDB.Raw("SELECT version()").Scan(&ver).Error; err == nil && ver != "" {
		info.Version = ver
	}
	return info
}

// GetDatabaseInfo 获取当前数据库类型及版本信息
// @Summary 获取数据库信息
// @Description 返回当前使用的数据库类型（sqlite/postgres）、名称/路径及版本字符串，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=status.DatabaseInfoResponse} "获取成功"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/db-info [get]
func GetDatabaseInfo(c *gin.Context) {
	var info DatabaseInfoResponse
	if !config.Config.Database.Enabled {
		info = getSQLiteInfo(c.Request.Context())
	} else {
		info = getPostgresInfo(c.Request.Context())
	}
	c.JSON(http.StatusOK, response.OK(info))
}

// ExportDatabase 导出数据库
// @Summary 导出数据库
// @Description SQLite 时直接下载 .db 文件；PostgreSQL 时执行 pg_dump 并流式下载 .sql 文件，需要管理员权限
// @Tags admin
// @Produce application/octet-stream
// @Security SessionCookie
// @Success 200 {file} binary "数据库文件"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "导出失败"
// @Router /api/v1/admin/db-export [get]
func ExportDatabase(c *gin.Context) {
	if !config.Config.Database.Enabled {
		exportSQLite(c)
	} else {
		exportPostgres(c)
	}
}

// exportSQLite 以 HTTP 附件方式下载 SQLite .db 文件
func exportSQLite(c *gin.Context) {
	path := config.Config.Database.SQLitePath
	if path == "" {
		path = "./data/openflare.db"
	}

	f, err := os.Open(path) //nolint:gosec // path is loaded from server startup configuration, not user input
	if err != nil {
		response.AbortInternal(c, "无法打开数据库文件: "+err.Error())
		return
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			_ = closeErr
		}
	}()

	fi, err := f.Stat()
	if err != nil {
		response.AbortInternal(c, "无法读取数据库文件信息: "+err.Error())
		return
	}

	c.Header("Content-Disposition", `attachment; filename="openflare.db"`)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", fi.Size()))
	c.Status(http.StatusOK)
	http.ServeContent(c.Writer, c.Request, "openflare.db", fi.ModTime(), f)
}

// exportPostgres 执行 pg_dump 并将输出流式传输给客户端
func exportPostgres(c *gin.Context) {
	dbCfg := config.Config.Database

	// 检查 pg_dump 是否可用
	pgDumpPath, err := exec.LookPath("pg_dump")
	if err != nil {
		response.AbortInternal(c, "pg_dump 不可用，请确保服务器已安装 PostgreSQL 客户端工具")
		return
	}

	args := []string{
		"--no-password",
		"-h", dbCfg.Host,
		"-p", fmt.Sprintf("%d", dbCfg.Port),
		"-U", dbCfg.Username,
		dbCfg.Database,
	}

	cmd := exec.CommandContext(c.Request.Context(), pgDumpPath, args...) //nolint:gosec // pgDumpPath is a looked up command path, args are from database configuration
	if dbCfg.Password != "" {
		cmd.Env = append(os.Environ(), "PGPASSWORD="+dbCfg.Password)
	} else {
		cmd.Env = os.Environ()
	}

	fileName := fmt.Sprintf("openflare_%s.sql", time.Now().Format("20060102_150405"))
	c.Header("Content-Disposition", `attachment; filename="`+fileName+`"`)
	c.Header("Content-Type", "application/octet-stream")
	c.Status(http.StatusOK)

	cmd.Stdout = c.Writer
	cmd.Stderr = nil // 忽略 stderr 以避免污染输出流

	if err := cmd.Run(); err != nil {
		// 响应头已发出，无法再写 JSON 错误，记录到服务器日志
		log.Printf("[db-export] pg_dump failed: %v\n", err)
	}
}
