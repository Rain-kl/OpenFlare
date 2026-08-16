// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package geoip

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// IPInfoService resolves geographic information using the ipinfo.io service.
type IPInfoService struct {
	Client *http.Client
	// 每天 1000 次请求，限制由 IP 地址的所有人共享。
	// APIToken string
}

// ipInfoResponse 定义了 ipinfo.io 服务返回的 JSON 响应的结构，只包含免费额度可用的字段。
type ipInfoResponse struct {
	IP          string `json:"ip"`
	Hostname    string `json:"hostname"`
	City        string `json:"city"`
	Region      string `json:"region"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"` // ipinfo.io 返回 "country" 的 ISO 代码，这里为了与 GeoInfo 保持一致，额外添加一个 CountryCode
	Loc         string `json:"loc"`         // Latitude,Longitude
	Org         string `json:"org"`
	Postal      string `json:"postal"`
	Timezone    string `json:"timezone"`
}

// NewIPInfoService 创建并返回一个 IPInfoService 的新实例。
func NewIPInfoService() (*IPInfoService, error) {
	return &IPInfoService{
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}, nil
}

// Name 返回服务的名称。
func (s *IPInfoService) Name() string {
	return "ipinfo.io"
}

// GetGeoInfo 使用 ipinfo.io 服务检索给定 IP 地址的地理位置信息。
// 免费额度主要提供国家信息。
func (s *IPInfoService) GetGeoInfo(ip net.IP) (*GeoInfo, error) {
	// IPinfo 免费额度不需要 API token 就可以查询基本的 IP 信息。
	// API URL: https://ipinfo.io/json (查询自身IP) 或 https://ipinfo.io/YOUR_IP/json
	apiURL := fmt.Sprintf("https://ipinfo.io/%s/json", ip.String())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for ipinfo.io: %w", err)
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get geo info from ipinfo.io: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ipinfo.io returned non-200 status: %d %s", resp.StatusCode, resp.Status)
	}

	var apiResp ipInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode ipinfo.io response: %w", err)
	}

	latitude, longitude := parseIPInfoCoordinates(apiResp.Loc)

	// IPinfo 的 "country" 字段直接返回 ISO 2-letter code，例如 "US", "CN"
	// 我们需要将 "country" 字段作为 ISOCode，并尝试获取其对应的国家名称。
	// IPinfo 响应中通常不直接提供完整的国家名称，但我们可以通过 CountryCode 映射。
	// 为了简化并符合 GeoInfo 结构，我们直接使用 Country 作为 ISOCode，并尝试从 CountryCode 获取名称。
	// 实际上，IPinfo 的 'country' 字段就是 ISO 2-letter code。
	// 如果需要完整的国家名称，可能需要一个本地的 ISO 代码到名称的映射。
	// 为了与 GetRegionUnicodeEmoji 函数兼容，我们直接使用 country 作为 ISOCode。
	name := formatIPInfoLocation(apiResp)
	return &GeoInfo{
		ISOCode:   apiResp.Country,
		Name:      name,
		Latitude:  latitude,
		Longitude: longitude,
	}, nil
}

// UpdateDatabase 对于 ipinfo.io 是一个空操作，因为它是一个 Web 服务。
func (s *IPInfoService) UpdateDatabase() error {
	// 无需执行任何操作，因为数据由外部服务提供
	return nil
}

// Close 对于 ipinfo.io 是一个空操作，因为没有需要关闭的持久连接。
func (s *IPInfoService) Close() error {
	// 无需执行任何操作
	return nil
}

func parseIPInfoCoordinates(value string) (*float64, *float64) {
	parts := strings.Split(strings.TrimSpace(value), ",")
	if len(parts) != isoCountryCodeLength {
		return nil, nil
	}

	latitudeValue, latErr := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	longitudeValue, lonErr := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if latErr != nil || lonErr != nil {
		return nil, nil
	}

	return float64Pointer(latitudeValue), float64Pointer(longitudeValue)
}

func formatIPInfoLocation(resp ipInfoResponse) string {
	parts := make([]string, 0, 3) //nolint:mnd // 3 parts: city, region, country
	if city := strings.TrimSpace(resp.City); city != "" {
		parts = append(parts, city)
	}
	if region := strings.TrimSpace(resp.Region); region != "" {
		parts = append(parts, region)
	}
	if country := strings.TrimSpace(resp.Country); country != "" {
		parts = append(parts, country)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}
