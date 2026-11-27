// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package geoip

import (
	"context"
	"net"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/model"
	pkggeoip "github.com/Rain-kl/Wavelet/pkg/geoip"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
	"github.com/stretchr/testify/require"
)

type stubGeoProvider struct {
	info *pkggeoip.GeoInfo
}

func (s *stubGeoProvider) Name() string                                 { return "stub" }
func (s *stubGeoProvider) GetGeoInfo(net.IP) (*pkggeoip.GeoInfo, error) { return s.info, nil }
func (s *stubGeoProvider) UpdateDatabase() error                        { return nil }
func (s *stubGeoProvider) Close() error                                 { return nil }

func TestApplyNodeGeoFromIPSetsGeoSourceAndRespectsManual(t *testing.T) {
	prev := pkggeoip.CurrentProvider
	pkggeoip.CurrentProvider = &stubGeoProvider{
		info: &pkggeoip.GeoInfo{
			ISOCode: "JP",
			Name:    "Japan",
		},
	}
	t.Cleanup(func() { pkggeoip.CurrentProvider = prev })

	node := &model.OpenFlareNode{IP: "203.0.113.10"}
	ApplyNodeGeoFromIP(context.Background(), node, node.IP)
	require.Equal(t, pkgnetintel.GeoSourceCountry, node.GeoSource)
	require.NotNil(t, node.GeoLatitude)

	// City coordinates from provider mark GeoSourceCity.
	lat, lon := 35.6762, 139.6503
	pkggeoip.CurrentProvider = &stubGeoProvider{
		info: &pkggeoip.GeoInfo{
			ISOCode:   "JP",
			Name:      "Tokyo",
			Latitude:  &lat,
			Longitude: &lon,
		},
	}
	ApplyNodeGeoFromIP(context.Background(), node, "203.0.113.11")
	require.Equal(t, pkgnetintel.GeoSourceCity, node.GeoSource)
	require.InDelta(t, lat, *node.GeoLatitude, 1e-6)

	// Manual override stays untouched and marked manual.
	manualLat := 1.23
	manualNode := &model.OpenFlareNode{
		IP:                "203.0.113.10",
		GeoManualOverride: true,
		GeoName:           "Custom",
		GeoLatitude:       &manualLat,
	}
	ApplyNodeGeoFromIP(context.Background(), manualNode, manualNode.IP)
	require.Equal(t, pkgnetintel.GeoSourceManual, manualNode.GeoSource)
	require.InDelta(t, manualLat, *manualNode.GeoLatitude, 1e-6)
}
