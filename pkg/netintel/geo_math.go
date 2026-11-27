// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netintel

import (
	"math"
	"net/netip"
	"slices"
	"strings"
)

const (
	earthRadiusKm          = 6371.0
	fiberSpeedKmPerMs      = 200.0
	degToRad               = math.Pi / 180.0
	radToDeg               = 180.0 / math.Pi
	minCentroidNorm        = 1e-9
	maxRegionCodeLen       = 32
	minGeoConfidenceFactor = 0.25
	maxGeoCityConfidence   = 0.98
	geoCountryConfidence   = 0.40
	geoASNCountryConf      = 0.25
	geoAccuracyScaleKm     = 500.0
	defaultCityAccuracyKm  = 25.0
	haversineHalfDivisor   = 2.0
)

var defaultRegionDrilldownCountries = map[string]struct{}{
	"CN": {},
	"US": {},
	"RU": {},
	"CA": {},
	"BR": {},
	"AU": {},
	"IN": {},
	"JP": {},
	"DE": {},
	"GB": {},
}

var confirmedPublicAnycastPrefixes = []netip.Prefix{
	netip.MustParsePrefix("1.1.1.0/24"),
	netip.MustParsePrefix("1.0.0.0/24"),
	netip.MustParsePrefix("8.8.8.0/24"),
	netip.MustParsePrefix("8.8.4.0/24"),
	netip.MustParsePrefix("9.9.9.0/24"),
	netip.MustParsePrefix("149.112.112.0/24"),
	netip.MustParsePrefix("208.67.222.0/24"),
	netip.MustParsePrefix("208.67.220.0/24"),
	netip.MustParsePrefix("2606:4700:4700::/48"),
	netip.MustParsePrefix("2001:4860:4860::/48"),
	netip.MustParsePrefix("2620:fe::/48"),
	netip.MustParsePrefix("2620:119:35::/48"),
}

// DistanceKm computes great-circle spherical distance between two coordinates in kilometers.
func DistanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	phi1 := lat1 * degToRad
	phi2 := lat2 * degToRad
	dPhi := (lat2 - lat1) * degToRad
	dLambda := (lon2 - lon1) * degToRad

	sinDPhi := math.Sin(dPhi / haversineHalfDivisor)
	sinDLambda := math.Sin(dLambda / haversineHalfDivisor)
	a := sinDPhi*sinDPhi + math.Cos(phi1)*math.Cos(phi2)*sinDLambda*sinDLambda
	if a < 0 {
		a = 0
	}
	if a > 1 {
		a = 1
	}
	c := haversineHalfDivisor * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c
}

// GeoBaseRTTMs converts spherical distance in kilometers into the physical fiber round-trip baseline (2d / 200 km/ms).
func GeoBaseRTTMs(distKm float64) float64 {
	if distKm <= 0 || math.IsNaN(distKm) || math.IsInf(distKm, 0) {
		return 0
	}
	return 2.0 * distKm / fiberSpeedKmPerMs
}

// SphericalCentroid accumulates weighted coordinates in 3D Cartesian space so antimeridian crossing averages cleanly.
type SphericalCentroid struct {
	x         float64
	y         float64
	z         float64
	weightSum float64
}

// Add appends a weighted latitude/longitude observation into the spherical centroid accumulator.
func (c *SphericalCentroid) Add(lat, lon, weight float64) {
	if c == nil || weight <= 0 || math.IsNaN(lat) || math.IsNaN(lon) || math.IsNaN(weight) {
		return
	}
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return
	}
	latRad := lat * degToRad
	lonRad := lon * degToRad
	cosLat := math.Cos(latRad)
	c.x += weight * cosLat * math.Cos(lonRad)
	c.y += weight * cosLat * math.Sin(lonRad)
	c.z += weight * math.Sin(latRad)
	c.weightSum += weight
}

// Result returns the normalized spherical centroid coordinates when available.
func (c *SphericalCentroid) Result() (lat, lon float64, ok bool) {
	if c == nil || c.weightSum <= 0 {
		return 0, 0, false
	}
	x := c.x / c.weightSum
	y := c.y / c.weightSum
	z := c.z / c.weightSum
	hyp := math.Sqrt(x*x + y*y)
	norm := math.Sqrt(hyp*hyp + z*z)
	if norm < minCentroidNorm {
		return 0, 0, false
	}
	lat = math.Atan2(z, hyp) * radToDeg
	lon = math.Atan2(y, x) * radToDeg
	return lat, lon, true
}

// FormatSubdivisionCode normalizes a GeoIP subdivision code into canonical ISO 3166-2 format (e.g. "CN-GD").
func FormatSubdivisionCode(countryISO, subISO string) string {
	sub := strings.ToUpper(strings.TrimSpace(subISO))
	if sub == "" {
		return ""
	}
	cc := strings.ToUpper(strings.TrimSpace(countryISO))
	var out string
	if strings.Contains(sub, "-") || cc == "" {
		out = sub
	} else {
		out = cc + "-" + sub
	}
	if len(out) > maxRegionCodeLen {
		out = out[:maxRegionCodeLen]
	}
	return out
}

// IsRegionDrilldownCountry reports whether countryISO uses region-level matrix aggregation by default.
func IsRegionDrilldownCountry(countryISO string) bool {
	_, ok := defaultRegionDrilldownCountries[strings.ToUpper(strings.TrimSpace(countryISO))]
	return ok
}

// NormalizeMatrixRegion returns rawRegion normalized when countryISO participates in region drilldown, otherwise "".
func NormalizeMatrixRegion(countryISO, rawRegion string) string {
	if !IsRegionDrilldownCountry(countryISO) {
		return ""
	}
	return FormatSubdivisionCode(countryISO, rawRegion)
}

// DetectIPFamily returns "ipv4", "ipv6", or "unknown" for rawIP (supports optional host:port).
func DetectIPFamily(rawIP string) string {
	trimmed := strings.TrimSpace(rawIP)
	if trimmed == "" {
		return IPFamilyUnknown
	}
	if ap, err := netip.ParseAddrPort(trimmed); err == nil {
		addr := ap.Addr().Unmap()
		if addr.Is4() {
			return IPFamilyIPv4
		}
		if addr.Is6() {
			return IPFamilyIPv6
		}
		return IPFamilyUnknown
	}
	trimmed = strings.TrimPrefix(strings.TrimSuffix(trimmed, "]"), "[")
	if addr, err := netip.ParseAddr(trimmed); err == nil {
		addr = addr.Unmap()
		if addr.Is4() {
			return IPFamilyIPv4
		}
		if addr.Is6() {
			return IPFamilyIPv6
		}
	}
	return IPFamilyUnknown
}

// EvaluateAnycastStatus classifies an IP address into "confirmed", "unknown", or "unicast".
func EvaluateAnycastStatus(addr netip.Addr, traitsIsAnycast bool, asnTags []string) string {
	if traitsIsAnycast {
		return AnycastStatusConfirmed
	}
	if addr.IsValid() {
		unmapped := addr.Unmap()
		for _, prefix := range confirmedPublicAnycastPrefixes {
			if prefix.Contains(unmapped) {
				return AnycastStatusConfirmed
			}
		}
	}
	if slices.ContainsFunc(asnTags, func(tag string) bool {
		return strings.EqualFold(strings.TrimSpace(tag), "anycast")
	}) {
		return AnycastStatusUnknown
	}
	return AnycastStatusUnicast
}

// GeoLocationConfidence estimates a [0, 1] confidence score from GeoSource and location accuracy radius.
func GeoLocationConfidence(geoSource string, accuracyKm float64) float64 {
	switch strings.ToLower(strings.TrimSpace(geoSource)) {
	case GeoSourceManual:
		return 1.0
	case GeoSourceCity:
		acc := accuracyKm
		if acc <= 0 {
			acc = defaultCityAccuracyKm
		}
		// 25km -> ~0.95, 200km -> ~0.71, 1000km -> ~0.33
		score := 1.0 / (1.0 + acc/geoAccuracyScaleKm)
		return math.Max(minGeoConfidenceFactor, math.Min(maxGeoCityConfidence, score))
	case GeoSourceCountry:
		return geoCountryConfidence
	case GeoSourceASNCountry:
		return geoASNCountryConf
	default:
		return 0.0
	}
}
