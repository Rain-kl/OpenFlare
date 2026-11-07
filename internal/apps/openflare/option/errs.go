// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package option provides handler-level error message constants for the openflare option module.
package option

const (
	errInvalidParams       = "invalid parameters"
	errOptionInitFailed    = "Failed to initialize system options"
	errGeoIPProvider       = "geolocation provider must be disabled, mmdb, ip-api, geojs or ipinfo"
	errGeoIPIPEmpty        = "IP address must not be empty"
	errGeoIPIPInvalid      = "invalid IP address format"
	errGeoIPLookupDisabled = "GeoIP lookup is disabled"
)
