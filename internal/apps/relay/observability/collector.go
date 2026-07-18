// Package observability collects relay node profile data for heartbeat reporting.
package observability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"time"

	edgeobs "github.com/Rain-kl/Wavelet/internal/apps/edge/observability"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/config"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/frps"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/state"
	service "github.com/Rain-kl/Wavelet/pkg/protocol"
)

// BuildProfile collects the node's system profile and returns it only when the
// fingerprint has changed since the last heartbeat, avoiding redundant uploads.
func BuildProfile(cfg *config.Config, stateStore *state.Store) *service.AgentNodeSystemProfile {
	profile := collectProfile(cfg)
	if profile == nil || stateStore == nil {
		return profile
	}
	fingerprint := fingerprintProfile(profile)
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

// BuildSnapshot captures a point-in-time metric snapshot including memory,
// disk, network I/O, and CPU usage computed from a delta against the last
// persisted CPU stat.
func BuildSnapshot(cfg *config.Config, stateStore *state.Store) *service.AgentNodeMetricSnapshot {
	now := time.Now().UTC()
	metric := &service.AgentNodeMetricSnapshot{CapturedAtUnix: now.Unix()}

	metric.MemoryTotalBytes, metric.MemoryUsedBytes = edgeobs.ReadMemInfo()
	metric.StorageTotalBytes, metric.StorageUsedBytes = edgeobs.StatFilesystem(cfg.DataDir)
	// Host NIC totals are not collected.
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
			metric.CPUUsagePercent = float64(deltaTotal-deltaIdle) / float64(deltaTotal) * 100
		}
	}
	snapshot.LastCPUStatTotal = totalCPU
	snapshot.LastCPUStatIdle = idleCPU
	snapshot.LastMetricAtUnix = now.Unix()
	_ = stateStore.Save(snapshot)
	return metric
}

// BuildHealthEvents converts a RuntimeStatus into a list of health events.
// An empty slice is returned when frps is healthy.
func BuildHealthEvents(status frps.RuntimeStatus) []service.AgentNodeHealthEvent {
	if strings.TrimSpace(status.Status) == "healthy" {
		return []service.AgentNodeHealthEvent{}
	}
	message := strings.TrimSpace(status.LastError)
	if message == "" {
		message = "frps runtime is not healthy"
	}
	return []service.AgentNodeHealthEvent{{
		EventType:       "frps_unhealthy",
		Severity:        "critical",
		Message:         message,
		TriggeredAtUnix: time.Now().UTC().Unix(),
	}}
}

func collectProfile(cfg *config.Config) *service.AgentNodeSystemProfile {
	hostname, _ := os.Hostname()
	osName, osVersion := edgeobs.ReadLinuxOSRelease()
	totalMemory, _ := edgeobs.ReadMemInfo()
	totalDisk, _ := edgeobs.StatFilesystem(cfg.DataDir)
	return &service.AgentNodeSystemProfile{
		Hostname:         strings.TrimSpace(hostname),
		OSName:           osName,
		OSVersion:        osVersion,
		KernelVersion:    edgeobs.ReadFirstLine("/proc/sys/kernel/osrelease"),
		Architecture:     runtime.GOARCH,
		CPUModel:         edgeobs.ReadLinuxCPUModel(),
		CPUCores:         runtime.NumCPU(),
		TotalMemoryBytes: totalMemory,
		TotalDiskBytes:   totalDisk,
		UptimeSeconds:    edgeobs.ReadLinuxUptimeSeconds(),
		ReportedAtUnix:   time.Now().UTC().Unix(),
	}
}

func fingerprintProfile(profile *service.AgentNodeSystemProfile) string {
	raw, err := json.Marshal(profile)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
