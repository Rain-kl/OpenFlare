// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

const (
	agentTokenPositiveCacheTTL = 2 * time.Minute
	agentTokenNegativeCacheTTL = 10 * time.Minute
)

type cachedAgentNode struct {
	node      *model.OpenFlareNode
	expiresAt time.Time
}

type accessTokenAuthCache struct {
	mu              sync.RWMutex
	positive        map[string]cachedAgentNode
	negative        map[string]time.Time
	now             func() time.Time
	loadNodeByToken func(context.Context, string) (*model.OpenFlareNode, error)
}

var tokenCache = newAccessTokenAuthCache()

func newAccessTokenAuthCache() *accessTokenAuthCache {
	return &accessTokenAuthCache{
		positive: make(map[string]cachedAgentNode),
		negative: make(map[string]time.Time),
		now:      time.Now,
		loadNodeByToken: func(ctx context.Context, token string) (*model.OpenFlareNode, error) {
			return model.GetOpenFlareNodeByAccessToken(ctx, token)
		},
	}
}

func (c *accessTokenAuthCache) authenticate(ctx context.Context, token string) (*model.OpenFlareNode, error) {
	now := c.now()
	if node, ok := c.getNode(token, now); ok {
		return node, nil
	}
	if c.isMissing(token, now) {
		return nil, gorm.ErrRecordNotFound
	}

	node, err := c.loadNodeByToken(ctx, token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.storeMissing(token, now.Add(agentTokenNegativeCacheTTL))
		}
		return nil, err
	}

	c.storeNode(token, node)
	return cloneNode(node), nil
}

func (c *accessTokenAuthCache) getNode(token string, now time.Time) (*model.OpenFlareNode, bool) {
	c.mu.RLock()
	entry, ok := c.positive[token]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if now.After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.positive, token)
		c.mu.Unlock()
		return nil, false
	}
	return cloneNode(entry.node), true
}

func (c *accessTokenAuthCache) isMissing(token string, now time.Time) bool {
	c.mu.RLock()
	expiresAt, ok := c.negative[token]
	c.mu.RUnlock()
	if !ok {
		return false
	}
	if now.After(expiresAt) {
		c.mu.Lock()
		delete(c.negative, token)
		c.mu.Unlock()
		return false
	}
	return true
}

func (c *accessTokenAuthCache) storeNode(token string, node *model.OpenFlareNode) {
	if token == "" || node == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.negative, token)
	c.positive[token] = cachedAgentNode{
		node:      cloneNode(node),
		expiresAt: c.now().Add(agentTokenPositiveCacheTTL),
	}
}

func (c *accessTokenAuthCache) storeMissing(token string, expiresAt time.Time) {
	if token == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.positive, token)
	c.negative[token] = expiresAt
}

func (c *accessTokenAuthCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.positive = make(map[string]cachedAgentNode)
	c.negative = make(map[string]time.Time)
}

// ResetAuthCacheForTest clears the in-memory access token cache for integration tests.
func ResetAuthCacheForTest() {
	tokenCache.reset()
}

// AuthenticateAccessToken validates X-Agent-Token against of_nodes.access_token.
func AuthenticateAccessToken(ctx context.Context, token string) (*model.OpenFlareNode, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New(errMissingAgentToken)
	}
	return tokenCache.authenticate(ctx, token)
}
