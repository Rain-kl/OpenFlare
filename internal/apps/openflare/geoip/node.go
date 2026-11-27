// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package geoip

import (
	"context"
	"net"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	"github.com/Rain-kl/Wavelet/internal/model"
	pkggeoip "github.com/Rain-kl/Wavelet/pkg/geoip"
	"github.com/Rain-kl/Wavelet/pkg/geoip/iputil"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
)

// ApplyNodeGeoFromIP resolves geographic metadata for node when geo is not manually locked.
// City-level coordinates from NetIntel MMDB take precedence and replace legacy country centroids.
func ApplyNodeGeoFromIP(ctx context.Context, node *model.OpenFlareNode, rawIP string) {
	if node == nil {
		return
	}
	if node.GeoManualOverride {
		node.GeoSource = pkgnetintel.GeoSourceManual
		return
	}
	node.GeoName = ""
	node.GeoLatitude = nil
	node.GeoLongitude = nil
	node.GeoSource = ""

	ip := net.ParseIP(strings.TrimSpace(rawIP))
	if ip == nil || !iputil.IsPublic(ip) {
		return
	}

	ni := netintel.LookupIP(ip.String())
	if ni != nil && ni.GeoSource == pkgnetintel.GeoSourceCity && ni.Latitude != nil && ni.Longitude != nil {
		node.GeoName = formatNodeGeoName(ni.CityName, ni.CountryName, ni.CountryISO)
		node.GeoLatitude = cloneCoordinate(ni.Latitude)
		node.GeoLongitude = cloneCoordinate(ni.Longitude)
		node.GeoSource = pkgnetintel.GeoSourceCity
		return
	}

	info, err := GeoInfoFromIP(ip)
	if err != nil {
		logger.WarnF(ctx, "[GeoIP] resolve node geo failed: ip=%s error=%v", ip.String(), err)
	}
	if info != nil {
		if strings.TrimSpace(info.Name) != "" {
			node.GeoName = strings.TrimSpace(info.Name)
		}
		if info.Latitude != nil && info.Longitude != nil {
			node.GeoLatitude = cloneCoordinate(info.Latitude)
			node.GeoLongitude = cloneCoordinate(info.Longitude)
			node.GeoSource = pkgnetintel.GeoSourceCity
			return
		}
		if applyCountryCentroid(node, info.ISOCode, info.Name) {
			node.GeoSource = pkgnetintel.GeoSourceCountry
			return
		}
	}

	if ni != nil && ni.CountryISO != "" {
		node.GeoName = formatNodeGeoName(ni.CityName, ni.CountryName, ni.CountryISO)
		if applyCountryCentroid(node, ni.CountryISO, ni.CountryName) {
			node.GeoSource = pkgnetintel.GeoSourceCountry
			return
		}
	}
	node.GeoSource = pkgnetintel.GeoSourceUnknown
}

func formatNodeGeoName(city, countryName, countryISO string) string {
	city = strings.TrimSpace(city)
	countryName = strings.TrimSpace(countryName)
	countryISO = strings.TrimSpace(countryISO)
	if city != "" && countryName != "" && !strings.EqualFold(city, countryName) {
		return city + ", " + countryName
	}
	if city != "" {
		return city
	}
	if countryName != "" {
		return countryName
	}
	return countryISO
}

func applyCountryCentroid(node *model.OpenFlareNode, isoCode, name string) bool {
	if lat, lon, ok := pkggeoip.CountryCentroidByISO(isoCode); ok {
		node.GeoLatitude = cloneFloat(lat)
		node.GeoLongitude = cloneFloat(lon)
		return true
	}
	if lat, lon, ok := pkggeoip.CountryCentroidByName(name); ok {
		node.GeoLatitude = cloneFloat(lat)
		node.GeoLongitude = cloneFloat(lon)
		return true
	}
	return false
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
