// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nettools

import (
	"context"
	"errors"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
	"gorm.io/gorm"
)

// ProbeTargetNodeStateItem enriches a per-node backoff state row with node identity metadata.
type ProbeTargetNodeStateItem struct {
	*model.OpenFlareProbeTargetState
	NodeName   string `json:"node_name"`
	NodeGeo    string `json:"node_geo"`
	NodeStatus string `json:"node_status"`
}

// ProbeTargetItem bundles one managed probe target with its per-node probe states.
type ProbeTargetItem struct {
	*model.OpenFlareProbeTarget
	States []ProbeTargetNodeStateItem `json:"states"`
}

// ProbeTargetPage wraps paginated managed probe targets.
type ProbeTargetPage struct {
	Total   int64             `json:"total"`
	Results []ProbeTargetItem `json:"results"`
}

// ProbeTargetInput validates and creates an SSRF-checked active probe target.
type ProbeTargetInput struct {
	Name          string `json:"name"`
	IP            string `json:"ip"`
	ProbeProto    string `json:"probe_proto"`
	Port          int    `json:"port"`
	ASN           uint32 `json:"asn"`
	Country       string `json:"country"`
	Region        string `json:"region"`
	TargetType    string `json:"target_type"`
	AnycastStatus string `json:"anycast_status"`
	Enabled       *bool  `json:"enabled,omitempty"`
	Notes         string `json:"notes"`
}

// ProbeTargetUpdateInput partially updates a managed probe target.
type ProbeTargetUpdateInput struct {
	Name          *string `json:"name,omitempty"`
	ProbeProto    *string `json:"probe_proto,omitempty"`
	Port          *int    `json:"port,omitempty"`
	ASN           *uint32 `json:"asn,omitempty"`
	Country       *string `json:"country,omitempty"`
	Region        *string `json:"region,omitempty"`
	TargetType    *string `json:"target_type,omitempty"`
	AnycastStatus *string `json:"anycast_status,omitempty"`
	Enabled       *bool   `json:"enabled,omitempty"`
	Notes         *string `json:"notes,omitempty"`
}

// ListProbeTargets returns paginated probe targets along with per-node backoff & last-success states.
func ListProbeTargets(ctx context.Context, query model.ProbeTargetQuery) (*ProbeTargetPage, error) {
	if db.DB(ctx) == nil {
		return nil, errors.New(errDatabaseUnavailable)
	}
	targets, total, err := repository.ListOpenFlareProbeTargets(ctx, query)
	if err != nil {
		return nil, err
	}
	targetIDs := make([]uint, 0, len(targets))
	for _, t := range targets {
		if t != nil && t.ID > 0 {
			targetIDs = append(targetIDs, t.ID)
		}
	}
	states, err := repository.ListOpenFlareProbeTargetStates(ctx, "", targetIDs)
	if err != nil {
		return nil, err
	}
	nodes, _ := repository.ListOpenFlareNodes(ctx)
	nodeMap := make(map[string]model.OpenFlareNode, len(nodes))
	for i := range nodes {
		nodeMap[nodes[i].NodeID] = nodes[i]
	}

	statesByTarget := make(map[uint][]ProbeTargetNodeStateItem, len(targetIDs))
	for _, st := range states {
		if st == nil {
			continue
		}
		n := nodeMap[st.NodeID]
		statesByTarget[st.TargetID] = append(statesByTarget[st.TargetID], ProbeTargetNodeStateItem{
			OpenFlareProbeTargetState: st,
			NodeName:                  n.Name,
			NodeGeo:                   n.GeoName,
			NodeStatus:                n.Status,
		})
	}

	items := make([]ProbeTargetItem, 0, len(targets))
	for _, t := range targets {
		if t == nil {
			continue
		}
		stList := statesByTarget[t.ID]
		if stList == nil {
			stList = []ProbeTargetNodeStateItem{}
		}
		items = append(items, ProbeTargetItem{
			OpenFlareProbeTarget: t,
			States:               stList,
		})
	}
	return &ProbeTargetPage{
		Total:   total,
		Results: items,
	}, nil
}

// CreateProbeTarget validates public IP/SSRF constraints, normalizes metadata, and creates a managed probe target.
func CreateProbeTarget(ctx context.Context, input ProbeTargetInput) (*model.OpenFlareProbeTarget, error) {
	if db.DB(ctx) == nil {
		return nil, errors.New(errDatabaseUnavailable)
	}
	cleanIP, ipFamily, err := ValidatePublicIPAddress(input.IP)
	if err != nil {
		return nil, err
	}
	proto, port, err := normalizeProbeProtoAndPort(input.ProbeProto, input.Port)
	if err != nil {
		return nil, err
	}
	targetType, err := normalizeProbeTargetType(input.TargetType, model.ProbeTargetTypeCustomerISP)
	if err != nil {
		return nil, err
	}

	asn := input.ASN
	country := strings.ToUpper(strings.TrimSpace(input.Country))
	region := strings.TrimSpace(input.Region)
	var mmdbAnycast string
	if res := netintel.LookupIP(cleanIP); res != nil {
		if asn == 0 {
			asn = res.ASN
		}
		if country == "" {
			country = res.CountryISO
		}
		if region == "" {
			region = res.RegionCode
		}
		mmdbAnycast = res.AnycastStatus
	}
	if country != "" && region != "" {
		region = pkgnetintel.NormalizeMatrixRegion(country, region)
	}
	anycastStatus, err := normalizeProbeAnycastStatus(input.AnycastStatus, mmdbAnycast)
	if err != nil {
		return nil, err
	}

	if existing, lookupErr := repository.GetOpenFlareProbeTargetByEndpoint(ctx, cleanIP, proto, port); lookupErr == nil && existing != nil {
		return nil, errors.New(errProbeTargetDuplicate)
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = cleanIP
	}

	target := &model.OpenFlareProbeTarget{
		Name:          name,
		IP:            cleanIP,
		IPFamily:      ipFamily,
		ProbeProto:    proto,
		Port:          port,
		ASN:           asn,
		Country:       country,
		Region:        region,
		TargetType:    targetType,
		AnycastStatus: anycastStatus,
		Enabled:       enabled,
		Notes:         strings.TrimSpace(input.Notes),
	}
	if err := repository.CreateOpenFlareProbeTarget(ctx, target); err != nil {
		return nil, err
	}
	return target, nil
}

// UpdateProbeTarget modifies metadata, enablement, protocol/port, or anycast classification on an existing target.
func UpdateProbeTarget(ctx context.Context, id uint, input ProbeTargetUpdateInput) (*model.OpenFlareProbeTarget, error) {
	if db.DB(ctx) == nil {
		return nil, errors.New(errDatabaseUnavailable)
	}
	existing, err := repository.GetOpenFlareProbeTargetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errProbeTargetNotFound)
		}
		return nil, err
	}
	if err := applyProbeTargetPatch(existing, input); err != nil {
		return nil, err
	}
	if dup, lookupErr := repository.GetOpenFlareProbeTargetByEndpoint(ctx, existing.IP, existing.ProbeProto, existing.Port); lookupErr == nil &&
		dup != nil && dup.ID != existing.ID {
		return nil, errors.New(errProbeTargetDuplicate)
	}
	if err := repository.UpdateOpenFlareProbeTarget(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func applyProbeTargetPatch(existing *model.OpenFlareProbeTarget, input ProbeTargetUpdateInput) error {
	if input.Name != nil {
		if n := strings.TrimSpace(*input.Name); n != "" {
			existing.Name = n
		}
	}
	if input.ProbeProto != nil || input.Port != nil {
		rawProto := existing.ProbeProto
		if input.ProbeProto != nil {
			rawProto = *input.ProbeProto
		}
		rawPort := existing.Port
		if input.Port != nil {
			rawPort = *input.Port
		}
		proto, port, err := normalizeProbeProtoAndPort(rawProto, rawPort)
		if err != nil {
			return err
		}
		existing.ProbeProto = proto
		existing.Port = port
	}
	if input.ASN != nil {
		existing.ASN = *input.ASN
	}
	if input.Country != nil {
		existing.Country = strings.ToUpper(strings.TrimSpace(*input.Country))
	}
	if input.Region != nil {
		existing.Region = pkgnetintel.NormalizeMatrixRegion(existing.Country, *input.Region)
	} else if input.Country != nil && existing.Region != "" {
		existing.Region = pkgnetintel.NormalizeMatrixRegion(existing.Country, existing.Region)
	}
	if input.TargetType != nil {
		tt, err := normalizeProbeTargetType(*input.TargetType, existing.TargetType)
		if err != nil {
			return err
		}
		existing.TargetType = tt
	}
	if input.AnycastStatus != nil {
		ac, err := normalizeProbeAnycastStatus(*input.AnycastStatus, "")
		if err != nil {
			return err
		}
		existing.AnycastStatus = ac
	}
	if input.Enabled != nil {
		existing.Enabled = *input.Enabled
	}
	if input.Notes != nil {
		existing.Notes = strings.TrimSpace(*input.Notes)
	}
	return nil
}

// DeleteProbeTarget removes a managed probe target and its per-node failure/backoff states.
func DeleteProbeTarget(ctx context.Context, id uint) error {
	if db.DB(ctx) == nil {
		return errors.New(errDatabaseUnavailable)
	}
	err := repository.DeleteOpenFlareProbeTarget(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New(errProbeTargetNotFound)
	}
	return err
}

func normalizeProbeProtoAndPort(rawProto string, port int) (string, int, error) {
	proto := strings.ToLower(strings.TrimSpace(rawProto))
	if proto == "" {
		proto = model.ProbeProtoICMP
	}
	switch proto {
	case model.ProbeProtoICMP:
		return proto, 0, nil
	case model.ProbeProtoTCP:
		if port < 1 || port > maxPortNumber {
			return "", 0, errors.New(errInvalidPort)
		}
		return proto, port, nil
	case model.ProbeProtoTraceroute:
		if port < 0 || port > maxPortNumber {
			return "", 0, errors.New(errInvalidPort)
		}
		return proto, port, nil
	default:
		return "", 0, errors.New(errProbeTargetProto)
	}
}

func normalizeProbeTargetType(raw, fallback string) (string, error) {
	tt := strings.ToLower(strings.TrimSpace(raw))
	if tt == "" {
		tt = fallback
	}
	switch tt {
	case model.ProbeTargetTypeAnchor, model.ProbeTargetTypeCustomerISP, model.ProbeTargetTypeManual:
		return tt, nil
	default:
		return "", errors.New(errProbeTargetType)
	}
}

func normalizeProbeAnycastStatus(raw, fallback string) (string, error) {
	ac := strings.ToLower(strings.TrimSpace(raw))
	if ac == "" {
		ac = strings.ToLower(strings.TrimSpace(fallback))
	}
	if ac == "" {
		ac = pkgnetintel.AnycastStatusUnknown
	}
	switch ac {
	case pkgnetintel.AnycastStatusUnknown, pkgnetintel.AnycastStatusUnicast, pkgnetintel.AnycastStatusConfirmed:
		return ac, nil
	default:
		return "", errors.New(errProbeTargetAnycast)
	}
}
