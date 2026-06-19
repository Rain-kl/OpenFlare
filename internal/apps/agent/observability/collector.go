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

	metric.NetworkRxBytes, metric.NetworkTxBytes = edgeobs.ReadLinuxNetworkTotals()
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

func BuildOpenrestyObservation(managed *ManagedOpenRestyMetrics) *protocol.NodeOpenrestyObservation {
	if managed == nil {
		return nil
	}
	return &protocol.NodeOpenrestyObservation{
		CapturedAtUnix:       time.Now().UTC().Unix(),
		OpenrestyRxBytes:     managed.OpenrestyRxBytes,
		OpenrestyTxBytes:     managed.OpenrestyTxBytes,
		OpenrestyConnections: managed.OpenrestyConnections,
	}
}

func BuildHealthEvents(snapshot *state.Snapshot) []protocol.NodeHealthEvent {
	if snapshot == nil {
		return []protocol.NodeHealthEvent{}
	}
	events := make([]protocol.NodeHealthEvent, 0, 2)
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
	raw, err := json.Marshal(profile)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}