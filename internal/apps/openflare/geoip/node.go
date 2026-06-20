// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package geoip

import (
	"context"
	"net"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	pkggeoip "github.com/Rain-kl/Wavelet/pkg/geoip"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

// ApplyNodeGeoFromIP resolves geographic metadata for node when geo is not manually locked.
func ApplyNodeGeoFromIP(ctx context.Context, node *model.OpenFlareNode, rawIP string) {
	if node == nil || node.GeoManualOverride {
		return
	}
	node.GeoName = ""
	node.GeoLatitude = nil
	node.GeoLongitude = nil

	ip := net.ParseIP(strings.TrimSpace(rawIP))
	if ip == nil {
		return
	}

	info, err := GeoInfoFromIP(ip)
	if err != nil {
		logger.WarnF(ctx, "[GeoIP] resolve node geo failed: ip=%s error=%v", ip.String(), err)
		return
	}
	if info == nil {
		return
	}
	if strings.TrimSpace(info.Name) != "" {
		node.GeoName = strings.TrimSpace(info.Name)
	}
	if info.Latitude != nil && info.Longitude != nil {
		node.GeoLatitude = cloneCoordinate(info.Latitude)
		node.GeoLongitude = cloneCoordinate(info.Longitude)
		return
	}
	applyCountryCentroid(node, info.ISOCode, info.Name)
}

func applyCountryCentroid(node *model.OpenFlareNode, isoCode, name string) {
	if lat, lon, ok := pkggeoip.CountryCentroidByISO(isoCode); ok {
		node.GeoLatitude = cloneFloat(lat)
		node.GeoLongitude = cloneFloat(lon)
		return
	}
	if lat, lon, ok := pkggeoip.CountryCentroidByName(name); ok {
		node.GeoLatitude = cloneFloat(lat)
		node.GeoLongitude = cloneFloat(lon)
	}
}

func cloneFloat(value float64) *float64 {
	cloned := value
	return &cloned
}

func cloneCoordinate(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
