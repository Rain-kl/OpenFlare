// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/qualityapi"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
)

// SimulateInput requests an offline/debug SmartDNS routing simulation for a client IP and domain.
type SimulateInput struct {
	Domain   string `json:"domain"`
	QType    string `json:"qtype"` // "A" or "AAAA"
	ClientIP string `json:"client_ip"`
	PolicyID *uint  `json:"policy_id,omitempty"`
}

// SimulatorClientContext exposes resolved client Geo/ASN metadata used during simulation.
type SimulatorClientContext struct {
	ClientIP      string   `json:"client_ip"`
	ClientPrefix  string   `json:"client_prefix"`
	IPFamily      string   `json:"ip_family"`
	ASN           uint32   `json:"asn"`
	ASNName       string   `json:"asn_name,omitempty"`
	CountryISO    string   `json:"country_iso"`
	RegionCode    string   `json:"region_code,omitempty"`
	CityName      string   `json:"city_name,omitempty"`
	Latitude      *float64 `json:"latitude,omitempty"`
	Longitude     *float64 `json:"longitude,omitempty"`
	GeoAccuracyKm float64  `json:"geo_accuracy_km"`
	GeoSource     string   `json:"geo_source"`
	AnycastStatus string   `json:"anycast_status,omitempty"`
}

// SimulateResult combines client context and 4-stage SmartDNS selection trace.
type SimulateResult struct {
	Domain    string                 `json:"domain"`
	QType     string                 `json:"qtype"`
	Client    SimulatorClientContext `json:"client"`
	Selection SelectionResult        `json:"selection"`
}

// SimulateResolution runs JDM evaluation, qualityapi lookup, and 4-stage scoring without sending UDP packets.
func SimulateResolution(ctx context.Context, input SimulateInput) (*SimulateResult, error) {
	domain := normalizeDomain(input.Domain)
	if domain == "" {
		return nil, newLogicError(http.StatusBadRequest, errSimulatorDomainInvalid)
	}
	parsedIP := net.ParseIP(strings.TrimSpace(input.ClientIP))
	if parsedIP == nil {
		return nil, newLogicError(http.StatusBadRequest, errSimulatorIPInvalid)
	}
	qtype := strings.ToUpper(strings.TrimSpace(input.QType))
	if qtype == "" {
		qtype = "A"
	}
	if qtype != "A" && qtype != "AAAA" {
		return nil, newLogicError(http.StatusBadRequest, errSimulatorQTypeInvalid)
	}

	bundle, err := DefaultSnapshotManager().EnsureCurrent(ctx)
	if err != nil {
		return nil, err
	}

	var matchedPolicy protocol.DNSSnapshotPolicy
	if input.PolicyID != nil && *input.PolicyID > 0 {
		found := false
		for _, p := range bundle.Envelope.Policies {
			if p.ID == *input.PolicyID {
				matchedPolicy = p
				found = true
				break
			}
		}
		if !found {
			// Allow simulating disabled policies stored in DB too.
			pv, gErr := GetPolicy(ctx, *input.PolicyID)
			if gErr != nil {
				return nil, gErr
			}
			matchedPolicy = policyViewToSnapshotPolicy(pv)
		}
	} else {
		p, ok := matchPolicyForDomain(&bundle.Envelope, domain)
		if !ok {
			return nil, newLogicError(http.StatusNotFound, errPolicyNotFound)
		}
		matchedPolicy = p
	}

	clientProfile, clientCtx := buildClientProfileFromIP(parsedIP, familyForQType(qtype))
	nodeIDs := make([]string, 0, len(bundle.Envelope.Nodes))
	for _, n := range bundle.Envelope.Nodes {
		nodeIDs = append(nodeIDs, n.NodeID)
	}
	qualities := qualityapi.Default().EvaluateNodes(ctx, nodeIDs, clientProfile)

	selection := SelectNodes(SelectInput{
		Domain:    domain,
		QType:     qtype,
		ClientKey: clientCtx.ClientPrefix,
		Client:    clientProfile,
		Nodes:     bundle.Envelope.Nodes,
		Policy:    matchedPolicy,
		Quality:   qualities,
		Now:       time.Now().UTC(),
	})

	return &SimulateResult{
		Domain:    domain,
		QType:     qtype,
		Client:    clientCtx,
		Selection: selection,
	}, nil
}

func buildClientProfileFromIP(ip net.IP, answerFamily string) (qualityapi.ClientProfile, SimulatorClientContext) {
	ipStr := ip.String()
	prefix := normalizeClientPrefix(ip)
	lookup := netintel.LookupIP(ipStr)

	profile := qualityapi.ClientProfile{
		ClientIP: ipStr,
		IPFamily: answerFamily,
	}
	if profile.IPFamily == "" {
		profile.IPFamily = pkgnetintel.DetectIPFamily(ipStr)
	}
	simClient := SimulatorClientContext{
		ClientIP:     ipStr,
		ClientPrefix: prefix,
		IPFamily:     profile.IPFamily,
	}
	if lookup != nil {
		profile.ASN = lookup.ASN
		profile.CountryISO = lookup.CountryISO
		profile.RegionCode = lookup.RegionCode
		profile.ContinentCode = lookup.ContinentCode
		profile.Latitude = lookup.Latitude
		profile.Longitude = lookup.Longitude
		profile.GeoAccuracyKm = lookup.AccuracyRadiusKm
		profile.GeoSource = lookup.GeoSource
		profile.AnycastStatus = lookup.AnycastStatus

		simClient.ASN = lookup.ASN
		simClient.ASNName = lookup.ASNName
		simClient.CountryISO = lookup.CountryISO
		simClient.RegionCode = lookup.RegionCode
		simClient.CityName = lookup.CityName
		simClient.Latitude = lookup.Latitude
		simClient.Longitude = lookup.Longitude
		simClient.GeoAccuracyKm = lookup.AccuracyRadiusKm
		simClient.GeoSource = lookup.GeoSource
		simClient.AnycastStatus = lookup.AnycastStatus
	}
	return profile, simClient
}

func normalizeClientPrefix(ip net.IP) string {
	if ip == nil {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		masked := v4.Mask(net.CIDRMask(24, 32))
		return masked.String() + "/24"
	}
	if v6 := ip.To16(); v6 != nil {
		masked := v6.Mask(net.CIDRMask(48, 128))
		return masked.String() + "/48"
	}
	return ip.String()
}

func policyViewToSnapshotPolicy(pv *PolicyView) protocol.DNSSnapshotPolicy {
	if pv == nil {
		return protocol.DNSSnapshotPolicy{}
	}
	return protocol.DNSSnapshotPolicy{
		ID:              pv.ID,
		Name:            pv.Name,
		ScopeType:       pv.ScopeType,
		ScopeDomains:    pv.ScopeDomains,
		StrategyType:    pv.StrategyType,
		Priority:        pv.Priority,
		TTL:             pv.TTL,
		MaxAnswers:      pv.MaxAnswers,
		IsDefault:       pv.IsDefault,
		FallbackStrat:   pv.FallbackStrategy,
		FallbackNodeIDs: pv.FallbackNodeIDs,
		JDMJSON:         pv.JDMJSON,
		Revision:        pv.Revision,
	}
}
