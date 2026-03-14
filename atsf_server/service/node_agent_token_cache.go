package service

import (
	"atsflare/model"
	"errors"
	"time"

	ristretto "github.com/dgraph-io/ristretto/v2"
	"gorm.io/gorm"
)

const (
	agentTokenPositiveCacheTTL = 2 * time.Minute
	agentTokenNegativeCacheTTL = 10 * time.Minute
	agentTokenNegativeCacheCap = 10000
)

type cachedAgentNode struct {
	node      *model.Node
	expiresAt time.Time
}

type cachedMissingAgentToken struct {
	expiresAt time.Time
}

type agentTokenAuthCache struct {
	positive        *ristretto.Cache[string, cachedAgentNode]
	negative        *ristretto.Cache[string, cachedMissingAgentToken]
	now             func() time.Time
	loadNodeByToken func(string) (*model.Node, error)
}

var nodeAgentTokenCache = newAgentTokenAuthCache()

func newAgentTokenAuthCache() *agentTokenAuthCache {
	return &agentTokenAuthCache{
		positive: mustNewAgentTokenPositiveCache(),
		negative: mustNewAgentTokenNegativeCache(),
		now:      time.Now,
		loadNodeByToken: func(token string) (*model.Node, error) {
			return model.GetNodeByAgentToken(token)
		},
	}
}

func mustNewAgentTokenPositiveCache() *ristretto.Cache[string, cachedAgentNode] {
	cache, err := ristretto.NewCache(&ristretto.Config[string, cachedAgentNode]{
		NumCounters: 1e5,
		MaxCost:     2e4,
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}
	return cache
}

func mustNewAgentTokenNegativeCache() *ristretto.Cache[string, cachedMissingAgentToken] {
	cache, err := ristretto.NewCache(&ristretto.Config[string, cachedMissingAgentToken]{
		NumCounters: 1e5,
		MaxCost:     agentTokenNegativeCacheCap,
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}
	return cache
}

func (c *agentTokenAuthCache) authenticate(token string) (*model.Node, error) {
	now := c.now()
	if node, ok := c.getNode(token, now); ok {
		return node, nil
	}
	if c.isMissing(token, now) {
		return nil, gorm.ErrRecordNotFound
	}

	node, err := c.loadNodeByToken(token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.storeMissing(token, now.Add(agentTokenNegativeCacheTTL))
		}
		return nil, err
	}

	c.storeNode(token, node, now.Add(agentTokenPositiveCacheTTL))
	return cloneCachedNode(node), nil
}

func (c *agentTokenAuthCache) getNode(token string, now time.Time) (*model.Node, bool) {
	entry, ok := c.positive.Get(token)
	if !ok {
		return nil, false
	}
	if now.After(entry.expiresAt) {
		c.positive.Del(token)
		return nil, false
	}
	return cloneCachedNode(entry.node), true
}

func (c *agentTokenAuthCache) isMissing(token string, now time.Time) bool {
	entry, ok := c.negative.Get(token)
	if !ok {
		return false
	}
	if now.After(entry.expiresAt) {
		c.negative.Del(token)
		return false
	}
	return true
}

func (c *agentTokenAuthCache) storeNode(token string, node *model.Node, expiresAt time.Time) {
	if token == "" || node == nil {
		return
	}
	c.negative.Del(token)
	c.positive.Set(token, cachedAgentNode{
		node:      cloneCachedNode(node),
		expiresAt: expiresAt,
	}, 1)
	c.positive.Wait()
}

func (c *agentTokenAuthCache) storeMissing(token string, expiresAt time.Time) {
	if token == "" {
		return
	}
	c.positive.Del(token)
	c.negative.Set(token, cachedMissingAgentToken{
		expiresAt: expiresAt,
	}, 1)
	c.negative.Wait()
}

func (c *agentTokenAuthCache) invalidate(token string) {
	if token == "" {
		return
	}
	c.positive.Del(token)
	c.negative.Del(token)
}

func (c *agentTokenAuthCache) reset() {
	c.positive.Clear()
	c.negative.Clear()
}

func cloneCachedNode(node *model.Node) *model.Node {
	if node == nil {
		return nil
	}
	cloned := *node
	return &cloned
}

func authenticateAgentTokenWithCache(token string) (*model.Node, error) {
	return nodeAgentTokenCache.authenticate(token)
}

func refreshAgentTokenCache(node *model.Node) {
	if node == nil {
		return
	}
	nodeAgentTokenCache.storeNode(
		node.AgentToken,
		node,
		nodeAgentTokenCache.now().Add(agentTokenPositiveCacheTTL),
	)
}

func invalidateAgentTokenCache(token string) {
	nodeAgentTokenCache.invalidate(token)
}
