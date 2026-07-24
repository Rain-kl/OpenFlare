// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/apps/upload/filesrv"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/shared"
	uploadstats "github.com/Rain-kl/Wavelet/internal/apps/upload/stats"
	"github.com/Rain-kl/Wavelet/internal/infra/diskcache"
	"github.com/Rain-kl/Wavelet/internal/infra/objectstore"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemCleanupHandler_Execute(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	deleteCount := 0
	// Mock S3 存储并记录 Delete，cleanup 不应物理删除共享对象。
	storageMock := objectstore.MockStorage(
		func(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
			return nil
		},
		func(ctx context.Context, key string) (*objectstore.Object, error) { return nil, nil },
		func(ctx context.Context, key string) error {
			deleteCount++
			return nil
		},
	)
	defer storageMock()
	objectstore.IsEnabledFunc = func() bool { return true }
	defer func() { objectstore.IsEnabledFunc = func() bool { return false } }()
	objectstore.ResetCache()

	ctx := context.Background()
	err := db.DB(ctx).AutoMigrate(&model.PushHistory{})
	require.NoError(t, err)

	// 准备测试数据：创建一些上传记录
	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)

	records := []*model.Upload{
		// 超过1小时且状态为 pending 的记录 —— 应被清理
		{
			UserID: 1001, FileName: "old_file_1.jpg", FilePath: "uploads/old_1.jpg",
			FileSize: 1024, MimeType: "image/jpeg", Extension: "jpg", Hash: "hash1",
			Type: "attachment", Status: model.UploadStatusPending,
			CreatedAt: twoHoursAgo,
		},
		{
			UserID: 1001, FileName: "old_file_2.png", FilePath: "uploads/old_2.png",
			FileSize: 2048, MimeType: "image/png", Extension: "png", Hash: "hash2",
			Type: "attachment", Status: model.UploadStatusPending,
			CreatedAt: twoHoursAgo,
		},
		// 状态为 used 的记录 —— 不应被清理
		{
			UserID: 1001, FileName: "used_file.jpg", FilePath: "uploads/used.jpg",
			FileSize: 512, MimeType: "image/jpeg", Extension: "jpg", Hash: "hash3",
			Type: "attachment", Status: model.UploadStatusUsed,
			CreatedAt: twoHoursAgo,
		},
		// 不到1小时的 pending 记录 —— 不应被清理
		{
			UserID: 1001, FileName: "recent_file.jpg", FilePath: "uploads/recent.jpg",
			FileSize: 256, MimeType: "image/jpeg", Extension: "jpg", Hash: "hash4",
			Type: "attachment", Status: model.UploadStatusPending,
			CreatedAt: now.Add(-10 * time.Minute),
		},
	}
	for _, r := range records {
		err := db.DB(ctx).Create(r).Error
		require.NoError(t, err)
		require.NoError(t, uploadstats.ApplyUploadStatsAdd(ctx, r))
	}

	// 准备推送历史测试数据：1个旧的（应删除），1个新的（应保留）
	oldPush := &model.PushHistory{
		EventKey:  "admin_login",
		Channel:   "email",
		Target:    "admin@test.com",
		Title:     "Old Login",
		Content:   "Old Content",
		Level:     "INFO",
		Status:    "success",
		CreatedAt: now.AddDate(0, 0, -10),
	}
	newPush := &model.PushHistory{
		EventKey:  "admin_login",
		Channel:   "lark",
		Target:    "http://webhook.com",
		Title:     "New Login",
		Content:   "New Content",
		Level:     "INFO",
		Status:    "success",
		CreatedAt: now,
	}
	err = db.DB(ctx).Create(oldPush).Error
	require.NoError(t, err)
	err = db.DB(ctx).Create(newPush).Error
	require.NoError(t, err)

	oldTaskLog := &model.TaskExecution{
		TaskID:      "old_low_frequency_task_log",
		TaskType:    "low:frequency",
		TaskName:    "低频任务",
		Status:      model.TaskExecutionStatusSucceeded,
		CreatedAt:   now.AddDate(0, 0, -31),
		UpdatedAt:   now.AddDate(0, 0, -31),
		TriggeredBy: "system",
	}
	err = repository.CreateTaskExecution(ctx, oldTaskLog)
	require.NoError(t, err)

	// 执行 handler
	handler := &SystemCleanupHandler{}
	result, err := handler.Execute(ctx, nil)

	// 验证结果
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Message, "系统清理完成。成功清理未使用的上传文件 2/2 个；清理历史推送审计日志 1 条；清理任务执行日志 1 条。")

	// 验证数据库状态：pending 且超过1小时的应被标记为 deleted
	var pendingCount int64
	db.DB(ctx).Model(&model.Upload{}).Where("status = ?", model.UploadStatusPending).Count(&pendingCount)
	assert.Equal(t, int64(1), pendingCount, "应只剩1条 pending 记录（最近的文件）")

	var deletedCount int64
	db.DB(ctx).Model(&model.Upload{}).Where("status = ?", model.UploadStatusDeleted).Count(&deletedCount)
	assert.Equal(t, int64(2), deletedCount, "应有2条被标记为 deleted")

	var usedCount int64
	db.DB(ctx).Model(&model.Upload{}).Where("status = ?", model.UploadStatusUsed).Count(&usedCount)
	assert.Equal(t, int64(1), usedCount, "used 状态的文件不应受影响")
	assert.Equal(t, 0, deleteCount, "记录级 cleanup 不应调用 storage backend Delete")

	var totalStats model.UploadStat
	err = db.DB(ctx).
		Where("dimension = ? AND stat_key = ?", model.UploadStatDimensionTotal, "").
		First(&totalStats).Error
	require.NoError(t, err)
	assert.Equal(t, int64(2), totalStats.FileCount, "cleanup 后统计只应保留 used 与最近 pending 记录")
	assert.Equal(t, int64(768), totalStats.FileSize, "cleanup 后统计大小应只扣减一次")

	_, err = handler.Execute(ctx, nil)
	require.NoError(t, err)
	var statsAfterSecondRun model.UploadStat
	err = db.DB(ctx).
		Where("dimension = ? AND stat_key = ?", model.UploadStatDimensionTotal, "").
		First(&statsAfterSecondRun).Error
	require.NoError(t, err)
	assert.Equal(t, totalStats.FileCount, statsAfterSecondRun.FileCount, "重复 cleanup 不应再次扣减统计")
	assert.Equal(t, totalStats.FileSize, statsAfterSecondRun.FileSize, "重复 cleanup 不应再次扣减统计大小")
	assert.Equal(t, 0, deleteCount, "重复 cleanup 仍不应调用 storage backend Delete")

	// 验证推送历史数据状态：10天前的应被删除，今天的应保留
	var pushCount int64
	db.DB(ctx).Model(&model.PushHistory{}).Count(&pushCount)
	assert.Equal(t, int64(1), pushCount, "应只剩1条推送历史记录")

	var remainingPush model.PushHistory
	err = db.DB(ctx).First(&remainingPush).Error
	require.NoError(t, err)
	assert.Equal(t, "New Login", remainingPush.Title)

	var taskLogCount int64
	err = db.DB(ctx).Model(&model.TaskExecution{}).Where("task_id = ?", "old_low_frequency_task_log").Count(&taskLogCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(0), taskLogCount, "过期低频任务日志应被清理")
}

func TestSystemCleanupHandler_ExecuteNoFiles(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	// Mock S3 存储
	storageMock := objectstore.MockStorage(
		func(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
			return nil
		},
		func(ctx context.Context, key string) (*objectstore.Object, error) { return nil, nil },
		func(ctx context.Context, key string) error { return nil },
	)
	defer storageMock()

	ctx := context.Background()
	err := db.DB(ctx).AutoMigrate(&model.PushHistory{})
	require.NoError(t, err)

	// 没有任何上传记录
	handler := &SystemCleanupHandler{}
	result, err := handler.Execute(ctx, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Message, "系统清理完成。成功清理未使用的上传文件 0/0 个；清理历史推送审计日志 0 条；清理任务执行日志 0 条。")
}

func TestSystemCleanupHandler_ImplementsTaskHandler(t *testing.T) {
	// 编译期验证 SystemCleanupHandler 实现了 TaskHandler 接口
	var _ task.TaskHandler = (*SystemCleanupHandler)(nil)
}

func TestWarmImageCacheHandlerValidatePayload(t *testing.T) {
	tests := []struct {
		name        string
		payload     []byte
		wantQuality string
		wantErr     bool
	}{
		{
			name:        "normalizes quality",
			payload:     []byte(`{"quality":" HIGH "}`),
			wantQuality: shared.ImageQualityHigh,
		},
		{
			name:    "empty payload",
			wantErr: true,
		},
		{
			name:    "invalid json",
			payload: []byte(`{`),
			wantErr: true,
		},
		{
			name:    "origin is not a compressed quality",
			payload: []byte(`{"quality":"origin"}`),
			wantErr: true,
		},
		{
			name:    "unsupported quality",
			payload: []byte(`{"quality":"maximum"}`),
			wantErr: true,
		},
	}

	handler := &WarmImageCacheHandler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPayload, err := handler.ValidatePayload(tt.payload)
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Fatalf("ValidatePayload(%s) error = %v, want error presence = %t", tt.payload, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			var got WarmImageCachePayload
			if err := json.Unmarshal(gotPayload, &got); err != nil {
				t.Fatalf("json.Unmarshal(%s) returned error: %v", gotPayload, err)
			}
			if got.Quality != tt.wantQuality {
				t.Errorf("ValidatePayload(%s).Quality = %q, want %q", tt.payload, got.Quality, tt.wantQuality)
			}
		})
	}
}

func TestWarmImageCacheHandlerExecute(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	cache := diskcache.GetGlobalCache()
	if err := cache.Clear(); err != nil {
		t.Fatalf("Clear() before test returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := cache.Clear(); err != nil {
			t.Errorf("Clear() after test returned error: %v", err)
		}
	})

	testDir := t.TempDir()
	ctx := context.Background()
	active := objectstore.DefaultConfig()
	active.Local.Root = testDir
	if err := objectstore.SaveActiveConfig(ctx, active); err != nil {
		t.Fatalf("SaveActiveConfig() returned error: %v", err)
	}

	firstPath := filepath.Join(testDir, "first.png")
	secondPath := filepath.Join(testDir, "second.jpg")
	writeTaskTestPNG(t, firstPath, color.RGBA{R: 255, A: 255})
	writeTaskTestPNG(t, secondPath, color.RGBA{G: 255, A: 255})

	records := []model.Upload{
		{
			ID:        4101,
			UserID:    1001,
			FileName:  "first.png",
			FilePath:  firstPath,
			MimeType:  "image/png",
			Extension: "png",
			Status:    model.UploadStatusUsed,
		},
		{
			ID:        4102,
			UserID:    1001,
			FileName:  "second.jpg",
			FilePath:  secondPath,
			MimeType:  "application/octet-stream",
			Extension: "jpg",
			Status:    model.UploadStatusPending,
		},
		{
			ID:        4103,
			UserID:    1001,
			FileName:  "notes.txt",
			FilePath:  filepath.Join(testDir, "notes.txt"),
			MimeType:  "text/plain",
			Extension: "txt",
			Status:    model.UploadStatusUsed,
		},
		{
			ID:        4104,
			UserID:    1001,
			FileName:  "deleted.png",
			FilePath:  firstPath,
			MimeType:  "image/png",
			Extension: "png",
			Status:    model.UploadStatusDeleted,
		},
	}
	for i := range records {
		if info, err := os.Stat(records[i].FilePath); err == nil {
			records[i].FileSize = info.Size()
		}
		if err := dbConn.Create(&records[i]).Error; err != nil {
			t.Fatalf("failed to create upload %d: %v", records[i].ID, err)
		}
	}

	handler := &WarmImageCacheHandler{}
	payload := []byte(`{"quality":"low"}`)

	result, err := handler.Execute(context.Background(), payload)
	if err != nil {
		t.Fatalf("Execute(%s) returned error: %v", payload, err)
	}
	if result == nil {
		t.Fatal("Execute() result = nil, want non-nil")
	}
	if result.Message != "图片缓存预热完成，共处理 2 张，生成 2 张，命中 0 张，失败 0 张" {
		t.Errorf("Execute() message = %q, want generated summary", result.Message)
	}

	for i := range records[:2] {
		key := filesrv.ImageCompressionCacheKey(&records[i], shared.ImageQualityLow)
		got, err := cache.Get(key)
		if err != nil {
			t.Errorf("cache.Get(%q) returned error: %v", key, err)
			continue
		}
		if len(got) == 0 {
			t.Errorf("cache.Get(%q) returned empty WebP data", key)
		}
	}

	secondResult, err := handler.Execute(context.Background(), payload)
	if err != nil {
		t.Fatalf("second Execute(%s) returned error: %v", payload, err)
	}
	if secondResult.Message != "图片缓存预热完成，共处理 2 张，生成 0 张，命中 2 张，失败 0 张" {
		t.Errorf("second Execute() message = %q, want cache-hit summary", secondResult.Message)
	}
}

func TestWarmImageCacheHandlerImplementsTaskInterfaces(t *testing.T) {
	var _ task.TaskHandler = (*WarmImageCacheHandler)(nil)
	var _ task.PayloadValidator = (*WarmImageCacheHandler)(nil)
}

func writeTaskTestPNG(t *testing.T, path string, fill color.RGBA) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, fill)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() returned error: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) returned error: %v", path, err)
	}
}
