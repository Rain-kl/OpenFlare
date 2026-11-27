// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	dnsInstanceContextKey    = "smartdns_instance"
	dnsAuthErrorHeader       = "X-OpenFlare-DNS-Auth-Error"
	dnsAuthErrorInvalidToken = "invalid_token"
	dnsQuarantinedHeader     = "X-OpenFlare-Quarantined"
	dnsQuarantinedHeaderTrue = "true"
)

// DNSInstanceRegisterAuth authenticates standalone openflare-dns self-registration via discovery token or existing access token.
func DNSInstanceRegisterAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		accessTok := extractDNSToken(c, protocol.HeaderDNSToken)
		if accessTok != "" {
			if inst, err := repository.GetDNSInstanceByAccessToken(ctx, accessTok); err == nil && inst != nil {
				if inst.IsQuarantined() {
					c.Header(dnsQuarantinedHeader, dnsQuarantinedHeaderTrue)
					response.AbortForbidden(c, errInstanceQuarantined)
					return
				}
				c.Set(dnsInstanceContextKey, inst)
				c.Next()
				return
			}
		}

		discoveryTok := extractDNSToken(c, protocol.HeaderDNSDiscoveryToken)
		if discoveryTok == "" {
			discoveryTok = accessTok
		}
		if err := validateDiscoveryToken(ctx, discoveryTok); err != nil {
			response.AbortUnauthorized(c, errDiscoveryTokenInvalid)
			return
		}
		c.Next()
	}
}

// DNSInstanceAuth authenticates ongoing heartbeat, snapshot pull, and WebSocket requests from a registered SmartDNS instance.
func DNSInstanceAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		token := extractDNSToken(c, protocol.HeaderDNSToken)
		if token == "" {
			c.Header(dnsAuthErrorHeader, dnsAuthErrorInvalidToken)
			response.AbortUnauthorized(c, errTokenInvalid)
			return
		}
		inst, err := repository.GetDNSInstanceByAccessToken(ctx, token)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				response.AbortInternal(c, errTokenInvalid)
				return
			}
			c.Header(dnsAuthErrorHeader, dnsAuthErrorInvalidToken)
			response.AbortUnauthorized(c, errTokenInvalid)
			return
		}
		if inst.IsQuarantined() {
			c.Header(dnsQuarantinedHeader, dnsQuarantinedHeaderTrue)
			response.AbortForbidden(c, errInstanceQuarantined)
			return
		}
		c.Set(dnsInstanceContextKey, inst)
		c.Next()
	}
}

// InstanceFromContext returns the authenticated SmartDNS instance from Gin context.
func InstanceFromContext(c *gin.Context) (*model.OpenFlareDNSInstance, bool) {
	v, ok := c.Get(dnsInstanceContextKey)
	if !ok {
		return nil, false
	}
	inst, ok := v.(*model.OpenFlareDNSInstance)
	return inst, ok
}

func extractDNSToken(c *gin.Context, header string) string {
	if val := strings.TrimSpace(c.GetHeader(header)); val != "" {
		return val
	}
	if auth := strings.TrimSpace(c.GetHeader("Authorization")); auth != "" {
		if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
			return strings.TrimSpace(auth[7:])
		}
	}
	if q := strings.TrimSpace(c.Query("token")); q != "" {
		return q
	}
	return ""
}

func validateDiscoveryToken(ctx context.Context, provided string) error {
	trimmed := strings.TrimSpace(provided)
	if trimmed == "" {
		return errors.New(errDiscoveryTokenInvalid)
	}
	expected, err := ensureSmartDNSDiscoveryToken(ctx)
	if err != nil || strings.TrimSpace(expected) == "" {
		return errors.New(errDiscoveryTokenInvalid)
	}
	if subtle.ConstantTimeCompare([]byte(trimmed), []byte(expected)) != 1 {
		return errors.New(errDiscoveryTokenInvalid)
	}
	return nil
}
