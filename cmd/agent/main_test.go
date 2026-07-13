package main

import (
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/config"
)

func TestNewGeoIPUpdaterWiresCountryAndCity(t *testing.T) {
	cfg := &config.Config{
		MMDBPath:            "/data/GeoLite2-Country.mmdb",
		MMDBDownloadURL:     "https://geo.example/GeoLite2-Country.mmdb",
		CityMMDBPath:        "/data/GeoLite2-City.mmdb",
		CityMMDBDownloadURL: "https://geo.example/GeoLite2-City.mmdb",
		MMDBUpdateInterval:  config.MillisecondDuration(time.Hour),
	}
	updater := newGeoIPUpdater(cfg)
	if updater.MMDBPath != cfg.MMDBPath || updater.DownloadURL != cfg.MMDBDownloadURL ||
		updater.CityMMDBPath != cfg.CityMMDBPath || updater.CityDownloadURL != cfg.CityMMDBDownloadURL ||
		updater.UpdateInterval != time.Hour {
		t.Fatalf("GeoIP updater wiring incomplete: %#v", updater)
	}
}
