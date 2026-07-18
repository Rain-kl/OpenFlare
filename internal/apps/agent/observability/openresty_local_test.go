package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/config"
)

func TestCollectEdgeHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openflare/observability" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"captured_at_unix":1710403200,"connections":{"active":12,"reading":1,"writing":2,"waiting":9}}`))
	}))
	defer server.Close()

	health := CollectEdgeHealth(context.Background(), &config.Config{
		OpenrestyObservabilityPort: mustPort(server.URL),
	})
	if health == nil {
		t.Fatal("expected edge health")
	}
	if !health.OK || health.Connections != 12 {
		t.Fatalf("unexpected health: %+v", health)
	}
}

func mustPort(rawURL string) int {
	u := rawURL
	idx := stringsLastColon(u)
	if idx < 0 {
		return 0
	}
	var port int
	for _, ch := range u[idx+1:] {
		if ch < '0' || ch > '9' {
			break
		}
		port = port*10 + int(ch-'0')
	}
	return port
}

func stringsLastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}

func TestCollectEdgeHealthHandlesUnavailableEndpoint(t *testing.T) {
	if health := CollectEdgeHealth(context.Background(), &config.Config{
		OpenrestyObservabilityPort: 1,
	}); health != nil {
		t.Fatalf("expected nil health, got %+v", health)
	}
}
