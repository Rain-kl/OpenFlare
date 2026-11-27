// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netintel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/Rain-kl/Wavelet/internal/infra/task"
)

const (
	// NetIntelSyncTask is the Asynq task name for NetIntel dataset synchronization.
	NetIntelSyncTask = "openflare:netintel_sync"
	// TaskTypeNetIntelSync is the admin task type for NetIntel dataset synchronization.
	TaskTypeNetIntelSync = "of_netintel_sync"
)

// SyncPayload configures an on-demand or scheduled NetIntel dataset sync run.
type SyncPayload struct {
	Force bool `json:"force"`
}

// NetIntelSyncMeta describes the NetIntel dataset sync task in the admin task catalog.
var NetIntelSyncMeta = task.TaskMeta{
	Type:         TaskTypeNetIntelSync,
	AsynqTask:    NetIntelSyncTask,
	Name:         "OpenFlare network intelligence database sync",
	Description:  "Synchronize GeoIP/ASN MMDB, bgp.tools routing tables, and optional CAIDA AS relationships, then reload in-memory datasets",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
	Params: []task.TaskParam{
		{
			Name:        "force",
			Label:       "Force redownload",
			Type:        "boolean",
			Required:    false,
			Description: "Bypass freshness window and force redownloading upstream NetIntel files",
		},
	},
}

// SyncHandler executes Asynq jobs for NetIntel database synchronization.
type SyncHandler struct{}

// ValidatePayload normalizes optional JSON parameters for NetIntel sync jobs.
func (h *SyncHandler) ValidatePayload(payload []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return json.Marshal(SyncPayload{})
	}
	var req SyncPayload
	if err := json.Unmarshal(trimmed, &req); err != nil {
		return nil, fmt.Errorf("invalid netintel sync payload: %w", err)
	}
	return json.Marshal(req)
}

// Execute runs dataset synchronization, hot reload, and missing node ASN backfill.
func (h *SyncHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
	normalized, err := h.ValidatePayload(payload)
	if err != nil {
		return nil, task.PermanentError(err.Error())
	}
	var req SyncPayload
	_ = json.Unmarshal(normalized, &req)

	task.AppendLog(ctx, "Starting NetIntel dataset sync (force=%t)", req.Force)
	summary, err := SyncDatasets(ctx, req.Force)
	if err != nil {
		task.AppendLog(ctx, "NetIntel dataset sync failed: %v", err)
		return nil, err
	}

	detailBytes, _ := json.Marshal(summary)
	msg := fmt.Sprintf(
		"NetIntel sync finished: updated=%d skipped=%d errors=%d backfilled_nodes=%d (%dms)",
		len(summary.UpdatedFiles),
		len(summary.SkippedFiles),
		len(summary.Errors),
		summary.BackfilledNodes,
		summary.DurationMs,
	)
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{
		Message: msg,
		Detail:  string(detailBytes),
	}, nil
}
