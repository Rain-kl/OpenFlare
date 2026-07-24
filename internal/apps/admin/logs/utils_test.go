// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"net/http"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

func setupTestDB(t *testing.T) *gorm.DB {
	dbConn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite in memory: %v", err)
	}
	err = dbConn.AutoMigrate(&model.SystemConfig{})
	if err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}
	db.SetDB(dbConn)
	return dbConn
}

func TestWebSocketCheckOrigin(t *testing.T) {
	dbConn := setupTestDB(t)

	// Clean up global DB after test
	defer db.SetDB(nil)

	// Seed ConfigKeyServerAddress with allowed frontend origin
	allowedOrigin := "http://localhost:3000"
	if err := dbConn.Create(&model.SystemConfig{
		Key:   model.ConfigKeyServerAddress,
		Value: allowedOrigin,
	}).Error; err != nil {
		t.Fatalf("failed to seed server address config: %v", err)
	}

	upgrader := getUpgrader()
	if upgrader.CheckOrigin == nil {
		t.Fatal("expected CheckOrigin to be defined")
	}

	tests := []struct {
		name   string
		origin string
		host   string
		wantOK bool
	}{
		{
			name:   "empty origin (non-browser clients)",
			origin: "",
			host:   "localhost:8000",
			wantOK: true,
		},
		{
			name:   "same-origin request",
			origin: "http://localhost:8000",
			host:   "localhost:8000",
			wantOK: true,
		},
		{
			name:   "same-origin request case insensitive",
			origin: "HTTP://LOCALHOST:8000",
			host:   "localhost:8000",
			wantOK: true,
		},
		{
			name:   "configured allowed origin request",
			origin: "http://localhost:3000",
			host:   "localhost:8000",
			wantOK: true,
		},
		{
			name:   "configured allowed origin request with trailing slash",
			origin: "http://localhost:3000/",
			host:   "localhost:8000",
			wantOK: true,
		},
		{
			name:   "unauthorized third-party origin",
			origin: "http://evil.com",
			host:   "localhost:8000",
			wantOK: false,
		},
		{
			name:   "invalid origin format",
			origin: "::not-a-valid-url",
			host:   "localhost:8000",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/admin/logs/ws", nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			got := upgrader.CheckOrigin(req)
			if got != tt.wantOK {
				t.Errorf("CheckOrigin() = %v, want %v", got, tt.wantOK)
			}
		})
	}
}
