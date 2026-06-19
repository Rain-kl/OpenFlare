package observability

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/agent/config"
)

func TestCollectManagedOpenRestyMetrics(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc(openRestyObservabilityPath, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"window_started_at_unix":1710403200,"window_ended_at_unix":1710403210,"request_count":12,"error_count":2,"unique_visitor_count":5,"status_codes":{"200":10,"502":2},"top_domains":{"app.example.com":9,"api.example.com":3},"source_countries":{},"openresty_rx_bytes":4096,"openresty_tx_bytes":8192}`))
	})
	mux.HandleFunc(openRestyStubStatusPath, func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("Active connections: 7 \nserver accepts handled requests\n 10 10 12 \nReading: 1 Writing: 2 Waiting: 4 \n"))
	})

	server := httptest.NewUnstartedServer(mux)
	server.Listener = listener
	server.Start()
	defer server.Close()

	metrics := CollectManagedOpenRestyMetrics(&config.Config{
		OpenrestyObservabilityPort: port,
	})
	if metrics == nil || metrics.TrafficReport == nil {
		t.Fatalf("expected managed openresty metrics, got %+v", metrics)
	}
	if metrics.TrafficReport.RequestCount != 12 || metrics.TrafficReport.ErrorCount != 2 {
		t.Fatalf("unexpected traffic report: %+v", metrics.TrafficReport)
	}
	if metrics.OpenrestyRxBytes != 4096 || metrics.OpenrestyTxBytes != 8192 {
		t.Fatalf("unexpected openresty byte counters: %+v", metrics)
	}
	if metrics.OpenrestyConnections != 7 {
		t.Fatalf("unexpected openresty connections: %+v", metrics)
	}
}

func TestParseStubStatusActiveConnections(t *testing.T) {
	if value := parseStubStatusActiveConnections("Active connections: 19\n"); value != 19 {
		t.Fatalf("unexpected active connections: %d", value)
	}
}

func TestNormalizeCountMapDropsEmptyKeys(t *testing.T) {
	normalized := normalizeCountMap(map[string]int64{
		"":                4,
		" 200 ":           3,
		"app.example.com": 0,
	})
	if len(normalized) != 1 || normalized["200"] != 3 {
		t.Fatalf("unexpected normalized map: %+v", normalized)
	}
}

func TestCollectManagedOpenRestyMetricsHandlesUnavailableEndpoint(t *testing.T) {
	cfg := &config.Config{OpenrestyObservabilityPort: 1}
	if metrics := CollectManagedOpenRestyMetrics(cfg); metrics != nil {
		t.Fatalf("expected nil metrics for unavailable endpoint, got %+v", metrics)
	}
}

func TestOpenRestyObservabilityPathsAreStable(t *testing.T) {
	if !strings.HasPrefix(openRestyObservabilityPath, "/openflare/") {
		t.Fatalf("unexpected observability path: %s", openRestyObservabilityPath)
	}
	if !strings.HasPrefix(openRestyStubStatusPath, "/openflare/") {
		t.Fatalf("unexpected stub status path: %s", openRestyStubStatusPath)
	}
}
