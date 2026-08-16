// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package geoipdata holds shared GeoIP database filename constants.
//
// MaxMind MMDB files are NOT embedded into the agent binary. Docker images
// COPY them onto the default data paths; bare binary installs seed via download
// on first start (see geoipupdate).
package geoipdata

const (
	// DefaultMMDBName is the default Country database filename.
	DefaultMMDBName = "GeoLite2-Country.mmdb"
	// DefaultCityMMDBName is the default City database filename.
	DefaultCityMMDBName = "GeoLite2-City.mmdb"
)
