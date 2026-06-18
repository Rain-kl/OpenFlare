// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package db_manage provides router handlers for managing database tables,
// overview information, and executing custom SQL queries.
package db_manage

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

const (
	binaryKB        = 0
	binaryMB        = 1
	binaryGB        = 2
	valueThreshold  = 10
	maxStringLength = 200
)

// DBOverviewResponse 数据库运行概览响应结构体
type DBOverviewResponse struct {
	Type        string `json:"type"`
	Version     string `json:"version"`
	Name        string `json:"name"`
	Size        string `json:"size"`
	TableCount  int64  `json:"table_count"`
	Connections int64  `json:"connections"`
}

// GetTableDataRequest 分页拉取表数据请求结构体
type GetTableDataRequest struct {
	Table    string `form:"table" binding:"required"`
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"pageSize,default=10"`
}

// TableDataResponse 动态数据表响应结构体
type TableDataResponse struct {
	Columns []string                 `json:"columns"`
	Total   int64                    `json:"total"`
	Results []map[string]interface{} `json:"results"`
}

// ExecuteSQLRequest 执行自定义 SQL 请求结构体
type ExecuteSQLRequest struct {
	SQL string `json:"sql" binding:"required"`
}

// ExecuteSQLResponse 执行自定义 SQL 响应结构体
type ExecuteSQLResponse struct {
	Type            string                   `json:"type"` // "select" 或 "exec"
	Columns         []string                 `json:"columns,omitempty"`
	Results         []map[string]interface{} `json:"results,omitempty"`
	AffectedRows    int64                    `json:"affected_rows"`
	ExecutionTimeMs int64                    `json:"execution_time_ms"`
}

// formatBytes 格式化字节大小为可读字符串
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

	if value == math.Trunc(value) {
		if value >= valueThreshold {
			return fmt.Sprintf("%.0f %s", value, suffix)
		}
		return fmt.Sprintf("%.1f %s", value, suffix)
	}
	return fmt.Sprintf("%.1f %s", value, suffix)
}

// getSQLiteOverview 获取 SQLite 数据库概览信息
func getSQLiteOverview(gormDB *gorm.DB) (DBOverviewResponse, error) {
	name := config.Config.Database.SQLitePath
	if name == "" {
		name = "./data/openflare.db"
	}

	var version string
	var ver string
	if err := gormDB.Raw("SELECT sqlite_version()").Scan(&ver).Error; err == nil {
		version = "SQLite " + ver
	} else {
		version = "SQLite"
	}

	var sizeStr string
	if fi, err := os.Stat(name); err == nil {
		size := fi.Size()
		if size < 0 {
			size = 0
		}
		sizeStr = formatBytes(uint64(size))
	} else {
		sizeStr = "0 B"
	}

	var tableCount int64
	if err := gormDB.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tableCount).Error; err != nil {
		tableCount = 0
	}

	var connCount int64
	if sqlDB, err := gormDB.DB(); err == nil {
		connCount = int64(sqlDB.Stats().OpenConnections)
	} else {
		connCount = 1
	}

	return DBOverviewResponse{
		Type:        "sqlite",
		Version:     version,
		Name:        name,
		Size:        sizeStr,
		TableCount:  tableCount,
		Connections: connCount,
	}, nil
}

// getPostgresOverview 获取 PostgreSQL 数据库概览信息
func getPostgresOverview(gormDB *gorm.DB) (DBOverviewResponse, error) {
	name := config.Config.Database.Database

	var version string
	var ver string
	if err := gormDB.Raw("SELECT version()").Scan(&ver).Error; err == nil {
		version = ver
	} else {
		version = "PostgreSQL"
	}

	var sizeStr string
	var sizeBytes sql.NullInt64
	if err := gormDB.Raw("SELECT pg_database_size(current_database())").Scan(&sizeBytes).Error; err == nil && sizeBytes.Valid {
		size := sizeBytes.Int64
		if size < 0 {
			size = 0
		}
		sizeStr = formatBytes(uint64(size))
	} else {
		sizeStr = "0 B"
	}

	var tableCount int64
	if err := gormDB.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema()").Scan(&tableCount).Error; err != nil {
		tableCount = 0
	}

	var connCount int64
	var pgc sql.NullInt64
	if err := gormDB.Raw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database()").Scan(&pgc).Error; err == nil && pgc.Valid {
		connCount = pgc.Int64
	} else {
		if sqlDB, err := gormDB.DB(); err == nil {
			connCount = int64(sqlDB.Stats().OpenConnections)
		} else {
			connCount = 1
		}
	}

	return DBOverviewResponse{
		Type:        "postgres",
		Version:     version,
		Name:        name,
		Size:        sizeStr,
		TableCount:  tableCount,
		Connections: connCount,
	}, nil
}

// GetDBOverview 获取数据库运行概览
// @Summary 获取数据库运行概览
// @Description 获取数据库类型、版本、名称、文件大小、表数量及当前连接数，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=db_manage.DBOverviewResponse} "获取成功"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/db-manage/overview [get]
func GetDBOverview(c *gin.Context) {
	gormDB := db.DB(c.Request.Context())
	if gormDB == nil {
		response.AbortInternal(c, "数据库未初始化")
		return
	}

	var overview DBOverviewResponse
	var err error

	if !config.Config.Database.Enabled {
		overview, err = getSQLiteOverview(gormDB)
	} else {
		overview, err = getPostgresOverview(gormDB)
	}

	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(overview))
}

// ListDBTables 获取数据库所有表名
// @Summary 获取数据库所有表名
// @Description 返回当前数据库的所有用户自定义表名称列表，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]string} "获取成功"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/db-manage/tables [get]
func ListDBTables(c *gin.Context) {
	gormDB := db.DB(c.Request.Context())
	if gormDB == nil {
		response.AbortInternal(c, "数据库未初始化")
		return
	}

	var tables []string
	var err error

	if !config.Config.Database.Enabled {
		err = gormDB.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name").Scan(&tables).Error
	} else {
		err = gormDB.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() ORDER BY table_name").Scan(&tables).Error
	}

	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(tables))
}

// GetDBTableData 获取数据表 data
func GetDBTableData(c *gin.Context) {
	var req GetTableDataRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	gormDB := db.DB(c.Request.Context())
	if gormDB == nil {
		response.AbortInternal(c, "数据库未初始化")
		return
	}

	// 安全转义表名并拼接
	quotedTable := `"` + strings.ReplaceAll(req.Table, `"`, `""`) + `"`

	var total int64
	if err := gormDB.Raw("SELECT count(*) FROM " + quotedTable).Scan(&total).Error; err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	offset := (req.Page - 1) * req.PageSize
	if offset < 0 {
		offset = 0
	}
	limit := req.PageSize
	if limit <= 0 {
		limit = 10
	}

	rows, err := gormDB.Raw("SELECT * FROM "+quotedTable+" LIMIT ? OFFSET ?", limit, offset).Rows()
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	defer func() {
		_ = rows.Close()
	}()

	cols, err := rows.Columns()
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	results, err := scanTableRows(rows, cols)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(TableDataResponse{
		Columns: cols,
		Total:   total,
		Results: results,
	}))
}

// scanTableRows 扫描并提取数据表行数据，做截断处理
func scanTableRows(rows *sql.Rows, cols []string) ([]map[string]interface{}, error) {
	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, colName := range cols {
			val := columns[i]
			if b, ok := val.([]byte); ok {
				strVal := string(b)
				runes := []rune(strVal)
				if len(runes) > maxStringLength {
					strVal = string(runes[:maxStringLength]) + "..."
				}
				rowMap[colName] = strVal
			} else if str, ok := val.(string); ok {
				runes := []rune(str)
				if len(runes) > maxStringLength {
					str = string(runes[:maxStringLength]) + "..."
				}
				rowMap[colName] = str
			} else {
				rowMap[colName] = val
			}
		}
		results = append(results, rowMap)
	}
	return results, nil
}

// executeSQLQuery 执行并解析查询类 SQL 语句
func executeSQLQuery(gormDB *gorm.DB, sqlStr string, startTime time.Time) (ExecuteSQLResponse, error) {
	rows, err := gormDB.Raw(sqlStr).Rows()
	if err != nil {
		return ExecuteSQLResponse{}, err
	}
	defer func() {
		_ = rows.Close()
	}()

	cols, err := rows.Columns()
	if err != nil {
		return ExecuteSQLResponse{}, err
	}

	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return ExecuteSQLResponse{}, err
		}

		rowMap := make(map[string]interface{})
		for i, colName := range cols {
			val := columns[i]
			if b, ok := val.([]byte); ok {
				rowMap[colName] = string(b)
			} else {
				rowMap[colName] = val
			}
		}
		results = append(results, rowMap)
	}

	executionTime := time.Since(startTime).Milliseconds()
	return ExecuteSQLResponse{
		Type:            "select",
		Columns:         cols,
		Results:         results,
		AffectedRows:    int64(len(results)),
		ExecutionTimeMs: executionTime,
	}, nil
}

// executeSQLMutation 执行修改/更新类 SQL 语句
func executeSQLMutation(gormDB *gorm.DB, sqlStr string, startTime time.Time) (ExecuteSQLResponse, error) {
	tx := gormDB.Exec(sqlStr)
	if tx.Error != nil {
		return ExecuteSQLResponse{}, tx.Error
	}

	executionTime := time.Since(startTime).Milliseconds()
	return ExecuteSQLResponse{
		Type:            "exec",
		AffectedRows:    tx.RowsAffected,
		ExecutionTimeMs: executionTime,
	}, nil
}

// ExecuteSQL 执行 SQL 查询
// @Summary 执行 SQL 查询
// @Description 在当前数据库中执行任意自定义 SQL，如果是查询语句将返回格式化后的列与数据集，否则返回受影响行数，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body db_manage.ExecuteSQLRequest true "SQL 请求参数"
// @Success 200 {object} response.Any{data=db_manage.ExecuteSQLResponse} "执行完毕"
// @Failure 400 {object} response.Any "SQL 语句错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/db-manage/query [post]
func ExecuteSQL(c *gin.Context) {
	var req ExecuteSQLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	gormDB := db.DB(c.Request.Context())
	if gormDB == nil {
		response.AbortInternal(c, "数据库未初始化")
		return
	}

	trimmedSQL := strings.TrimSpace(req.SQL)
	if trimmedSQL == "" {
		response.AbortBadRequest(c, "SQL 语句不能为空")
		return
	}

	startTime := time.Now()

	// 识别是否是查询语句（SELECT, SHOW, EXPLAIN 等）
	isQuery := false
	lowerSQL := strings.ToLower(trimmedSQL)
	queryKeywords := []string{"select", "show", "explain", "describe", "pragma"}
	for _, kw := range queryKeywords {
		if strings.HasPrefix(lowerSQL, kw) {
			isQuery = true
			break
		}
	}

	var resp ExecuteSQLResponse
	var err error

	if isQuery {
		resp, err = executeSQLQuery(gormDB, trimmedSQL, startTime)
	} else {
		resp, err = executeSQLMutation(gormDB, trimmedSQL, startTime)
	}

	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(resp))
}
