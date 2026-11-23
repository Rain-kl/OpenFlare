// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
)

// AuthenticateRelayToken verifies an openflare-relay access token.
func AuthenticateRelayToken(ctx context.Context, rawToken string) (*model.TunnelRelay, error) {
	token := ExtractRawToken(rawToken, bootstrapKindRelay)
	if token == "" {
		return nil, errors.New(errRelayTokenInvalid)
	}
	relay, err := repository.GetTunnelRelayByAccessToken(ctx, token)
	if err != nil {
		return nil, errors.New(errRelayTokenInvalid)
	}
	if !constantTimeTokenEqual(relay.AccessToken, token) {
		return nil, errors.New(errRelayTokenInvalid)
	}
	return relay, nil
}

// ValidateRelayDiscoveryToken verifies the global relay discovery bootstrap token.
func ValidateRelayDiscoveryToken(ctx context.Context, rawToken string) (string, error) {
	token := ExtractRawToken(rawToken, bootstrapKindRelay)
	if token == "" {
		return "", errors.New(errRelayDiscoveryTokenInvalid)
	}
	sc, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyRelayDiscoveryToken)
	if err != nil || strings.TrimSpace(sc.Value) == "" {
		return "", errors.New(errRelayDiscoveryTokenInvalid)
	}
	if !constantTimeTokenEqual(strings.TrimSpace(sc.Value), token) {
		return "", errors.New(errRelayDiscoveryTokenInvalid)
	}
	return token, nil
}

// RegisterRelay handles both idempotent discovery bootstrap registration and access-token re-registration.
func RegisterRelay(ctx context.Context, authRelay *model.TunnelRelay, req protocol.RelayRegisterRequest, clientIP string) (*protocol.RelayRegisterResponse, error) {
	if authRelay != nil {
		if authRelay.IsQuarantined() {
			return nil, errors.New(errRelayQuarantined)
		}
		if req.NoisePublicKey == "" && authRelay.NoisePrivateKey == "" {
			priv, pub, err := GenerateNoiseKeypair()
			if err == nil {
				authRelay.NoisePrivateKey = priv
				authRelay.NoisePublicKey = pub
				_ = repository.UpdateTunnelRelayColumns(ctx, authRelay, map[string]any{
					"noise_private_key": priv,
					"noise_public_key":  pub,
				})
			}
		}
		return &protocol.RelayRegisterResponse{
			ID:              authRelay.ID,
			RelayID:         authRelay.RelayID,
			PoolID:          authRelay.PoolID,
			AccessToken:     authRelay.AccessToken,
			PublicAddr:      authRelay.PublicAddr,
			GatewaySecret:   authRelay.GatewaySecret,
			NoisePrivateKey: authRelay.NoisePrivateKey,
			NoisePublicKey:  authRelay.NoisePublicKey,
		}, nil
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New(errRelayNameRequired)
	}
	publicAddr := strings.TrimSpace(req.PublicAddr)
	if publicAddr == "" {
		publicAddr = strings.TrimSpace(clientIP)
	}
	if publicAddr == "" {
		return nil, errors.New(errRelayPublicAddrRequired)
	}
	poolID, err := resolvePoolIDForMutation(ctx, 0, req.PoolKey)
	if err != nil {
		return nil, err
	}
	transportPort, wsPort, gatewayPort, err := validateRelayPorts(req.TransportPort, req.WSPort, req.GatewayPort)
	if err != nil {
		return nil, err
	}
	transports, err := normalizeRelayTransports(req.SupportedTransports)
	if err != nil {
		return nil, err
	}

	relayID := strings.TrimSpace(req.RelayID)
	if relayID == "" {
		relayID, err = GenerateRandomToken("relay-", 12)
		if err != nil {
			return nil, err
		}
	}
	accessToken, err := GenerateRandomToken("rly_", 24)
	if err != nil {
		return nil, err
	}
	gatewaySecret, err := GenerateRandomToken("gws_", 24)
	if err != nil {
		return nil, err
	}
	privKey, pubKey, err := GenerateNoiseKeypair()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.NoisePublicKey) != "" {
		pubKey = strings.TrimSpace(req.NoisePublicKey)
		privKey = ""
	}
	weight := req.Weight
	if weight <= 0 {
		weight = defaultRelayWeight
	}
	metaJSON, err := encodeJSONObject(req.Metadata)
	if err != nil {
		metaJSON = "{}"
	}
	now := time.Now().UTC()

	candidate := &model.TunnelRelay{
		RelayID:                 relayID,
		PoolID:                  poolID,
		Name:                    name,
		AccessToken:             accessToken,
		PublicAddr:              publicAddr,
		IPManualOverride:        strings.TrimSpace(req.PublicAddr) != "",
		TransportPort:           transportPort,
		WSPort:                  wsPort,
		GatewayAddr:             strings.TrimSpace(req.GatewayAddr),
		GatewayPort:             gatewayPort,
		GatewayTLS:              req.GatewayTLS,
		GatewaySecret:           gatewaySecret,
		SupportedTransportsJSON: encodeJSONStringSlice(transports),
		NoisePrivateKey:         privKey,
		NoisePublicKey:          pubKey,
		GeoName:                 strings.TrimSpace(req.GeoName),
		GeoLatitude:             cloneCoordinate(req.GeoLatitude),
		GeoLongitude:            cloneCoordinate(req.GeoLongitude),
		GeoManualOverride:       strings.TrimSpace(req.GeoName) != "" || (req.GeoLatitude != nil && req.GeoLongitude != nil),
		ZoneTag:                 strings.TrimSpace(req.ZoneTag),
		ColocatedEdgeNodeID:     strings.TrimSpace(req.ColocatedEdgeNodeID),
		Weight:                  weight,
		Status:                  model.TunnelNodeStatusOnline,
		RuntimeStatus:           "healthy",
		Version:                 strings.TrimSpace(req.Version),
		EngineVersion:           strings.TrimSpace(req.EngineVersion),
		ContractVersion:         req.ContractVersion,
		CapabilitiesJSON:        encodeJSONStringSlice(req.Capabilities),
		LastSeenAt:              &now,
		MetadataJSON:            metaJSON,
	}
	applyRelayGeo(ctx, candidate, publicAddr)

	persisted, err := repository.CreateOrGetDiscoveryTunnelRelay(ctx, candidate)
	if err != nil {
		return nil, err
	}
	if persisted.IsQuarantined() {
		return nil, errors.New(errRelayQuarantined)
	}
	websocket.NotifyTunnelPoolsChanged(ctx)
	return &protocol.RelayRegisterResponse{
		ID:              persisted.ID,
		RelayID:         persisted.RelayID,
		PoolID:          persisted.PoolID,
		AccessToken:     persisted.AccessToken,
		PublicAddr:      persisted.PublicAddr,
		GatewaySecret:   persisted.GatewaySecret,
		NoisePrivateKey: persisted.NoisePrivateKey,
		NoisePublicKey:  persisted.NoisePublicKey,
	}, nil
}

// HeartbeatRelay records relay metrics & verified Rathole sessions and refreshes attachment leases.
func HeartbeatRelay(ctx context.Context, relay *model.TunnelRelay, payload protocol.RelayHeartbeatPayload, clientIP string) (*protocol.RelayHeartbeatResponse, error) {
	if relay == nil {
		return nil, errors.New(errRelayNotFound)
	}
	if relay.IsQuarantined() {
		return nil, errors.New(errRelayQuarantined)
	}
	now := time.Now().UTC()
	hbMs := resolveTunnelHeartbeatMS(ctx)
	leaseTTLSeconds := resolveTunnelLeaseTTLSeconds(ctx)
	leaseTTL := time.Duration(leaseTTLSeconds) * time.Second

	changes := map[string]any{
		"status":               model.TunnelNodeStatusOnline,
		"last_seen_at":         &now,
		"active_connectors":    payload.ActiveConnectors,
		"active_streams":       payload.ActiveStreams,
		"cpu_usage_percent":    payload.CPUUsagePercent,
		"memory_usage_percent": payload.MemoryUsagePercent,
		"last_error":           strings.TrimSpace(payload.LastError),
	}
	if v := strings.TrimSpace(payload.Version); v != "" {
		changes["version"] = v
	}
	if ev := strings.TrimSpace(payload.EngineVersion); ev != "" {
		changes["engine_version"] = ev
	}
	if payload.ContractVersion > 0 {
		changes["contract_version"] = payload.ContractVersion
	}
	if payload.Capabilities != nil {
		changes["capabilities_json"] = encodeJSONStringSlice(payload.Capabilities)
	}
	if rs := strings.TrimSpace(payload.RuntimeStatus); rs != "" {
		changes["runtime_status"] = rs
	}
	if !relay.IPManualOverride {
		reportedAddr := strings.TrimSpace(payload.PublicAddr)
		if reportedAddr == "" {
			reportedAddr = strings.TrimSpace(clientIP)
		}
		if reportedAddr != "" && reportedAddr != relay.PublicAddr {
			changes["public_addr"] = reportedAddr
			relay.PublicAddr = reportedAddr
			applyRelayGeo(ctx, relay, reportedAddr)
			changes["geo_name"] = relay.GeoName
			changes["geo_latitude"] = relay.GeoLatitude
			changes["geo_longitude"] = relay.GeoLongitude
		}
	}
	if payload.TransportPort > 0 && payload.TransportPort <= 65535 && payload.TransportPort != relay.TransportPort {
		changes["transport_port"] = payload.TransportPort
	}
	if payload.WSPort >= 0 && payload.WSPort <= 65535 && payload.WSPort != relay.WSPort {
		changes["ws_port"] = payload.WSPort
	}
	if payload.GatewayPort > 0 && payload.GatewayPort <= 65535 && payload.GatewayPort != relay.GatewayPort {
		changes["gateway_port"] = payload.GatewayPort
	}
	if payload.GatewayTLS != nil {
		changes["gateway_tls"] = *payload.GatewayTLS
	}
	if ga := strings.TrimSpace(payload.GatewayAddr); ga != "" {
		changes["gateway_addr"] = ga
	}
	if len(payload.SupportedTransports) > 0 {
		if normalized, err := normalizeRelayTransports(payload.SupportedTransports); err == nil {
			changes["supported_transports_json"] = encodeJSONStringSlice(normalized)
		}
	}
	if relay.UpdateRequested && strings.TrimSpace(payload.AppliedUpdateTag) != "" &&
		strings.TrimSpace(payload.AppliedUpdateTag) == strings.TrimSpace(relay.UpdateTag) {
		changes["update_requested"] = false
		relay.UpdateRequested = false
	}

	if err := repository.UpdateTunnelRelayColumns(ctx, relay, changes); err != nil {
		logger.ErrorF(ctx, "update relay heartbeat failed: relay_id=%s err=%v", relay.RelayID, err)
		return nil, err
	}

	proofs := make([]repository.VerifiedAttachmentProof, 0, len(payload.VerifiedAttachments))
	for _, item := range payload.VerifiedAttachments {
		if strings.TrimSpace(item.ConnectorID) == "" {
			continue
		}
		proofs = append(proofs, repository.VerifiedAttachmentProof{
			ConnectorID:   strings.TrimSpace(item.ConnectorID),
			ServiceName:   strings.TrimSpace(item.ServiceName),
			RTTMs:         item.RTTMs,
			ActiveStreams: item.ActiveStreams,
		})
	}
	routingChanged, err := repository.ReconcileRelayVerifiedAttachments(ctx, relay.ID, proofs, now, leaseTTL)
	if err != nil {
		logger.ErrorF(ctx, "reconcile relay verified attachments failed: relay_id=%s err=%v", relay.RelayID, err)
		return nil, err
	}
	if routingChanged {
		websocket.NotifyAgentConfigChanged(ctx)
	}

	return &protocol.RelayHeartbeatResponse{
		RelayID:           relay.RelayID,
		PoolID:            relay.PoolID,
		HeartbeatInterval: hbMs,
		LeaseTTLSeconds:   leaseTTLSeconds,
		Draining:          relay.Draining,
		Quarantined:       relay.IsQuarantined(),
		QuarantineReason:  relay.QuarantineReason,
		ForceSync:         routingChanged,
		UpdateSettings:    buildTunnelUpdateSettings(ctx, relay.AutoUpdateEnabled, relay.UpdateRequested, relay.UpdateChannel, relay.UpdateTag),
	}, nil
}

// GetRelayRuntimeConfig compiles the authoritative Rathole server + HTTP gateway spec for a relay PoP.
func GetRelayRuntimeConfig(ctx context.Context, relay *model.TunnelRelay) (*protocol.RelayRuntimeConfigResponse, error) {
	if relay == nil {
		return nil, errors.New(errRelayNotFound)
	}
	if relay.IsQuarantined() {
		return nil, errors.New(errRelayQuarantined)
	}
	now := time.Now().UTC()
	attachments, err := repository.ListRelayAuthorizedAttachments(ctx, relay.ID, now)
	if err != nil {
		return nil, err
	}
	tunnels := loadTunnelMap(ctx)
	services := make([]protocol.RelayServiceSpec, 0, len(attachments))
	attachedTunnels := make(map[uint]struct{})

	for _, att := range attachments {
		tun, ok := tunnels[att.TunnelID]
		if !ok || !tun.Enabled || tun.IsQuarantined() {
			continue
		}
		attachedTunnels[tun.ID] = struct{}{}
		services = append(services, protocol.RelayServiceSpec{
			ServiceName:    att.ServiceName,
			ServiceType:    "http_ingress",
			TunnelID:       tun.ID,
			TunnelPublicID: tun.TunnelID,
			ConnectorID:    att.ConnectorID,
			Token:          att.SessionToken,
			Transport:      att.Transport,
			State:          att.State,
		})
	}

	// Include enabled L4 TCP/UDP services assigned explicitly to this relay, or floating (relay_id == 0)
	// for tunnels currently attached to this relay.
	allL4, err := repository.ListTunnelServices(ctx, 0, 0)
	if err == nil {
		for _, svc := range allL4 {
			if !svc.Enabled {
				continue
			}
			tun, ok := tunnels[svc.TunnelID]
			if !ok || !tun.Enabled || tun.IsQuarantined() {
				continue
			}
			if svc.RelayID != relay.ID {
				if svc.RelayID != 0 {
					continue
				}
				if _, attached := attachedTunnels[svc.TunnelID]; !attached {
					continue
				}
			}
			services = append(services, protocol.RelayServiceSpec{
				ServiceName:    l4ServiceName(svc.ID, svc.TunnelID),
				ServiceType:    svc.ServiceType,
				TunnelID:       tun.ID,
				TunnelPublicID: tun.TunnelID,
				Token:          deriveL4ServiceToken(&tun, &svc),
				Transport:      tun.PreferredTransport,
				ListenPort:     svc.ListenPort,
				State:          model.TunnelAttachmentStateActive,
			})
		}
	}

	return &protocol.RelayRuntimeConfigResponse{
		ID:                  relay.ID,
		RelayID:             relay.RelayID,
		PoolID:              relay.PoolID,
		Name:                relay.Name,
		PublicAddr:          relay.PublicAddr,
		TransportPort:       relay.TransportPort,
		WSPort:              relay.WSPort,
		GatewayAddr:         relay.GatewayAddr,
		GatewayPort:         relay.GatewayPort,
		GatewayTLS:          relay.GatewayTLS,
		GatewaySecret:       relay.GatewaySecret,
		SupportedTransports: decodeJSONStringSlice(ctx, relay.SupportedTransportsJSON),
		NoisePrivateKey:     relay.NoisePrivateKey,
		NoisePublicKey:      relay.NoisePublicKey,
		Draining:            relay.Draining,
		LeaseTTLSeconds:     resolveTunnelLeaseTTLSeconds(ctx),
		UpdatedAtUnix:       relay.UpdatedAt.Unix(),
		Services:            services,
	}, nil
}

func loadTunnelMap(ctx context.Context) map[uint]model.Tunnel {
	tunnels, err := repository.ListTunnels(ctx)
	if err != nil {
		return map[uint]model.Tunnel{}
	}
	out := make(map[uint]model.Tunnel, len(tunnels))
	for _, item := range tunnels {
		out[item.ID] = item
	}
	return out
}

func l4ServiceName(serviceID, tunnelID uint) string {
	return fmt.Sprintf("l4_t%d_s%d", tunnelID, serviceID)
}

func deriveL4ServiceToken(tun *model.Tunnel, svc *model.TunnelService) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", tun.AccessToken, tun.ID, svc.ID)))
	return "l4_" + hex.EncodeToString(sum[:16])
}

func isRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
