// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package data embeds the MaxMind GeoLite2 Country database for the control plane.
//
// Server keeps a Country-only embed so MaxMind provider can seed without network.
// Agent does NOT use this package — Agent MMDB files are image COPY / download only.
package data

import "embed"

// FS holds the embedded GeoLite2-Country.mmdb database.
//
//go:embed GeoLite2-Country.mmdb
var FS embed.FS

// DefaultMMDBName is the filename of the embedded MaxMind Country database.
const DefaultMMDBName = "GeoLite2-Country.mmdb"
