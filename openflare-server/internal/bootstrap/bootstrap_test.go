// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"testing"

	admin_push "github.com/Rain-kl/Wavelet/internal/apps/admin/push"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
)

func TestInitSyncsPushEventsOnce(t *testing.T) {
	ResetInitRuntimeOnceForTest()
	t.Cleanup(ResetInitRuntimeOnceForTest)

	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	if err := dbConn.AutoMigrate(&model.PushEvent{}); err != nil {
		t.Fatalf("auto migrate push events failed: %v", err)
	}

	RegisterPushDomainEvents()

	wantCount := len(admin_push.BuiltInEvents)
	if wantCount < 1 {
		t.Fatalf("built-in push events = %d, want at least 1", wantCount)
	}

	ctx := context.Background()
	Init(ctx, Options{API: true})
	Init(ctx, Options{}) // second Init must not duplicate events (initRuntimeOnce)

	var count int64
	if err := dbConn.Model(&model.PushEvent{}).Count(&count).Error; err != nil {
		t.Fatalf("count push events failed: %v", err)
	}
	if count != int64(wantCount) {
		t.Fatalf("push event count = %d, want %d", count, wantCount)
	}

	var adminLogin model.PushEvent
	if err := dbConn.Where("event_key = ?", "admin_login").First(&adminLogin).Error; err != nil {
		t.Fatalf("admin_login event not found after Init: %v", err)
	}
	if adminLogin.Name != "管理员登录" {
		t.Fatalf("admin_login name = %q, want %q", adminLogin.Name, "管理员登录")
	}
}
