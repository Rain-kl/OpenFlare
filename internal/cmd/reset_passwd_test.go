// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
)

func TestResetPasswdCmd_WithUserAndPassword(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	// Seed test user
	user := model.User{
		ID:          1001,
		Username:    "testuser1",
		Nickname:    "Test User 1",
		Email:       "test1@example.com",
		IsActive:    true,
		LastLoginAt: time.Now(),
	}
	_ = user.SetEncryptedPassword("oldpassword")
	if err := dbConn.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create access token to test invalidation/deletion
	token := model.AccessToken{
		ID:          1,
		UserID:      user.ID,
		Name:        "testtoken",
		TokenHash:   "somehash",
		MaskedToken: "some...",
	}
	if err := dbConn.Create(&token).Error; err != nil {
		t.Fatalf("failed to create access token: %v", err)
	}

	// Override PreRun to bypass goose migrations in unit tests
	oldPreRun := resetPasswdCmd.PreRun
	resetPasswdCmd.PreRun = nil
	defer func() { resetPasswdCmd.PreRun = oldPreRun }()

	// Execute command with args
	rootCmd.SetArgs([]string{"reset-passwd", "--user", "testuser1", "--password", "newpassword123"})

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := rootCmd.Execute()
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("command execute failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("Password reset successfully")) {
		t.Errorf("expected output to contain success message, got: %s", output)
	}

	// Verify password in DB
	var dbUser model.User
	if err := dbConn.Where("id = ?", user.ID).First(&dbUser).Error; err != nil {
		t.Fatalf("failed to query user from DB: %v", err)
	}
	if !dbUser.CheckPassword("newpassword123") {
		t.Errorf("password was not updated correctly in DB")
	}

	// Verify token deleted
	var count int64
	dbConn.Model(&model.AccessToken{}).Where("user_id = ?", user.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected access tokens to be deleted, got %d", count)
	}
}

func TestResetPasswdCmd_WithUserAndRandomPassword(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	// Seed test user
	user := model.User{
		ID:          1002,
		Username:    "testuser2",
		Nickname:    "Test User 2",
		Email:       "test2@example.com",
		IsActive:    true,
		LastLoginAt: time.Now(),
	}
	_ = user.SetEncryptedPassword("oldpassword")
	if err := dbConn.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Override PreRun
	oldPreRun := resetPasswdCmd.PreRun
	resetPasswdCmd.PreRun = nil
	defer func() { resetPasswdCmd.PreRun = oldPreRun }()

	// Reset flags
	usernameFlag = ""
	passwordFlag = ""

	// Execute command with args (no --password)
	rootCmd.SetArgs([]string{"reset-passwd", "--user", "testuser2"})

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := rootCmd.Execute()
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("command execute failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("Password reset successfully")) {
		t.Errorf("expected output to contain success message, got: %s", output)
	}

	// Verify password in DB (should be updated and not equal to old one)
	var dbUser model.User
	if err := dbConn.Where("id = ?", user.ID).First(&dbUser).Error; err != nil {
		t.Fatalf("failed to query user from DB: %v", err)
	}
	if dbUser.CheckPassword("oldpassword") {
		t.Errorf("expected password to change, but it matches the old one")
	}
}

func TestResetPasswdCmd_InteractiveMode(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	// Seed test user
	user := model.User{
		ID:          1003,
		Username:    "testuser3",
		Nickname:    "Test User 3",
		Email:       "test3@example.com",
		IsActive:    true,
		LastLoginAt: time.Now(),
	}
	_ = user.SetEncryptedPassword("oldpassword")
	if err := dbConn.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Override PreRun
	oldPreRun := resetPasswdCmd.PreRun
	resetPasswdCmd.PreRun = nil
	defer func() { resetPasswdCmd.PreRun = oldPreRun }()

	// Mock stdin
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer inR.Close()
	defer inW.Close()

	oldStdin := os.Stdin
	os.Stdin = inR
	defer func() { os.Stdin = oldStdin }()

	// Write username to stdin
	_, _ = inW.WriteString("testuser3\n")
	inW.Close()

	// Reset flags
	usernameFlag = ""
	passwordFlag = ""
	rootCmd.SetArgs([]string{"reset-passwd"})

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = rootCmd.Execute()
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("command execute failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("Password reset successfully")) {
		t.Errorf("expected output to contain success message, got: %s", output)
	}

	// Verify user password changed in DB
	var dbUser model.User
	if err := dbConn.Where("id = ?", user.ID).First(&dbUser).Error; err != nil {
		t.Fatalf("failed to query user from DB: %v", err)
	}
	if dbUser.CheckPassword("oldpassword") {
		t.Errorf("expected password to change, but it matches the old one")
	}
}
