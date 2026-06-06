package observability

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rain-kl/openflare/openflare-agent/internal/config"
	"github.com/rain-kl/openflare/openflare-agent/internal/protocol"
)

const openRestyObservabilityPath = "/openflare/observability"
const openRestyStubStatusPath = "/openflare/stub_status"

var stubStatusActivePattern = regexp.MustCompile(`Active connections:\s+(\d+)`)

type ManagedOpenRestyMetrics struct {
	TrafficReport        *protocol.NodeTrafficReport
	OpenrestyRxBytes     int64
	OpenrestyTxBytes     int64
	OpenrestyConnections int64
}

type openRestyObservabilityResponse struct {
	WindowStartedAtUnix int64            `json:"window_started_at_unix"`
	WindowEndedAtUnix   int64            `json:"window_ended_at_unix"`
	RequestCount        int64            `json:"request_count"`
	ErrorCount          int64            `json:"error_count"`
	UniqueVisitorCount  int64            `json:"unique_visitor_count"`
	StatusCodes         map[string]int64 `json:"status_codes"`
	TopDomains          map[string]int64 `json:"top_domains"`
	SourceCountries     map[string]int64 `json:"source_countries"`
	OpenrestyRxBytes    int64            `json:"openresty_rx_bytes"`
	OpenrestyTxBytes    int64            `json:"openresty_tx_bytes"`
}

func CollectManagedOpenRestyMetrics(cfg *config.Config) *ManagedOpenRestyMetrics {
	if cfg == nil || cfg.OpenrestyObservabilityPort <= 0 {
		return nil
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.OpenrestyObservabilityPort)
	client := &http.Client{Timeout: 1500 * time.Millisecond}

	observabilityResp := openRestyObservabilityResponse{}
	if err := fetchLocalJSON(client, baseURL+openRestyObservabilityPath, &observabilityResp); err != nil {
		return nil
	}

	result := &ManagedOpenRestyMetrics{
		TrafficReport: &protocol.NodeTrafficReport{
			WindowStartedAtUnix: observabilityResp.WindowStartedAtUnix,
			WindowEndedAtUnix:   observabilityResp.WindowEndedAtUnix,
			RequestCount:        observabilityResp.RequestCount,
			ErrorCount:          observabilityResp.ErrorCount,
			UniqueVisitorCount:  observabilityResp.UniqueVisitorCount,
			StatusCodes:         normalizeCountMap(observabilityResp.StatusCodes),
			TopDomains:          normalizeCountMap(observabilityResp.TopDomains),
			SourceCountries:     normalizeCountMap(observabilityResp.SourceCountries),
		},
		OpenrestyRxBytes: observabilityResp.OpenrestyRxBytes,
		OpenrestyTxBytes: observabilityResp.OpenrestyTxBytes,
	}

	if text, err := fetchLocalText(client, baseURL+openRestyStubStatusPath); err == nil {
		result.OpenrestyConnections = parseStubStatusActiveConnections(text)
	}

	return result
}

func fetchLocalJSON(client *http.Client, url string, target any) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected local observability status: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func fetchLocalText(client *http.Client, url string) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected local stub status: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func parseStubStatusActiveConnections(raw string) int64 {
	matches := stubStatusActivePattern.FindStringSubmatch(raw)
	if len(matches) != 2 {
		return 0
	}
	value, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func normalizeCountMap(values map[string]int64) map[string]int64 {
	if len(values) == 0 {
		return map[string]int64{}
	}
	result := make(map[string]int64, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" || value <= 0 {
			continue
		}
		result[key] = value
	}
	return result
}
