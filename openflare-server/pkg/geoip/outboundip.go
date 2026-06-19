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

type HTTPOutboundIPStrategy struct {
	Client  *http.Client
	Adapter OutboundIPAPIAdapter
}

func NewHTTPOutboundIPStrategy(adapter OutboundIPAPIAdapter, client *http.Client) *HTTPOutboundIPStrategy {
	if client == nil {
		client = &http.Client{Timeout: defaultOutboundIPLookupTimeout}
	}
	return &HTTPOutboundIPStrategy{
		Client:  client,
		Adapter: adapter,
	}
}

func (s *HTTPOutboundIPStrategy) Name() string {
	if s == nil || s.Adapter == nil {
		return "http-outbound-ip"
	}
	return s.Adapter.Name()
}

func (s *HTTPOutboundIPStrategy) GetOutboundIP(ctx context.Context) (net.IP, error) {
	if s == nil || s.Adapter == nil {
		return nil, errors.New("outbound IP adapter is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: defaultOutboundIPLookupTimeout}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.Adapter.Endpoint(), nil)
	if err != nil {
		return nil, fmt.Errorf("%s create request failed: %w", s.Name(), err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %w", s.Name(), err)
	}
	defer response.Body.Close()
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

type RealIPCCAdapter struct {
	URL string
}

type realIPCCResponse struct {
	IP string `json:"ip"`
}

func NewRealIPCCOutboundIPStrategy() *HTTPOutboundIPStrategy {
	return NewHTTPOutboundIPStrategy(RealIPCCAdapter{}, nil)
}

func (a RealIPCCAdapter) Name() string {
	return "realip.cc"
}

func (a RealIPCCAdapter) Endpoint() string {
	if strings.TrimSpace(a.URL) != "" {
		return strings.TrimSpace(a.URL)
	}
	return "https://realip.cc"
}

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

func DefaultOutboundIPStrategies() []OutboundIPStrategy {
	return []OutboundIPStrategy{
		NewRealIPCCOutboundIPStrategy(),
	}
}

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
