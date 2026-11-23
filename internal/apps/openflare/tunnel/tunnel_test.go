// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTunnelTestDB(t *testing.T) (context.Context, *gorm.DB) {
	t.Helper()
	gdb, _, cleanup := testhelper.SetupTestEnvironment(t)
	t.Cleanup(cleanup)
	require.NoError(t, gdb.AutoMigrate(
		&model.TunnelRelayPool{},
		&model.TunnelRelay{},
		&model.Tunnel{},
		&model.TunnelConnector{},
		&model.TunnelAttachment{},
		&model.TunnelService{},
		&model.Zone{},
		&model.ZoneDomain{},
		&model.ProxyRoute{},
		&model.OpenFlareNode{},
	))
	ctx := context.Background()

	pool, err := repository.GetDefaultTunnelRelayPool(ctx)
	if err != nil || pool == nil {
		require.NoError(t, repository.CreateTunnelRelayPool(ctx, &model.TunnelRelayPool{
			PoolKey:           "default",
			Name:              "Default Pool",
			IsDefault:         true,
			RoutingPolicyJSON: "{}",
		}))
	}
	return ctx, gdb
}

func TestGenerateNoiseKeypairAndBootstrapCodec(t *testing.T) {
	priv, pub, err := GenerateNoiseKeypair()
	require.NoError(t, err)
	privBytes, err := base64.StdEncoding.DecodeString(priv)
	require.NoError(t, err)
	pubBytes, err := base64.StdEncoding.DecodeString(pub)
	require.NoError(t, err)
	assert.Len(t, privBytes, 32)
	assert.Len(t, pubBytes, 32)
	assert.NotEqual(t, priv, pub)

	wrapped, err := EncodeBootstrapToken("https://ctrl.example.com/", bootstrapKindTunnel, "tun_abc123", "apac")
	require.NoError(t, err)
	assert.Contains(t, wrapped, "oftok1_")

	decoded, err := DecodeBootstrapToken(wrapped)
	require.NoError(t, err)
	assert.Equal(t, 1, decoded.Version)
	assert.Equal(t, "https://ctrl.example.com", decoded.ServerURL)
	assert.Equal(t, bootstrapKindTunnel, decoded.Kind)
	assert.Equal(t, "tun_abc123", decoded.Token)
	assert.Equal(t, "apac", decoded.PoolKey)
	assert.Equal(t, "tun_abc123", ExtractRawToken(wrapped, bootstrapKindTunnel))
	assert.Equal(t, "raw_secret", ExtractRawToken("raw_secret", bootstrapKindTunnel))
}

func TestRelayRegistrationNegotiationHeartbeatAndDeterministicEdgeRouting(t *testing.T) {
	ctx, gdb := setupTunnelTestDB(t)

	// 1. Register two relays (one co-located with edge-1, one remote lower-RTT).
	bootstrap, err := GetRelayBootstrapToken(ctx)
	require.NoError(t, err)
	_, err = ValidateRelayDiscoveryToken(ctx, bootstrap.BootstrapToken)
	require.NoError(t, err)

	reg1, err := RegisterRelay(ctx, nil, protocol.RelayRegisterRequest{
		RelayID:             "relay-edge1",
		Name:                "HK Edge1 Colocated",
		PublicAddr:          "198.51.100.10",
		TransportPort:       7443,
		GatewayPort:         8443,
		ZoneTag:             "hk-a",
		ColocatedEdgeNodeID: "edge-1",
		Weight:              100,
	}, "198.51.100.10")
	require.NoError(t, err)
	assert.NotEmpty(t, reg1.NoisePrivateKey)
	assert.NotEmpty(t, reg1.NoisePublicKey)
	assert.NotEmpty(t, reg1.GatewaySecret)

	reg2, err := RegisterRelay(ctx, nil, protocol.RelayRegisterRequest{
		RelayID:       "relay-fast",
		Name:          "HK Fast Remote",
		PublicAddr:    "198.51.100.20",
		TransportPort: 7443,
		GatewayPort:   8443,
		ZoneTag:       "hk-b",
		Weight:        120,
	}, "198.51.100.20")
	require.NoError(t, err)

	// 2. Create logical tunnel + HTTP proxy route + L4 TCP service.
	tunDeploy, err := CreateTunnel(ctx, TunnelInput{
		Name:               "Prod Intranet",
		HARelayCount:       2,
		PreferredTransport: "noise",
		AllowedTargetCIDRs: []string{"10.0.0.0/8"},
	})
	require.NoError(t, err)
	tunID := tunDeploy.Tunnel.ID

	zone := &model.Zone{Domain: "example.com"}
	require.NoError(t, gdb.Create(zone).Error)
	zd := &model.ZoneDomain{ZoneID: zone.ID, Domain: "app.example.com"}
	require.NoError(t, gdb.Create(zd).Error)
	route := &model.ProxyRoute{
		SiteName:             "app-tunnel",
		UpstreamType:         "tunnel",
		TunnelID:             &tunID,
		TunnelTargetAddr:     "10.0.0.5:8080",
		TunnelTargetProtocol: "http",
		Enabled:              true,
	}
	require.NoError(t, repository.CreateProxyRouteRecord(ctx, route))
	require.NoError(t, repository.BindZoneDomainTx(gdb, zd, route.ID))

	svc, err := CreateTunnelService(ctx, tunID, TunnelServiceInput{
		Name:        "ssh-admin",
		ServiceType: "tcp",
		RelayID:     reg2.ID,
		ListenPort:  2222,
		TargetAddr:  "10.0.0.5:22",
	})
	require.NoError(t, err)
	assert.Equal(t, 2222, svc.ListenPort)

	// 3. Connector pulls config and negotiates closest-2 attachments.
	tunModel, err := AuthenticateTunnelToken(ctx, tunDeploy.BootstrapToken)
	require.NoError(t, err)
	cfgResp, err := GetConnectorConfig(ctx, tunModel, "conn-alpha")
	require.NoError(t, err)
	assert.Len(t, cfgResp.CandidateRelays, 2)
	assert.Len(t, cfgResp.Routes, 1)
	assert.Equal(t, []string{"app.example.com"}, cfgResp.Routes[0].Domains)
	assert.Len(t, cfgResp.Services, 1)

	neg1, err := NegotiateConnectorAttachments(ctx, tunModel, protocol.TunnelNegotiateAttachmentsRequest{
		ConnectorID: "conn-alpha",
		Version:     "0.1.0",
		Desired: []protocol.TunnelDesiredAttachment{
			{RelayID: reg1.ID, Transport: "noise", RTTMs: 35},
			{RelayID: reg2.ID, Transport: "noise", RTTMs: 12},
		},
	}, "203.0.113.5")
	require.NoError(t, err)
	assert.Len(t, neg1.Attachments, 2)
	token1 := neg1.Attachments[0].SessionToken

	// Renewing same relay MUST preserve existing session_token so Rathole session isn't reset.
	neg2, err := NegotiateConnectorAttachments(ctx, tunModel, protocol.TunnelNegotiateAttachmentsRequest{
		ConnectorID: "conn-alpha",
		Desired: []protocol.TunnelDesiredAttachment{
			{RelayID: reg1.ID, Transport: "noise", RTTMs: 36},
			{RelayID: reg2.ID, Transport: "noise", RTTMs: 14},
		},
	}, "203.0.113.5")
	require.NoError(t, err)
	assert.Equal(t, token1, neg2.Attachments[0].SessionToken)

	// 4. Both relays pull runtime config and report verified attachments via heartbeat.
	relay1Model, err := AuthenticateRelayToken(ctx, reg1.AccessToken)
	require.NoError(t, err)
	relay2Model, err := AuthenticateRelayToken(ctx, reg2.AccessToken)
	require.NoError(t, err)

	relay1Cfg, err := GetRelayRuntimeConfig(ctx, relay1Model)
	require.NoError(t, err)
	assert.Len(t, relay1Cfg.Services, 1)
	relay2Cfg, err := GetRelayRuntimeConfig(ctx, relay2Model)
	require.NoError(t, err)
	assert.Len(t, relay2Cfg.Services, 2) // 1 http_ingress + 1 L4 tcp service bound to relay2

	hb1, err := HeartbeatRelay(ctx, relay1Model, protocol.RelayHeartbeatPayload{
		Version:          "0.1.0",
		ActiveConnectors: 1,
		VerifiedAttachments: []protocol.RelayVerifiedAttachment{
			{ConnectorID: "conn-alpha", ServiceName: relay1Cfg.Services[0].ServiceName, RTTMs: 35, ActiveStreams: 2},
		},
	}, "198.51.100.10")
	require.NoError(t, err)
	assert.True(t, hb1.ForceSync)

	hb2, err := HeartbeatRelay(ctx, relay2Model, protocol.RelayHeartbeatPayload{
		Version:          "0.1.0",
		ActiveConnectors: 1,
		VerifiedAttachments: []protocol.RelayVerifiedAttachment{
			{ConnectorID: "conn-alpha", ServiceName: relay2Cfg.Services[0].ServiceName, RTTMs: 12, ActiveStreams: 5},
		},
	}, "198.51.100.20")
	require.NoError(t, err)
	assert.True(t, hb2.ForceSync)

	// 5. Verify Edge tunnel-routing.v1 ordering:
	// - edge-1 (colocated with relay1) MUST rank relay1 first despite higher RTT (35 vs 12).
	// - edge-2 (remote) MUST rank relay2 first (lower RTT 12 vs 35).
	routingEdge1, err := BuildEdgeTunnelRouting(ctx, &model.OpenFlareNode{NodeID: "edge-1"})
	require.NoError(t, err)
	require.Len(t, routingEdge1.Tunnels, 1)
	require.Len(t, routingEdge1.Tunnels[0].Relays, 2)
	assert.Equal(t, "relay-edge1", routingEdge1.Tunnels[0].Relays[0].RelayPublicID)
	assert.True(t, routingEdge1.Tunnels[0].Relays[0].IsColocated)
	assert.Equal(t, "relay-fast", routingEdge1.Tunnels[0].Relays[1].RelayPublicID)

	// Determinism check: subsequent heartbeat with tiny RTT fluctuation must NOT change Edge snapshot.
	_, err = HeartbeatRelay(ctx, relay1Model, protocol.RelayHeartbeatPayload{
		Version: "0.1.0",
		VerifiedAttachments: []protocol.RelayVerifiedAttachment{
			{ConnectorID: "conn-alpha", ServiceName: relay1Cfg.Services[0].ServiceName, RTTMs: 34, ActiveStreams: 9},
		},
	}, "198.51.100.10")
	require.NoError(t, err)
	routingEdge1Second, err := BuildEdgeTunnelRouting(ctx, &model.OpenFlareNode{NodeID: "edge-1"})
	require.NoError(t, err)
	assert.Equal(t, routingEdge1, routingEdge1Second)

	routingEdge2, err := BuildEdgeTunnelRouting(ctx, &model.OpenFlareNode{NodeID: "edge-2"})
	require.NoError(t, err)
	assert.Equal(t, "relay-fast", routingEdge2.Tunnels[0].Relays[0].RelayPublicID)
	assert.False(t, routingEdge2.Tunnels[0].Relays[0].IsColocated)

	// 6. Quarantine relay1 -> token invalid & removed from Edge routing.
	_, err = QuarantineRelay(ctx, reg1.ID, "compromised key")
	require.NoError(t, err)
	_, err = AuthenticateRelayToken(ctx, reg1.AccessToken)
	assert.Error(t, err)
	routingAfterQuarantine, err := BuildEdgeTunnelRouting(ctx, &model.OpenFlareNode{NodeID: "edge-1"})
	require.NoError(t, err)
	require.Len(t, routingAfterQuarantine.Tunnels[0].Relays, 1)
	assert.Equal(t, "relay-fast", routingAfterQuarantine.Tunnels[0].Relays[0].RelayPublicID)

	// 7. Revoke connector -> rejected and attachments removed.
	require.NoError(t, RevokeConnector(ctx, tunID, "conn-alpha"))
	revoked, err := CheckConnectorNotRevoked(ctx, tunID, "conn-alpha")
	require.NoError(t, err)
	assert.True(t, revoked)
	hbConn, err := HeartbeatConnector(ctx, tunModel, protocol.TunnelConnectorHeartbeatPayload{
		ConnectorID: "conn-alpha",
	}, "203.0.113.5")
	require.NoError(t, err)
	assert.True(t, hbConn.Revoked)
}

func TestGetLatestTunnelReleaseFiltering(t *testing.T) {
	ctx, _ := setupTunnelTestDB(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `[
			{"tag_name":"edge-v9.9.9","name":"Edge Release","prerelease":false,"draft":false},
			{"tag_name":"tunnel-v0.3.0-rc.1","name":"Tunnel RC","prerelease":true,"draft":false},
			{"tag_name":"tunnel-v0.2.1","name":"Tunnel 0.2.1","prerelease":false,"draft":false},
			{"tag_name":"tunnel-v0.1.0","name":"Tunnel 0.1.0","prerelease":false,"draft":false}
		]`)
	}))
	defer srv.Close()

	oldFn := releaseHTTPClientFn
	releaseHTTPClientFn = func() *http.Client { return srv.Client() }
	defer func() { releaseHTTPClientFn = oldFn }()

	// Intercept GitHub endpoint by pointing githubReleasesEndpointTemplate? Instead use RoundTripper on srv.Client.
	client := &http.Client{
		Timeout: time.Second,
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			cloned := req.Clone(req.Context())
			cloned.URL.Scheme = "http"
			cloned.URL.Host = srv.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(cloned)
		}),
	}
	releaseHTTPClientFn = func() *http.Client { return client }

	stable, err := GetLatestTunnelRelease(ctx, "stable")
	require.NoError(t, err)
	assert.Equal(t, "tunnel-v0.2.1", stable.TagName)

	preview, err := GetLatestTunnelRelease(ctx, "preview")
	require.NoError(t, err)
	assert.Equal(t, "tunnel-v0.3.0-rc.1", preview.TagName)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}
