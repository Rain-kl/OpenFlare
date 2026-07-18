package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/config"
)

const openRestyObservabilityPath = "/openflare/observability"

// EdgeHealthSnapshot is the L2 OpenResty health probe result.
type EdgeHealthSnapshot struct {
	OK             bool
	CapturedAtUnix int64
	Connections    int64
	Reading        int64
	Writing        int64
	Waiting        int64
}

type openRestyObservabilityResponse struct {
	OK             bool  `json:"ok"`
	CapturedAtUnix int64 `json:"captured_at_unix"`
	Connections    struct {
		Active  int64 `json:"active"`
		Reading int64 `json:"reading"`
		Writing int64 `json:"writing"`
		Waiting int64 `json:"waiting"`
	} `json:"connections"`
}

// CollectEdgeHealth probes the local OpenResty observability JSON endpoint.
func CollectEdgeHealth(ctx context.Context, cfg *config.Config) *EdgeHealthSnapshot {
	if cfg == nil || cfg.OpenrestyObservabilityPort <= 0 {
		return nil
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.OpenrestyObservabilityPort)
	client := &http.Client{Timeout: 1500 * time.Millisecond}

	var resp openRestyObservabilityResponse
	if err := fetchLocalJSON(ctx, client, baseURL+openRestyObservabilityPath, &resp); err != nil {
		return nil
	}

	captured := resp.CapturedAtUnix
	if captured <= 0 {
		captured = time.Now().UTC().Unix()
	}
	return &EdgeHealthSnapshot{
		OK:             resp.OK,
		CapturedAtUnix: captured,
		Connections:    resp.Connections.Active,
		Reading:        resp.Connections.Reading,
		Writing:        resp.Connections.Writing,
		Waiting:        resp.Connections.Waiting,
	}
}

func fetchLocalJSON(ctx context.Context, client *http.Client, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected local observability status: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}
