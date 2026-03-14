package service

import (
	"atsflare/model"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	NodeHealthEventStatusActive   = "active"
	NodeHealthEventStatusResolved = "resolved"
	NodeHealthSeverityInfo        = "info"
	NodeHealthSeverityWarning     = "warning"
	NodeHealthSeverityCritical    = "critical"
)

type AgentNodeSystemProfile struct {
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

type AgentNodeMetricSnapshot struct {
	CapturedAtUnix       int64   `json:"captured_at_unix"`
	CPUUsagePercent      float64 `json:"cpu_usage_percent"`
	MemoryUsedBytes      int64   `json:"memory_used_bytes"`
	MemoryTotalBytes     int64   `json:"memory_total_bytes"`
	StorageUsedBytes     int64   `json:"storage_used_bytes"`
	StorageTotalBytes    int64   `json:"storage_total_bytes"`
	DiskReadBytes        int64   `json:"disk_read_bytes"`
	DiskWriteBytes       int64   `json:"disk_write_bytes"`
	NetworkRxBytes       int64   `json:"network_rx_bytes"`
	NetworkTxBytes       int64   `json:"network_tx_bytes"`
	OpenrestyRxBytes     int64   `json:"openresty_rx_bytes"`
	OpenrestyTxBytes     int64   `json:"openresty_tx_bytes"`
	OpenrestyConnections int64   `json:"openresty_connections"`
}

type AgentNodeTrafficReport struct {
	WindowStartedAtUnix int64            `json:"window_started_at_unix"`
	WindowEndedAtUnix   int64            `json:"window_ended_at_unix"`
	RequestCount        int64            `json:"request_count"`
	ErrorCount          int64            `json:"error_count"`
	UniqueVisitorCount  int64            `json:"unique_visitor_count"`
	StatusCodes         map[string]int64 `json:"status_codes"`
	TopDomains          map[string]int64 `json:"top_domains"`
	SourceCountries     map[string]int64 `json:"source_countries"`
}

type AgentBufferedObservabilityRecord struct {
	WindowStartedAtUnix int64                    `json:"window_started_at_unix"`
	Snapshot            *AgentNodeMetricSnapshot `json:"snapshot,omitempty"`
	TrafficReport       *AgentNodeTrafficReport  `json:"traffic_report,omitempty"`
}

type AgentNodeHealthEvent struct {
	EventType       string            `json:"event_type"`
	Severity        string            `json:"severity"`
	Message         string            `json:"message"`
	TriggeredAtUnix int64             `json:"triggered_at_unix"`
	Metadata        map[string]string `json:"metadata"`
}

func persistHeartbeatObservability(nodeID string, payload AgentNodePayload, reportedAt time.Time) {
	if strings.TrimSpace(nodeID) == "" {
		return
	}
	if payload.Profile == nil && payload.Snapshot == nil && payload.TrafficReport == nil && len(payload.BufferedObservability) == 0 && payload.HealthEvents == nil {
		return
	}

	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := persistNodeSystemProfile(tx, nodeID, payload.Profile, reportedAt); err != nil {
			return err
		}
		if err := persistBufferedObservability(tx, nodeID, payload.BufferedObservability, reportedAt); err != nil {
			return err
		}
		if err := persistNodeMetricSnapshot(tx, nodeID, payload.Snapshot, reportedAt); err != nil {
			return err
		}
		if err := persistNodeTrafficReport(tx, nodeID, payload.TrafficReport, reportedAt); err != nil {
			return err
		}
		if payload.HealthEvents != nil {
			if err := reconcileNodeHealthEvents(tx, nodeID, payload.HealthEvents, reportedAt); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		slog.Error("persist heartbeat observability failed", "node_id", nodeID, "error", err)
	}
}

func persistBufferedObservability(tx *gorm.DB, nodeID string, records []AgentBufferedObservabilityRecord, reportedAt time.Time) error {
	for _, record := range records {
		if err := persistNodeMetricSnapshot(tx, nodeID, record.Snapshot, reportedAt); err != nil {
			return err
		}
		if err := persistNodeTrafficReport(tx, nodeID, record.TrafficReport, reportedAt); err != nil {
			return err
		}
	}
	return nil
}

func persistNodeSystemProfile(tx *gorm.DB, nodeID string, profile *AgentNodeSystemProfile, reportedAt time.Time) error {
	if profile == nil {
		return nil
	}
	record := &model.NodeSystemProfile{
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
		RawJSON:          marshalJSON(profile),
	}
	return tx.Model(&model.NodeSystemProfile{}).Where("node_id = ?", nodeID).Assign(record).FirstOrCreate(record).Error
}

func persistNodeMetricSnapshot(tx *gorm.DB, nodeID string, snapshot *AgentNodeMetricSnapshot, reportedAt time.Time) error {
	if snapshot == nil {
		return nil
	}
	record := &model.NodeMetricSnapshot{
		NodeID:               nodeID,
		CapturedAt:           timeFromUnix(snapshot.CapturedAtUnix, reportedAt),
		CPUUsagePercent:      snapshot.CPUUsagePercent,
		MemoryUsedBytes:      snapshot.MemoryUsedBytes,
		MemoryTotalBytes:     snapshot.MemoryTotalBytes,
		StorageUsedBytes:     snapshot.StorageUsedBytes,
		StorageTotalBytes:    snapshot.StorageTotalBytes,
		DiskReadBytes:        snapshot.DiskReadBytes,
		DiskWriteBytes:       snapshot.DiskWriteBytes,
		NetworkRxBytes:       snapshot.NetworkRxBytes,
		NetworkTxBytes:       snapshot.NetworkTxBytes,
		OpenrestyRxBytes:     snapshot.OpenrestyRxBytes,
		OpenrestyTxBytes:     snapshot.OpenrestyTxBytes,
		OpenrestyConnections: snapshot.OpenrestyConnections,
		RawJSON:              marshalJSON(snapshot),
	}
	return tx.Where("node_id = ? AND captured_at = ?", nodeID, record.CapturedAt).Assign(record).FirstOrCreate(record).Error
}

func persistNodeTrafficReport(tx *gorm.DB, nodeID string, report *AgentNodeTrafficReport, reportedAt time.Time) error {
	if report == nil {
		return nil
	}
	if report.WindowEndedAtUnix > 0 && report.WindowStartedAtUnix > report.WindowEndedAtUnix {
		return errors.New("traffic report window_started_at_unix 不能大于 window_ended_at_unix")
	}
	record := &model.NodeRequestReport{
		NodeID:              nodeID,
		WindowStartedAt:     timeFromUnix(report.WindowStartedAtUnix, reportedAt),
		WindowEndedAt:       timeFromUnix(report.WindowEndedAtUnix, reportedAt),
		RequestCount:        report.RequestCount,
		ErrorCount:          report.ErrorCount,
		UniqueVisitorCount:  report.UniqueVisitorCount,
		StatusCodesJSON:     marshalJSON(report.StatusCodes),
		TopDomainsJSON:      marshalJSON(report.TopDomains),
		SourceCountriesJSON: marshalJSON(report.SourceCountries),
		RawJSON:             marshalJSON(report),
	}
	return tx.Where("node_id = ? AND window_started_at = ? AND window_ended_at = ?", nodeID, record.WindowStartedAt, record.WindowEndedAt).Assign(record).FirstOrCreate(record).Error
}

func reconcileNodeHealthEvents(tx *gorm.DB, nodeID string, events []AgentNodeHealthEvent, reportedAt time.Time) error {
	activeTypes := make(map[string]AgentNodeHealthEvent, len(events))
	for _, event := range events {
		eventType := normalizeHealthEventType(event.EventType)
		if eventType == "" {
			continue
		}
		event.EventType = eventType
		event.Severity = normalizeHealthSeverity(event.Severity)
		if event.TriggeredAtUnix <= 0 {
			event.TriggeredAtUnix = reportedAt.Unix()
		}
		activeTypes[eventType] = event
	}

	var activeEvents []*model.NodeHealthEvent
	if err := tx.Where("node_id = ? AND status = ?", nodeID, NodeHealthEventStatusActive).Find(&activeEvents).Error; err != nil {
		return err
	}

	activeByType := make(map[string]*model.NodeHealthEvent, len(activeEvents))
	for _, event := range activeEvents {
		activeByType[event.EventType] = event
	}

	for eventType, event := range activeTypes {
		triggeredAt := timeFromUnix(event.TriggeredAtUnix, reportedAt)
		if existing, ok := activeByType[eventType]; ok {
			existing.Severity = event.Severity
			existing.Message = strings.TrimSpace(event.Message)
			existing.LastTriggeredAt = triggeredAt
			existing.ReportedAt = reportedAt
			existing.RawJSON = marshalJSON(event)
			existing.ResolvedAt = nil
			if err := tx.Save(existing).Error; err != nil {
				return err
			}
			continue
		}
		record := &model.NodeHealthEvent{
			NodeID:           nodeID,
			EventType:        eventType,
			Severity:         event.Severity,
			Status:           NodeHealthEventStatusActive,
			Message:          strings.TrimSpace(event.Message),
			FirstTriggeredAt: triggeredAt,
			LastTriggeredAt:  triggeredAt,
			ReportedAt:       reportedAt,
			RawJSON:          marshalJSON(event),
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
		existing.Status = NodeHealthEventStatusResolved
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
	case NodeHealthSeverityCritical:
		return NodeHealthSeverityCritical
	case NodeHealthSeverityInfo:
		return NodeHealthSeverityInfo
	default:
		return NodeHealthSeverityWarning
	}
}

func timeFromUnix(unixSeconds int64, fallback time.Time) time.Time {
	if unixSeconds <= 0 {
		return fallback
	}
	return time.Unix(unixSeconds, 0).UTC()
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
