package geoip

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeOutboundIPStrategy struct {
	name string
	ip   net.IP
	err  error
}

func (f fakeOutboundIPStrategy) Name() string {
	return f.name
}

func (f fakeOutboundIPStrategy) GetOutboundIP(ctx context.Context) (net.IP, error) {
	return f.ip, f.err
}

func TestRealIPCCAdapterDecodeIP(t *testing.T) {
	ip, err := RealIPCCAdapter{}.DecodeIP(strings.NewReader(`{"ip":"8.8.8.8","country":"United States"}`))
	if err != nil {
		t.Fatalf("DecodeIP failed: %v", err)
	}
	if ip.String() != "8.8.8.8" {
		t.Fatalf("unexpected IP: %s", ip.String())
	}
}

func TestHTTPOutboundIPStrategyUsesAdapter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ip":"8.8.4.4"}`))
	}))
	defer server.Close()

	strategy := NewHTTPOutboundIPStrategy(RealIPCCAdapter{URL: server.URL}, server.Client())
	ip, err := strategy.GetOutboundIP(context.Background())
	if err != nil {
		t.Fatalf("GetOutboundIP failed: %v", err)
	}
	if ip.String() != "8.8.4.4" {
		t.Fatalf("unexpected outbound IP: %s", ip.String())
	}
}

func TestGetOutboundIPFallsBackToNextStrategy(t *testing.T) {
	ip, err := GetOutboundIP(
		context.Background(),
		fakeOutboundIPStrategy{name: "first", err: errors.New("temporary failure")},
		fakeOutboundIPStrategy{name: "second", ip: net.ParseIP("1.1.1.1")},
	)
	if err != nil {
		t.Fatalf("GetOutboundIP failed: %v", err)
	}
	if ip.String() != "1.1.1.1" {
		t.Fatalf("unexpected outbound IP: %s", ip.String())
	}
}

func TestHTTPOutboundIPStrategyRejectsPrivateIP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ip":"172.17.0.2"}`))
	}))
	defer server.Close()

	strategy := NewHTTPOutboundIPStrategy(RealIPCCAdapter{URL: server.URL}, server.Client())
	if _, err := strategy.GetOutboundIP(context.Background()); err == nil {
		t.Fatal("expected private IP to be rejected")
	}
}
