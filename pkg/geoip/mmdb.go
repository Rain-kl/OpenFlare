// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package geoip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/oschwald/maxminddb-golang"
)

// GeoIPURL is the default download URL for the MaxMind GeoLite2 country database.
var GeoIPURL = "https://raw.githubusercontent.com/Loyalsoldier/geoip/release/GeoLite2-Country.mmdb"

// GeoIPFilePath is the default local path for the MaxMind country database file.
var GeoIPFilePath = "./data/GeoLite2-Country.mmdb"

// Record is the MaxMind database record structure for country lookups.
type Record struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
}

// MaxMindGeoIPService resolves geographic information using a local MaxMind MMDB database.
type MaxMindGeoIPService struct {
	maxMindDBReader *maxminddb.Reader
	dbFilePath      string
	mu              sync.RWMutex
}

// Name returns the provider identifier for the MaxMind database service.
func (s *MaxMindGeoIPService) Name() string {
	return "MaxMind"
}

// NewMaxMindGeoIPService creates a MaxMind service using the default database path and URL.
func NewMaxMindGeoIPService() (*MaxMindGeoIPService, error) {
	return NewMaxMindGeoIPServiceWithConfig(GeoIPFilePath, GeoIPURL)
}

// NewMaxMindGeoIPServiceWithConfig creates a MaxMind service with custom database path and download URL.
func NewMaxMindGeoIPServiceWithConfig(dbFilePath string, downloadURL string) (*MaxMindGeoIPService, error) {
	if dbFilePath == "" {
		dbFilePath = GeoIPFilePath
	}
	if downloadURL == "" {
		downloadURL = GeoIPURL
	}
	service := &MaxMindGeoIPService{
		dbFilePath: dbFilePath,
	}

	if err := os.MkdirAll(filepath.Dir(service.dbFilePath), geoipDataDirPerm); err != nil {
		return nil, fmt.Errorf("failed to create data directory for MaxMind database: %w", err)
	}

	if _, err := os.Stat(service.dbFilePath); os.IsNotExist(err) {
		if err := DownloadMaxMindDatabase(context.Background(), service.dbFilePath, downloadURL); err != nil {
			return nil, fmt.Errorf("failed to download initial MaxMind database: %w", err)
		}
	}

	if err := service.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize MaxMind database: %w", err)
	}

	return service, nil
}

func (s *MaxMindGeoIPService) initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.maxMindDBReader != nil {
		_ = s.maxMindDBReader.Close()
		s.maxMindDBReader = nil
	}

	reader, err := maxminddb.Open(s.dbFilePath)
	if err != nil {
		return fmt.Errorf("error opening MaxMind database at %s: %w", s.dbFilePath, err)
	}
	s.maxMindDBReader = reader
	return nil
}

// GetGeoInfo looks up geographic information for ip in the MaxMind database.
func (s *MaxMindGeoIPService) GetGeoInfo(ip net.IP) (*GeoInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.maxMindDBReader == nil {
		return nil, errors.New("MaxMind database is not initialized or failed to open")
	}
	if ip == nil {
		return nil, errors.New("IP address cannot be nil")
	}

	var record Record
	if err := s.maxMindDBReader.Lookup(ip, &record); err != nil {
		return nil, fmt.Errorf("error looking up IP %s in MaxMind database: %w", ip.String(), err)
	}

	geoInfo := &GeoInfo{
		ISOCode: record.Country.ISOCode,
		Name:    record.Country.Names["en"],
	}
	if geoInfo.Name == "" && geoInfo.ISOCode != "" {
		geoInfo.Name = geoInfo.ISOCode
	}

	return geoInfo, nil
}

// UpdateDatabase downloads the latest MaxMind database and reloads the reader.
func (s *MaxMindGeoIPService) UpdateDatabase() error {
	if err := DownloadMaxMindDatabase(context.Background(), s.dbFilePath, GeoIPURL); err != nil {
		return err
	}
	return s.initialize()
}

// DownloadMaxMindDatabase downloads the MaxMind database from downloadURL to dbFilePath.
func DownloadMaxMindDatabase(ctx context.Context, dbFilePath string, downloadURL string) error {
	if dbFilePath == "" {
		dbFilePath = GeoIPFilePath
	}
	if downloadURL == "" {
		downloadURL = GeoIPURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil) //nolint:gosec // URL from trusted GeoIP provider config
	if err != nil {
		return fmt.Errorf("failed to initiate MaxMind database download: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to initiate MaxMind database download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download MaxMind database: HTTP status %s", resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(dbFilePath), geoipDataDirPerm); err != nil {
		return fmt.Errorf("failed to create data directory for MaxMind database update: %w", err)
	}

	tempPath := dbFilePath + ".download"
	out, err := os.Create(tempPath) //nolint:gosec // tempPath is derived from configured dbFilePath
	if err != nil {
		return fmt.Errorf("failed to create MaxMind database file at %s: %w", tempPath, err)
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err = io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to write MaxMind database file: %w", err)
	}
	if err = out.Close(); err != nil {
		return fmt.Errorf("failed to close MaxMind database file: %w", err)
	}
	if err = os.Rename(tempPath, dbFilePath); err != nil {
		return fmt.Errorf("failed to move MaxMind database file into place: %w", err)
	}
	return nil
}

// Close closes the MaxMind database reader.
func (s *MaxMindGeoIPService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.maxMindDBReader != nil {
		err := s.maxMindDBReader.Close()
		s.maxMindDBReader = nil
		if err != nil {
			return fmt.Errorf("error closing MaxMind database: %w", err)
		}
	}
	return nil
}
