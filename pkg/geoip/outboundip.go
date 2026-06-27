package geoip

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/pkg/geoip/iputil"
)

const defaultOutboundIPLookupTimeout = 5 * time.Second

// OutboundIPStrategy defines a lookup strategy for the current public egress IP.
type OutboundIPStrategy interface {
	Name() string
	GetOutboundIP(ctx context.Context) (net.IP, error)
}

// OutboundIPAPIAdapter adapts a third-party HTTP API response into an IP value.
type OutboundIPAPIAdapter interface {
	Name() string
	Endpoint() string
	DecodeIP(io.Reader) (net.IP, error)
}

// HTTPOutboundIPStrategy resolves the public egress IP via an HTTP API adapter.
type HTTPOutboundIPStrategy struct {
	Client  *http.Client
	Adapter OutboundIPAPIAdapter
}

// NewHTTPOutboundIPStrategy creates a strategy that queries adapter over HTTP.
func NewHTTPOutboundIPStrategy(adapter OutboundIPAPIAdapter, client *http.Client) *HTTPOutboundIPStrategy {
	if client == nil {
		client = &http.Client{Timeout: defaultOutboundIPLookupTimeout}
	}
	return &HTTPOutboundIPStrategy{
		Client:  client,
		Adapter: adapter,
	}
}

// Name returns the strategy or adapter identifier.
func (s *HTTPOutboundIPStrategy) Name() string {
	if s == nil || s.Adapter == nil {
		return "http-outbound-ip"
	}
	return s.Adapter.Name()
}

// GetOutboundIP queries the configured HTTP endpoint for the current public IP.
func (s *HTTPOutboundIPStrategy) GetOutboundIP(ctx context.Context) (net.IP, error) {
	if s == nil || s.Adapter == nil {
		return nil, errors.New("outbound IP adapter is nil")
	}
	if ctx == nil {
		return nil, errors.New("context is required")
	}

	if s.Client != nil {
		return s.query(ctx, s.Client)
	}

	dialer := &net.Dialer{
		Timeout:   defaultOutboundIPLookupTimeout,
		KeepAlive: 30 * time.Second,
	}

	// Try IPv4 first
	ipv4Client := &http.Client{
		Timeout: defaultOutboundIPLookupTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, "tcp4", addr)
			},
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	ip, err := s.query(ctx, ipv4Client)
	if err == nil && ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4, nil
		}
	}

	// Fallback to standard client (dual-stack: tcp)
	fallbackClient := &http.Client{
		Timeout: defaultOutboundIPLookupTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, "tcp", addr)
			},
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	ip, err = s.query(ctx, fallbackClient)
	if err != nil {
		return nil, err
	}
	// Always prioritize IPv4 for relay compatibility
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4, nil
	}
	// Return IPv6 only if no IPv4 is available
	return ip, nil
}

func (s *HTTPOutboundIPStrategy) query(ctx context.Context, client *http.Client) (net.IP, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.Adapter.Endpoint(), nil)
	if err != nil {
		return nil, fmt.Errorf("%s create request failed: %w", s.Name(), err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %w", s.Name(), err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned non-200 status: %d %s", s.Name(), response.StatusCode, response.Status)
	}
	ip, err := s.Adapter.DecodeIP(response.Body)
	if err != nil {
		return nil, fmt.Errorf("%s decode response failed: %w", s.Name(), err)
	}
	if !iputil.IsPublic(ip) {
		return nil, fmt.Errorf("%s returned non-public IP: %s", s.Name(), ip.String())
	}
	return ip, nil
}

// RealIPCCAdapter decodes public IP responses from realip.cc.
type RealIPCCAdapter struct {
	URL string
}

type realIPCCResponse struct {
	IP string `json:"ip"`
}

// NewRealIPCCOutboundIPStrategy creates the default realip.cc lookup strategy.
func NewRealIPCCOutboundIPStrategy() *HTTPOutboundIPStrategy {
	return NewHTTPOutboundIPStrategy(RealIPCCAdapter{}, nil)
}

// Name returns the realip.cc adapter identifier.
func (a RealIPCCAdapter) Name() string {
	return "realip.cc"
}

// Endpoint returns the realip.cc API URL.
func (a RealIPCCAdapter) Endpoint() string {
	if strings.TrimSpace(a.URL) != "" {
		return strings.TrimSpace(a.URL)
	}
	return "https://realip.cc"
}

// DecodeIP parses a realip.cc JSON response into a public IP address.
func (a RealIPCCAdapter) DecodeIP(reader io.Reader) (net.IP, error) {
	var payload realIPCCResponse
	if err := json.NewDecoder(reader).Decode(&payload); err != nil {
		return nil, err
	}
	ip := net.ParseIP(strings.TrimSpace(payload.IP))
	if ip == nil {
		return nil, fmt.Errorf("invalid IP %q", payload.IP)
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4, nil
	}
	return ip, nil
}

// PlainTextIPAdapter decodes public IP responses from raw plain text endpoints.
type PlainTextIPAdapter struct {
	ProviderName string
	URL          string
}

// Name returns the provider name.
func (a PlainTextIPAdapter) Name() string {
	return a.ProviderName
}

// Endpoint returns the plain text API URL.
func (a PlainTextIPAdapter) Endpoint() string {
	return a.URL
}

// DecodeIP parses a plain text response into a public IP address.
func (a PlainTextIPAdapter) DecodeIP(reader io.Reader) (net.IP, error) {
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	ipStr := strings.TrimSpace(string(body))
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid plain text IP %q", ipStr)
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4, nil
	}
	return ip, nil
}

// DefaultOutboundIPStrategies returns the built-in public egress IP lookup strategies.
func DefaultOutboundIPStrategies() []OutboundIPStrategy {
	return []OutboundIPStrategy{
		NewRealIPCCOutboundIPStrategy(),
		NewHTTPOutboundIPStrategy(PlainTextIPAdapter{ProviderName: "ifconfig.me", URL: "https://ifconfig.me"}, nil),
		NewHTTPOutboundIPStrategy(PlainTextIPAdapter{ProviderName: "ip.sb", URL: "https://api.ip.sb/ip"}, nil),
		NewHTTPOutboundIPStrategy(PlainTextIPAdapter{ProviderName: "icanhazip.com", URL: "https://icanhazip.com"}, nil),
	}
}

// GetOutboundIP tries each strategy until one returns a public egress IP.
func GetOutboundIP(ctx context.Context, strategies ...OutboundIPStrategy) (net.IP, error) {
	if len(strategies) == 0 {
		strategies = DefaultOutboundIPStrategies()
	}
	var errs []error
	for _, strategy := range strategies {
		if strategy == nil {
			continue
		}
		ip, err := strategy.GetOutboundIP(ctx)
		if err == nil && ip != nil {
			return ip, nil
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", strategy.Name(), err))
		}
	}
	if len(errs) == 0 {
		return nil, errors.New("no outbound IP lookup strategy configured")
	}
	return nil, errors.Join(errs...)
}
