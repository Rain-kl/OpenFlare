// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package geoip

import (
	"errors"
	"net"
)

// EmptyProvider is a no-op GeoIP backend used before a real provider is configured.
type EmptyProvider struct{}

// Name returns the provider identifier.
func (e *EmptyProvider) Name() string {
	return "EmptyProvider"
}

// Initialize prepares the empty provider for use.
func (e *EmptyProvider) Initialize() error {
	return nil
}

// GetGeoInfo reports that no GeoIP provider has been configured.
func (e *EmptyProvider) GetGeoInfo(_ net.IP) (*GeoInfo, error) {
	return nil, errors.New("you are using an empty GeoIP provider, please set a valid provider")
}

// UpdateDatabase reports that no GeoIP provider has been configured.
func (e *EmptyProvider) UpdateDatabase() error {
	return errors.New("you are using an empty GeoIP provider, please set a valid provider")
}

// Close releases resources held by the empty provider.
func (e *EmptyProvider) Close() error {
	return nil
}
