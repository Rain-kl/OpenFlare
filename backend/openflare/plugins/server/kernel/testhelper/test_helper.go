// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package testhelper 提供测试辅助工具
package testhelper

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"testing"
	"time"

	"Wavelet/core/contracts"
	"Wavelet/openflare/plugins/server/kernel/model"
	analyticsmodel "Wavelet/openflare/plugins/server/kernel/model/analytics"
	"Wavelet/openflare/plugins/server/kernel/ofupload"
	"Wavelet/openflare/plugins/server/kernel/repository"
	"Wavelet/openflare/plugins/server/kernel/repository/logstore"
	oftask "Wavelet/openflare/plugins/server/kernel/task"
	"Wavelet/pkg/idgen"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	configTypeSystem   = "system"
	configTypeBusiness = "business"
	configValueTrue    = "true"
	configValueFalse   = "false"
)

type testConfigService struct {
	db *gorm.DB
}

// NewMockSystemConfigService creates a test SystemConfigService backed by GORM.
func NewMockSystemConfigService(db *gorm.DB) contracts.SystemConfigService {
	return &testConfigService{db: db}
}

func (s *testConfigService) GetByKey(ctx context.Context, key string) (contracts.SystemConfigDTO, error) {
	var cfg contracts.SystemConfigDTO
	err := s.db.WithContext(ctx).Table("w_system_configs").Where("key = ?", key).First(&cfg).Error
	return cfg, err
}

func (s *testConfigService) ListByKeys(ctx context.Context, keys []string) (map[string]contracts.SystemConfigDTO, error) {
	var cfgs []contracts.SystemConfigDTO
	if err := s.db.WithContext(ctx).Table("w_system_configs").Where("key IN ?", keys).Find(&cfgs).Error; err != nil {
		return nil, err
	}
	res := make(map[string]contracts.SystemConfigDTO, len(cfgs))
	for _, c := range cfgs {
		res[c.Key] = c
	}
	return res, nil
}

func (s *testConfigService) ListVisible(ctx context.Context) ([]contracts.SystemConfigDTO, error) {
	var cfgs []contracts.SystemConfigDTO
	err := s.db.WithContext(ctx).Table("w_system_configs").Where("visibility = ?", 1).Find(&cfgs).Error
	return cfgs, err
}

func (s *testConfigService) ListByType(ctx context.Context, configType string) ([]contracts.SystemConfigDTO, error) {
	var cfgs []contracts.SystemConfigDTO
	err := s.db.WithContext(ctx).Table("w_system_configs").Where("type = ?", configType).Find(&cfgs).Error
	return cfgs, err
}

func (s *testConfigService) GetIntByKey(ctx context.Context, key string) (int, error) {
	var cfg contracts.SystemConfigDTO
	if err := s.db.WithContext(ctx).Table("w_system_configs").Where("key = ?", key).First(&cfg).Error; err != nil {
		return 0, err
	}
	return strconv.Atoi(cfg.Value)
}

func (s *testConfigService) GetBoolByKey(ctx context.Context, key string) (bool, error) {
	var cfg contracts.SystemConfigDTO
	if err := s.db.WithContext(ctx).Table("w_system_configs").Where("key = ?", key).First(&cfg).Error; err != nil {
		return false, err
	}
	return strconv.ParseBool(cfg.Value)
}

func (s *testConfigService) SaveOrUpdate(ctx context.Context, key, value string) error {
	var cfg model.SystemConfig
	if err := s.db.WithContext(ctx).Table("w_system_configs").Where("key = ?", key).First(&cfg).Error; err != nil {
		cfg = model.SystemConfig{Key: key, Value: value, Type: "system"}
		return s.db.WithContext(ctx).Table("w_system_configs").Create(&cfg).Error
	}
	return s.db.WithContext(ctx).Table("w_system_configs").Where("key = ?", key).Update("value", value).Error
}

func (s *testConfigService) InvalidateCache(ctx context.Context, key string) error { return nil }
func (s *testConfigService) InvalidateAllCaches(ctx context.Context) error         { return nil }

type testSystemConfigEntity struct {
	Key         string `gorm:"primaryKey"`
	Value       string
	Type        string
	Visibility  int
	Description string
	UpdatedAt   time.Time
	CreatedAt   time.Time
}

func (testSystemConfigEntity) TableName() string {
	return "w_system_configs"
}

// SetupTestEnvironment initializes an in-memory SQLite DB and seeds default
// configurations. Redis is no longer owned by OpenFlare.
func SetupTestEnvironment(t *testing.T) (*gorm.DB, any, func()) {
	t.Helper()
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite db: %v", err)
	}

	if sqlDB, err := sqliteDB.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}

	err = sqliteDB.AutoMigrate(
		&testSystemConfigEntity{},
		&model.User{},
		&model.AccessToken{},
		&model.Upload{},
		&model.UploadStat{},
		&model.TaskExecution{},
	)
	if err != nil {
		t.Fatalf("failed to auto migrate tables: %v", err)
	}

	repository.SetDBForTest(sqliteDB)
	repository.SetSystemConfigService(&testConfigService{db: sqliteDB})

	mockStorage := NewMockStorageService()
	ofupload.SetStorage(mockStorage)
	ofupload.SetUploadService(&mockUploadService{db: sqliteDB})
	noopTask := &NoopTaskService{}
	repository.SetTaskService(noopTask)
	oftask.SetService(noopTask)

	if err := idgen.Init(1); err != nil {
		t.Fatalf("idgen.Init: %v", err)
	}
	seedDefaultConfigs(t, sqliteDB)

	cleanup := func() {
		runExtraCleanups()
		repository.StopSystemConfigCacheListener()
		repository.SetAuthService(nil)
		repository.SetUserService(nil)
		repository.SetSystemConfigService(nil)
		repository.SetTaskService(nil)
		repository.SetDBForTest(nil)
		ofupload.SetStorage(nil)
		ofupload.SetUploadService(nil)
		oftask.SetService(nil)
	}

	return sqliteDB, nil, cleanup
}

type mockUploadService struct {
	db *gorm.DB
}

// NewMockUploadService creates a mock UploadService backed by GORM.
func NewMockUploadService(db *gorm.DB) contracts.UploadService {
	return &mockUploadService{db: db}
}

func (s *mockUploadService) GetByID(ctx context.Context, id uint64) (*contracts.UploadDTO, error) {
	var u contracts.UploadDTO
	err := s.db.WithContext(ctx).Table("w_uploads").Where("id = ?", id).First(&u).Error
	if err != nil {
		return &contracts.UploadDTO{
			ID:        id,
			Status:    "used",
			Type:      "openflare_pages_deployment",
			Size:      100,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}, nil
	}
	return &u, nil
}

func (s *mockUploadService) OpenStoredUpload(ctx context.Context, id uint64) (*contracts.OpenedUploadDTO, error) {
	u, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	body := io.ReadCloser(io.NopCloser(bytes.NewReader(nil)))
	storage := ofupload.CurrentStorage()
	if storage != nil {
		if obj, err := storage.Get(ctx, u.FilePath); err == nil && obj != nil && obj.Body != nil {
			body = obj.Body
		} else if obj, err := storage.Get(ctx, u.FileName); err == nil && obj != nil && obj.Body != nil {
			body = obj.Body
		}
	}
	return &contracts.OpenedUploadDTO{
		Upload:        *u,
		Body:          body,
		ContentType:   u.MimeType,
		ContentLength: u.Size,
	}, nil
}

func (s *mockUploadService) Remove(ctx context.Context, id uint64) error {
	if err := s.db.WithContext(ctx).Table("w_uploads").Where("id = ?", id).Update("status", "deleted").Error; err != nil {
		return err
	}
	return s.RebuildStats(ctx)
}

func (s *mockUploadService) RemoveOwned(ctx context.Context, id uint64, userID uint64) error {
	if err := s.db.WithContext(ctx).Table("w_uploads").Where("id = ? AND user_id = ?", id, userID).Update("status", "deleted").Error; err != nil {
		return err
	}
	return s.RebuildStats(ctx)
}

func (s *mockUploadService) FindByHash(ctx context.Context, hash string, size int64) (*contracts.UploadDTO, error) {
	var u contracts.UploadDTO
	err := s.db.WithContext(ctx).Table("w_uploads").Where("hash = ? AND size = ?", hash, size).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *mockUploadService) RebuildStats(ctx context.Context) error {
	var count int64
	_ = s.db.WithContext(ctx).Table("w_uploads").Where("status != ?", "deleted").Count(&count).Error
	var stat model.UploadStat
	if err := s.db.WithContext(ctx).Table("w_upload_stats").Where("dimension = ?", model.UploadStatDimensionTotal).First(&stat).Error; err != nil {
		stat = model.UploadStat{
			Dimension: model.UploadStatDimensionTotal,
			FileCount: int(count),
		}
		return s.db.WithContext(ctx).Table("w_upload_stats").Create(&stat).Error
	}
	stat.FileCount = int(count)
	return s.db.WithContext(ctx).Table("w_upload_stats").Save(&stat).Error
}

func getSeedConfigsPart1() []model.SystemConfig {
	return []model.SystemConfig{
		{Key: model.ConfigKeyUploadAllowedExtensions, Value: "jpg,png,webp", Type: configTypeSystem, Description: "允许上传的图片扩展名（逗号分隔）"},
		{Key: model.ConfigKeySiteName, Value: "OpenFlare", Type: configTypeSystem, Description: "系统平台的展示名称"},
		{Key: model.ConfigKeyPasswordLoginEnabled, Value: configValueTrue, Type: configTypeSystem},
		{Key: model.ConfigKeyRegistrationEnabled, Value: configValueFalse, Type: configTypeSystem},
		{Key: model.ConfigKeyPasswordRegisterEnabled, Value: configValueFalse, Type: configTypeSystem},
		{Key: model.ConfigKeyOIDCLoginEnabled, Value: configValueTrue, Type: configTypeSystem},
		{Key: model.ConfigKeyMaxAPIKeysPerUser, Value: "5", Type: "business"},
		{Key: model.ConfigKeyCapLoginEnabled, Value: configValueFalse, Type: configTypeSystem},
		{Key: model.ConfigKeyCapAutoSolve, Value: configValueTrue, Type: configTypeSystem},
		{Key: model.ConfigKeyCapChallengeCount, Value: "1", Type: configTypeSystem},
		{Key: model.ConfigKeyCapChallengeSize, Value: "32", Type: configTypeSystem},
		{Key: model.ConfigKeyCapChallengeDifficulty, Value: "4", Type: configTypeSystem},
		{Key: model.ConfigKeyCapChallengeTTL, Value: "600", Type: configTypeSystem},
		{Key: model.ConfigKeyCapTokenTTL, Value: "1200", Type: configTypeSystem},
	}
}

func getSeedConfigsPart2() []model.SystemConfig {
	return []model.SystemConfig{
		{Key: model.ConfigKeyServerAddress, Value: "", Type: configTypeSystem},
		{Key: model.ConfigKeySMTPHost, Value: "", Type: configTypeSystem},
		{Key: model.ConfigKeySMTPPort, Value: "587", Type: configTypeSystem},
		{Key: model.ConfigKeySMTPUsername, Value: "", Type: configTypeSystem},
		{Key: model.ConfigKeySMTPPassword, Value: "", Type: configTypeSystem},
		{Key: model.ConfigKeyEmailLoginVerificationEnabled, Value: configValueFalse, Type: configTypeSystem},
		{Key: model.ConfigKeyEmailRegisterVerificationEnabled, Value: configValueFalse, Type: configTypeSystem},
		{Key: model.ConfigKeyMenuDisplayConfig, Value: "{}", Type: configTypeSystem},
		{Key: model.ConfigKeySearchEngineIndexingEnabled, Value: configValueFalse, Type: configTypeSystem},
		{Key: model.ConfigKeyFileAccessWhitelist, Value: `["avatar"]`, Type: configTypeSystem},
		{Key: model.ConfigKeyDiskCacheMaxSizeMB, Value: "100", Type: configTypeSystem},
		{Key: model.ConfigKeyDiskCacheTTLMinutes, Value: "60", Type: configTypeSystem},
		{Key: model.ConfigKeyDiskCacheLRUEnabled, Value: configValueTrue, Type: configTypeSystem},
		{Key: model.ConfigKeyLoginSessionTTLHours, Value: "0", Type: configTypeSystem},
		{Key: model.ConfigKeyUpdateUpstreamRepository, Value: "Rain-kl/OpenFlare", Type: configTypeSystem},
		{Key: model.ConfigKeyStorageConfig, Value: `{"driver":"local","local":{"root":"."},"s3":{"region":"us-east-1"},"r2":{"region":"auto"},"minio":{"region":"us-east-1","path_style":true},"oss":{},"webdav":{}}`, Type: configTypeSystem},
		{Key: model.ConfigKeyRelayFRPSWebUIEnabled, Value: configValueFalse, Type: configTypeBusiness},
		{Key: model.ConfigKeyRelayFRPSWebUIPort, Value: "17500", Type: configTypeBusiness},
		{Key: model.ConfigKeyPagesMaxPackageSizeMB, Value: "100", Type: configTypeBusiness},
		{Key: model.ConfigKeyPagesMaxHistoryCount, Value: "20", Type: configTypeBusiness},
		{Key: model.ConfigKeyLogRetentionDaysPostgres, Value: "90", Type: configTypeBusiness},
		{Key: model.ConfigKeyLogRetentionDaysSQLite, Value: "90", Type: configTypeBusiness},
		{Key: model.ConfigKeyLogRetentionDaysClickHouse, Value: "90", Type: configTypeBusiness},
	}
}

func seedDefaultConfigs(t *testing.T, tx *gorm.DB) {
	t.Helper()
	defaultConfigs := append(getSeedConfigsPart1(), getSeedConfigsPart2()...)
	if err := tx.Table("w_system_configs").Create(&defaultConfigs).Error; err != nil {
		t.Fatalf("failed to seed default system configs: %v", err)
	}

	publicKeys := []string{
		model.ConfigKeyUploadAllowedExtensions,
		model.ConfigKeySiteName,
		model.ConfigKeyPasswordLoginEnabled,
		model.ConfigKeyRegistrationEnabled,
		model.ConfigKeyPasswordRegisterEnabled,
		model.ConfigKeyOIDCLoginEnabled,
		model.ConfigKeyMaxAPIKeysPerUser,
		model.ConfigKeyCapLoginEnabled,
		model.ConfigKeyCapAutoSolve,
		model.ConfigKeyEmailLoginVerificationEnabled,
		model.ConfigKeyEmailRegisterVerificationEnabled,
		model.ConfigKeyMenuDisplayConfig,
		model.ConfigKeySearchEngineIndexingEnabled,
		model.ConfigKeyFileAccessWhitelist,
	}
	if err := tx.Table("w_system_configs").
		Where("key IN ?", publicKeys).
		Update("visibility", model.ConfigVisibilityVisible).Error; err != nil {
		t.Fatalf("failed to seed public system config visibility: %v", err)
	}
}

// SetupLogStoresForTest 将 logstore 指向测试已通过 SetDBForTest 注入的 sqlite 库。
func SetupLogStoresForTest(t *testing.T) {
	t.Helper()

	gdb := repository.DB(context.Background())
	require.NoError(t, idgen.Init(1))
	require.NoError(t, gdb.AutoMigrate(
		&analyticsmodel.NodeAccessLog{},
		&analyticsmodel.UserAccessLog{},
		&analyticsmodel.NodeMetricSnapshot{},
		&analyticsmodel.NodeEdgeHealth{},
		&analyticsmodel.NodeObsFrps{},
		&analyticsmodel.NodeObsFrpc{},
	))

	logstore.ResetForTest()
	logstore.SetConfigReader(func(_ context.Context, key string) (string, error) {
		if key == model.ConfigKeyLogDatabase {
			return "sqlite", nil
		}
		return "", nil
	})
	store, err := logstore.Active(context.Background())
	require.NoError(t, err)

	logstore.SetAccessLogHooks(logstore.AccessLogHooks{
		QueueNodeAccessLogs: func(logs []analyticsmodel.NodeAccessLog) {
			if err := store.AccessLogs.BatchInsertNodeAccessLogs(context.Background(), logs); err != nil {
				t.Errorf("batch insert node access logs failed in test hook: %v", err)
			}
		},
	})
	logstore.SetObservabilityHooks(logstore.ObservabilityHooks{
		QueueMetricSnapshot: func(record analyticsmodel.NodeMetricSnapshot) {
			if err := store.Observability.BatchInsertNodeMetricSnapshots(context.Background(), []analyticsmodel.NodeMetricSnapshot{record}); err != nil {
				t.Errorf("batch insert node metric snapshots failed in test hook: %v", err)
			}
		},
		QueueEdgeHealth: func(record analyticsmodel.NodeEdgeHealth) {
			if err := store.Observability.BatchInsertNodeEdgeHealth(context.Background(), []analyticsmodel.NodeEdgeHealth{record}); err != nil {
				t.Errorf("batch insert node edge health failed in test hook: %v", err)
			}
		},
		QueueNodeObsFrps: func(record analyticsmodel.NodeObsFrps) {
			if err := store.Observability.BatchInsertNodeObsFrps(context.Background(), []analyticsmodel.NodeObsFrps{record}); err != nil {
				t.Errorf("batch insert node obs frps failed in test hook: %v", err)
			}
		},
		QueueNodeObsFrpc: func(record analyticsmodel.NodeObsFrpc) {
			if err := store.Observability.BatchInsertNodeObsFrpc(context.Background(), []analyticsmodel.NodeObsFrpc{record}); err != nil {
				t.Errorf("batch insert node obs frpc failed in test hook: %v", err)
			}
		},
	})

	t.Cleanup(func() {
		logstore.SetAccessLogHooks(logstore.AccessLogHooks{})
		logstore.SetObservabilityHooks(logstore.ObservabilityHooks{})
		logstore.ResetForTest()
	})
}
