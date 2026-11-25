// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const (
	nodePageMetricWindow = 5 * time.Minute
	nodePagePingTasks    = 3
	nodePagePingSamples  = 12
)

// MonitoringView contains the compact system and probe summaries for a node row.
type MonitoringView struct {
	System string            `json:"system,omitempty"`
	Metric *MonitoringMetric `json:"metric,omitempty"`
	Ping   []MonitoringPing  `json:"ping,omitempty"`
}

// MonitoringMetric is the latest host resource and network rate measurement.
type MonitoringMetric struct {
	CapturedAt        time.Time `json:"captured_at"`
	CPUUsagePercent   float64   `json:"cpu_usage_percent"`
	MemoryUsedBytes   int64     `json:"memory_used_bytes"`
	MemoryTotalBytes  int64     `json:"memory_total_bytes"`
	StorageUsedBytes  int64     `json:"storage_used_bytes"`
	StorageTotalBytes int64     `json:"storage_total_bytes"`
	NetIn             int64     `json:"net_in"`
	NetOut            int64     `json:"net_out"`
}

// MonitoringPing provides recent measurements in chronological order for one assigned task.
type MonitoringPing struct {
	TaskID  uint                   `json:"task_id"`
	Name    string                 `json:"name"`
	Samples []MonitoringPingSample `json:"samples"`
}

// MonitoringPingSample is a probe result in milliseconds, or a negative value on failure.
type MonitoringPingSample struct {
	RecordedAt time.Time `json:"recorded_at"`
	Value      int       `json:"value"`
}

func attachNodeMonitoring(ctx context.Context, views []*View, now time.Time) error {
	nodeIDs := make([]string, 0, len(views))
	onlineIDs := make([]string, 0, len(views))
	byNode := make(map[string]*MonitoringView, len(views))
	for _, view := range views {
		nodeIDs = append(nodeIDs, view.NodeID)
		if view.Status == nodeStatusOnline {
			onlineIDs = append(onlineIDs, view.NodeID)
		}
		view.Monitoring = &MonitoringView{}
		byNode[view.NodeID] = view.Monitoring
	}

	names, systemErr := repository.ListOpenFlareNodeOperatingSystems(ctx, nodeIDs)
	for nodeID, name := range names {
		byNode[nodeID].System = name
	}
	return errors.Join(systemErr,
		attachNodePageMetrics(ctx, byNode, onlineIDs, now),
		attachNodePagePing(ctx, byNode, now))
}

func attachNodePageMetrics(ctx context.Context, byNode map[string]*MonitoringView, nodeIDs []string, now time.Time) error {
	if len(nodeIDs) == 0 {
		return nil
	}
	rows, err := repository.ListOpenFlareLatestMetricSnapshotsByNodeIDs(ctx, nodeIDs, now.Add(-nodePageMetricWindow))
	if err != nil {
		return fmt.Errorf("resource snapshots: %w", err)
	}
	for _, row := range rows {
		byNode[row.NodeID].Metric = &MonitoringMetric{
			CapturedAt: row.CapturedAt, CPUUsagePercent: row.CPUUsagePercent,
			MemoryUsedBytes: row.MemoryUsedBytes, MemoryTotalBytes: row.MemoryTotalBytes,
			StorageUsedBytes: row.StorageUsedBytes, StorageTotalBytes: row.StorageTotalBytes,
			NetIn: row.NetIn, NetOut: row.NetOut,
		}
	}
	return nil
}

func attachNodePagePing(ctx context.Context, byNode map[string]*MonitoringView, now time.Time) error {
	tasks, err := repository.ListOpenFlarePingTasks(ctx)
	if err != nil {
		return fmt.Errorf("ping tasks: %w", err)
	}
	windows := selectNodePagePingTasks(tasks, byNode, now)
	if len(windows) == 0 {
		return nil
	}
	rows, err := repository.ListRecentOpenFlarePingRecordsByTask(ctx, windows, nodePagePingSamples)
	if err != nil {
		return fmt.Errorf("ping samples: %w", err)
	}
	slices.Reverse(rows)
	for _, row := range rows {
		for i := range byNode[row.NodeID].Ping {
			series := &byNode[row.NodeID].Ping[i]
			if series.TaskID == row.TaskID {
				series.Samples = append(series.Samples, MonitoringPingSample{RecordedAt: row.RecordedAt, Value: row.Value})
				break
			}
		}
	}
	return nil
}

func selectNodePagePingTasks(tasks []*model.OpenFlarePingTask, byNode map[string]*MonitoringView, now time.Time) map[uint]analyticsmodel.PingTaskWindow {
	windows := make(map[uint]analyticsmodel.PingTaskWindow)
	for _, task := range tasks {
		window := analyticsmodel.PingTaskWindow{
			Since: now.Add(-max(nodePageMetricWindow, time.Duration(task.Interval*nodePagePingSamples)*time.Second)),
		}
		for _, nodeID := range task.Clients {
			view, found := byNode[nodeID]
			if !found || len(view.Ping) >= nodePagePingTasks ||
				slices.ContainsFunc(view.Ping, func(item MonitoringPing) bool { return item.TaskID == task.ID }) {
				continue
			}
			view.Ping = append(view.Ping, MonitoringPing{
				TaskID: task.ID, Name: task.Name, Samples: []MonitoringPingSample{},
			})
			window.NodeIDs = append(window.NodeIDs, nodeID)
		}
		if len(window.NodeIDs) > 0 {
			windows[task.ID] = window
		}
	}
	return windows
}
