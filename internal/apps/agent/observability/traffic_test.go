package observability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/config"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/state"
)

func TestCollectAccessLogsReturnsFactsOnly(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "openflare_access.log")
	content := []byte(
		"{\"ts\":\"2026-03-14T08:00:00Z\",\"host\":\"app.example.com\",\"path\":\"/login\",\"remote_addr\":\"10.0.0.1\",\"status\":200,\"request_length\":128,\"bytes_sent\":512,\"request_time\":0.015,\"user_agent\":\"Mozilla/5.0\",\"cache_status\":\"HIT\"}\n" +
			"{\"ts\":\"2026-03-14T08:00:05Z\",\"host\":\"api.example.com\",\"path\":\"/v1/ping\",\"remote_addr\":\"10.0.0.2\",\"status\":502,\"request_length\":64,\"bytes_sent\":256,\"request_time\":0.008,\"user_agent\":\"curl/8.0\",\"cache_status\":\"MISS\"}\n",
	)
	if err := os.WriteFile(logPath, content, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	stateStore := state.NewStore(filepath.Join(tempDir, "state.json"))
	accessLogs := CollectAccessLogs(&config.Config{AccessLogPath: logPath}, stateStore)
	if len(accessLogs) != 2 {
		t.Fatalf("expected access logs, got %+v", accessLogs)
	}
	if accessLogs[0].BytesSent != 512 || accessLogs[0].RequestLength != 128 {
		t.Fatalf("unexpected first log: %+v", accessLogs[0])
	}
	if accessLogs[0].RequestTimeMs != 15 {
		t.Fatalf("request_time_ms = %d, want 15", accessLogs[0].RequestTimeMs)
	}
	if accessLogs[0].Path != "/login" || accessLogs[1].Path != "/v1/ping" {
		t.Fatalf("unexpected access log paths: %+v", accessLogs)
	}
	if accessLogs[0].UserAgent != "Mozilla/5.0" || accessLogs[1].UserAgent != "curl/8.0" {
		t.Fatalf("unexpected user agents: %+v", accessLogs)
	}
	if accessLogs[0].CacheStatus != "HIT" || accessLogs[1].CacheStatus != "MISS" {
		t.Fatalf("unexpected cache status: %+v", accessLogs)
	}

	snapshot, err := stateStore.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if snapshot.AccessLogOffset != int64(len(content)) {
		t.Fatalf("unexpected access log offset: %d", snapshot.AccessLogOffset)
	}

	moreLogs := CollectAccessLogs(&config.Config{AccessLogPath: logPath}, stateStore)
	if len(moreLogs) != 0 {
		t.Fatalf("expected no new logs, got %+v", moreLogs)
	}
}

func TestCollectAccessLogsResetsOffsetAfterTruncate(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "openflare_access.log")
	if err := os.WriteFile(logPath, []byte("{\"ts\":\"2026-03-14T09:00:00Z\",\"host\":\"app.example.com\",\"path\":\"/\",\"remote_addr\":\"10.0.0.3\",\"status\":200,\"bytes_sent\":1}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	stateStore := state.NewStore(filepath.Join(tempDir, "state.json"))
	if err := stateStore.Save(&state.Snapshot{AccessLogOffset: 4096}); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	accessLogs := CollectAccessLogs(&config.Config{AccessLogPath: logPath}, stateStore)
	if len(accessLogs) != 1 {
		t.Fatalf("expected one access log after truncate reset, got %+v", accessLogs)
	}
}

func TestCollectAccessLogsTruncatesLongAccessLogPath(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "openflare_access.log")
	longPath := "/" + strings.Repeat("a", 140)
	content := []byte(
		"{\"ts\":\"2026-03-14T08:00:00Z\",\"host\":\"app.example.com\",\"path\":\"" + longPath + "\",\"remote_addr\":\"10.0.0.1\",\"status\":200}\n",
	)
	if err := os.WriteFile(logPath, content, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	stateStore := state.NewStore(filepath.Join(tempDir, "state.json"))
	accessLogs := CollectAccessLogs(&config.Config{AccessLogPath: logPath}, stateStore)
	if len(accessLogs) != 1 {
		t.Fatalf("expected one access log, got %+v", accessLogs)
	}
	if got := len([]rune(accessLogs[0].Path)); got != accessLogPathMaxRunes {
		t.Fatalf("expected truncated path length %d, got %d (%q)", accessLogPathMaxRunes, got, accessLogs[0].Path)
	}
}
