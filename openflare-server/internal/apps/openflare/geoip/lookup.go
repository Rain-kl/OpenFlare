// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package geoip provides OpenFlare-compatible GeoIP lookup helpers.
package geoip

import (
	"errors"
	"net"
	"strings"

	pkggeoip "github.com/rain-kl/openflare/pkg/geoip"
)

// LookupView is the legacy OpenFlare GeoIP lookup response shape.
type LookupView struct {
	Provider  string   `json:"provider"`
	IP        string   `json:"ip"`
	ISOCode   string   `json:"iso_code"`
	Name      string   `json:"name"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

const (
	errProviderInvalid = "归属方式仅支持 disabled、mmdb、ip-api、geojs、ipinfo"
	errIPEmpty         = "IP 不能为空"
	errIPInvalid       = "IP 格式无效"
	errLookupEmpty     = "未获取到 IP 归属结果"
)

// IsValidProvider reports whether provider is a supported GeoIP backend.
func IsValidProvider(provider string) bool {
	return pkggeoip.IsValidProvider(provider)
}

// GeoInfoFromIP resolves geographic information using the configured default provider.
func GeoInfoFromIP(ip net.IP) (*pkggeoip.GeoInfo, error) {
	return pkggeoip.GetGeoInfo(ip)
}

// Lookup resolves geographic information for rawIP using the given provider.
func Lookup(provider, rawIP string) (*LookupView, error) {
	trimmedProvider := strings.TrimSpace(provider)
	if !pkggeoip.IsValidProvider(trimmedProvider) {
		return nil, errors.New(errProviderInvalid)
	}

	trimmedIP := strings.TrimSpace(rawIP)
	if trimmedIP == "" {
		return nil, errors.New(errIPEmpty)
	}
	parsedIP := net.ParseIP(trimmedIP)
	if parsedIP == nil {
		return nil, errors.New(errIPInvalid)
	}

	if trimmedProvider == pkggeoip.ProviderDisabled {
		return &LookupView{
			Provider: trimmedProvider,
			IP:       parsedIP.String(),
		}, nil
	}

	info, err := pkggeoip.LookupGeoInfoWithProvider(trimmedProvider, parsedIP)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, errors.New(errLookupEmpty)
	}

	return &LookupView{
		Provider:  trimmedProvider,
		IP:        parsedIP.String(),
		ISOCode:   info.ISOCode,
		Name:      info.Name,
		Latitude:  info.Latitude,
		Longitude: info.Longitude,
	}, nil
}
