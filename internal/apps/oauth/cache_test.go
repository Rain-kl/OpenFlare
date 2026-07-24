// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

func setupOauthCacheTest(t *testing.T) (*miniredis.Miniredis, func()) {
	t.Helper()

	miniRedis, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	db.Redis = redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	})

	ResetOauthRAMCacheForTest()

	cleanup := func() {
		StopOauthCacheListener()
		ResetOauthRAMCacheForTest()
		db.Redis.Close()
		miniRedis.Close()
		db.Redis = nil
	}
	return miniRedis, cleanup
}

func TestTokenCache_GetSetInvalidate(t *testing.T) {
	_, cleanup := setupOauthCacheTest(t)
	defer cleanup()
	ctx := context.Background()

	tokenHash := "test-token-hash"
	token := &model.AccessToken{
		ID:        123,
		UserID:    456,
		TokenHash: tokenHash,
		Name:      "test-token",
	}

	// 1. Get from empty cache -> miss
	_, err := GetCachedToken(ctx, tokenHash)
	if err == nil {
		t.Fatal("expected cache miss for un-cached token")
	}

	// 2. Set to cache
	SetCachedToken(ctx, tokenHash, token)

	// 3. Get from cache -> hit
	cached, err := GetCachedToken(ctx, tokenHash)
	if err != nil {
		t.Fatalf("GetCachedToken() failed: %v", err)
	}
	if cached.ID != token.ID || cached.UserID != token.UserID {
		t.Fatalf("expected cached token %+v, got %+v", token, cached)
	}

	// 4. Invalidate cache
	InvalidateCachedToken(ctx, tokenHash)

	// 5. Get from cache -> miss
	_, err = GetCachedToken(ctx, tokenHash)
	if err == nil {
		t.Fatal("expected cache miss after invalidation")
	}
}

func TestUserCache_GetSetInvalidate(t *testing.T) {
	_, cleanup := setupOauthCacheTest(t)
	defer cleanup()
	ctx := context.Background()

	userID := uint64(789)
	user := &model.User{
		ID:       userID,
		Username: "testuser",
		Email:    "test@example.com",
	}

	// 1. Get from empty cache -> miss
	_, err := GetCachedUser(ctx, userID)
	if err == nil {
		t.Fatal("expected cache miss for un-cached user")
	}

	// 2. Set to cache
	SetCachedUser(ctx, userID, user)

	// 3. Get from cache -> hit
	cached, err := GetCachedUser(ctx, userID)
	if err != nil {
		t.Fatalf("GetCachedUser() failed: %v", err)
	}
	if cached.ID != user.ID || cached.Username != user.Username {
		t.Fatalf("expected cached user %+v, got %+v", user, cached)
	}

	// 4. Invalidate cache
	InvalidateCachedUser(ctx, userID)

	// 5. Get from cache -> miss
	_, err = GetCachedUser(ctx, userID)
	if err == nil {
		t.Fatal("expected cache miss after invalidation")
	}
}

func TestOauthCache_PubSubInvalidation(t *testing.T) {
	_, cleanup := setupOauthCacheTest(t)
	defer cleanup()
	ctx := context.Background()

	tokenHash := "pubsub-token-hash"
	token := &model.AccessToken{
		ID:        111,
		UserID:    222,
		TokenHash: tokenHash,
	}

	userID := uint64(333)
	user := &model.User{
		ID:       userID,
		Username: "pubsubuser",
	}

	// Set caches so they are stored in RAM
	SetCachedToken(ctx, tokenHash, token)
	SetCachedUser(ctx, userID, user)

	// Verify they are cached
	if _, ok := tokenRAM.GetIfPresent(tokenHash); !ok {
		t.Fatal("expected token to be in RAM cache")
	}
	if _, ok := userRAM.GetIfPresent(userID); !ok {
		t.Fatal("expected user to be in RAM cache")
	}

	// Give Pub/Sub subscription time to establish
	time.Sleep(100 * time.Millisecond)

	// Publish invalidation messages directly to simulate peer node invalidation
	if err := db.Redis.Publish(ctx, oauthTokenInvalidationChannel, tokenHash).Err(); err != nil {
		t.Fatalf("publish token invalidation error: %v", err)
	}
	if err := db.Redis.Publish(ctx, oauthUserInvalidationChannel, "333").Err(); err != nil {
		t.Fatalf("publish user invalidation error: %v", err)
	}

	// Wait for background pubsub handlers to process messages
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		_, tokenOk := tokenRAM.GetIfPresent(tokenHash)
		_, userOk := userRAM.GetIfPresent(userID)
		if !tokenOk && !userOk {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, ok := tokenRAM.GetIfPresent(tokenHash); ok {
		t.Fatal("expected token RAM cache to be invalidated by Pub/Sub")
	}
	if _, ok := userRAM.GetIfPresent(userID); ok {
		t.Fatal("expected user RAM cache to be invalidated by Pub/Sub")
	}
}

func TestOauthCache_PubSubResetAll(t *testing.T) {
	_, cleanup := setupOauthCacheTest(t)
	defer cleanup()
	ctx := context.Background()

	tokenHash := "reset-token-hash"
	token := &model.AccessToken{
		ID:        444,
		UserID:    555,
		TokenHash: tokenHash,
	}

	userID := uint64(666)
	user := &model.User{
		ID:       userID,
		Username: "resetuser",
	}

	SetCachedToken(ctx, tokenHash, token)
	SetCachedUser(ctx, userID, user)

	// Give Pub/Sub subscription time to establish
	time.Sleep(100 * time.Millisecond)

	// Publish dynamic reset/wildcard to clear all
	if err := db.Redis.Publish(ctx, oauthTokenInvalidationChannel, "*").Err(); err != nil {
		t.Fatalf("publish token reset error: %v", err)
	}
	if err := db.Redis.Publish(ctx, oauthUserInvalidationChannel, "reset").Err(); err != nil {
		t.Fatalf("publish user reset error: %v", err)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		_, tokenOk := tokenRAM.GetIfPresent(tokenHash)
		_, userOk := userRAM.GetIfPresent(userID)
		if !tokenOk && !userOk {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, ok := tokenRAM.GetIfPresent(tokenHash); ok {
		t.Fatal("expected token RAM cache to be fully cleared by '*'")
	}
	if _, ok := userRAM.GetIfPresent(userID); ok {
		t.Fatal("expected user RAM cache to be fully cleared by 'reset'")
	}
}
