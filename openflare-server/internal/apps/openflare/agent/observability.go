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
	nodeAccessLogRetentionDays   = 90
	nodeAccessLogRetentionWindow = nodeAccessLogRetentionDays * 24 * time.Hour
	accessLogPathMaxLength       = 100
)

// NodeSystemProfile is the agent-reported system profile.
type NodeSystemProfile struct {
	Hostname         string `json:"hostname"`
	OSName           string `json:"os_name"`
	OSVersion        string `json:"os_version"`
	KernelVersion    string `json:"kernel_version"`
	Architecture     string `json:"architecture"`
	CPUModel         string `json:"cpu_model"`
	CPUCores         int    `json:"cpu_cores"`
	TotalMemoryBytes int64  `json:"total_memory_bytes"`
	TotalDiskBytes   int64  `json:"total_disk_bytes"`
	UptimeSeconds    int64  `json:"uptime_seconds"`
	ReportedAtUnix   int64  `json:"reported_at_unix"`
}

// NodeMetricSnapshot is the agent-reported capacity snapshot.
type NodeMetricSnapshot struct {
	CapturedAtUnix    int64   `json:"captured_at_unix"`
	CPUUsagePercent   float64 `json:"cpu_usage_percent"`
	MemoryUsedBytes   int64   `json:"memory_used_bytes"`
	MemoryTotalBytes  int64   `json:"memory_total_bytes"`
	StorageUsedBytes  int64   `json:"storage_used_bytes"`
	StorageTotalBytes int64   `json:"storage_total_bytes"`
	DiskReadBytes     int64   `json:"disk_read_bytes"`
	DiskWriteBytes    int64   `json:"disk_write_bytes"`
	NetworkRxBytes    int64   `json:"network_rx_bytes"`
	NetworkTxBytes    int64   `json:"network_tx_bytes"`
}

// NodeOpenrestyObservation is the agent-reported openresty network observation.
type NodeOpenrestyObservation struct {
	CapturedAtUnix       int64 `json:"captured_at_unix"`
	OpenrestyRxBytes     int64 `json:"openresty_rx_bytes"`
	OpenrestyTxBytes     int64 `json:"openresty_tx_bytes"`
	OpenrestyConnections int64 `json:"openresty_connections"`
}

// NodeTrafficReport is the agent-reported traffic window.
type NodeTrafficReport struct {
	WindowStartedAtUnix int64            `json:"window_started_at_unix"`
	WindowEndedAtUnix   int64            `json:"window_ended_at_unix"`
	RequestCount        int64            `json:"request_count"`
	ErrorCount          int64            `json:"error_count"`
	UniqueVisitorCount  int64            `json:"unique_visitor_count"`
	StatusCodes         map[string]int64 `json:"status_codes"`
	TopDomains          map[string]int64 `json:"top_domains"`
	SourceCountries     map[string]int64 `json:"source_countries"`
}

// NodeAccessLog is a single access log row from the agent.
type NodeAccessLog struct {
	LoggedAtUnix int64  `json:"logged_at_unix"`
	RemoteAddr   string `json:"remote_addr"`
	Host         string `json:"host"`
	Path         string `json:"path"`
	StatusCode   int    `json:"status_code"`
}

// BufferedObservabilityRecord is a buffered observability window from the agent.
type BufferedObservabilityRecord struct {
	WindowStartedAtUnix  int64                     `json:"window_started_at_unix"`
	Snapshot             *NodeMetricSnapshot       `json:"snapshot,omitempty"`
	OpenrestyObservation *NodeOpenrestyObservation `json:"openresty_observation,omitempty"`
	TrafficReport        *NodeTrafficReport        `json:"traffic_report,omitempty"`
	AccessLogs           []NodeAccessLog           `json:"access_logs,omitempty"`
}

// NodeHealthEvent is an agent-reported health event.
type NodeHealthEvent struct {
	EventType       string            `json:"event_type"`
	Severity        string            `json:"severity"`
	Message         string            `json:"message"`
	TriggeredAtUnix int64             `json:"triggered_at_unix"`
	Metadata        map[string]string `json:"metadata"`
}

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

	if err := conn.Transaction(func(tx *gorm.DB) error {
		if err := persistNodeSystemProfile(tx, nodeID, payload.Profile, reportedAt); err != nil {
			return err
		}
		if err := persistBufferedObservability(tx, nodeID, payload.BufferedObservability, reportedAt); err != nil {
			return err
		}
		if err := persistNodeMetricSnapshot(tx, nodeID, payload.Snapshot, reportedAt); err != nil {
			return err
		}
		if err := persistNodeOpenrestyObservation(tx, nodeID, payload.OpenrestyObservation, reportedAt); err != nil {
			return err
		}
		if err := persistNodeTrafficReport(tx, nodeID, payload.TrafficReport, reportedAt); err != nil {
			return err
		}
		if err := persistNodeAccessLogs(tx, nodeID, payload.AccessLogs, reportedAt); err != nil {
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
	}
}

func persistBufferedObservability(tx *gorm.DB, nodeID string, records []BufferedObservabilityRecord, reportedAt time.Time) error {
	for _, record := range records {
		if err := persistNodeMetricSnapshot(tx, nodeID, record.Snapshot, reportedAt); err != nil {
			return err
		}
		if err := persistNodeOpenrestyObservation(tx, nodeID, record.OpenrestyObservation, reportedAt); err != nil {
			return err
		}
		if err := persistNodeTrafficReport(tx, nodeID, record.TrafficReport, reportedAt); err != nil {
			return err
		}
		if err := persistNodeAccessLogs(tx, nodeID, record.AccessLogs, reportedAt); err != nil {
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

func persistNodeMetricSnapshot(tx *gorm.DB, nodeID string, snapshot *NodeMetricSnapshot, reportedAt time.Time) error {
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
	exists, err := metricSnapshotExists(tx, nodeID, record.CapturedAt)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return tx.Create(record).Error
}

func persistNodeOpenrestyObservation(tx *gorm.DB, nodeID string, obs *NodeOpenrestyObservation, reportedAt time.Time) error {
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
	return tx.Create(record).Error
}

func persistNodeTrafficReport(tx *gorm.DB, nodeID string, report *NodeTrafficReport, reportedAt time.Time) error {
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
	exists, err := requestReportExists(tx, nodeID, record.WindowStartedAt, record.WindowEndedAt)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return tx.Create(record).Error
}

func persistNodeAccessLogs(tx *gorm.DB, nodeID string, logs []NodeAccessLog, reportedAt time.Time) error {
	if len(logs) == 0 {
		return nil
	}
	resolver, err := newAccessLogRegionResolver()
	if err != nil {
		slog.Warn("initialize access log geo resolver failed", "node_id", nodeID, "error", err)
	}
	if resolver != nil {
		defer resolver.Close()
	}
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
		exists, err := accessLogExists(tx, record)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
	}
	_, err = deleteAccessLogsByNodeBefore(tx, nodeID, reportedAt.Add(-nodeAccessLogRetentionWindow))
	return err
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

func metricSnapshotExists(tx *gorm.DB, nodeID string, capturedAt time.Time) (bool, error) {
	var count int64
	if err := tx.Model(&model.OpenFlareMetricSnapshot{}).
		Where("node_id = ? AND captured_at = ?", nodeID, capturedAt).
		Limit(1).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func requestReportExists(tx *gorm.DB, nodeID string, windowStartedAt, windowEndedAt time.Time) (bool, error) {
	var count int64
	if err := tx.Model(&model.OpenFlareRequestReport{}).
		Where("node_id = ? AND window_started_at = ? AND window_ended_at = ?", nodeID, windowStartedAt, windowEndedAt).
		Limit(1).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func deleteAccessLogsByNodeBefore(tx *gorm.DB, nodeID string, before time.Time) (int64, error) {
	result := tx.Where("node_id = ? AND logged_at < ?", nodeID, before).Delete(&model.OpenFlareAccessLog{})
	return result.RowsAffected, result.Error
}

func accessLogExists(tx *gorm.DB, record *model.OpenFlareAccessLog) (bool, error) {
	var count int64
	if err := tx.Model(&model.OpenFlareAccessLog{}).
		Where(
			"node_id = ? AND logged_at = ? AND remote_addr = ? AND host = ? AND path = ? AND status_code = ?",
			record.NodeID,
			record.LoggedAt,
			record.RemoteAddr,
			record.Host,
			record.Path,
			record.StatusCode,
		).
		Limit(1).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
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
	return truncateForDatabase(message, 4096)
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
