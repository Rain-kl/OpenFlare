package service

import (
	"atsflare/model"
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestAgentTokenAuthCacheUsesPositiveCacheUntilLogicalExpiry(t *testing.T) {
	cache := newAgentTokenAuthCache()
	cache.reset()
	baseTime := time.Date(2026, 3, 14, 16, 0, 0, 0, time.UTC)
	currentTime := baseTime
	cache.now = func() time.Time {
		return currentTime
	}

	loadCount := 0
	cache.loadNodeByToken = func(token string) (*model.Node, error) {
		loadCount++
		return &model.Node{
			NodeID:     fmt.Sprintf("node-%d", loadCount),
			Name:       "edge",
			AgentToken: token,
		}, nil
	}

	first, err := cache.authenticate("token-a")
	if err != nil {
		t.Fatalf("expected first auth to succeed: %v", err)
	}
	if loadCount != 1 {
		t.Fatalf("expected one db load, got %d", loadCount)
	}

	second, err := cache.authenticate("token-a")
	if err != nil {
		t.Fatalf("expected cached auth to succeed: %v", err)
	}
	if loadCount != 1 {
		t.Fatalf("expected cache hit without db load, got %d", loadCount)
	}
	if first.NodeID != second.NodeID {
		t.Fatalf("expected cached node to match original, got %s and %s", first.NodeID, second.NodeID)
	}

	currentTime = baseTime.Add(agentTokenPositiveCacheTTL + time.Second)
	third, err := cache.authenticate("token-a")
	if err != nil {
		t.Fatalf("expected auth after expiry to succeed: %v", err)
	}
	if loadCount != 2 {
		t.Fatalf("expected reload after logical expiry, got %d loads", loadCount)
	}
	if third.NodeID == second.NodeID {
		t.Fatalf("expected refreshed cache entry after expiry, got unchanged node id %s", third.NodeID)
	}
}

func TestAgentTokenAuthCacheRefreshesAfterMissingEntryExpires(t *testing.T) {
	cache := newAgentTokenAuthCache()
	cache.reset()
	baseTime := time.Date(2026, 3, 14, 16, 30, 0, 0, time.UTC)
	currentTime := baseTime
	cache.now = func() time.Time {
		return currentTime
	}

	loadCount := 0
	cache.loadNodeByToken = func(token string) (*model.Node, error) {
		loadCount++
		if loadCount == 1 {
			return nil, gorm.ErrRecordNotFound
		}
		return &model.Node{
			NodeID:     "node-recovered",
			Name:       "edge",
			AgentToken: token,
		}, nil
	}

	_, err := cache.authenticate("token-missing")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected first lookup to miss, got %v", err)
	}
	if loadCount != 1 {
		t.Fatalf("expected one db load for first miss, got %d", loadCount)
	}

	_, err = cache.authenticate("token-missing")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected cached missing lookup to miss, got %v", err)
	}
	if loadCount != 1 {
		t.Fatalf("expected missing cache hit without db load, got %d", loadCount)
	}

	currentTime = baseTime.Add(agentTokenNegativeCacheTTL + time.Second)
	node, err := cache.authenticate("token-missing")
	if err != nil {
		t.Fatalf("expected lookup after missing expiry to reload successfully: %v", err)
	}
	if loadCount != 2 {
		t.Fatalf("expected db reload after missing cache expiry, got %d", loadCount)
	}
	if node.NodeID != "node-recovered" {
		t.Fatalf("unexpected recovered node: %+v", node)
	}
}
