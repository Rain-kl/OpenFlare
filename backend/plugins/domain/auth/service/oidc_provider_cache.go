// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package service implements domain business services and orchestration for the auth plugin.
package service

import (
	"context"
	"net/http"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"
)

// OIDCProviderCache 进程级 OIDC provider 缓存。
type OIDCProviderCache struct {
	mu      sync.RWMutex
	entries map[string]*oidc.Provider // key: normalized issuer URL
	sfGroup singleflight.Group
}

// NewOIDCProviderCache creates a new OIDCProviderCache.
func NewOIDCProviderCache() *OIDCProviderCache {
	return &OIDCProviderCache{
		entries: make(map[string]*oidc.Provider),
	}
}

// discoveryContext 从请求 ctx 提取 HTTP 客户端，并绑定到不可取消的 Background ctx。
func discoveryContext(ctx context.Context) context.Context {
	bg := context.Background()
	if client, ok := ctx.Value(oauth2.HTTPClient).(*http.Client); ok && client != nil {
		bg = oidc.ClientContext(bg, client)
	}
	return bg
}

// Get 返回缓存的 provider；若无则通过 oidc.NewProvider 获取并写入缓存。
func (c *OIDCProviderCache) Get(ctx context.Context, issuer string) (*oidc.Provider, error) {
	c.mu.RLock()
	if p, ok := c.entries[issuer]; ok {
		c.mu.RUnlock()
		return p, nil
	}
	c.mu.RUnlock()

	discCtx := discoveryContext(ctx)
	v, err, _ := c.sfGroup.Do(issuer, func() (any, error) {
		c.mu.RLock()
		if p, ok := c.entries[issuer]; ok {
			c.mu.RUnlock()
			return p, nil
		}
		c.mu.RUnlock()

		p, err := oidc.NewProvider(discCtx, issuer)
		if err != nil {
			return nil, err
		}

		c.mu.Lock()
		c.entries[issuer] = p
		c.mu.Unlock()
		return p, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*oidc.Provider), nil //nolint:forcetypeassert
}

// Invalidate 从缓存中移除指定 issuer 对应的 provider。
func (c *OIDCProviderCache) Invalidate(issuer string) {
	c.mu.Lock()
	delete(c.entries, issuer)
	c.mu.Unlock()
}
