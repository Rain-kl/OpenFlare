// Package geoipdata embeds the default MaxMind GeoLite2 databases.
package geoipdata

import "embed"

// FS holds the embedded GeoLite2 Country and City databases.
//
//go:embed GeoLite2-Country.mmdb GeoLite2-City.mmdb
var FS embed.FS

const (
	// DefaultMMDBName is the filename of the embedded MaxMind Country database.
	DefaultMMDBName = "GeoLite2-Country.mmdb"
	// DefaultCityMMDBName is the filename of the embedded MaxMind City database.
	DefaultCityMMDBName = "GeoLite2-City.mmdb"
)
