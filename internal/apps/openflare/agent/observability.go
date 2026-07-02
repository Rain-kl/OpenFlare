// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	healthEventStatusActive      = "active"
	healthEventStatusResolved    = "resolved"
	healthSeverityInfo           = "info"
	healthSeverityWarning        = "warning"
	healthSeverityCritical       = "critical"
	accessLogPathMaxLength       = 100
	healthEventMessageMaxLength  = 4096
)

// PersistHeartbeatObservability stores profile, snapshots, traffic, access logs, and health events.
func PersistHeartbeatObservability(ctx context.Context, nodeID string, payload NodePayload, reportedAt time.Time) {
	if strings.TrimSpace(nodeID) == "" {
		return
	}
	if payload.Profile == nil &&
		payload.Snapshot == nil &&
		payload.TrafficReport == nil &&
		len(payload.AccessLogs) == 0 &&
		len(payload.BufferedObservability) == 0 &&
		payload.HealthEvents == nil {
		return
	}

	conn := db.DB(ctx)
	if conn == nil {
		return
	}

	accessLogRecords, err := buildNodeAccessLogRecords(nodeID, payload.AccessLogs, payload.BufferedObservability, reportedAt)
	if err != nil {
		zap.L().Error("build heartbeat access logs failed", zap.String("node_id", nodeID), zap.Error(err))
		return
	}

	if err := conn.Transaction(func(tx *gorm.DB) error {
		if err := persistNodeSystemProfile(tx, nodeID, payload.Profile, reportedAt); err != nil {
			return err
		}
		if payload.HealthEvents != nil {
			if err := reconcileNodeHealthEvents(tx, nodeID, payload.HealthEvents, reportedAt); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		zap.L().Error("persist heartbeat observability failed", zap.String("node_id", nodeID), zap.Error(err))
		return
	}

	if err := persistBufferedObservability(ctx, nodeID, payload.BufferedObservability, reportedAt); err != nil {
		zap.L().Error("persist buffered observability failed", zap.String("node_id", nodeID), zap.Error(err))
	}
	if err := persistNodeMetricSnapshot(ctx, nodeID, payload.Snapshot, reportedAt); err != nil {
		zap.L().Error("persist metric snapshot failed", zap.String("node_id", nodeID), zap.Error(err))
	}
	if err := persistNodeOpenrestyObservation(ctx, nodeID, payload.OpenrestyObservation, reportedAt); err != nil {
		zap.L().Error("persist openresty observation failed", zap.String("node_id", nodeID), zap.Error(err))
	}
	if err := persistNodeTrafficReport(ctx, nodeID, payload.TrafficReport, reportedAt); err != nil {
		zap.L().Error("persist traffic report failed", zap.String("node_id", nodeID), zap.Error(err))
	}

	if err := persistNodeAccessLogs(ctx, nodeID, accessLogRecords, reportedAt); err != nil {
		zap.L().Error("persist heartbeat access logs failed", zap.String("node_id", nodeID), zap.Error(err))
	}
}

func persistBufferedObservability(ctx context.Context, nodeID string, records []BufferedObservabilityRecord, reportedAt time.Time) error {
	for _, record := range records {
		if err := persistNodeMetricSnapshot(ctx, nodeID, record.Snapshot, reportedAt); err != nil {
			return err
		}
		if err := persistNodeOpenrestyObservation(ctx, nodeID, record.OpenrestyObservation, reportedAt); err != nil {
			return err
		}
		if err := persistNodeTrafficReport(ctx, nodeID, record.TrafficReport, reportedAt); err != nil {
			return err
		}
	}
	return nil
}

func persistNodeSystemProfile(tx *gorm.DB, nodeID string, profile *NodeSystemProfile, reportedAt time.Time) error {
	if profile == nil {
		return nil
	}
	record := &model.OpenFlareNodeSystemProfile{
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
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "node_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"hostname",
			"os_name",
			"os_version",
			"kernel_version",
			"architecture",
			"cpu_model",
			"cpu_cores",
			"total_memory_bytes",
			"total_disk_bytes",
			"uptime_seconds",
			"reported_at",
			"updated_at",
		}),
	}).Create(record).Error
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
		NetworkRxBytes:    snapshot.NetworkRxBytes,
		NetworkTxBytes:    snapshot.NetworkTxBytes,
	}
	return model.InsertOpenFlareMetricSnapshot(ctx, record)
}

func persistNodeOpenrestyObservation(ctx context.Context, nodeID string, obs *NodeOpenrestyObservation, reportedAt time.Time) error {
	if obs == nil {
		return nil
	}
	record := &model.OpenFlareNodeObservationOpenresty{
		NodeID:               nodeID,
		CapturedAt:           timeFromUnix(obs.CapturedAtUnix, reportedAt),
		OpenrestyRxBytes:     obs.OpenrestyRxBytes,
		OpenrestyTxBytes:     obs.OpenrestyTxBytes,
		OpenrestyConnections: obs.OpenrestyConnections,
	}
	return model.InsertOpenFlareNodeObservationOpenresty(ctx, record)
}

func persistNodeTrafficReport(ctx context.Context, nodeID string, report *NodeTrafficReport, reportedAt time.Time) error {
	if report == nil {
		return nil
	}
	if report.WindowEndedAtUnix > 0 && report.WindowStartedAtUnix > report.WindowEndedAtUnix {
		return errors.New("traffic report window_started_at_unix 不能大于 window_ended_at_unix")
	}
	record := &model.OpenFlareRequestReport{
		NodeID:              nodeID,
		WindowStartedAt:     timeFromUnix(report.WindowStartedAtUnix, reportedAt),
		WindowEndedAt:       timeFromUnix(report.WindowEndedAtUnix, reportedAt),
		RequestCount:        report.RequestCount,
		ErrorCount:          report.ErrorCount,
		UniqueVisitorCount:  report.UniqueVisitorCount,
		StatusCodesJSON:     marshalJSON(report.StatusCodes),
		TopDomainsJSON:      marshalJSON(report.TopDomains),
		SourceCountriesJSON: marshalJSON(report.SourceCountries),
	}
	return model.InsertOpenFlareRequestReport(ctx, record)
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
			record := &model.OpenFlareAccessLog{
				NodeID:     nodeID,
				LoggedAt:   timeFromUnix(item.LoggedAtUnix, reportedAt),
				RemoteAddr: strings.TrimSpace(item.RemoteAddr),
				Region:     "",
				Host:       strings.TrimSpace(item.Host),
				Path:       truncateForDatabase(strings.TrimSpace(item.Path), accessLogPathMaxLength),
				StatusCode: item.StatusCode,
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

func persistNodeAccessLogs(ctx context.Context, nodeID string, records []*model.OpenFlareAccessLog, reportedAt time.Time) error {
	if len(records) == 0 {
		return nil
	}
	return model.InsertOpenFlareAccessLogsBatch(ctx, records)
}

func reconcileNodeHealthEvents(tx *gorm.DB, nodeID string, events []NodeHealthEvent, reportedAt time.Time) error {
	return ReconcileScopedNodeHealthEvents(tx, nodeID, events, reportedAt, nil)
}

// ReconcileScopedNodeHealthEvents reconciles health events, optionally scoped to managed event types.
func ReconcileScopedNodeHealthEvents(tx *gorm.DB, nodeID string, events []NodeHealthEvent, reportedAt time.Time, managedEventTypes map[string]struct{}) error {
	activeTypes := make(map[string]NodeHealthEvent, len(events))
	for _, event := range events {
		eventType := normalizeHealthEventType(event.EventType)
		if eventType == "" {
			continue
		}
		if len(managedEventTypes) > 0 {
			if _, ok := managedEventTypes[eventType]; !ok {
				continue
			}
		}
		event.EventType = eventType
		event.Severity = normalizeHealthSeverity(event.Severity)
		if event.TriggeredAtUnix <= 0 {
			event.TriggeredAtUnix = reportedAt.Unix()
		}
		activeTypes[eventType] = event
	}

	var activeEvents []*model.OpenFlareHealthEvent
	query := tx.Where("node_id = ? AND status = ?", nodeID, healthEventStatusActive)
	if len(managedEventTypes) > 0 {
		scopedTypes := make([]string, 0, len(managedEventTypes))
		for eventType := range managedEventTypes {
			eventType = normalizeHealthEventType(eventType)
			if eventType != "" {
				scopedTypes = append(scopedTypes, eventType)
			}
		}
		if len(scopedTypes) == 0 {
			return nil
		}
		query = query.Where("event_type IN ?", scopedTypes)
	}
	if err := query.Find(&activeEvents).Error; err != nil {
		return err
	}

	activeByType := make(map[string]*model.OpenFlareHealthEvent, len(activeEvents))
	for _, event := range activeEvents {
		activeByType[event.EventType] = event
	}

	for eventType, event := range activeTypes {
		triggeredAt := timeFromUnix(event.TriggeredAtUnix, reportedAt)
		if existing, ok := activeByType[eventType]; ok {
			existing.Severity = event.Severity
			existing.Message = normalizeHealthEventMessage(event.Message)
			existing.LastTriggeredAt = triggeredAt
			existing.ReportedAt = reportedAt
			existing.MetadataJSON = marshalJSON(event.Metadata)
			existing.ResolvedAt = nil
			if err := tx.Save(existing).Error; err != nil {
				return err
			}
			continue
		}
		record := &model.OpenFlareHealthEvent{
			NodeID:           nodeID,
			EventType:        eventType,
			Severity:         event.Severity,
			Status:           healthEventStatusActive,
			Message:          normalizeHealthEventMessage(event.Message),
			FirstTriggeredAt: triggeredAt,
			LastTriggeredAt:  triggeredAt,
			ReportedAt:       reportedAt,
			MetadataJSON:     marshalJSON(event.Metadata),
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
	}

	for _, existing := range activeEvents {
		if _, ok := activeTypes[existing.EventType]; ok {
			continue
		}
		resolvedAt := reportedAt
		existing.Status = healthEventStatusResolved
		existing.ReportedAt = reportedAt
		existing.ResolvedAt = &resolvedAt
		if err := tx.Save(existing).Error; err != nil {
			return err
		}
	}

	return nil
}

func normalizeHealthEventType(eventType string) string {
	eventType = strings.TrimSpace(strings.ToLower(eventType))
	eventType = strings.ReplaceAll(eventType, " ", "_")
	return eventType
}

func normalizeHealthSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case healthSeverityCritical:
		return healthSeverityCritical
	case healthSeverityInfo:
		return healthSeverityInfo
	default:
		return healthSeverityWarning
	}
}

func normalizeHealthEventMessage(message string) string {
	return truncateForDatabase(message, healthEventMessageMaxLength)
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
