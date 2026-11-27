// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"net"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
)

const (
	ctxRelayKey               = "tunnel_relay"
	ctxTunnelKey              = "tunnel_entity"
	ctxRelayDiscoveryKey      = "relay_discovery"
	quarantinedHeader         = "X-OpenFlare-Quarantined"
	quarantinedHeaderTrue     = "true"
	connectorRevokedHeader    = "X-OpenFlare-Connector-Revoked"
	connectorRevokedTrue      = "true"
	authErrorHeader           = "X-OpenFlare-Auth-Error"
	authErrorInvalidToken     = "invalid_token"
	authErrorRevokedConnector = "connector_revoked"
)

func extractRelayHeaderToken(c *gin.Context) string {
	if token := strings.TrimSpace(c.GetHeader(protocol.HeaderRelayToken)); token != "" {
		return token
	}
	if token := strings.TrimSpace(c.GetHeader(protocol.HeaderRelayBootstrapToken)); token != "" {
		return token
	}
	return extractBearerToken(c)
}

func extractTunnelHeaderToken(c *gin.Context) string {
	if token := strings.TrimSpace(c.GetHeader(protocol.HeaderTunnelToken)); token != "" {
		return token
	}
	return extractBearerToken(c)
}

func extractBearerToken(c *gin.Context) string {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

// RelayRegisterAuth validates either an existing relay access token or the global relay discovery token.
func RelayRegisterAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawToken := extractRelayHeaderToken(c)
		if relay, err := AuthenticateRelayToken(c.Request.Context(), rawToken); err == nil {
			if relay.IsQuarantined() {
				c.Header(quarantinedHeader, quarantinedHeaderTrue)
				response.AbortForbidden(c, errRelayQuarantined)
				return
			}
			c.Set(ctxRelayKey, relay)
			c.Next()
			return
		}
		if _, err := ValidateRelayDiscoveryToken(c.Request.Context(), rawToken); err != nil {
			c.Header(authErrorHeader, authErrorInvalidToken)
			response.AbortUnauthorized(c, errRelayDiscoveryTokenInvalid)
			return
		}
		c.Set(ctxRelayDiscoveryKey, true)
		c.Next()
	}
}

// RelayAuth authenticates an openflare-relay request using X-Relay-Token and blocks quarantined relays.
func RelayAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawToken := extractRelayHeaderToken(c)
		relay, err := AuthenticateRelayToken(c.Request.Context(), rawToken)
		if err != nil {
			c.Header(authErrorHeader, authErrorInvalidToken)
			response.AbortUnauthorized(c, errRelayTokenInvalid)
			return
		}
		if relay.IsQuarantined() {
			c.Header(quarantinedHeader, quarantinedHeaderTrue)
			response.AbortForbidden(c, errRelayQuarantined)
			return
		}
		c.Set(ctxRelayKey, relay)
		c.Next()
	}
}

// TunnelAuth authenticates an openflared connector request and rejects quarantined tunnels or revoked connectors.
//
//nolint:revive // keeps explicit Tunnel prefix to distinguish from Relay and RelayPool types/handlers and preserve Swagger schema
func TunnelAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawToken := extractTunnelHeaderToken(c)
		tun, err := AuthenticateTunnelToken(c.Request.Context(), rawToken)
		if err != nil {
			c.Header(authErrorHeader, authErrorInvalidToken)
			response.AbortUnauthorized(c, errTunnelTokenInvalid)
			return
		}
		if tun.IsQuarantined() {
			c.Header(quarantinedHeader, quarantinedHeaderTrue)
			response.AbortForbidden(c, errTunnelQuarantined)
			return
		}
		if connID := connectorIDFromRequest(c); connID != "" {
			revoked, revErr := CheckConnectorNotRevoked(c.Request.Context(), tun.ID, connID)
			if revErr == nil && revoked {
				c.Header(connectorRevokedHeader, connectorRevokedTrue)
				c.Header(authErrorHeader, authErrorRevokedConnector)
				response.AbortForbidden(c, errTunnelConnectorRevoked)
				return
			}
		}
		c.Set(ctxTunnelKey, tun)
		c.Next()
	}
}

// RelayFromContext returns the authenticated relay from Gin context.
func RelayFromContext(c *gin.Context) (*model.TunnelRelay, bool) {
	value, ok := c.Get(ctxRelayKey)
	if !ok {
		return nil, false
	}
	relay, ok := value.(*model.TunnelRelay)
	return relay, ok
}

// TunnelFromContext returns the authenticated tunnel from Gin context.
//
//nolint:revive // keeps explicit Tunnel prefix to distinguish from Relay and RelayPool types/handlers and preserve Swagger schema
func TunnelFromContext(c *gin.Context) (*model.Tunnel, bool) {
	value, ok := c.Get(ctxTunnelKey)
	if !ok {
		return nil, false
	}
	tun, ok := value.(*model.Tunnel)
	return tun, ok
}

func connectorIDFromRequest(c *gin.Context) string {
	if id := strings.TrimSpace(c.GetHeader(protocol.HeaderConnectorID)); id != "" {
		return id
	}
	return strings.TrimSpace(c.Query("connector_id"))
}

func clientIPFromRequest(c *gin.Context) string {
	if ip := strings.TrimSpace(c.ClientIP()); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return ""
}
