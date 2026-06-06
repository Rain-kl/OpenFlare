package geoipdata

import "embed"

//go:embed GeoLite2-Country.mmdb
var FS embed.FS

const DefaultMMDBName = "GeoLite2-Country.mmdb"
