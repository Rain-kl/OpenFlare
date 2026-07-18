package observability

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/config"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/state"
)

type accessLogRecord struct {
	Timestamp     string  `json:"ts"`
	Host          string  `json:"host"`
	RemoteAddr    string  `json:"remote_addr"`
	Path          string  `json:"path"`
	UserAgent     string  `json:"user_agent"`
	Status        int     `json:"status"`
	BytesSent     int64   `json:"bytes_sent"`
	RequestLength int64   `json:"request_length"`
	RequestTime   float64 `json:"request_time"`
}

const (
	combinedAccessLogMatchGroupCount = 5
	// requestTimeSecondsToMs converts OpenResty $request_time (seconds float) to ms.
	requestTimeSecondsToMs = 1000.0
	// roundHalfUp is added before int64 truncate to round to nearest millisecond.
	roundHalfUp = 0.5
)

var combinedAccessLogPattern = regexp.MustCompile(`^(\S+)\s+\S+\s+\S+\s+\[([^]]+)]\s+"\S+\s+(\S+)(?:\s+[^"]*)?"\s+(\d{3})\s+\S+`)

// trafficAggregate collects access-log facts for the current heartbeat window.
// Pre-aggregation (UV/TopN/TrafficReport) is intentionally not built.
type trafficAggregate struct {
	logs []protocol.NodeAccessLog
}

// CollectAccessLogs tails access.log and returns L1 fact rows for the current heartbeat.
func CollectAccessLogs(cfg *config.Config, stateStore *state.Store) []protocol.NodeAccessLog {
	if cfg == nil || stateStore == nil {
		return nil
	}
	aggregate := readAccessLogDelta(cfg, stateStore)
	if aggregate == nil {
		return nil
	}
	return aggregate.accessLogs()
}

func readAccessLogDelta(cfg *config.Config, stateStore *state.Store) *trafficAggregate {
	snapshot, err := stateStore.Load()
	if err != nil {
		return nil
	}

	logPath := managedAccessLogPath(cfg)
	file, err := os.Open(logPath) //nolint:gosec // path is the configured managed access log location
	if err != nil {
		if os.IsNotExist(err) {
			if snapshot.AccessLogOffset != 0 {
				snapshot.AccessLogOffset = 0
				_ = stateStore.Save(snapshot)
			}
			return nil
		}
		return nil
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			slog.Error("failed to close access log file", "error", err)
		}
	}(file)

	info, err := file.Stat()
	if err != nil {
		return nil
	}

	offset := snapshot.AccessLogOffset
	if offset < 0 || offset > info.Size() {
		offset = 0
	}
	if _, err = file.Seek(offset, io.SeekStart); err != nil {
		return nil
	}

	reader := bufio.NewReader(file)
	currentOffset := offset
	aggregate := newTrafficAggregate()

	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			currentOffset += int64(len(line))
			aggregate.consume(line)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil
		}
	}

	snapshot.AccessLogOffset = currentOffset
	_ = stateStore.Save(snapshot)

	return aggregate
}

func managedAccessLogPath(cfg *config.Config) string {
	if cfg == nil || strings.TrimSpace(cfg.AccessLogPath) == "" {
		return ""
	}
	return cfg.AccessLogPath
}

func newTrafficAggregate() *trafficAggregate {
	return &trafficAggregate{}
}

func (aggregate *trafficAggregate) consume(line []byte) {
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" {
		return
	}

	record, ok := parseAccessLogRecord(trimmed)
	if !ok {
		return
	}

	aggregate.logs = append(aggregate.logs, protocol.NodeAccessLog{
		LoggedAtUnix:  record.Timestamp.Unix(),
		RemoteAddr:    strings.TrimSpace(record.RemoteAddr),
		Host:          strings.TrimSpace(record.Host),
		Path:          normalizeAccessLogPath(record.Path),
		UserAgent:     strings.TrimSpace(record.UserAgent),
		StatusCode:    record.Status,
		BytesSent:     record.BytesSent,
		RequestLength: record.RequestLength,
		RequestTimeMs: record.RequestTimeMs,
	})
}

type parsedAccessLogRecord struct {
	Timestamp     time.Time
	Host          string
	RemoteAddr    string
	Path          string
	UserAgent     string
	Status        int
	BytesSent     int64
	RequestLength int64
	RequestTimeMs int64
}

func parseAccessLogRecord(raw string) (parsedAccessLogRecord, bool) {
	record, ok := parseJSONAccessLogRecord(raw)
	if ok {
		return record, true
	}
	return parseCombinedAccessLogRecord(raw)
}

func parseJSONAccessLogRecord(raw string) (parsedAccessLogRecord, bool) {
	var record accessLogRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return parsedAccessLogRecord{}, false
	}
	timestamp, err := parseAccessLogTime(record.Timestamp)
	if err != nil {
		return parsedAccessLogRecord{}, false
	}
	requestTimeMs := int64(0)
	if record.RequestTime > 0 {
		requestTimeMs = int64(record.RequestTime*requestTimeSecondsToMs + roundHalfUp)
	}
	return parsedAccessLogRecord{
		Timestamp:     timestamp,
		Host:          strings.TrimSpace(record.Host),
		RemoteAddr:    strings.TrimSpace(record.RemoteAddr),
		Path:          normalizeAccessLogPath(record.Path),
		UserAgent:     strings.TrimSpace(record.UserAgent),
		Status:        record.Status,
		BytesSent:     record.BytesSent,
		RequestLength: record.RequestLength,
		RequestTimeMs: requestTimeMs,
	}, true
}

func parseCombinedAccessLogRecord(raw string) (parsedAccessLogRecord, bool) {
	matches := combinedAccessLogPattern.FindStringSubmatch(raw)
	if len(matches) != combinedAccessLogMatchGroupCount {
		return parsedAccessLogRecord{}, false
	}
	timestamp, err := parseAccessLogTime(matches[2])
	if err != nil {
		return parsedAccessLogRecord{}, false
	}
	status, err := strconv.Atoi(matches[4])
	if err != nil {
		return parsedAccessLogRecord{}, false
	}
	return parsedAccessLogRecord{
		Timestamp:  timestamp,
		RemoteAddr: strings.TrimSpace(matches[1]),
		Path:       normalizeAccessLogPath(matches[3]),
		Status:     status,
	}, true
}

func (aggregate *trafficAggregate) accessLogs() []protocol.NodeAccessLog {
	if aggregate == nil || len(aggregate.logs) == 0 {
		return []protocol.NodeAccessLog{}
	}
	return append([]protocol.NodeAccessLog(nil), aggregate.logs...)
}

func parseAccessLogTime(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, errors.New("empty access log time")
	}
	timestamp, err := time.Parse(time.RFC3339, trimmed)
	if err == nil {
		return timestamp, nil
	}
	return time.Parse("02/Jan/2006:15:04:05 -0700", trimmed)
}

const accessLogPathMaxRunes = 100

func normalizeAccessLogPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return truncateAccessLogPath(trimmed)
	}
	if strings.HasPrefix(trimmed, "/") {
		return truncateAccessLogPath(trimmed)
	}
	return truncateAccessLogPath("/" + trimmed)
}

func truncateAccessLogPath(value string) string {
	runes := []rune(value)
	if len(runes) <= accessLogPathMaxRunes {
		return value
	}
	return string(runes[:accessLogPathMaxRunes])
}
