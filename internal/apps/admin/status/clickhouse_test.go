// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
	"github.com/gin-gonic/gin"
)

// restoreConfig 恢复测试中临时修改的全局配置。
func restoreConfig(t *testing.T) {
	t.Helper()
	dbEnabled := config.Config.Database.Enabled
	chEnabled := config.Config.ClickHouse.Enabled
	t.Cleanup(func() {
		config.Config.Database.Enabled = dbEnabled
		config.Config.ClickHouse.Enabled = chEnabled
	})
}

func TestAvailableTargets(t *testing.T) {
	restoreConfig(t)
	logstore.ResetForTest()
	t.Cleanup(logstore.ResetForTest)

	// 当前 clickhouse → 主库（postgres/sqlite 按启动配置）。
	logstore.SetConfigReader(func(_ context.Context, key string) (string, error) {
		if key == "log_database" {
			return logDBNameClickHouse, nil
		}
		return "", nil
	})
	config.Config.Database.Enabled = true
	config.Config.ClickHouse.Enabled = true
	if got := availableTargets(logDBNameClickHouse); !reflect.DeepEqual(got, []string{logDBNamePostgres}) {
		t.Fatalf("clickhouse active + postgres main: got %v, want [postgres]", got)
	}

	config.Config.Database.Enabled = false
	if got := availableTargets(logDBNameClickHouse); !reflect.DeepEqual(got, []string{logDBNameSQLite}) {
		t.Fatalf("clickhouse active + sqlite main: got %v, want [sqlite]", got)
	}

	// 当前主库 → clickhouse（CH 启用时）。
	logstore.SetConfigReader(func(_ context.Context, key string) (string, error) {
		if key == "log_database" {
			return logDBNamePostgres, nil
		}
		return "", nil
	})
	config.Config.ClickHouse.Enabled = true
	if got := availableTargets(logDBNamePostgres); !reflect.DeepEqual(got, []string{logDBNameClickHouse}) {
		t.Fatalf("postgres active: got %v, want [clickhouse]", got)
	}

	// CH 禁用时排除 clickhouse。
	config.Config.ClickHouse.Enabled = false
	if got := availableTargets(logDBNamePostgres); len(got) != 0 {
		t.Fatalf("CH disabled: got %v, want empty", got)
	}
}

// TestGetLogDatabaseStatusSmoke 覆盖 handler 的 CH 激活分支（无需 DB/CH 连接）。
func TestGetLogDatabaseStatusSmoke(t *testing.T) {
	restoreConfig(t)
	config.Config.Database.Enabled = false
	config.Config.ClickHouse.Enabled = true
	logstore.ResetForTest()
	t.Cleanup(logstore.ResetForTest)

	logstore.SetConfigReader(func(_ context.Context, key string) (string, error) {
		if key == "log_database" {
			return logDBNameClickHouse, nil
		}
		return "", nil
	})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/status/log-database", nil)

	GetLogDatabaseStatus(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data LogDatabaseStatus `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if resp.Data.ActiveDatabase != logDBNameClickHouse {
		t.Fatalf("active_database = %q, want clickhouse", resp.Data.ActiveDatabase)
	}
	if resp.Data.Migration != "idle" {
		t.Fatalf("migration = %q, want idle", resp.Data.Migration)
	}
	for _, key := range []string{logDBNamePostgres, logDBNameSQLite, logDBNameClickHouse} {
		if got := resp.Data.RetentionDays[key]; got != defaultLogRetentionDays {
			t.Fatalf("retention_days[%s] = %d, want default %d", key, got, defaultLogRetentionDays)
		}
	}
	if got := resp.Data.AvailableTargets; !reflect.DeepEqual(got, []string{logDBNameSQLite}) {
		t.Fatalf("available_targets = %v, want [sqlite] (test main DB disabled)", got)
	}
}
