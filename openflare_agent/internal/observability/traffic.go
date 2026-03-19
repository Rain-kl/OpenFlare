package observability

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"openflare-agent/internal/config"
	"openflare-agent/internal/protocol"
	"openflare-agent/internal/state"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type accessLogRecord struct {
	Timestamp     string `json:"ts"`
	Host          string `json:"host"`
	RemoteAddr    string `json:"remote_addr"`
	Path          string `json:"path"`
	Status        int    `json:"status"`
	BytesSent     int64  `json:"bytes_sent"`
	RequestLength int64  `json:"request_length"`
}

var combinedAccessLogPattern = regexp.MustCompile(`^(\S+)\s+\S+\s+\S+\s+\[([^\]]+)\]\s+"(?:\S+)\s+(\S+)(?:\s+[^"]*)?"\s+(\d{3})\s+\S+`)

type trafficAggregate struct {
	windowStartedAt  time.Time
	windowEndedAt    time.Time
	requestCount     int64
	errorCount       int64
	openrestyRxBytes int64
	openrestyTxBytes int64
	statusCodes      map[string]int64
	topDomains       map[string]int64
	visitors         map[string]struct{}
	logs             []protocol.NodeAccessLog
}

func BuildTrafficReport(cfg *config.Config, stateStore *state.Store, managed *managedOpenRestyMetrics) *protocol.NodeTrafficReport {
	report, _, _ := BuildTrafficObservability(cfg, stateStore, managed)
	return report
}

func BuildTrafficObservability(cfg *config.Config, stateStore *state.Store, managed *managedOpenRestyMetrics) (*protocol.NodeTrafficReport, []protocol.NodeAccessLog, *managedOpenRestyMetrics) {
	if cfg == nil || stateStore == nil {
		if managed != nil && managed.TrafficReport != nil {
			return managed.TrafficReport, nil, managed
		}
		return nil, nil, managed
	}

	aggregate := readAccessLogDelta(cfg, stateStore)
	accessLogs := []protocol.NodeAccessLog{}
	if aggregate != nil {
		accessLogs = aggregate.accessLogs()
	}
	if managed != nil && managed.TrafficReport != nil {
		return managed.TrafficReport, accessLogs, managed
	}
	if aggregate == nil {
		return nil, accessLogs, managed
	}
	fallbackManaged := aggregate.managedMetrics()
	return aggregate.report(), accessLogs, fallbackManaged
}

func readAccessLogDelta(cfg *config.Config, stateStore *state.Store) *trafficAggregate {
	snapshot, err := stateStore.Load()
	if err != nil {
		return nil
	}

	logPath := managedAccessLogPath(cfg)
	file, err := os.Open(logPath)
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
	defer file.Close()

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
	if cfg == nil || strings.TrimSpace(cfg.RouteConfigPath) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(cfg.RouteConfigPath), "openflare_access.log")
}

func newTrafficAggregate() *trafficAggregate {
	return &trafficAggregate{
		statusCodes: make(map[string]int64),
		topDomains:  make(map[string]int64),
		visitors:    make(map[string]struct{}),
	}
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

	if aggregate.windowStartedAt.IsZero() || record.Timestamp.Before(aggregate.windowStartedAt) {
		aggregate.windowStartedAt = record.Timestamp
	}
	if aggregate.windowEndedAt.IsZero() || record.Timestamp.After(aggregate.windowEndedAt) {
		aggregate.windowEndedAt = record.Timestamp
	}

	aggregate.requestCount++
	if record.Status >= 500 {
		aggregate.errorCount++
	}
	if record.Status > 0 {
		aggregate.statusCodes[strconv.Itoa(record.Status)]++
	}
	if record.RequestLength > 0 {
		aggregate.openrestyRxBytes += record.RequestLength
	}
	if record.BytesSent > 0 {
		aggregate.openrestyTxBytes += record.BytesSent
	}
	if host := strings.TrimSpace(record.Host); host != "" {
		aggregate.topDomains[host]++
	}
	if remoteAddr := strings.TrimSpace(record.RemoteAddr); remoteAddr != "" {
		aggregate.visitors[remoteAddr] = struct{}{}
	}
	aggregate.logs = append(aggregate.logs, protocol.NodeAccessLog{
		LoggedAtUnix: record.Timestamp.Unix(),
		RemoteAddr:   strings.TrimSpace(record.RemoteAddr),
		Host:         strings.TrimSpace(record.Host),
		Path:         normalizeAccessLogPath(record.Path),
		StatusCode:   record.Status,
	})
}

type parsedAccessLogRecord struct {
	Timestamp     time.Time
	Host          string
	RemoteAddr    string
	Path          string
	Status        int
	BytesSent     int64
	RequestLength int64
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
	return parsedAccessLogRecord{
		Timestamp:     timestamp,
		Host:          strings.TrimSpace(record.Host),
		RemoteAddr:    strings.TrimSpace(record.RemoteAddr),
		Path:          normalizeAccessLogPath(record.Path),
		Status:        record.Status,
		BytesSent:     record.BytesSent,
		RequestLength: record.RequestLength,
	}, true
}

func parseCombinedAccessLogRecord(raw string) (parsedAccessLogRecord, bool) {
	matches := combinedAccessLogPattern.FindStringSubmatch(raw)
	if len(matches) != 5 {
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

func (aggregate *trafficAggregate) report() *protocol.NodeTrafficReport {
	if aggregate.requestCount == 0 || aggregate.windowStartedAt.IsZero() || aggregate.windowEndedAt.IsZero() {
		return nil
	}

	return &protocol.NodeTrafficReport{
		WindowStartedAtUnix: aggregate.windowStartedAt.Unix(),
		WindowEndedAtUnix:   aggregate.windowEndedAt.Unix(),
		RequestCount:        aggregate.requestCount,
		ErrorCount:          aggregate.errorCount,
		UniqueVisitorCount:  int64(len(aggregate.visitors)),
		StatusCodes:         cloneTrafficCounts(aggregate.statusCodes, 0),
		TopDomains:          topCounts(aggregate.topDomains, 8),
		SourceCountries:     map[string]int64{},
	}
}

func (aggregate *trafficAggregate) accessLogs() []protocol.NodeAccessLog {
	if aggregate == nil || len(aggregate.logs) == 0 {
		return []protocol.NodeAccessLog{}
	}
	return append([]protocol.NodeAccessLog(nil), aggregate.logs...)
}

func (aggregate *trafficAggregate) managedMetrics() *managedOpenRestyMetrics {
	if aggregate == nil {
		return nil
	}
	report := aggregate.report()
	if report == nil && aggregate.openrestyRxBytes <= 0 && aggregate.openrestyTxBytes <= 0 {
		return nil
	}
	return &managedOpenRestyMetrics{
		TrafficReport:    report,
		OpenrestyRxBytes: aggregate.openrestyRxBytes,
		OpenrestyTxBytes: aggregate.openrestyTxBytes,
	}
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

func cloneTrafficCounts(values map[string]int64, limit int) map[string]int64 {
	if len(values) == 0 {
		return map[string]int64{}
	}
	items := make([]trafficCountItem, 0, len(values))
	for key, value := range values {
		items = append(items, trafficCountItem{key: key, value: value})
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].value == items[j].value {
			return items[i].key < items[j].key
		}
		return items[i].value > items[j].value
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	result := make(map[string]int64, len(items))
	for _, item := range items {
		result[item.key] = item.value
	}
	return result
}

type trafficCountItem struct {
	key   string
	value int64
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

func topCounts(values map[string]int64, limit int) map[string]int64 {
	return cloneTrafficCounts(values, limit)
}
