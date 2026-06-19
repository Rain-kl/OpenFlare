// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"net"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/model"
	pkggeoip "github.com/rain-kl/openflare/pkg/geoip"
)

type fakeGeoIPProvider struct {
	info *pkggeoip.GeoInfo
}

func (f *fakeGeoIPProvider) Name() string { return "fake-geoip" }

func (f *fakeGeoIPProvider) GetGeoInfo(ip net.IP) (*pkggeoip.GeoInfo, error) {
	return f.info, nil
}

func (f *fakeGeoIPProvider) UpdateDatabase() error { return nil }

func (f *fakeGeoIPProvider) Close() error { return nil }

func withFakeGeoIPProvider(t *testing.T, info *pkggeoip.GeoInfo) {
	t.Helper()
	previous := pkggeoip.CurrentProvider
	pkggeoip.CurrentProvider = &fakeGeoIPProvider{info: info}
	t.Cleanup(func() {
		pkggeoip.CurrentProvider = previous
	})
}

func geoipFloat(value float64) *float64 {
	return &value
}

func TestApplyGeoInfoFromIP(t *testing.T) {
	latitude := 31.2304
	longitude := 121.4737
	withFakeGeoIPProvider(t, &pkggeoip.GeoInfo{
		Name:      "Shanghai",
		Latitude:  geoipFloat(latitude),
		Longitude: geoipFloat(longitude),
	})

	node := &model.OpenFlareNode{IP: "203.0.113.10"}
	applyGeoInfoFromIP(node, node.IP)

	if node.GeoName != "Shanghai" {
		t.Fatalf("expected geo_name Shanghai, got %q", node.GeoName)
	}
	if node.GeoLatitude == nil || *node.GeoLatitude != latitude {
		t.Fatalf("unexpected geo_latitude: %+v", node.GeoLatitude)
	}
	if node.GeoLongitude == nil || *node.GeoLongitude != longitude {
		t.Fatalf("unexpected geo_longitude: %+v", node.GeoLongitude)
	}
}

func TestApplyGeoInfoFromIPSkipsInvalidIP(t *testing.T) {
	withFakeGeoIPProvider(t, &pkggeoip.GeoInfo{Name: "Should Not Apply"})

	node := &model.OpenFlareNode{
		IP:           "203.0.113.10",
		GeoName:      "Existing",
		GeoLatitude:  geoipFloat(1),
		GeoLongitude: geoipFloat(2),
	}
	applyGeoInfoFromIP(node, "not-an-ip")

	if node.GeoName != "" || node.GeoLatitude != nil || node.GeoLongitude != nil {
		t.Fatalf("expected geo fields to be cleared on invalid IP, got %+v", node)
	}
}

func TestApplyNodeRuntimeRespectsGeoManualOverride(t *testing.T) {
	withFakeGeoIPProvider(t, &pkggeoip.GeoInfo{
		Name:      "Shanghai",
		Latitude:  geoipFloat(31.2304),
		Longitude: geoipFloat(121.4737),
	})

	node := &model.OpenFlareNode{
		GeoManualOverride: true,
		GeoName:           "Manual",
		GeoLatitude:       geoipFloat(10),
		GeoLongitude:      geoipFloat(20),
	}
	applyNodeRuntime(node, NodePayload{
		IP:      "203.0.113.10",
		Version: "1.0.0",
	}, true)

	if node.GeoName != "Manual" {
		t.Fatalf("expected manual geo_name to be preserved, got %q", node.GeoName)
	}
	if node.GeoLatitude == nil || *node.GeoLatitude != 10 {
		t.Fatalf("expected manual geo_latitude to be preserved, got %+v", node.GeoLatitude)
	}
}

func TestCollectHeartbeatChangesTracksGeoFields(t *testing.T) {
	before := &model.OpenFlareNode{
		IP:      "10.0.0.1",
		GeoName: "Old Region",
	}
	after := &model.OpenFlareNode{
		IP:           "203.0.113.10",
		GeoName:      "New Region",
		GeoLatitude:  geoipFloat(31.2304),
		GeoLongitude: geoipFloat(121.4737),
	}

	changes := collectHeartbeatChanges(before, after)
	if changes["ip"] != after.IP {
		t.Fatalf("expected ip change, got %+v", changes)
	}
	if changes["geo_name"] != after.GeoName {
		t.Fatalf("expected geo_name change, got %+v", changes)
	}
	if changes["geo_latitude"] != after.GeoLatitude {
		t.Fatalf("expected geo_latitude change, got %+v", changes)
	}
	if changes["geo_longitude"] != after.GeoLongitude {
		t.Fatalf("expected geo_longitude change, got %+v", changes)
	}
}
