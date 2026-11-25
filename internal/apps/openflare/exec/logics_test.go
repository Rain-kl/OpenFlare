// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/infra/config"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupExecTestDB(t *testing.T) context.Context {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "exec.db")
	conn, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := conn.AutoMigrate(
		&model.OpenFlareNode{},
		&model.OpenFlareExecTask{},
		&model.OpenFlareExecTaskResult{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	db.SetDB(conn)
	t.Cleanup(func() {
		db.SetDB(nil)
	})
	return context.Background()
}

func TestExecRunRecordAndList(t *testing.T) {
	ctx := setupExecTestDB(t)

	offlineNode := &model.OpenFlareNode{
		NodeID:      "node-offline",
		Name:        "Offline Node",
		AccessToken: "tok-offline",
		NodeType:    "edge_node",
	}
	if err := db.DB(ctx).Create(offlineNode).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	input := RunInput{
		Command: "uname -a",
		Clients: []string{"node-offline"},
	}
	previous := config.Config.EnableWebShell
	t.Cleanup(func() { config.Config.EnableWebShell = previous })
	config.Config.EnableWebShell = false
	if resp, err := RunCommand(ctx, 42, input); resp != nil || !errors.Is(err, ErrWebShellDisabled) {
		t.Fatalf("expected disabled execution, got response=%+v err=%v", resp, err)
	}

	config.Config.EnableWebShell = true
	resp, err := RunCommand(ctx, 42, input)
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if len(resp.OfflineClients) != 1 || len(resp.Results) != 1 || resp.Results[0].ExitCode == nil || *resp.Results[0].ExitCode != -1 {
		t.Fatalf("expected offline node marked with exit_code -1, got %+v", resp)
	}

	// A restart with WebShell disabled still allows outstanding results and history.
	config.Config.EnableWebShell = false
	if err := RecordResult(ctx, "node-offline", protocol.RemoteExecResultPayload{
		TaskID:     resp.TaskID,
		Result:     "Linux test 6.8.0\n",
		ExitCode:   0,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("RecordResult: %v", err)
	}

	task, err := GetTask(ctx, resp.TaskID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if len(task.Results) != 1 || task.Results[0].NodeName != "Offline Node" || task.Results[0].ExitCode == nil || *task.Results[0].ExitCode != 0 {
		t.Fatalf("unexpected task view: %+v", task)
	}

	list, err := ListTasks(ctx, 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListTasks: len=%d err=%v", len(list), err)
	}

	if err := DeleteTask(ctx, resp.TaskID); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
}
