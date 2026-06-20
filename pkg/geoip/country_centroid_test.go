package geoip

import "testing"

func TestCountryCentroidGermany(t *testing.T) {
	lat, lon, ok := CountryCentroidByISO("DE")
	if !ok {
		t.Fatal("expected DE centroid")
	}
	if lat < 47 || lat > 55 || lon < 5 || lon > 15 {
		t.Fatalf("unexpected Germany centroid: %f,%f", lat, lon)
	}
}
