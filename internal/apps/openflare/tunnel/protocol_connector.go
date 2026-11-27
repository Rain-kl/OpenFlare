// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
)

// AuthenticateTunnelToken verifies an openflared tunnel access token (or wrapped oftok1_ bootstrap token).
func AuthenticateTunnelToken(ctx context.Context, rawToken string) (*model.Tunnel, error) {
	token := ExtractRawToken(rawToken, bootstrapKindTunnel)
	if token == "" {
		return nil, errors.New(errTunnelTokenInvalid)
	}
	tun, err := repository.GetTunnelByAccessToken(ctx, token)
	if err != nil {
		return nil, errors.New(errTunnelTokenInvalid)
	}
	if !constantTimeTokenEqual(tun.AccessToken, token) {
		return nil, errors.New(errTunnelTokenInvalid)
	}
	return tun, nil
}

// CheckConnectorNotRevoked rejects or reports revoked connector replicas.
func CheckConnectorNotRevoked(ctx context.Context, tunnelID uint, connectorID string) (bool, error) {
	trimmed := strings.TrimSpace(connectorID)
	if trimmed == "" {
		return false, nil
	}
	conn, err := repository.GetTunnelConnectorByConnectorID(ctx, trimmed)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if conn.TunnelID != tunnelID {
		return false, nil
	}
	return conn.IsRevoked(), nil
}

// HeartbeatConnector upserts one openflared connector replica and returns sync/revocation/update directives.
func HeartbeatConnector(ctx context.Context, tun *model.Tunnel, payload protocol.TunnelConnectorHeartbeatPayload, clientIP string) (*protocol.TunnelConnectorHeartbeatResponse, error) {
	if tun == nil {
		return nil, errors.New(errTunnelNotFound)
	}
	if tun.IsQuarantined() {
		return nil, errors.New(errTunnelQuarantined)
	}
	connectorID := strings.TrimSpace(payload.ConnectorID)
	if connectorID == "" {
		return nil, errors.New(errTunnelConnectorRequired)
	}

	now := time.Now().UTC()
	existing, err := repository.GetTunnelConnectorByConnectorID(ctx, connectorID)
	if err == nil && existing.IsRevoked() {
		return &protocol.TunnelConnectorHeartbeatResponse{
			TunnelID:          tun.ID,
			TunnelPublicID:    tun.TunnelID,
			ConnectorID:       connectorID,
			ConfigRevision:    tun.ConfigRevision,
			HeartbeatInterval: resolveTunnelHeartbeatMS(ctx),
			Revoked:           true,
			Quarantined:       true,
			QuarantineReason:  errTunnelConnectorRevoked,
		}, nil
	}

	if err := ensureConnectorCapacity(ctx, tun, connectorID, now); err != nil {
		return nil, err
	}

	relaysJSON := "[]"
	if len(payload.ConnectedRelays) > 0 {
		if encoded, err := json.Marshal(payload.ConnectedRelays); err == nil {
			relaysJSON = string(encoded)
		}
	}
	publicIP := strings.TrimSpace(payload.PublicIP)
	if publicIP == "" {
		publicIP = strings.TrimSpace(clientIP)
	}
	status := model.TunnelNodeStatusOnline
	if payload.Draining {
		status = model.TunnelNodeStatusDraining
	}
	conn := &model.TunnelConnector{
		ConnectorID:         connectorID,
		TunnelID:            tun.ID,
		Hostname:            strings.TrimSpace(payload.Hostname),
		OSArch:              strings.TrimSpace(payload.OSArch),
		PublicIP:            publicIP,
		Version:             strings.TrimSpace(payload.Version),
		EngineVersion:       strings.TrimSpace(payload.EngineVersion),
		ContractVersion:     payload.ContractVersion,
		CapabilitiesJSON:    encodeJSONStringSlice(payload.Capabilities),
		AppliedRevision:     payload.AppliedRevision,
		Status:              status,
		LastError:           strings.TrimSpace(payload.LastError),
		ConnectedRelaysJSON: relaysJSON,
		Draining:            payload.Draining,
		LastSeenAt:          &now,
	}
	if err := repository.UpsertTunnelConnector(ctx, conn); err != nil {
		logger.ErrorF(ctx, "upsert tunnel connector failed: tunnel_id=%d connector_id=%s err=%v", tun.ID, connectorID, err)
		return nil, err
	}

	if tun.UpdateRequested && strings.TrimSpace(payload.AppliedUpdateTag) != "" &&
		strings.TrimSpace(payload.AppliedUpdateTag) == strings.TrimSpace(tun.UpdateTag) {
		_ = repository.UpdateTunnelColumns(ctx, tun, map[string]any{colTunnelUpdateRequested: false})
		tun.UpdateRequested = false
	}

	return &protocol.TunnelConnectorHeartbeatResponse{
		TunnelID:          tun.ID,
		TunnelPublicID:    tun.TunnelID,
		ConnectorID:       connectorID,
		ConfigRevision:    tun.ConfigRevision,
		HeartbeatInterval: resolveTunnelHeartbeatMS(ctx),
		ForceSync:         payload.AppliedRevision < tun.ConfigRevision,
		Disabled:          !tun.Enabled,
		Quarantined:       tun.IsQuarantined(),
		QuarantineReason:  tun.QuarantineReason,
		UpdateSettings:    buildTunnelUpdateSettings(ctx, tun.AutoUpdateEnabled, tun.UpdateRequested, tun.UpdateChannel, tun.UpdateTag),
	}, nil
}

// GetConnectorConfig returns candidate relays in the tunnel's pool, existing grants, CIDR policy, and routes.
func GetConnectorConfig(ctx context.Context, tun *model.Tunnel, connectorID string) (*protocol.TunnelConnectorConfigResponse, error) {
	if tun == nil {
		return nil, errors.New(errTunnelNotFound)
	}
	if tun.IsQuarantined() {
		return nil, errors.New(errTunnelQuarantined)
	}
	if revoked, err := CheckConnectorNotRevoked(ctx, tun.ID, connectorID); err != nil {
		return nil, err
	} else if revoked {
		return nil, errors.New(errTunnelConnectorRevoked)
	}

	now := time.Now().UTC()
	minSeenAt := now.Add(-resolveRelayOfflineWindow(ctx))
	candidates, err := repository.ListAvailableRelaysByPool(ctx, tun.PoolID, minSeenAt)
	if err != nil {
		return nil, err
	}
	candidateViews := make([]protocol.TunnelRelayCandidate, 0, len(candidates))
	relayByID := make(map[uint]model.TunnelRelay, len(candidates))
	for i := range candidates {
		r := candidates[i]
		relayByID[r.ID] = r
		gatewayAddr := strings.TrimSpace(r.GatewayAddr)
		if gatewayAddr == "" {
			gatewayAddr = r.PublicAddr
		}
		candidateViews = append(candidateViews, protocol.TunnelRelayCandidate{
			ID:                  r.ID,
			RelayID:             r.RelayID,
			PoolID:              r.PoolID,
			Name:                r.Name,
			PublicAddr:          r.PublicAddr,
			TransportPort:       r.TransportPort,
			WSPort:              r.WSPort,
			GatewayAddr:         gatewayAddr,
			GatewayPort:         r.GatewayPort,
			GatewayTLS:          r.GatewayTLS,
			HealthPath:          protocol.RelayHealthPath,
			SupportedTransports: decodeJSONStringSlice(ctx, r.SupportedTransportsJSON),
			NoisePublicKey:      r.NoisePublicKey,
			GeoName:             r.GeoName,
			GeoLatitude:         cloneCoordinate(r.GeoLatitude),
			GeoLongitude:        cloneCoordinate(r.GeoLongitude),
			ZoneTag:             r.ZoneTag,
			Weight:              r.Weight,
			Draining:            r.Draining,
		})
	}

	grantViews := make([]protocol.TunnelConnectorAttachmentGrant, 0)
	if trimmedID := strings.TrimSpace(connectorID); trimmedID != "" {
		existing, err := repository.ListTunnelAttachments(ctx, tun.ID, 0, trimmedID)
		if err != nil {
			return nil, err
		}
		for _, att := range existing {
			if att.State == model.TunnelAttachmentStateStale {
				continue
			}
			var expiresUnix int64
			if att.LeaseExpiresAt != nil {
				expiresUnix = att.LeaseExpiresAt.Unix()
			}
			grantViews = append(grantViews, protocol.TunnelConnectorAttachmentGrant{
				RelayID:            att.RelayID,
				RelayPublicID:      relayByID[att.RelayID].RelayID,
				ServiceName:        att.ServiceName,
				SessionToken:       att.SessionToken,
				Transport:          att.Transport,
				State:              att.State,
				LeaseExpiresAtUnix: expiresUnix,
			})
		}
	}

	routes, err := repository.ListEnabledProxyRoutesByTunnelID(ctx, tun.ID)
	if err != nil {
		return nil, err
	}
	httpRules := make([]protocol.TunnelHTTPRouteRule, 0, len(routes))
	for _, r := range routes {
		domains := make([]string, 0, len(r.ZoneDomains))
		for _, zd := range r.ZoneDomains {
			domains = append(domains, zd.Domain)
		}
		proto := strings.TrimSpace(r.TunnelTargetProtocol)
		if proto == "" {
			proto = "http"
		}
		httpRules = append(httpRules, protocol.TunnelHTTPRouteRule{
			RouteID:        r.ID,
			SiteName:       r.SiteName,
			Domains:        domains,
			TargetAddr:     r.TunnelTargetAddr,
			TargetProtocol: proto,
			OriginHost:     r.OriginHost,
			OptionsJSON:    r.TunnelOptionsJSON,
		})
	}

	l4Rows, err := repository.ListTunnelServices(ctx, tun.ID, 0)
	if err != nil {
		return nil, err
	}
	l4Rules := make([]protocol.TunnelL4ServiceRule, 0, len(l4Rows))
	for _, svc := range l4Rows {
		if !svc.Enabled {
			continue
		}
		l4Rules = append(l4Rules, protocol.TunnelL4ServiceRule{
			ID:          svc.ID,
			Name:        svc.Name,
			ServiceType: svc.ServiceType,
			RelayID:     svc.RelayID,
			ListenPort:  svc.ListenPort,
			TargetAddr:  svc.TargetAddr,
			OptionsJSON: svc.OptionsJSON,
		})
	}

	return &protocol.TunnelConnectorConfigResponse{
		TunnelID:           tun.ID,
		TunnelPublicID:     tun.TunnelID,
		Name:               tun.Name,
		ConfigRevision:     tun.ConfigRevision,
		Enabled:            tun.Enabled,
		PoolID:             tun.PoolID,
		HARelayCount:       tun.HARelayCount,
		PreferredTransport: tun.PreferredTransport,
		FallbackTransports: decodeJSONStringSlice(ctx, tun.FallbackTransportsJSON),
		ClientProxyURL:     tun.ClientProxyURL,
		AllowedTargetCIDRs: decodeJSONStringSlice(ctx, tun.AllowedTargetCIDRsJSON),
		BlockedTargetCIDRs: decodeJSONStringSlice(ctx, tun.BlockedTargetCIDRsJSON),
		CandidateRelays:    candidateViews,
		Attachments:        grantViews,
		Routes:             httpRules,
		Services:           l4Rules,
	}, nil
}

// NegotiateConnectorAttachments reconciles a connector's closest-K probed relays, reusing existing
// session tokens to prevent unnecessary Rathole tunnel restarts during lease renewals.
func NegotiateConnectorAttachments(
	ctx context.Context,
	tun *model.Tunnel,
	req protocol.TunnelNegotiateAttachmentsRequest,
	clientIP string,
) (*protocol.TunnelNegotiateAttachmentsResponse, error) {
	connectorID, err := validateNegotiateAttachmentsInput(ctx, tun, req)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if err := ensureConnectorCapacity(ctx, tun, connectorID, now); err != nil {
		return nil, err
	}

	if err := upsertNegotiatingConnector(ctx, tun.ID, connectorID, req, clientIP, now); err != nil {
		return nil, err
	}

	existing, err := repository.ListTunnelAttachments(ctx, tun.ID, 0, connectorID)
	if err != nil {
		return nil, err
	}
	existingByRelay := make(map[uint]model.TunnelAttachment, len(existing))
	for _, att := range existing {
		existingByRelay[att.RelayID] = att
	}

	leaseTTLSeconds := resolveTunnelLeaseTTLSeconds(ctx)
	expiresAt := now.Add(time.Duration(leaseTTLSeconds) * time.Second)
	desiredRows := make([]model.TunnelAttachment, 0, len(req.Desired))
	grants := make([]protocol.TunnelConnectorAttachmentGrant, 0, len(req.Desired))
	seenRelays := make(map[uint]struct{}, len(req.Desired))
	wakeRelayIDs := make([]string, 0, len(req.Desired))

	for _, want := range req.Desired {
		if want.RelayID == 0 {
			return nil, errors.New(errTunnelDesiredRelaysInvalid)
		}
		if _, dup := seenRelays[want.RelayID]; dup {
			continue
		}
		seenRelays[want.RelayID] = struct{}{}

		row, grant, relayPublicID, err := negotiateSingleRelayAttachment(ctx, tun, connectorID, want, existingByRelay, expiresAt)
		if err != nil {
			return nil, err
		}
		desiredRows = append(desiredRows, row)
		grants = append(grants, grant)
		wakeRelayIDs = append(wakeRelayIDs, relayPublicID)
	}

	if err := repository.ReconcileConnectorAttachments(ctx, connectorID, desiredRows); err != nil {
		return nil, err
	}

	wakeAffectedRelays(ctx, existingByRelay, seenRelays, wakeRelayIDs)

	return &protocol.TunnelNegotiateAttachmentsResponse{
		TunnelID:        tun.ID,
		ConfigRevision:  tun.ConfigRevision,
		LeaseTTLSeconds: leaseTTLSeconds,
		Attachments:     grants,
	}, nil
}

func validateNegotiateAttachmentsInput(
	ctx context.Context,
	tun *model.Tunnel,
	req protocol.TunnelNegotiateAttachmentsRequest,
) (string, error) {
	if tun == nil {
		return "", errors.New(errTunnelNotFound)
	}
	if tun.IsQuarantined() {
		return "", errors.New(errTunnelQuarantined)
	}
	if !tun.Enabled {
		return "", errors.New(errTunnelDisabled)
	}
	connectorID := strings.TrimSpace(req.ConnectorID)
	if connectorID == "" {
		return "", errors.New(errTunnelConnectorRequired)
	}
	revoked, err := CheckConnectorNotRevoked(ctx, tun.ID, connectorID)
	if err != nil {
		return "", err
	}
	if revoked {
		return "", errors.New(errTunnelConnectorRevoked)
	}
	if len(req.Desired) == 0 {
		return "", errors.New(errTunnelDesiredRelaysInvalid)
	}
	return connectorID, nil
}

func upsertNegotiatingConnector(
	ctx context.Context,
	tunnelID uint,
	connectorID string,
	req protocol.TunnelNegotiateAttachmentsRequest,
	clientIP string,
	now time.Time,
) error {
	publicIP := strings.TrimSpace(req.PublicIP)
	if publicIP == "" {
		publicIP = strings.TrimSpace(clientIP)
	}
	return repository.UpsertTunnelConnector(ctx, &model.TunnelConnector{
		ConnectorID:      connectorID,
		TunnelID:         tunnelID,
		Hostname:         strings.TrimSpace(req.Hostname),
		OSArch:           strings.TrimSpace(req.OSArch),
		PublicIP:         publicIP,
		Version:          strings.TrimSpace(req.Version),
		EngineVersion:    strings.TrimSpace(req.EngineVersion),
		ContractVersion:  req.ContractVersion,
		CapabilitiesJSON: encodeJSONStringSlice(req.Capabilities),
		Status:           model.TunnelNodeStatusOnline,
		LastSeenAt:       &now,
	})
}

func negotiateSingleRelayAttachment(
	ctx context.Context,
	tun *model.Tunnel,
	connectorID string,
	want protocol.TunnelDesiredAttachment,
	existingByRelay map[uint]model.TunnelAttachment,
	expiresAt time.Time,
) (model.TunnelAttachment, protocol.TunnelConnectorAttachmentGrant, string, error) {
	relay, err := repository.GetTunnelRelayByID(ctx, want.RelayID)
	if err != nil || relay.IsQuarantined() || relay.Draining || (tun.PoolID > 0 && relay.PoolID != tun.PoolID) {
		return model.TunnelAttachment{}, protocol.TunnelConnectorAttachmentGrant{}, "", errors.New(errTunnelRelayNotInPool)
	}

	transport := strings.ToLower(strings.TrimSpace(want.Transport))
	if transport == "" {
		transport = model.TunnelTransportNoise
	}
	supported := decodeJSONStringSlice(ctx, relay.SupportedTransportsJSON)
	if !slices.Contains(supported, transport) {
		return model.TunnelAttachment{}, protocol.TunnelConnectorAttachmentGrant{}, "", errors.New(errRelayTransportsInvalid)
	}

	prev, hasPrev := existingByRelay[relay.ID]
	serviceName := fmt.Sprintf("http_t%d_%s", tun.ID, sanitizeConnectorToken(connectorID))
	sessionToken, state, err := resolveAttachmentSession(prev, hasPrev)
	if err != nil {
		return model.TunnelAttachment{}, protocol.TunnelConnectorAttachmentGrant{}, "", err
	}

	row := model.TunnelAttachment{
		TunnelID:       tun.ID,
		ConnectorID:    connectorID,
		RelayID:        relay.ID,
		ServiceName:    serviceName,
		SessionToken:   sessionToken,
		Transport:      transport,
		State:          state,
		RTTMs:          max(0, want.RTTMs),
		LeaseExpiresAt: &expiresAt,
	}
	grant := protocol.TunnelConnectorAttachmentGrant{
		RelayID:            relay.ID,
		RelayPublicID:      relay.RelayID,
		ServiceName:        serviceName,
		SessionToken:       sessionToken,
		Transport:          transport,
		State:              state,
		LeaseExpiresAtUnix: expiresAt.Unix(),
	}
	return row, grant, relay.RelayID, nil
}

func resolveAttachmentSession(prev model.TunnelAttachment, hasPrev bool) (string, string, error) {
	if hasPrev && prev.SessionToken != "" && prev.State != model.TunnelAttachmentStateStale {
		state := model.TunnelAttachmentStateConnecting
		if prev.State == model.TunnelAttachmentStateActive {
			state = model.TunnelAttachmentStateActive
		}
		return prev.SessionToken, state, nil
	}
	sessionToken, err := GenerateRandomToken("ses_", sessionTokenBytes)
	if err != nil {
		return "", "", err
	}
	return sessionToken, model.TunnelAttachmentStateConnecting, nil
}

func wakeAffectedRelays(
	ctx context.Context,
	existingByRelay map[uint]model.TunnelAttachment,
	seenRelays map[uint]struct{},
	wakeRelayIDs []string,
) {
	// Also wake previously attached relays that are now transitioning to draining.
	for relayID := range existingByRelay {
		if _, kept := seenRelays[relayID]; !kept {
			if prevRelay, err := repository.GetTunnelRelayByID(ctx, relayID); err == nil {
				wakeRelayIDs = append(wakeRelayIDs, prevRelay.RelayID)
			}
		}
	}

	woken := make(map[string]struct{}, len(wakeRelayIDs))
	for _, rid := range wakeRelayIDs {
		if rid == "" {
			continue
		}
		if _, ok := woken[rid]; ok {
			continue
		}
		woken[rid] = struct{}{}
		websocket.NotifyTunnelRelayChanged(ctx, rid)
	}
}

func ensureConnectorCapacity(ctx context.Context, tun *model.Tunnel, connectorID string, now time.Time) error {
	if tun.MaxConnectors <= 0 {
		return nil
	}
	minSeen := now.Add(-resolveRelayOfflineWindow(ctx))
	count, err := repository.CountActiveTunnelConnectors(ctx, tun.ID, minSeen, connectorID)
	if err != nil {
		return err
	}
	if int(count) >= tun.MaxConnectors {
		return errors.New(errTunnelConnectorLimitHit)
	}
	return nil
}

func sanitizeConnectorToken(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	cleaned := b.String()
	if len(cleaned) > maxSanitizedConnectorTokenLen {
		return cleaned[:maxSanitizedConnectorTokenLen]
	}
	if cleaned == "" {
		return "conn"
	}
	return cleaned
}
