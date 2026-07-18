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

func TestCountryCentroidHongKongSingaporeTaiwan(t *testing.T) {
	cases := []struct {
		iso      string
		name     string
		minLat   float64
		maxLat   float64
		minLon   float64
		maxLon   float64
	}{
		{iso: "HK", name: "Hong Kong", minLat: 22, maxLat: 23, minLon: 113, maxLon: 115},
		{iso: "SG", name: "Singapore", minLat: 1, maxLat: 2, minLon: 103, maxLon: 104},
		{iso: "TW", name: "Taiwan", minLat: 22, maxLat: 26, minLon: 119, maxLon: 122},
	}
	for _, tc := range cases {
		lat, lon, ok := CountryCentroidByISO(tc.iso)
		if !ok {
			t.Fatalf("expected %s centroid by ISO", tc.iso)
		}
		if lat < tc.minLat || lat > tc.maxLat || lon < tc.minLon || lon > tc.maxLon {
			t.Fatalf("%s ISO centroid out of range: %f,%f", tc.iso, lat, lon)
		}
		lat, lon, ok = CountryCentroidByName(tc.name)
		if !ok {
			t.Fatalf("expected %s centroid by name", tc.name)
		}
		if lat < tc.minLat || lat > tc.maxLat || lon < tc.minLon || lon > tc.maxLon {
			t.Fatalf("%s name centroid out of range: %f,%f", tc.name, lat, lon)
		}
	}
}
