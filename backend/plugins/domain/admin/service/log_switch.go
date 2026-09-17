// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/admin/errs"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/repository"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	// LogDBSwitchTask 切换日志数据库任务标识。
	LogDBSwitchTask = "logs:db_switch"
	// TaskTypeLogDBSwitch 管理端任务类型。
	TaskTypeLogDBSwitch = "logs_db_switch"

	targetPostgres   = "postgres"
	targetSQLite     = "sqlite"
	targetClickHouse = "clickhouse"

	errParseTaskPayloadFailed = "参数解析失败: %w"
	errInvalidLogTarget       = "目标日志库不合法: %s"
)

// LogDBSwitchMeta 描述切换日志数据库任务。
var LogDBSwitchMeta = contracts.TaskMetaDTO{
	Type:         TaskTypeLogDBSwitch,
	AsynqTask:    LogDBSwitchTask,
	Name:         "切换日志数据库",
	DisplayName:  "切换日志数据库",
	Description:  "复制迁移用户访问日志并在成功后切换日志主库（期间禁止日志写入）",
	Category:     "system",
	SupportsTime: false,
	MaxRetry:     3,
	Queue:        "default",
	Retryable:    true,
	Params: []contracts.TaskParamDTO{
		{
			Name:        "target",
			Label:       "目标日志库",
			Type:        "string",
			Required:    true,
			Placeholder: "postgres|sqlite|clickhouse",
			Description: "迁移目标：postgres（主库为 PG 时）、sqlite（主库为 SQLite 时）或 clickhouse",
		},
	},
}

type logDBSwitchPayload struct {
	Target string `json:"target"`
}

// LogDBSwitchHandler 切换日志数据库任务处理器。
type LogDBSwitchHandler struct{}

// ValidatePayload 校验并规范化参数。
func (h *LogDBSwitchHandler) ValidatePayload(payload []byte) ([]byte, error) {
	var p logDBSwitchPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf(errParseTaskPayloadFailed, err)
	}
	p.Target = normalizeTarget(p.Target)
	if !validTarget(p.Target) {
		return nil, fmt.Errorf(errInvalidLogTarget, p.Target)
	}
	out, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func normalizeTarget(v string) string {
	switch v {
	case targetPostgres, "postgresql":
		return targetPostgres
	case targetSQLite, "sqlite3":
		return targetSQLite
	case targetClickHouse, "ch":
		return targetClickHouse
	}
	return v
}

func validTarget(v string) bool {
	return v == targetPostgres || v == targetSQLite || v == targetClickHouse
}

// Execute 执行迁移。
func (h *LogDBSwitchHandler) Execute(ctx context.Context, payload []byte) (*contracts.TaskResultDTO, error) {
	var p logDBSwitchPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf(errParseTaskPayloadFailed, err)
	}
	p.Target = normalizeTarget(p.Target)
	if err := validateSwitch(ctx, p.Target); err != nil {
		return nil, err
	}

	source, err := currentLogDatabase(ctx)
	if err != nil {
		return nil, err
	}

	taskSvc := GetTaskService(ctx)
	if taskSvc != nil {
		taskSvc.AppendLog(ctx, "开始切换日志数据库：%s -> %s", source, p.Target)
	}

	if err := setMigrationFlag(ctx, logMigrationInProgress); err != nil {
		return nil, err
	}
	defer func() {
		if err := setMigrationFlag(ctx, ""); err != nil {
			logger.ErrorF(ctx, "清除日志迁移冻结标记失败: %v", err)
		}
	}()

	rc := GetRiskControlService(ctx)
	if rc != nil {
		if err := rc.SwitchLogEngine(ctx, p.Target); err != nil {
			return nil, err
		}
	}

	if err := flipLogDatabase(ctx, p.Target); err != nil {
		return nil, err
	}

	if taskSvc != nil {
		taskSvc.AppendLog(ctx, "日志数据库已切换为 %s，写入恢复", p.Target)
	}
	return &contracts.TaskResultDTO{Message: fmt.Sprintf("日志数据库已从 %s 切换为 %s", source, p.Target)}, nil
}

func validateSwitch(ctx context.Context, target string) error {
	source, err := currentLogDatabase(ctx)
	if err != nil {
		return err
	}
	if source == target {
		return errors.New(errs.ErrSameLogTarget)
	}
	switch target {
	case targetClickHouse:
		if !GetClickHouseConfig().Enabled {
			return errors.New(errs.ErrClickHouseNotEnabled)
		}
	case targetPostgres:
		if !GetDBConfig().Enabled {
			return errors.New(errs.ErrPostgresNotEnabled)
		}
	case targetSQLite:
		if GetDBConfig().Enabled {
			return errors.New(errs.ErrSQLiteNotAllowedAsLogDB)
		}
	}
	return nil
}

func currentLogDatabase(ctx context.Context) (string, error) {
	cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyLogDatabase)
	if err != nil {
		return "", fmt.Errorf(errs.ErrReadLogDatabaseFailed, err)
	}
	if cfg.Value == "" {
		return "", errors.New(errs.ErrLogDatabaseEmpty)
	}
	return cfg.Value, nil
}

func setMigrationFlag(ctx context.Context, v string) error {
	return repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyLogDBMigration, v)
}

func flipLogDatabase(ctx context.Context, target string) error {
	return repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyLogDatabase, target)
}
