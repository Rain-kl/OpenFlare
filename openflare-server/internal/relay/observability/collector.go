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

	"github.com/Rain-kl/Wavelet/internal/relay/config"
	"github.com/Rain-kl/Wavelet/internal/relay/frps"
	"github.com/Rain-kl/Wavelet/internal/relay/state"
	service "github.com/Rain-kl/Wavelet/pkg/protocol"
)

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

func BuildSnapshot(cfg *config.Config, stateStore *state.Store) *service.AgentNodeMetricSnapshot {
	now := time.Now().UTC()
	metric := &service.AgentNodeMetricSnapshot{CapturedAtUnix: now.Unix()}

	metric.MemoryTotalBytes, metric.MemoryUsedBytes = readMemInfo()
	metric.StorageTotalBytes, metric.StorageUsedBytes = statFilesystem(cfg.DataDir)
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
			metric.CPUUsagePercent = float64(deltaTotal-deltaIdle) / float64(deltaTotal) * 100
		}
	}
	snapshot.LastCPUStatTotal = totalCPU
	snapshot.LastCPUStatIdle = idleCPU
	snapshot.LastMetricAtUnix = now.Unix()
	_ = stateStore.Save(snapshot)
	return metric
}

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
	osName, osVersion := readLinuxOSRelease()
	totalMemory, _ := readMemInfo()
	totalDisk, _ := statFilesystem(cfg.DataDir)
	return &service.AgentNodeSystemProfile{
		Hostname:         strings.TrimSpace(hostname),
		OSName:           osName,
		OSVersion:        osVersion,
		KernelVersion:    readFirstLine("/proc/sys/kernel/osrelease"),
		Architecture:     runtime.GOARCH,
		CPUModel:         readLinuxCPUModel(),
		CPUCores:         runtime.NumCPU(),
		TotalMemoryBytes: totalMemory,
		TotalDiskBytes:   totalDisk,
		UptimeSeconds:    readLinuxUptimeSeconds(),
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

func readLinuxOSRelease() (string, string) {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return runtime.GOOS, ""
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if !ok {
			continue
		}
		values[key] = strings.Trim(value, `"`)
	}
	if pretty := strings.TrimSpace(values["PRETTY_NAME"]); pretty != "" {
		return pretty, strings.TrimSpace(values["VERSION_ID"])
	}
	if name := strings.TrimSpace(values["NAME"]); name != "" {
		return name, strings.TrimSpace(values["VERSION_ID"])
	}
	return runtime.GOOS, ""
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

	var totalKB, availableKB int64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			totalKB = parseMemInfoValue(line)
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			availableKB = parseMemInfoValue(line)
		}
	}
	total := totalKB * 1024
	used := total - availableKB*1024
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
	for _, line := range strings.Split(string(content), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, 0
		}
		var total uint64
		for index := 1; index < len(fields); index++ {
			value, err := strconv.ParseUint(fields[index], 10, 64)
			if err != nil {
				return 0, 0
			}
			total += value
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

	var rx, tx int64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		name, data, ok := strings.Cut(strings.TrimSpace(scanner.Text()), ":")
		if !ok || strings.TrimSpace(name) == "lo" {
			continue
		}
		fields := strings.Fields(data)
		if len(fields) < 16 {
			continue
		}
		if value, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
			rx += value
		}
		if value, err := strconv.ParseInt(fields[8], 10, 64); err == nil {
			tx += value
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

	var readBytes, writeBytes int64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 || shouldSkipDiskDevice(fields[2]) {
			continue
		}
		if value, err := strconv.ParseInt(fields[5], 10, 64); err == nil {
			readBytes += value * 512
		}
		if value, err := strconv.ParseInt(fields[9], 10, 64); err == nil {
			writeBytes += value * 512
		}
	}
	return readBytes, writeBytes
}

func shouldSkipDiskDevice(device string) bool {
	return device == "" || strings.HasPrefix(device, "loop") || strings.HasPrefix(device, "ram") || strings.HasPrefix(device, "dm-")
}

func statFilesystem(path string) (int64, int64) {
	if strings.TrimSpace(path) == "" {
		path = string(os.PathSeparator)
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(filepath.Clean(path), &stat); err != nil {
		return 0, 0
	}
	total := int64(stat.Blocks) * int64(stat.Bsize)
	used := total - int64(stat.Bavail)*int64(stat.Bsize)
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
