// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/model"
	"go.uber.org/zap"
)

const (
	accessLogPathMaxLength        = 100
	accessLogUserAgentMaxLength   = 512
	accessLogCacheStatusMaxLength = 32
)

// PersistHeartbeatObservability stores profile, host metrics, edge health, and access logs.
func PersistHeartbeatObservability(ctx context.Context, nodeID string, payload NodePayload, reportedAt time.Time) {
	if strings.TrimSpace(nodeID) == "" {
		return
	}
	if payload.Profile == nil &&
		payload.HostMetrics == nil &&
		payload.EdgeHealth == nil &&
		len(payload.AccessLogs) == 0 &&
		len(payload.Buffered) == 0 &&
		payload.HealthEvents == nil {
		return
	}

	accessLogRecords, err := buildNodeAccessLogRecords(nodeID, payload.AccessLogs, payload.Buffered, reportedAt)
	if err != nil {
		zap.L().Error("build heartbeat access logs failed", zap.String("node_id", nodeID), zap.Error(err))
		return
	}

	profile := buildNodeSystemProfileModel(nodeID, payload.Profile, reportedAt)
	healthEvents := healthEventInputs(payload.HealthEvents)
	if err := repository.PersistOpenFlareNodePGObservability(
		ctx,
		profile,
		nodeID,
		healthEvents,
		payload.HealthEvents != nil,
		reportedAt,
		nil,
	); err != nil {
		zap.L().Error("persist heartbeat observability failed", zap.String("node_id", nodeID), zap.Error(err))
		return
	}

	if err := persistBufferedObservability(ctx, nodeID, payload.Buffered, reportedAt); err != nil {
		zap.L().Error("persist buffered observability failed", zap.String("node_id", nodeID), zap.Error(err))
	}
	if err := persistNodeMetricSnapshot(ctx, nodeID, payload.HostMetrics, reportedAt); err != nil {
		zap.L().Error("persist metric snapshot failed", zap.String("node_id", nodeID), zap.Error(err))
	}
	if err := persistNodeEdgeHealth(ctx, nodeID, payload.EdgeHealth, payload.OpenrestyStatus, reportedAt); err != nil {
		zap.L().Error("persist edge health failed", zap.String("node_id", nodeID), zap.Error(err))
	}

	if err := persistNodeAccessLogs(ctx, nodeID, accessLogRecords, reportedAt); err != nil {
		zap.L().Error("persist heartbeat access logs failed", zap.String("node_id", nodeID), zap.Error(err))
	}
}

func persistBufferedObservability(ctx context.Context, nodeID string, records []BufferedObservabilityRecord, reportedAt time.Time) error {
	for _, record := range records {
		if err := persistNodeMetricSnapshot(ctx, nodeID, record.HostMetrics, reportedAt); err != nil {
			return err
		}
		if err := persistNodeEdgeHealth(ctx, nodeID, record.EdgeHealth, "", reportedAt); err != nil {
			return err
		}
	}
	return nil
}

func persistNodeEdgeHealth(ctx context.Context, nodeID string, health *NodeEdgeHealth, fallbackStatus string, reportedAt time.Time) error {
	if health == nil {
		return nil
	}
	status := strings.TrimSpace(health.Status)
	if status == "" {
		status = strings.TrimSpace(fallbackStatus)
	}
	if status == "" {
		status = openrestyStatusUnknown
	}
	return repository.InsertOpenFlareEdgeHealth(ctx, &model.OpenFlareEdgeHealth{
		NodeID:      nodeID,
		CapturedAt:  timeFromUnix(health.CapturedAtUnix, reportedAt),
		Status:      status,
		Connections: health.Connections,
	})
}

func buildNodeSystemProfileModel(nodeID string, profile *NodeSystemProfile, reportedAt time.Time) *model.OpenFlareNodeSystemProfile {
	if profile == nil {
		return nil
	}
	return &model.OpenFlareNodeSystemProfile{
		NodeID:           nodeID,
		Hostname:         strings.TrimSpace(profile.Hostname),
		OSName:           strings.TrimSpace(profile.OSName),
		OSVersion:        strings.TrimSpace(profile.OSVersion),
		KernelVersion:    strings.TrimSpace(profile.KernelVersion),
		Architecture:     strings.TrimSpace(profile.Architecture),
		CPUModel:         strings.TrimSpace(profile.CPUModel),
		CPUCores:         profile.CPUCores,
		TotalMemoryBytes: profile.TotalMemoryBytes,
		TotalDiskBytes:   profile.TotalDiskBytes,
		UptimeSeconds:    profile.UptimeSeconds,
		ReportedAt:       timeFromUnix(profile.ReportedAtUnix, reportedAt),
	}
}

func healthEventInputs(events []NodeHealthEvent) []repository.OpenFlareHealthEventInput {
	if events == nil {
		return nil
	}
	out := make([]repository.OpenFlareHealthEventInput, 0, len(events))
	for _, event := range events {
		out = append(out, repository.OpenFlareHealthEventInput{
			EventType:       event.EventType,
			Severity:        event.Severity,
			Message:         event.Message,
			TriggeredAtUnix: event.TriggeredAtUnix,
			Metadata:        event.Metadata,
		})
	}
	return out
}

func persistNodeMetricSnapshot(ctx context.Context, nodeID string, snapshot *NodeMetricSnapshot, reportedAt time.Time) error {
	if snapshot == nil {
		return nil
	}
	record := &model.OpenFlareMetricSnapshot{
		NodeID:            nodeID,
		CapturedAt:        timeFromUnix(snapshot.CapturedAtUnix, reportedAt),
		CPUUsagePercent:   snapshot.CPUUsagePercent,
		MemoryUsedBytes:   snapshot.MemoryUsedBytes,
		MemoryTotalBytes:  snapshot.MemoryTotalBytes,
		StorageUsedBytes:  snapshot.StorageUsedBytes,
		StorageTotalBytes: snapshot.StorageTotalBytes,
		DiskReadBytes:     snapshot.DiskReadBytes,
		DiskWriteBytes:    snapshot.DiskWriteBytes,
		// NetworkRx/Tx no longer collected from agents; CH columns remain 0.
	}
	return repository.InsertOpenFlareMetricSnapshot(ctx, record)
}

func buildNodeAccessLogRecords(nodeID string, direct []NodeAccessLog, buffered []BufferedObservabilityRecord, reportedAt time.Time) ([]*model.OpenFlareAccessLog, error) {
	total := len(direct)
	for _, record := range buffered {
		total += len(record.AccessLogs)
	}
	if total == 0 {
		return nil, nil
	}

	resolver, err := newAccessLogRegionResolver()
	if err != nil {
		slog.Warn("initialize access log geo resolver failed", "node_id", nodeID, "error", err)
	}
	if resolver != nil {
		defer resolver.Close()
	}

	records := make([]*model.OpenFlareAccessLog, 0, total)
	appendLogs := func(logs []NodeAccessLog) {
		for _, item := range logs {
			bytesSent := item.BytesSent
			if bytesSent < 0 {
				bytesSent = 0
			}
			requestLength := item.RequestLength
			if requestLength < 0 {
				requestLength = 0
			}
			requestTimeMs := item.RequestTimeMs
			if requestTimeMs < 0 {
				requestTimeMs = 0
			}
			record := &model.OpenFlareAccessLog{
				NodeID:        nodeID,
				LoggedAt:      timeFromUnix(item.LoggedAtUnix, reportedAt),
				RemoteAddr:    strings.TrimSpace(item.RemoteAddr),
				Region:        "",
				Host:          strings.TrimSpace(item.Host),
				Path:          truncateForDatabase(strings.TrimSpace(item.Path), accessLogPathMaxLength),
				UserAgent:     truncateForDatabase(strings.TrimSpace(item.UserAgent), accessLogUserAgentMaxLength),
				CacheStatus:   truncateForDatabase(strings.TrimSpace(item.CacheStatus), accessLogCacheStatusMaxLength),
				StatusCode:    item.StatusCode,
				BytesSent:     bytesSent,
				RequestLength: requestLength,
				RequestTimeMs: requestTimeMs,
			}
			if resolver != nil {
				record.Region = resolver.Resolve(record.RemoteAddr)
			}
			records = append(records, record)
		}
	}
	appendLogs(direct)
	for _, record := range buffered {
		appendLogs(record.AccessLogs)
	}
	return records, nil
}

func persistNodeAccessLogs(ctx context.Context, _ string, records []*model.OpenFlareAccessLog, _ time.Time) error {
	if len(records) == 0 {
		return nil
	}
	return repository.InsertOpenFlareAccessLogsBatch(ctx, records)
}

// ReconcileScopedNodeHealthEvents reconciles health events, optionally scoped to managed event types.
func ReconcileScopedNodeHealthEvents(ctx context.Context, nodeID string, events []NodeHealthEvent, reportedAt time.Time, managedEventTypes map[string]struct{}) error {
	return repository.ReconcileOpenFlareHealthEvents(ctx, nodeID, healthEventInputs(events), reportedAt, managedEventTypes)
}

func timeFromUnix(unixSeconds int64, fallback time.Time) time.Time {
	if unixSeconds <= 0 {
		return fallback
	}
	return time.Unix(unixSeconds, 0).UTC()
}

// MarshalJSON serializes a value for database JSON columns.
func MarshalJSON(value any) string {
	return marshalJSON(value)
}

func marshalJSON(value any) string {
	if value == nil {
		return ""
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}
