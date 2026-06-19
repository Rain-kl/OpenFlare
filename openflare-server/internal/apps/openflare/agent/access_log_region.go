// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"log/slog"
	"net"
	"strings"

	pkggeoip "github.com/rain-kl/openflare/pkg/geoip"
)

var accessLogGeoProviderFactory = func() (pkggeoip.GeoIPService, error) {
	return pkggeoip.NewMaxMindGeoIPService()
}

type accessLogRegionResolver struct {
	provider pkggeoip.GeoIPService
	cache    map[string]string
}

func newAccessLogRegionResolver() (*accessLogRegionResolver, error) {
	provider, err := accessLogGeoProviderFactory()
	if err != nil {
		return nil, err
	}
	return &accessLogRegionResolver{
		provider: provider,
		cache:    make(map[string]string),
	}, nil
}

func (r *accessLogRegionResolver) Close() {
	if r == nil || r.provider == nil {
		return
	}
	if err := r.provider.Close(); err != nil {
		slog.Warn("close access log geo provider failed", "error", err)
	}
}

func (r *accessLogRegionResolver) Resolve(rawIP string) string {
	if r == nil || r.provider == nil {
		return ""
	}
	normalizedIP := normalizeAccessLogIP(rawIP)
	if normalizedIP == "" {
		return ""
	}
	if cached, ok := r.cache[normalizedIP]; ok {
		return cached
	}

	info, err := r.provider.GetGeoInfo(net.ParseIP(normalizedIP))
	if err != nil || info == nil {
		r.cache[normalizedIP] = ""
		return ""
	}

	region := strings.TrimSpace(info.Name)
	if region == "" {
		region = strings.TrimSpace(info.ISOCode)
	}
	r.cache[normalizedIP] = region
	return region
}

func normalizeAccessLogIP(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	if ip := net.ParseIP(trimmed); ip != nil {
		return ip.String()
	}

	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimSuffix(trimmed, "]")
	if ip := net.ParseIP(trimmed); ip != nil {
		return ip.String()
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return ""
}
