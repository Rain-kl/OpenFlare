// Package observability provides system and service level observability data collection for the agent.
package observability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/config"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/state"
	edgeobs "github.com/Rain-kl/Wavelet/internal/apps/edge/observability"
)

const nodeHealthEventInitialCapacity = 2

// BuildProfile collects the system profile and returns it only if the fingerprint has changed.
func BuildProfile(cfg *config.Config, stateStore *state.Store) *protocol.NodeSystemProfile {
	profile := collectProfile(cfg)
	if profile == nil {
		return nil
	}
	fingerprint := fingerprintProfile(profile)
	if stateStore == nil {
		return profile
	}
	snapshot, err := stateStore.Load()
	if err != nil {
		return profile
	}
	if snapshot.LastProfileFingerprint == fingerprint {
		return nil
	}
	snapshot.LastProfileFingerprint = fingerprint
	if err = stateStore.Save(snapshot); err != nil {
		return profile
	}
	return profile
}

// BuildSnapshot captures current system metrics and returns a metric snapshot.
func BuildSnapshot(cfg *config.Config, stateStore *state.Store) *protocol.NodeMetricSnapshot {
	now := time.Now().UTC()
	metric := &protocol.NodeMetricSnapshot{
		CapturedAtUnix: now.Unix(),
	}

	memTotal, memUsed := edgeobs.ReadMemInfo()
	metric.MemoryTotalBytes = memTotal
	metric.MemoryUsedBytes = memUsed

	storageTotal, storageUsed := edgeobs.StatFilesystem(cfg.DataDir)
	metric.StorageTotalBytes = storageTotal
	metric.StorageUsedBytes = storageUsed

	// Host NIC totals are not collected (product no longer surfaces host NIC trends).
	metric.DiskReadBytes, metric.DiskWriteBytes = edgeobs.ReadLinuxDiskTotals()

	if stateStore == nil {
		return metric
	}

	totalCPU, idleCPU := edgeobs.ReadLinuxCPUStat()
	snapshot, err := stateStore.Load()
	if err != nil {
		return metric
	}
	if snapshot.LastCPUStatTotal > 0 && totalCPU > snapshot.LastCPUStatTotal && idleCPU >= snapshot.LastCPUStatIdle {
		deltaTotal := totalCPU - snapshot.LastCPUStatTotal
		deltaIdle := idleCPU - snapshot.LastCPUStatIdle
		if deltaTotal > 0 && deltaIdle <= deltaTotal {
			metric.CPUUsagePercent = (float64(deltaTotal-deltaIdle) / float64(deltaTotal)) * 100
		}
	}
	snapshot.LastCPUStatTotal = totalCPU
	snapshot.LastCPUStatIdle = idleCPU
	snapshot.LastMetricAtUnix = now.Unix()
	_ = stateStore.Save(snapshot)

	return metric
}

// BuildEdgeHealth builds the edge_health payload from a local probe and node status.
func BuildEdgeHealth(probe *EdgeHealthSnapshot, openrestyStatus, openrestyMessage string) *protocol.NodeEdgeHealth {
	status := strings.TrimSpace(openrestyStatus)
	message := strings.TrimSpace(openrestyMessage)
	if probe == nil {
		if status == "" {
			return nil
		}
		return &protocol.NodeEdgeHealth{
			CapturedAtUnix: time.Now().UTC().Unix(),
			Status:         status,
			Message:        message,
			Connections:    0,
		}
	}

	captured := probe.CapturedAtUnix
	if captured <= 0 {
		captured = time.Now().UTC().Unix()
	}
	if status == "" {
		status = protocol.OpenrestyStatusUnknown
		if probe.OK {
			status = protocol.OpenrestyStatusHealthy
		}
	}
	return &protocol.NodeEdgeHealth{
		CapturedAtUnix: captured,
		Status:         status,
		Message:        message,
		Connections:    probe.Connections,
	}
}

// BuildHealthEvents converts system snapshot health state into a list of health events.
func BuildHealthEvents(snapshot *state.Snapshot) []protocol.NodeHealthEvent {
	if snapshot == nil {
		return []protocol.NodeHealthEvent{}
	}
	events := make([]protocol.NodeHealthEvent, 0, nodeHealthEventInitialCapacity)
	nowUnix := time.Now().UTC().Unix()
	if strings.TrimSpace(snapshot.OpenrestyStatus) == protocol.OpenrestyStatusUnhealthy {
		events = append(events, protocol.NodeHealthEvent{
			EventType:       "openresty_unhealthy",
			Severity:        "critical",
			Message:         strings.TrimSpace(snapshot.OpenrestyMessage),
			TriggeredAtUnix: nowUnix,
		})
	}
	if strings.TrimSpace(snapshot.LastError) != "" {
		events = append(events, protocol.NodeHealthEvent{
			EventType:       "sync_error",
			Severity:        "warning",
			Message:         strings.TrimSpace(snapshot.LastError),
			TriggeredAtUnix: nowUnix,
		})
	}
	return events
}

func collectProfile(cfg *config.Config) *protocol.NodeSystemProfile {
	hostname, _ := os.Hostname()
	osName, osVersion := edgeobs.ReadLinuxOSRelease()
	kernelVersion := edgeobs.ReadFirstLine("/proc/sys/kernel/osrelease")
	cpuModel := edgeobs.ReadLinuxCPUModel()
	totalMemory, _ := edgeobs.ReadMemInfo()
	totalDisk, _ := edgeobs.StatFilesystem(cfg.DataDir)
	uptimeSeconds := edgeobs.ReadLinuxUptimeSeconds()

	return &protocol.NodeSystemProfile{
		Hostname:         strings.TrimSpace(hostname),
		OSName:           osName,
		OSVersion:        osVersion,
		KernelVersion:    kernelVersion,
		Architecture:     runtime.GOARCH,
		CPUModel:         cpuModel,
		CPUCores:         runtime.NumCPU(),
		TotalMemoryBytes: totalMemory,
		TotalDiskBytes:   totalDisk,
		UptimeSeconds:    uptimeSeconds,
		ReportedAtUnix:   time.Now().UTC().Unix(),
	}
}

func fingerprintProfile(profile *protocol.NodeSystemProfile) string {
	cloned := *profile
	cloned.UptimeSeconds = 0
	cloned.ReportedAtUnix = 0
	raw, err := json.Marshal(&cloned)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
