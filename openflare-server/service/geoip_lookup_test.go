package service

import (
	"net"
	"testing"

	"github.com/rain-kl/openflare/openflare-server/utils/geoip"
)

type fakeLookupProvider struct{}

func (f *fakeLookupProvider) Name() string {
	return "fake-lookup"
}

func (f *fakeLookupProvider) GetGeoInfo(ip net.IP) (*geoip.GeoInfo, error) {
	return &geoip.GeoInfo{
		ISOCode:   "US",
		Name:      "United States",
		Latitude:  geoipFloat(37.7749),
		Longitude: geoipFloat(-122.4194),
	}, nil
}

func (f *fakeLookupProvider) UpdateDatabase() error {
	return nil
}

func (f *fakeLookupProvider) Close() error {
	return nil
}

func TestLookupGeoIP(t *testing.T) {
	previousFactory := geoip.ProviderFactoryForTest()
	geoip.SetProviderFactoryForTest(func(provider string) (geoip.GeoIPService, error) {
		return &fakeLookupProvider{}, nil
	})
	defer geoip.SetProviderFactoryForTest(previousFactory)

	view, err := LookupGeoIP("ipinfo", "8.8.8.8")
	if err != nil {
		t.Fatalf("LookupGeoIP failed: %v", err)
	}
	if view.Provider != "ipinfo" {
		t.Fatalf("expected provider ipinfo, got %s", view.Provider)
	}
	if view.IP != "8.8.8.8" {
		t.Fatalf("expected IP 8.8.8.8, got %s", view.IP)
	}
	if view.ISOCode != "US" || view.Name != "United States" {
		t.Fatalf("unexpected lookup view: %+v", view)
	}
	if view.Latitude == nil || view.Longitude == nil {
		t.Fatalf("expected coordinates, got %+v", view)
	}
}

func TestLookupGeoIPRejectsInvalidInput(t *testing.T) {
	if _, err := LookupGeoIP("invalid", "8.8.8.8"); err == nil {
		t.Fatal("expected invalid provider to fail")
	}
	if _, err := LookupGeoIP("ipinfo", "not-an-ip"); err == nil {
		t.Fatal("expected invalid IP to fail")
	}
}
