// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package geoip

import (
	"net"
	"testing"

	pkggeoip "github.com/rain-kl/openflare/pkg/geoip"
)

type fakeLookupProvider struct{}

func (f *fakeLookupProvider) Name() string { return "fake-lookup" }

func (f *fakeLookupProvider) GetGeoInfo(_ net.IP) (*pkggeoip.GeoInfo, error) {
	lat := 37.7749
	lon := -122.4194
	return &pkggeoip.GeoInfo{
		ISOCode:   "US",
		Name:      "United States",
		Latitude:  &lat,
		Longitude: &lon,
	}, nil
}

func (f *fakeLookupProvider) UpdateDatabase() error { return nil }

func (f *fakeLookupProvider) Close() error { return nil }

func TestLookupWithProvider(t *testing.T) {
	previousFactory := pkggeoip.ProviderFactoryForTest()
	pkggeoip.SetProviderFactoryForTest(func(provider string) (pkggeoip.GeoIPService, error) {
		return &fakeLookupProvider{}, nil
	})
	t.Cleanup(func() {
		pkggeoip.SetProviderFactoryForTest(previousFactory)
	})

	view, err := Lookup("ipinfo", "8.8.8.8")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if view.Provider != "ipinfo" || view.IP != "8.8.8.8" {
		t.Fatalf("unexpected lookup view: %+v", view)
	}
	if view.ISOCode != "US" || view.Name != "United States" {
		t.Fatalf("unexpected geo fields: %+v", view)
	}
	if view.Latitude == nil || view.Longitude == nil {
		t.Fatalf("expected coordinates, got %+v", view)
	}
}

func TestLookupRejectsInvalidInput(t *testing.T) {
	if _, err := Lookup("invalid", "8.8.8.8"); err == nil {
		t.Fatal("expected invalid provider to fail")
	}
	if _, err := Lookup("ipinfo", "not-an-ip"); err == nil {
		t.Fatal("expected invalid IP to fail")
	}
}

func TestLookupDisabledProvider(t *testing.T) {
	view, err := Lookup("disabled", "8.8.8.8")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if view.Provider != "disabled" || view.IP != "8.8.8.8" {
		t.Fatalf("unexpected disabled view: %+v", view)
	}
}
