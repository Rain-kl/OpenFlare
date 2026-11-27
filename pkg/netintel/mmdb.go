// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netintel

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

type mmdbGeoRecord struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	RegisteredCountry struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"registered_country"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
	CountryCode string `maxminddb:"country_code"`
}

type mmdbASNRecord struct {
	AutonomousSystemNumber       uint32 `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
	ASN                          uint32 `maxminddb:"asn"`
	ASName                       string `maxminddb:"as_name"`
}

type mmdbBundle struct {
	country *maxminddb.Reader
	city    *maxminddb.Reader
	asn     *maxminddb.Reader
}

const mmdbDatasetCount = 3

func loadMMDBBundle(dir string, manifest *Manifest, fallbackCountryBytes []byte) (*mmdbBundle, []DatasetFileStatus) {
	bundle := &mmdbBundle{}
	statuses := make([]DatasetFileStatus, 0, mmdbDatasetCount)

	countryReader, countryStatus := openMMDBCandidate(dir, manifest, "geoip_country",
		[]string{"GeoLite2-Country.mmdb", "sapics-country.mmdb"}, fallbackCountryBytes)
	bundle.country = countryReader
	statuses = append(statuses, countryStatus)

	cityReader, cityStatus := openMMDBCandidate(dir, manifest, "geoip_city",
		[]string{"GeoLite2-City.mmdb"}, nil)
	bundle.city = cityReader
	statuses = append(statuses, cityStatus)

	asnReader, asnStatus := openMMDBCandidate(dir, manifest, "geoip_asn",
		[]string{"GeoLite2-ASN.mmdb", "sapics-asn.mmdb"}, nil)
	bundle.asn = asnReader
	statuses = append(statuses, asnStatus)

	return bundle, statuses
}

func applyMMDBManifest(status *DatasetFileStatus, manifest *Manifest) {
	if manifest == nil {
		return
	}
	entry, ok := manifest.Datasets[status.Key]
	if !ok {
		return
	}
	status.SourceURL = entry.SourceURL
	status.License = entry.License
	if entry.Filename != "" {
		status.Filename = entry.Filename
	}
	if entry.FetchedAtUnix > 0 {
		t := time.Unix(entry.FetchedAtUnix, 0).UTC()
		status.UpdatedAt = &t
	}
}

func findMMDBPath(dir string, status *DatasetFileStatus, candidates []string) string {
	if strings.TrimSpace(dir) == "" {
		return ""
	}
	ordered := make([]string, 0, len(candidates)+1)
	if status.Filename != "" {
		ordered = append(ordered, status.Filename)
	}
	for _, c := range candidates {
		if c != status.Filename {
			ordered = append(ordered, c)
		}
	}
	for _, name := range ordered {
		p := filepath.Join(dir, name)
		info, err := os.Stat(p)
		if err != nil || info.IsDir() || info.Size() == 0 {
			continue
		}
		status.Filename = name
		status.Path = p
		status.SizeBytes = info.Size()
		modTime := info.ModTime().UTC()
		if status.UpdatedAt == nil {
			status.UpdatedAt = &modTime
		}
		return p
	}
	return ""
}

func openMMDBCandidate(dir string, manifest *Manifest, key string, candidates []string, fallbackBytes []byte) (*maxminddb.Reader, DatasetFileStatus) {
	status := DatasetFileStatus{
		Key:      key,
		Filename: candidates[0],
	}
	applyMMDBManifest(&status, manifest)

	if chosenPath := findMMDBPath(dir, &status, candidates); chosenPath != "" {
		reader, loadErr := readAndOpenMMDB(chosenPath)
		if loadErr != nil {
			status.Error = loadErr.Error()
		} else {
			status.Loaded = true
			status.RecordCount = int(reader.Metadata.NodeCount) //nolint:gosec // node count fits in int on 64-bit
			return reader, status
		}
	}

	if len(fallbackBytes) > 0 {
		reader, err := maxminddb.FromBytes(fallbackBytes)
		if err == nil {
			status.Loaded = true
			status.Filename = "embedded:" + candidates[0]
			status.SizeBytes = int64(len(fallbackBytes))
			status.RecordCount = int(reader.Metadata.NodeCount) //nolint:gosec // node count fits in int on 64-bit
			status.SourceURL = "embedded-fallback"
			status.License = "CC BY-SA 4.0 / MaxMind GeoLite2 EULA"
			return reader, status
		}
		status.Error = err.Error()
	}
	return nil, status
}

func readAndOpenMMDB(path string) (*maxminddb.Reader, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // path is resolved inside configured data dir
	if err != nil {
		return nil, fmt.Errorf("read mmdb: %w", err)
	}
	reader, err := maxminddb.FromBytes(raw)
	if err != nil {
		return nil, fmt.Errorf("parse mmdb: %w", err)
	}
	return reader, nil
}

func (b *mmdbBundle) lookupGeo(ip net.IP) (countryISO, countryName, cityName string, lat, lon *float64) {
	if b == nil || ip == nil {
		return "", "", "", nil, nil
	}
	if b.city != nil {
		var rec mmdbGeoRecord
		if err := b.city.Lookup(ip, &rec); err == nil {
			countryISO, countryName, cityName, lat, lon = extractGeoFields(&rec)
			if countryISO != "" || lat != nil {
				return countryISO, countryName, cityName, lat, lon
			}
		}
	}
	if b.country != nil {
		var rec mmdbGeoRecord
		if err := b.country.Lookup(ip, &rec); err == nil {
			return extractGeoFields(&rec)
		}
	}
	return "", "", "", nil, nil
}

func (b *mmdbBundle) lookupASN(ip net.IP) (uint32, string) {
	if b == nil || b.asn == nil || ip == nil {
		return 0, ""
	}
	var rec mmdbASNRecord
	if err := b.asn.Lookup(ip, &rec); err != nil {
		return 0, ""
	}
	asn := rec.AutonomousSystemNumber
	if asn == 0 {
		asn = rec.ASN
	}
	org := strings.TrimSpace(rec.AutonomousSystemOrganization)
	if org == "" {
		org = strings.TrimSpace(rec.ASName)
	}
	return asn, org
}

func extractGeoFields(rec *mmdbGeoRecord) (countryISO, countryName, cityName string, lat, lon *float64) {
	countryISO = strings.ToUpper(strings.TrimSpace(rec.Country.ISOCode))
	if countryISO == "" {
		countryISO = strings.ToUpper(strings.TrimSpace(rec.RegisteredCountry.ISOCode))
	}
	if countryISO == "" {
		countryISO = strings.ToUpper(strings.TrimSpace(rec.CountryCode))
	}
	countryName = strings.TrimSpace(rec.Country.Names["en"])
	if countryName == "" {
		countryName = strings.TrimSpace(rec.RegisteredCountry.Names["en"])
	}
	if countryName == "" {
		countryName = countryISO
	}
	cityName = strings.TrimSpace(rec.City.Names["en"])
	if rec.Location.Latitude != 0 || rec.Location.Longitude != 0 {
		la := rec.Location.Latitude
		lo := rec.Location.Longitude
		lat = &la
		lon = &lo
	}
	return countryISO, countryName, cityName, lat, lon
}
