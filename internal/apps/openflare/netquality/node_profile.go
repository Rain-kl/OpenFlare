// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netquality

import (
	"slices"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// NodeNetworkProfile captures an edge node's routing and line-product metadata for cold-start prediction.
type NodeNetworkProfile struct {
	NodeID            string
	ASN               uint32
	Country           string
	Region            string
	NetworkGroup      string
	GeoLatitude       *float64
	GeoLongitude      *float64
	GeoSource         string
	ObservedUpstreams []uint32
	RouteTags         []string
}

// BuildNodeNetworkProfile constructs an in-memory NodeNetworkProfile from a node row and observed AS edges.
func BuildNodeNetworkProfile(node model.OpenFlareNode, observedAdj map[uint32][]uint32) NodeNetworkProfile {
	if node.ASN == 0 && strings.TrimSpace(node.IP) != "" {
		netintel.EnrichNodeASN(&node)
	}
	profile := NodeNetworkProfile{
		NodeID:       strings.TrimSpace(node.NodeID),
		ASN:          node.ASN,
		NetworkGroup: strings.ToLower(strings.TrimSpace(node.NetworkGroup)),
		GeoLatitude:  node.GeoLatitude,
		GeoLongitude: node.GeoLongitude,
		GeoSource:    strings.TrimSpace(node.GeoSource),
	}

	if ip := strings.TrimSpace(node.IP); ip != "" {
		if res := netintel.LookupIP(ip); res != nil {
			profile.Country = res.CountryISO
			profile.Region = res.RegionCode
			if len(res.ASNTags) > 0 {
				profile.RouteTags = append([]string(nil), res.ASNTags...)
			}
		}
	}

	populateNodeProfileASN(&profile, observedAdj)
	return profile
}

func populateNodeProfileASN(profile *NodeNetworkProfile, observedAdj map[uint32][]uint32) {
	if profile.ASN == 0 {
		return
	}
	upstreamSet := make(map[uint32]struct{})
	for _, peer := range observedAdj[profile.ASN] {
		if peer != 0 && peer != profile.ASN {
			upstreamSet[peer] = struct{}{}
		}
	}
	asProf := netintel.ExtractASUpstreamProfile(profile.ASN)
	if len(profile.RouteTags) == 0 && len(asProf.Tags) > 0 {
		profile.RouteTags = append([]string(nil), asProf.Tags...)
	}
	if profile.Country == "" && asProf.Country != "" {
		profile.Country = asProf.Country
	}
	for _, prov := range asProf.DirectProviders {
		if prov != 0 && prov != profile.ASN {
			upstreamSet[prov] = struct{}{}
		}
	}
	if len(upstreamSet) == 0 {
		return
	}
	upstreams := make([]uint32, 0, len(upstreamSet))
	for u := range upstreamSet {
		upstreams = append(upstreams, u)
	}
	slices.Sort(upstreams)
	profile.ObservedUpstreams = upstreams
}
