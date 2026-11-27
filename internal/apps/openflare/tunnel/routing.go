// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const (
	unknownRTTMs    = 999999
	rttQuantizeStep = 20
)

type relayAggregate struct {
	connectors map[string]struct{}
	bestRTTMs  int
}

type scoredRelayEndpoint struct {
	endpoint   protocol.TunnelEdgeRelayEndpoint
	rawBestRTT int
	draining   bool
}

// BuildEdgeTunnelRouting compiles the deterministic tunnel-routing.v1 snapshot for an Edge node.
// Co-located relays on the same node_id are ranked first, followed by non-draining and lowest-RTT relays.
func BuildEdgeTunnelRouting(ctx context.Context, edgeNode *model.OpenFlareNode) (*protocol.TunnelRoutingSnapshot, error) {
	tunnels, err := repository.ListTunnels(ctx)
	if err != nil {
		return nil, err
	}
	relays, err := repository.ListTunnelRelays(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	attachments, err := repository.ListActiveVerifiedAttachments(ctx, now)
	if err != nil {
		return nil, err
	}

	edgeNodeID := ""
	if edgeNode != nil {
		edgeNodeID = strings.TrimSpace(edgeNode.NodeID)
	}
	offlineWindow := resolveRelayOfflineWindow(ctx)
	relayByID := filterOnlineTunnelRelays(relays, now, offlineWindow)
	grouped := groupVerifiedAttachmentsByTunnel(attachments, relayByID)

	// Sort tunnels deterministically by numeric ID ascending so SHA256 never churns on query order.
	sort.Slice(tunnels, func(i, j int) bool {
		return tunnels[i].ID < tunnels[j].ID
	})

	entries := make([]protocol.TunnelEdgeRoutingEntry, 0, len(tunnels))
	var maxStructuralUnix int64
	for _, tun := range tunnels {
		if !tun.Enabled || tun.IsQuarantined() {
			continue
		}
		if ts := tun.UpdatedAt.Unix(); ts > maxStructuralUnix {
			maxStructuralUnix = ts
		}
		entries = append(entries, protocol.TunnelEdgeRoutingEntry{
			TunnelID:           tun.ID,
			TunnelPublicID:     tun.TunnelID,
			Enabled:            tun.Enabled,
			Quarantined:        tun.IsQuarantined(),
			PreferredTransport: tun.PreferredTransport,
			Relays:             rankTunnelRelayEndpoints(grouped[tun.ID], relayByID, edgeNodeID),
		})
	}

	return &protocol.TunnelRoutingSnapshot{
		SchemaVersion:   protocol.TunnelContractVersion,
		GeneratedAtUnix: maxStructuralUnix,
		Tunnels:         entries,
	}, nil
}

func filterOnlineTunnelRelays(relays []model.TunnelRelay, now time.Time, offlineWindow time.Duration) map[uint]model.TunnelRelay {
	relayByID := make(map[uint]model.TunnelRelay, len(relays))
	for _, r := range relays {
		if r.IsQuarantined() || r.GatewayPort <= 0 {
			continue
		}
		if r.LastSeenAt == nil || now.Sub(*r.LastSeenAt) > offlineWindow {
			continue
		}
		if strings.TrimSpace(r.GatewayAddr) == "" && strings.TrimSpace(r.PublicAddr) == "" {
			continue
		}
		relayByID[r.ID] = r
	}
	return relayByID
}

func groupVerifiedAttachmentsByTunnel(attachments []model.TunnelAttachment, relayByID map[uint]model.TunnelRelay) map[uint]map[uint]*relayAggregate {
	grouped := make(map[uint]map[uint]*relayAggregate)
	for _, att := range attachments {
		if _, ok := relayByID[att.RelayID]; !ok {
			continue
		}
		byRelay := grouped[att.TunnelID]
		if byRelay == nil {
			byRelay = make(map[uint]*relayAggregate)
			grouped[att.TunnelID] = byRelay
		}
		agg := byRelay[att.RelayID]
		if agg == nil {
			agg = &relayAggregate{
				connectors: make(map[string]struct{}),
				bestRTTMs:  unknownRTTMs,
			}
			byRelay[att.RelayID] = agg
		}
		agg.connectors[att.ConnectorID] = struct{}{}
		if att.RTTMs > 0 && att.RTTMs < agg.bestRTTMs {
			agg.bestRTTMs = att.RTTMs
		}
	}
	return grouped
}

func rankTunnelRelayEndpoints(byRelay map[uint]*relayAggregate, relayByID map[uint]model.TunnelRelay, edgeNodeID string) []protocol.TunnelEdgeRelayEndpoint {
	scored := make([]scoredRelayEndpoint, 0, len(byRelay))
	for relayID, agg := range byRelay {
		relay := relayByID[relayID]
		gatewayURL := buildRelayGatewayURL(&relay)
		if gatewayURL == "" {
			continue
		}
		isColocated := edgeNodeID != "" && relay.ColocatedEdgeNodeID != "" && relay.ColocatedEdgeNodeID == edgeNodeID
		quantizedRTT := 0
		if agg.bestRTTMs < unknownRTTMs {
			quantizedRTT = quantizeRTT(agg.bestRTTMs)
		}
		scored = append(scored, scoredRelayEndpoint{
			rawBestRTT: agg.bestRTTMs,
			draining:   relay.Draining,
			endpoint: protocol.TunnelEdgeRelayEndpoint{
				RelayID:             relay.ID,
				RelayPublicID:       relay.RelayID,
				GatewayURL:          gatewayURL,
				GatewaySecret:       relay.GatewaySecret,
				ZoneTag:             relay.ZoneTag,
				ColocatedEdgeNodeID: relay.ColocatedEdgeNodeID,
				IsColocated:         isColocated,
				Weight:              relay.Weight,
				BestRTTMs:           quantizedRTT,
				ActiveConnectors:    len(agg.connectors),
			},
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		a, b := scored[i], scored[j]
		if a.endpoint.IsColocated != b.endpoint.IsColocated {
			return a.endpoint.IsColocated
		}
		if a.draining != b.draining {
			return !a.draining
		}
		if a.rawBestRTT != b.rawBestRTT {
			return a.rawBestRTT < b.rawBestRTT
		}
		if a.endpoint.Weight != b.endpoint.Weight {
			return a.endpoint.Weight > b.endpoint.Weight
		}
		return a.endpoint.RelayID < b.endpoint.RelayID
	})

	endpoints := make([]protocol.TunnelEdgeRelayEndpoint, len(scored))
	for i := range scored {
		endpoints[i] = scored[i].endpoint
	}
	return endpoints
}

func buildRelayGatewayURL(relay *model.TunnelRelay) string {
	if relay == nil {
		return ""
	}
	host := strings.TrimSpace(relay.GatewayAddr)
	if host == "" {
		host = strings.TrimSpace(relay.PublicAddr)
	}
	if host == "" || relay.GatewayPort <= 0 {
		return ""
	}
	scheme := "http"
	if relay.GatewayTLS {
		scheme = "https"
	}
	// Strip accidental scheme if stored in GatewayAddr.
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimRight(host, "/")
	return fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(relay.GatewayPort)))
}

func quantizeRTT(rtt int) int {
	if rtt <= 0 {
		return 0
	}
	q := ((rtt + rttQuantizeStep/2) / rttQuantizeStep) * rttQuantizeStep
	if q == 0 {
		return rttQuantizeStep
	}
	return q
}
