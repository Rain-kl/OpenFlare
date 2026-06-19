// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
)

func TestProcessLoginEmailVerificationSMTPFallback(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	const email = "smtpuser@example.com"
	now := time.Now()
	user := model.User{
		ID:          222,
		Username:    "smtpuser",
		Nickname:    "SMTP User",
		Email:       email,
		IsActive:    true,
		LastLoginAt: now,
	}
	if err := user.SetEncryptedPassword("newpassword123"); err != nil {
		t.Fatalf("set encrypted password failed: %v", err)
	}
	if err := dbConn.Create(&user).Error; err != nil {
		t.Fatalf("create test user failed: %v", err)
	}

	if err := dbConn.Model(&model.SystemConfig{}).
		Where("key = ?", model.ConfigKeySMTPHost).
		Update("value", "").Error; err != nil {
		t.Fatalf("clear SMTP host failed: %v", err)
	}
	if err := db.Redis.Del(context.Background(), db.PrefixedKey(repository.SystemConfigRedisHashKey)).Err(); err != nil {
		t.Fatalf("invalidate system config cache failed: %v", err)
	}

	ctx := context.Background()
	result, err := processLoginEmailVerification(ctx, "", &user)
	if err != nil {
		t.Fatalf("processLoginEmailVerification() error = %v, want nil", err)
	}
	expected := errSMTPInvalidUseTempCodePrefix + errSMTPInvalidUseTempCode
	if result.Status != LoginEmailVerificationRejected || result.Message != expected {
		t.Fatalf("processLoginEmailVerification() = %+v, want rejected with %q", result, expected)
	}

	codeKey := getEmailCodeKey("login", email)
	var storedCode string
	if err := db.GetJSON(ctx, codeKey, &storedCode); err != nil {
		t.Fatalf("get stored verification code failed: %v", err)
	}
	if storedCode != "888888" {
		t.Errorf("stored verification code = %q, want %q", storedCode, "888888")
	}

	passed, err := processLoginEmailVerification(ctx, "888888", &user)
	if err != nil {
		t.Fatalf("processLoginEmailVerification(valid code) error = %v, want nil", err)
	}
	if passed.Status != LoginEmailVerificationPassed {
		t.Fatalf("processLoginEmailVerification(valid code) status = %v, want passed", passed.Status)
	}
}

func TestProcessLoginEmailVerificationEmptyEmailFallback(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	now := time.Now()
	user := model.User{
		ID:          223,
		Username:    "emptyemailuser",
		Nickname:    "Empty Email User",
		Email:       "",
		IsActive:    true,
		LastLoginAt: now,
	}
	if err := user.SetEncryptedPassword("newpassword123"); err != nil {
		t.Fatalf("set encrypted password failed: %v", err)
	}
	if err := dbConn.Create(&user).Error; err != nil {
		t.Fatalf("create test user failed: %v", err)
	}

	for _, cfg := range []struct {
		key   string
		value string
	}{
		{model.ConfigKeySMTPHost, "smtp.example.com"},
		{model.ConfigKeySMTPPort, "587"},
		{model.ConfigKeySMTPUsername, "smtpuser"},
		{model.ConfigKeySMTPPassword, "smtppassword"},
	} {
		if err := dbConn.Model(&model.SystemConfig{}).Where("key = ?", cfg.key).Update("value", cfg.value).Error; err != nil {
			t.Fatalf("set %s failed: %v", cfg.key, err)
		}
	}
	if err := db.Redis.Del(context.Background(), db.PrefixedKey(repository.SystemConfigRedisHashKey)).Err(); err != nil {
		t.Fatalf("invalidate system config cache failed: %v", err)
	}

	ctx := context.Background()
	result, err := processLoginEmailVerification(ctx, "", &user)
	if err != nil {
		t.Fatalf("processLoginEmailVerification() error = %v, want nil", err)
	}
	expected := errSMTPInvalidUseTempCodePrefix + "该账号未绑定邮箱，使用临时码登录"
	if result.Status != LoginEmailVerificationRejected || result.Message != expected {
		t.Fatalf("processLoginEmailVerification() = %+v, want rejected with %q", result, expected)
	}
}

func TestProcessLoginEmailVerificationInvalidCode(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	const email = "codeduser@example.com"
	now := time.Now()
	user := model.User{
		ID:          224,
		Username:    "codeduser",
		Email:       email,
		IsActive:    true,
		LastLoginAt: now,
	}
	if err := dbConn.Create(&user).Error; err != nil {
		t.Fatalf("create test user failed: %v", err)
	}

	ctx := context.Background()
	if err := db.SetJSON(ctx, getEmailCodeKey("login", email), "123456", emailCodeExpiry); err != nil {
		t.Fatalf("seed verification code failed: %v", err)
	}

	result, err := processLoginEmailVerification(ctx, "000000", &user)
	if err != nil {
		t.Fatalf("processLoginEmailVerification() error = %v, want nil", err)
	}
	if result.Status != LoginEmailVerificationRejected || result.Message != errEmailCodeInvalidOrExpired {
		t.Fatalf("processLoginEmailVerification() = %+v, want rejected invalid code", result)
	}
}
