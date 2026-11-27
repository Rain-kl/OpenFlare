// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netquality"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/model"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
	"go.uber.org/zap"
)

const (
	accessLogPathMaxLength        = 100
	accessLogUserAgentMaxLength   = 512
	accessLogCacheStatusMaxLength = 32
)

// PersistHeartbeatObservability stores profile, host metrics, and access logs.
func PersistHeartbeatObservability(ctx context.Context, nodeID string, payload protocol.NodePayload, reportedAt time.Time) {
	if strings.TrimSpace(nodeID) == "" {
		return
	}
	if payload.Profile == nil &&
		payload.HostMetrics == nil &&
		len(payload.AccessLogs) == 0 &&
		len(payload.Buffered) == 0 &&
		payload.HealthEvents == nil {
		return
	}

	accessLogRecords, err := buildNodeAccessLogRecords(ctx, nodeID, payload.AccessLogs, payload.Buffered, reportedAt)
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

	if err := persistNodeAccessLogs(ctx, nodeID, accessLogRecords, reportedAt); err != nil {
		zap.L().Error("persist heartbeat access logs failed", zap.String("node_id", nodeID), zap.Error(err))
	}
}

func persistBufferedObservability(ctx context.Context, nodeID string, records []protocol.BufferedObservabilityRecord, reportedAt time.Time) error {
	for _, record := range records {
		if err := persistNodeMetricSnapshot(ctx, nodeID, record.HostMetrics, reportedAt); err != nil {
			return err
		}
	}
	return nil
}

func buildNodeSystemProfileModel(nodeID string, profile *protocol.NodeSystemProfile, reportedAt time.Time) *model.OpenFlareNodeSystemProfile {
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
		CPUPhysicalCores: profile.CPUPhysicalCores,
		Virtualization:   strings.TrimSpace(profile.Virtualization),
		GPUName:          strings.TrimSpace(profile.GPUName),
		TotalMemoryBytes: profile.TotalMemoryBytes,
		SwapTotalBytes:   profile.SwapTotalBytes,
		TotalDiskBytes:   profile.TotalDiskBytes,
		IPv4:             strings.TrimSpace(profile.IPv4),
		IPv6:             strings.TrimSpace(profile.IPv6),
		ProbeVersion:     strings.TrimSpace(profile.ProbeVersion),
		UptimeSeconds:    profile.UptimeSeconds,
		ReportedAt:       timeFromUnix(profile.ReportedAtUnix, reportedAt),
	}
}

func healthEventInputs(events []protocol.NodeHealthEvent) []repository.OpenFlareHealthEventInput {
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

func persistNodeMetricSnapshot(ctx context.Context, nodeID string, snapshot *protocol.NodeMetricSnapshot, reportedAt time.Time) error {
	if snapshot == nil {
		return nil
	}
	record := &model.OpenFlareMetricSnapshot{
		NodeID:            nodeID,
		CapturedAt:        timeFromUnix(snapshot.CapturedAtUnix, reportedAt),
		CPUUsagePercent:   snapshot.CPUUsagePercent,
		GPUUsagePercent:   snapshot.GPUUsagePercent,
		MemoryUsedBytes:   snapshot.MemoryUsedBytes,
		MemoryTotalBytes:  snapshot.MemoryTotalBytes,
		SwapUsedBytes:     snapshot.SwapUsedBytes,
		SwapTotalBytes:    snapshot.SwapTotalBytes,
		Load1:             snapshot.Load1,
		Load5:             snapshot.Load5,
		Load15:            snapshot.Load15,
		Temp:              snapshot.Temp,
		StorageUsedBytes:  snapshot.StorageUsedBytes,
		StorageTotalBytes: snapshot.StorageTotalBytes,
		DiskReadBytes:     snapshot.DiskReadBytes,
		DiskWriteBytes:    snapshot.DiskWriteBytes,
		NetIn:             snapshot.NetIn,
		NetOut:            snapshot.NetOut,
		NetworkRxBytes:    snapshot.NetTotalDown,
		NetworkTxBytes:    snapshot.NetTotalUp,
		ProcessCount:      snapshot.ProcessCount,
		TCPConnections:    snapshot.TCPConnections,
		UDPConnections:    snapshot.UDPConnections,
		UptimeSeconds:     snapshot.UptimeSeconds,
	}
	return repository.InsertOpenFlareMetricSnapshot(ctx, record)
}

func buildNodeAccessLogRecords(ctx context.Context, nodeID string, direct []protocol.NodeAccessLog, buffered []protocol.BufferedObservabilityRecord, reportedAt time.Time) ([]*model.OpenFlareAccessLog, error) {
	total := len(direct)
	for _, record := range buffered {
		total += len(record.AccessLogs)
	}
	if total == 0 {
		return nil, nil
	}

	records := make([]*model.OpenFlareAccessLog, 0, total)
	appendLogs := func(logs []protocol.NodeAccessLog) error {
		for _, item := range logs {
			wafAction, wafAudit, err := serializeWAFAudit(item.WAF)
			if err != nil {
				return err
			}
			path := truncateForDatabase(strings.TrimSpace(item.Path), accessLogPathMaxLength)
			if wafAction != "" {
				path = attackRequestPath(item.Path)
			}
			bytesSent := max(item.BytesSent, 0)
			requestLength := max(item.RequestLength, 0)
			requestTimeMs := max(item.RequestTimeMs, 0)
			normalizedIP := normalizeAccessLogIP(item.RemoteAddr)
			var clientASN uint32
			var clientCountry, clientRegion, geoSource string
			var clientLat, clientLon *float64
			var geoAccuracyKm float64
			ipFamily := pkgnetintel.DetectIPFamily(normalizedIP)
			if normalizedIP != "" {
				if lookup := netintel.LookupIP(normalizedIP); lookup != nil {
					clientASN = lookup.ASN
					clientCountry = lookup.CountryISO
					clientRegion = lookup.RegionCode
					clientLat = lookup.Latitude
					clientLon = lookup.Longitude
					geoAccuracyKm = lookup.AccuracyRadiusKm
					geoSource = lookup.GeoSource
				}
			}
			record := &model.OpenFlareAccessLog{
				NodeID:        nodeID,
				LoggedAt:      timeFromUnix(item.LoggedAtUnix, reportedAt),
				RemoteAddr:    strings.TrimSpace(item.RemoteAddr),
				Region:        resolveAccessLogRegion(ctx, item.RemoteAddr),
				Host:          strings.TrimSpace(item.Host),
				Path:          path,
				WAFAction:     wafAction,
				WAFAudit:      wafAudit,
				UserAgent:     truncateForDatabase(strings.TrimSpace(item.UserAgent), accessLogUserAgentMaxLength),
				CacheStatus:   truncateForDatabase(strings.TrimSpace(item.CacheStatus), accessLogCacheStatusMaxLength),
				StatusCode:    item.StatusCode,
				BytesSent:     bytesSent,
				RequestLength: requestLength,
				RequestTimeMs: requestTimeMs,
				ClientASN:     clientASN,
				TCPRTTUs:      item.TCPRTTUs,
			}
			if clientASN > 0 && item.TCPRTTUs > 0 {
				netquality.RecordPassiveSampleInput(ctx, netquality.PassiveSampleInput{
					NodeID:        nodeID,
					ClientIP:      normalizedIP,
					ClientASN:     clientASN,
					ClientCountry: clientCountry,
					ClientRegion:  clientRegion,
					IPFamily:      ipFamily,
					ClientLat:     clientLat,
					ClientLon:     clientLon,
					GeoAccuracyKm: geoAccuracyKm,
					GeoSource:     geoSource,
					TCPRTTUs:      item.TCPRTTUs,
					TCPMinRTTUs:   item.TCPMinRTTUs,
					TCPRetrans:    item.TCPRetrans,
					TCPSegsOut:    item.TCPSegsOut,
					TLSHandshake:  item.TLSHandshakeMs,
				})
			}
			records = append(records, record)
		}
		return nil
	}
	if err := appendLogs(direct); err != nil {
		return nil, err
	}
	for _, record := range buffered {
		if err := appendLogs(record.AccessLogs); err != nil {
			return nil, err
		}
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
func ReconcileScopedNodeHealthEvents(ctx context.Context, nodeID string, events []protocol.NodeHealthEvent, reportedAt time.Time, managedEventTypes map[string]struct{}) error {
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
