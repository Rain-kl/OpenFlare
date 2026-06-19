package observability

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Rain-kl/Wavelet/internal/agent/config"
	"github.com/Rain-kl/Wavelet/internal/agent/protocol"
	"github.com/Rain-kl/Wavelet/internal/agent/state"
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

	memTotal, memUsed := readMemInfo()
	metric.MemoryTotalBytes = memTotal
	metric.MemoryUsedBytes = memUsed

	storageTotal, storageUsed := statFilesystem(cfg.DataDir)
	metric.StorageTotalBytes = storageTotal
	metric.StorageUsedBytes = storageUsed

	metric.NetworkRxBytes, metric.NetworkTxBytes = readLinuxNetworkTotals()
	metric.DiskReadBytes, metric.DiskWriteBytes = readLinuxDiskTotals()

	if stateStore == nil {
		return metric
	}

	totalCPU, idleCPU := readLinuxCPUStat()
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
	osName, osVersion := readLinuxOSRelease()
	kernelVersion := readFirstLine("/proc/sys/kernel/osrelease")
	cpuModel := readLinuxCPUModel()
	totalMemory, _ := readMemInfo()
	totalDisk, _ := statFilesystem(cfg.DataDir)
	uptimeSeconds := readLinuxUptimeSeconds()

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

func readLinuxOSRelease() (string, string) {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return runtime.GOOS, ""
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[key] = strings.Trim(value, `"`)
	}
	if pretty := strings.TrimSpace(values["PRETTY_NAME"]); pretty != "" {
		return pretty, strings.TrimSpace(values["VERSION_ID"])
	}
	name := strings.TrimSpace(values["NAME"])
	if name == "" {
		name = runtime.GOOS
	}
	return name, strings.TrimSpace(values["VERSION_ID"])
}

func readLinuxCPUModel() string {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.ToLower(line), "model name") {
			_, value, ok := strings.Cut(line, ":")
			if ok {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func readMemInfo() (int64, int64) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	var memTotalKB int64
	var memAvailableKB int64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			memTotalKB = parseMemInfoValue(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			memAvailableKB = parseMemInfoValue(line)
		}
	}

	total := memTotalKB * 1024
	if total == 0 {
		return 0, 0
	}
	used := total - (memAvailableKB * 1024)
	if used < 0 {
		used = 0
	}
	return total, used
}

func parseMemInfoValue(line string) int64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	value, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func readLinuxUptimeSeconds() int64 {
	content, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(content))
	if len(fields) == 0 {
		return 0
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return int64(value)
}

func readLinuxCPUStat() (uint64, uint64) {
	content, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, 0
		}
		var total uint64
		for i := 1; i < len(fields); i++ {
			value, err := strconv.ParseUint(fields[i], 10, 64)
			if err != nil {
				return 0, 0
			}
			total += value
			if i == 4 {
				// idle
			}
		}
		idle, err := strconv.ParseUint(fields[4], 10, 64)
		if err != nil {
			return 0, 0
		}
		return total, idle
	}
	return 0, 0
}

func readLinuxNetworkTotals() (int64, int64) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	var rx int64
	var tx int64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, ":") {
			continue
		}
		name, data, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.TrimSpace(name) == "lo" {
			continue
		}
		fields := strings.Fields(data)
		if len(fields) < 16 {
			continue
		}
		rxValue, err := strconv.ParseInt(fields[0], 10, 64)
		if err == nil {
			rx += rxValue
		}
		txValue, err := strconv.ParseInt(fields[8], 10, 64)
		if err == nil {
			tx += txValue
		}
	}
	return rx, tx
}

func readLinuxDiskTotals() (int64, int64) {
	file, err := os.Open("/proc/diskstats")
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	var readBytes int64
	var writeBytes int64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}
		device := fields[2]
		if shouldSkipDiskDevice(device) {
			continue
		}
		readSectors, err := strconv.ParseInt(fields[5], 10, 64)
		if err == nil {
			readBytes += readSectors * 512
		}
		writeSectors, err := strconv.ParseInt(fields[9], 10, 64)
		if err == nil {
			writeBytes += writeSectors * 512
		}
	}
	return readBytes, writeBytes
}

func shouldSkipDiskDevice(device string) bool {
	switch {
	case device == "":
		return true
	case strings.HasPrefix(device, "loop"),
		strings.HasPrefix(device, "ram"),
		strings.HasPrefix(device, "dm-"):
		return true
	default:
		return false
	}
}

func statFilesystem(path string) (int64, int64) {
	if strings.TrimSpace(path) == "" {
		path = string(os.PathSeparator)
	}
	absPath := filepath.Clean(path)
	var stat syscall.Statfs_t
	if err := syscall.Statfs(absPath, &stat); err != nil {
		return 0, 0
	}
	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bavail) * int64(stat.Bsize)
	used := total - free
	if used < 0 {
		used = 0
	}
	return total, used
}

func readFirstLine(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}
