// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsrepo "github.com/Rain-kl/Wavelet/internal/repository/analytics"
)

const (
	openFlareHealthEventStatusActive   = "active"
	openFlareHealthEventStatusResolved = "resolved"
	openFlareHealthSeverityInfo        = "info"
	openFlareHealthSeverityWarning     = "warning"
	openFlareHealthSeverityCritical    = "critical"
	openFlareHealthEventMessageMaxLen  = 4096
)

// OpenFlareHealthEventInput describes a desired active health event for reconciliation.
type OpenFlareHealthEventInput struct {
	EventType       string
	Severity        string
	Message         string
	TriggeredAtUnix int64
	Metadata        map[string]string
}

func isMissingTableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "doesn't exist") ||
		strings.Contains(msg, "does not exist")
}

// InsertOpenFlareMetricSnapshot inserts a metric snapshot into ClickHouse.
func InsertOpenFlareMetricSnapshot(ctx context.Context, record *model.OpenFlareMetricSnapshot) error {
	return currentObservabilityStore().InsertMetricSnapshot(ctx, record)
}

// InsertOpenFlareEdgeHealth inserts an L2 edge health snapshot into ClickHouse.
func InsertOpenFlareEdgeHealth(ctx context.Context, record *model.OpenFlareEdgeHealth) error {
	return currentObservabilityStore().InsertEdgeHealth(ctx, record)
}

// InsertOpenFlareNodeObservationFrps inserts an FRPS observation into ClickHouse.
func InsertOpenFlareNodeObservationFrps(ctx context.Context, record *model.OpenFlareNodeObservationFrps) error {
	return currentObservabilityStore().InsertNodeObservationFrps(ctx, record)
}

// InsertOpenFlareNodeObservationFrpc inserts an FRPC observation into ClickHouse.
func InsertOpenFlareNodeObservationFrpc(ctx context.Context, record *model.OpenFlareNodeObservationFrpc) error {
	return currentObservabilityStore().InsertNodeObservationFrpc(ctx, record)
}

// ListOpenFlareMetricSnapshotsSince returns metric snapshots since the given time.
func ListOpenFlareMetricSnapshotsSince(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareMetricSnapshot, error) {
	return currentObservabilityStore().ListMetricSnapshots(ctx, nodeID, since, limit)
}

// ListOpenFlareLatestMetricSnapshotsSince returns the latest metric snapshot per node.
// Prefer ClickHouse LIMIT 1 BY; on CH unavailability fall back to store list + reduce.
func ListOpenFlareLatestMetricSnapshotsSince(ctx context.Context, nodeID string, since time.Time) ([]*model.OpenFlareMetricSnapshot, error) {
	rows, err := analyticsrepo.ListLatestNodeMetricSnapshots(ctx, analyticsrepo.NodeObservabilityFilter{
		NodeID: nodeID,
		Since:  since,
	})
	if err == nil {
		return fromAnalyticsNodeMetricSnapshots(rows), nil
	}
	// Fallback for unit tests (memory store) and environments without ClickHouse.
	all, listErr := ListOpenFlareMetricSnapshotsSince(ctx, nodeID, since, 0)
	if listErr != nil {
		return nil, err
	}
	return openFlareLatestMetricSnapshots(all), nil
}

func openFlareLatestMetricSnapshots(snapshots []*model.OpenFlareMetricSnapshot) []*model.OpenFlareMetricSnapshot {
	latestByNode := make(map[string]*model.OpenFlareMetricSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot == nil || snapshot.NodeID == "" {
			continue
		}
		if existing, ok := latestByNode[snapshot.NodeID]; ok && !snapshot.CapturedAt.After(existing.CapturedAt) {
			continue
		}
		latestByNode[snapshot.NodeID] = snapshot
	}
	result := make([]*model.OpenFlareMetricSnapshot, 0, len(latestByNode))
	for _, snapshot := range latestByNode {
		result = append(result, snapshot)
	}
	return result
}

// ListOpenFlareTrafficHourlySince returns hourly traffic rollup rows since the given time.
// Source: of_access_log_hourly (M5).
func ListOpenFlareTrafficHourlySince(ctx context.Context, nodeID string, since time.Time) ([]*model.OpenFlareTrafficHourly, error) {
	rows, err := analyticsrepo.ListNodeTrafficHourly(ctx, analyticsrepo.NodeObservabilityFilter{
		NodeID: nodeID,
		Since:  since,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*model.OpenFlareTrafficHourly, len(rows))
	for index, row := range rows {
		result[index] = &model.OpenFlareTrafficHourly{
			NodeID:             row.NodeID,
			Hour:               row.Hour,
			RequestCount:       row.RequestCount,
			ErrorCount:         row.ErrorCount,
			UniqueVisitorCount: row.UniqueVisitorCount,
		}
	}
	return result, nil
}

// ListOpenFlareAccessLogHourlySince returns of_access_log_hourly rows since the given time.
func ListOpenFlareAccessLogHourlySince(ctx context.Context, nodeID string, since time.Time) ([]*model.OpenFlareAccessLogHourly, error) {
	rows, err := analyticsrepo.ListAccessLogHourly(ctx, analyticsrepo.NodeObservabilityFilter{
		NodeID: nodeID,
		Since:  since,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*model.OpenFlareAccessLogHourly, len(rows))
	for index, row := range rows {
		result[index] = &model.OpenFlareAccessLogHourly{
			NodeID:        row.NodeID,
			Hour:          row.Hour,
			Host:          row.Host,
			RequestCount:  row.RequestCount,
			ErrorCount:    row.ErrorCount,
			BytesSent:     row.BytesSent,
			RequestLength: row.RequestLength,
		}
	}
	return result, nil
}

// ListOpenFlareMetricHourlySince returns hourly metric aggregates since the given time.
func ListOpenFlareMetricHourlySince(ctx context.Context, nodeID string, since time.Time) ([]*model.OpenFlareMetricHourly, error) {
	rows, err := analyticsrepo.ListNodeMetricHourly(ctx, analyticsrepo.NodeObservabilityFilter{
		NodeID: nodeID,
		Since:  since,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*model.OpenFlareMetricHourly, len(rows))
	for index, row := range rows {
		result[index] = &model.OpenFlareMetricHourly{
			Hour:                      row.Hour,
			AverageCPUUsagePercent:    row.AverageCPUUsagePercent,
			AverageMemoryUsagePercent: row.AverageMemoryUsagePercent,
			NetworkRxBytes:            row.NetworkRxBytes,
			NetworkTxBytes:            row.NetworkTxBytes,
			DiskReadBytes:             row.DiskReadBytes,
			DiskWriteBytes:            row.DiskWriteBytes,
			ReportedNodes:             row.ReportedNodes,
		}
	}
	return result, nil
}

// ListOpenFlareActiveHealthEvents returns active health events across all nodes.
func ListOpenFlareActiveHealthEvents(ctx context.Context) ([]*model.OpenFlareHealthEvent, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var rows []*model.OpenFlareHealthEvent
	if err := conn.Where("status = ?", "active").Order("last_triggered_at desc").Find(&rows).Error; err != nil {
		if isMissingTableError(err) {
			return []*model.OpenFlareHealthEvent{}, nil
		}
		return nil, err
	}
	return rows, nil
}

// ListOpenFlareHealthEvents returns health events for a node.
func ListOpenFlareHealthEvents(ctx context.Context, nodeID string, activeOnly bool, limit int) ([]*model.OpenFlareHealthEvent, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	query := conn.Model(&model.OpenFlareHealthEvent{}).Where("node_id = ?", nodeID).Order("last_triggered_at desc")
	if activeOnly {
		query = query.Where("status = ?", "active")
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	var rows []*model.OpenFlareHealthEvent
	if err := query.Find(&rows).Error; err != nil {
		if isMissingTableError(err) {
			return []*model.OpenFlareHealthEvent{}, nil
		}
		return nil, err
	}
	return rows, nil
}

// DeleteOpenFlareMetricSnapshotsBefore deletes metric snapshots captured before cutoff.
func DeleteOpenFlareMetricSnapshotsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return currentObservabilityStore().DeleteMetricSnapshotsBefore(ctx, cutoff)
}

// DeleteAllOpenFlareMetricSnapshots deletes all metric snapshots.
func DeleteAllOpenFlareMetricSnapshots(ctx context.Context) (int64, error) {
	return currentObservabilityStore().DeleteAllMetricSnapshots(ctx)
}

// DeleteOpenFlareEdgeHealthBefore deletes edge health rows captured before cutoff.
func DeleteOpenFlareEdgeHealthBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return currentObservabilityStore().DeleteEdgeHealthBefore(ctx, cutoff)
}

// DeleteAllOpenFlareEdgeHealth deletes all edge health snapshots.
func DeleteAllOpenFlareEdgeHealth(ctx context.Context) (int64, error) {
	return currentObservabilityStore().DeleteAllEdgeHealth(ctx)
}

// DeleteOpenFlareNodeObservationFrpsBefore deletes FRPS observations captured before cutoff.
func DeleteOpenFlareNodeObservationFrpsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return currentObservabilityStore().DeleteNodeObservationFrpsBefore(ctx, cutoff)
}

// DeleteAllOpenFlareNodeObservationFrps deletes all FRPS observations.
func DeleteAllOpenFlareNodeObservationFrps(ctx context.Context) (int64, error) {
	return currentObservabilityStore().DeleteAllNodeObservationFrps(ctx)
}

// DeleteOpenFlareNodeObservationFrpcBefore deletes FRPC observations captured before cutoff.
func DeleteOpenFlareNodeObservationFrpcBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return currentObservabilityStore().DeleteNodeObservationFrpcBefore(ctx, cutoff)
}

// DeleteAllOpenFlareNodeObservationFrpc deletes all FRPC observations.
func DeleteAllOpenFlareNodeObservationFrpc(ctx context.Context) (int64, error) {
	return currentObservabilityStore().DeleteAllNodeObservationFrpc(ctx)
}

// DeleteOpenFlareHealthEventsByNodeID deletes all health events for a node.
func DeleteOpenFlareHealthEventsByNodeID(ctx context.Context, nodeID string) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}
	result := conn.Where("node_id = ?", nodeID).Delete(&model.OpenFlareHealthEvent{})
	if result.Error != nil {
		if isMissingTableError(result.Error) {
			return 0, nil
		}
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// GetOpenFlareNodeSystemProfile returns the system profile for a node.
func GetOpenFlareNodeSystemProfile(ctx context.Context, nodeID string) (*model.OpenFlareNodeSystemProfile, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var profile model.OpenFlareNodeSystemProfile
	if err := conn.Where("node_id = ?", nodeID).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || isMissingTableError(err) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &profile, nil
}

// UpsertOpenFlareNodeSystemProfile inserts or updates the latest system profile for a node.
func UpsertOpenFlareNodeSystemProfile(ctx context.Context, record *model.OpenFlareNodeSystemProfile) error {
	if record == nil {
		return nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return upsertOpenFlareNodeSystemProfileTx(conn, record)
}

func upsertOpenFlareNodeSystemProfileTx(tx *gorm.DB, record *model.OpenFlareNodeSystemProfile) error {
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

// ReconcileOpenFlareHealthEvents reconciles active health events for a node.
// Desired active events are created or updated; previously active types not present are resolved.
// When managedEventTypes is non-empty, only those event types are considered.
// Runs inside a transaction so multi-row create/update/resolve stays atomic.
func ReconcileOpenFlareHealthEvents(
	ctx context.Context,
	nodeID string,
	events []OpenFlareHealthEventInput,
	reportedAt time.Time,
	managedEventTypes map[string]struct{},
) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		return reconcileOpenFlareHealthEventsTx(tx, nodeID, events, reportedAt, managedEventTypes)
	})
}

// PersistOpenFlareNodePGObservability upserts an optional system profile and optionally reconciles
// health events in a single transaction (Postgres-side heartbeat observability).
// When reconcileHealth is false, health events are left untouched.
func PersistOpenFlareNodePGObservability(
	ctx context.Context,
	profile *model.OpenFlareNodeSystemProfile,
	nodeID string,
	events []OpenFlareHealthEventInput,
	reconcileHealth bool,
	reportedAt time.Time,
	managedEventTypes map[string]struct{},
) error {
	if profile == nil && !reconcileHealth {
		return nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if profile != nil {
			if err := upsertOpenFlareNodeSystemProfileTx(tx, profile); err != nil {
				return err
			}
		}
		if reconcileHealth {
			if err := reconcileOpenFlareHealthEventsTx(tx, nodeID, events, reportedAt, managedEventTypes); err != nil {
				return err
			}
		}
		return nil
	})
}

func reconcileOpenFlareHealthEventsTx(
	tx *gorm.DB,
	nodeID string,
	events []OpenFlareHealthEventInput,
	reportedAt time.Time,
	managedEventTypes map[string]struct{},
) error {
	activeTypes := make(map[string]OpenFlareHealthEventInput, len(events))
	for _, event := range events {
		eventType := normalizeOpenFlareHealthEventType(event.EventType)
		if eventType == "" {
			continue
		}
		if len(managedEventTypes) > 0 {
			if _, ok := managedEventTypes[eventType]; !ok {
				continue
			}
		}
		event.EventType = eventType
		event.Severity = normalizeOpenFlareHealthSeverity(event.Severity)
		if event.TriggeredAtUnix <= 0 {
			event.TriggeredAtUnix = reportedAt.Unix()
		}
		activeTypes[eventType] = event
	}

	var activeEvents []*model.OpenFlareHealthEvent
	query := tx.Where("node_id = ? AND status = ?", nodeID, openFlareHealthEventStatusActive)
	if len(managedEventTypes) > 0 {
		scopedTypes := make([]string, 0, len(managedEventTypes))
		for eventType := range managedEventTypes {
			eventType = normalizeOpenFlareHealthEventType(eventType)
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
		triggeredAt := timeFromUnixSeconds(event.TriggeredAtUnix, reportedAt)
		if existing, ok := activeByType[eventType]; ok {
			existing.Severity = event.Severity
			existing.Message = normalizeOpenFlareHealthEventMessage(event.Message)
			existing.LastTriggeredAt = triggeredAt
			existing.ReportedAt = reportedAt
			existing.MetadataJSON = marshalOpenFlareHealthMetadata(event.Metadata)
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
			Status:           openFlareHealthEventStatusActive,
			Message:          normalizeOpenFlareHealthEventMessage(event.Message),
			FirstTriggeredAt: triggeredAt,
			LastTriggeredAt:  triggeredAt,
			ReportedAt:       reportedAt,
			MetadataJSON:     marshalOpenFlareHealthMetadata(event.Metadata),
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
		existing.Status = openFlareHealthEventStatusResolved
		existing.ReportedAt = reportedAt
		existing.ResolvedAt = &resolvedAt
		if err := tx.Save(existing).Error; err != nil {
			return err
		}
	}

	return nil
}

func normalizeOpenFlareHealthEventType(eventType string) string {
	eventType = strings.TrimSpace(strings.ToLower(eventType))
	eventType = strings.ReplaceAll(eventType, " ", "_")
	return eventType
}

func normalizeOpenFlareHealthSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case openFlareHealthSeverityCritical:
		return openFlareHealthSeverityCritical
	case openFlareHealthSeverityInfo:
		return openFlareHealthSeverityInfo
	default:
		return openFlareHealthSeverityWarning
	}
}

func normalizeOpenFlareHealthEventMessage(message string) string {
	if openFlareHealthEventMessageMaxLen <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(message))
	if len(runes) <= openFlareHealthEventMessageMaxLen {
		return string(runes)
	}
	return string(runes[:openFlareHealthEventMessageMaxLen])
}

func timeFromUnixSeconds(unixSeconds int64, fallback time.Time) time.Time {
	if unixSeconds <= 0 {
		return fallback
	}
	return time.Unix(unixSeconds, 0).UTC()
}

func marshalOpenFlareHealthMetadata(value map[string]string) string {
	if value == nil {
		return ""
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

// ListOpenFlareEdgeHealth returns L2 edge health snapshots.
func ListOpenFlareEdgeHealth(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareEdgeHealth, error) {
	return currentObservabilityStore().ListEdgeHealth(ctx, nodeID, since, limit)
}

// ListOpenFlareNodeObservationFrpc returns frpc observations.
func ListOpenFlareNodeObservationFrpc(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareNodeObservationFrpc, error) {
	return currentObservabilityStore().ListNodeObservationFrpc(ctx, nodeID, since, limit)
}

// ListOpenFlareNodeObservationFrps returns frps observations.
func ListOpenFlareNodeObservationFrps(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareNodeObservationFrps, error) {
	return currentObservabilityStore().ListNodeObservationFrps(ctx, nodeID, since, limit)
}
