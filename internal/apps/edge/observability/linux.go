package observability

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// ReadLinuxOSRelease returns the OS name and version from /etc/os-release.
func ReadLinuxOSRelease() (string, string) {
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

// ReadLinuxCPUModel returns the CPU model name from /proc/cpuinfo.
func ReadLinuxCPUModel() string {
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

// ReadMemInfo returns total and used memory bytes from /proc/meminfo.
func ReadMemInfo() (int64, int64) {
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

// ReadLinuxUptimeSeconds returns system uptime in seconds from /proc/uptime.
func ReadLinuxUptimeSeconds() int64 {
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

// ReadLinuxCPUStat returns aggregate CPU jiffies and idle jiffies from /proc/stat.
func ReadLinuxCPUStat() (uint64, uint64) {
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
		}
		idle, err := strconv.ParseUint(fields[4], 10, 64)
		if err != nil {
			return 0, 0
		}
		return total, idle
	}
	return 0, 0
}

// ReadLinuxNetworkTotals returns aggregate RX and TX byte totals from /proc/net/dev.
func ReadLinuxNetworkTotals() (int64, int64) {
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

// ReadLinuxDiskTotals returns aggregate disk read and write byte totals from /proc/diskstats.
func ReadLinuxDiskTotals() (int64, int64) {
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

// StatFilesystem returns total and used bytes for the filesystem containing path.
func StatFilesystem(path string) (int64, int64) {
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

// ReadFirstLine reads and returns the trimmed first line of a file.
func ReadFirstLine(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}