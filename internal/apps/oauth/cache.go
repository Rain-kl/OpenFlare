// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/cache/ram"
)

const (
	tokenCacheTTL = 5 * time.Minute
	userCacheTTL  = 5 * time.Minute

	//nolint:gosec // This is a Redis Pub/Sub channel name, not a credential
	oauthTokenInvalidationChannel = "oauth:token_invalidation"
	oauthUserInvalidationChannel  = "oauth:user_invalidation"
)

var (
	tokenRAM = ram.MustNew[string, *model.AccessToken](ram.Options{MaximumSize: 2048})
	userRAM  = ram.MustNew[uint64, *model.User](ram.Options{MaximumSize: 2048})

	tokenListenerOnce   sync.Once
	tokenListenerCtx    context.Context
	tokenListenerCancel context.CancelFunc

	userListenerOnce   sync.Once
	userListenerCtx    context.Context
	userListenerCancel context.CancelFunc
)

func tokenCacheKey(tokenHash string) string {
	return "oauth:token:" + tokenHash
}

func userCacheKey(userID uint64) string {
	return fmt.Sprintf("oauth:user:%d", userID)
}

func ensureTokenCacheListener() {
	if db.Redis == nil {
		return
	}
	tokenListenerOnce.Do(startTokenCacheInvalidationListener)
}

func startTokenCacheInvalidationListener() {
	tokenListenerCtx, tokenListenerCancel = context.WithCancel(context.Background())

	go func() {
		pubsub := db.Redis.Subscribe(tokenListenerCtx, oauthTokenInvalidationChannel)
		defer func() {
			_ = pubsub.Close()
		}()

		go func() {
			<-tokenListenerCtx.Done()
			_ = pubsub.Close()
		}()

		for msg := range pubsub.Channel() {
			tokenHash := msg.Payload
			if tokenHash == "" || tokenHash == "*" || tokenHash == "reset" {
				tokenRAM.InvalidateAll()
			} else {
				tokenRAM.Invalidate(tokenHash)
			}
		}
	}()
}

func publishTokenRAMInvalidation(ctx context.Context, tokenHash string) {
	if db.Redis == nil {
		return
	}
	_ = db.Redis.Publish(ctx, oauthTokenInvalidationChannel, tokenHash).Err()
}

func ensureUserCacheListener() {
	if db.Redis == nil {
		return
	}
	userListenerOnce.Do(startUserCacheInvalidationListener)
}

func startUserCacheInvalidationListener() {
	userListenerCtx, userListenerCancel = context.WithCancel(context.Background())

	go func() {
		pubsub := db.Redis.Subscribe(userListenerCtx, oauthUserInvalidationChannel)
		defer func() {
			_ = pubsub.Close()
		}()

		go func() {
			<-userListenerCtx.Done()
			_ = pubsub.Close()
		}()

		for msg := range pubsub.Channel() {
			userIDStr := msg.Payload
			if userIDStr == "" || userIDStr == "*" || userIDStr == "reset" {
				userRAM.InvalidateAll()
			} else if userID, err := strconv.ParseUint(userIDStr, 10, 64); err == nil {
				userRAM.Invalidate(userID)
			}
		}
	}()
}

func publishUserRAMInvalidation(ctx context.Context, userID uint64) {
	if db.Redis == nil {
		return
	}
	_ = db.Redis.Publish(ctx, oauthUserInvalidationChannel, strconv.FormatUint(userID, 10)).Err()
}

// GetCachedToken 获取缓存的 AccessToken
func GetCachedToken(ctx context.Context, tokenHash string) (*model.AccessToken, error) {
	ensureTokenCacheListener()

	if val, ok := tokenRAM.GetIfPresent(tokenHash); ok {
		return val, nil
	}

	if db.Redis != nil {
		var token model.AccessToken
		key := tokenCacheKey(tokenHash)
		if err := db.GetJSON(ctx, key, &token); err == nil {
			// Write back to local cache
			tokenRAM.Set(tokenHash, &token)
			return &token, nil
		}
	}
	return nil, fmt.Errorf("cache miss")
}

// SetCachedToken 设置 AccessToken 缓存
func SetCachedToken(ctx context.Context, tokenHash string, token *model.AccessToken) {
	ensureTokenCacheListener()

	tokenRAM.Set(tokenHash, token)
	if db.Redis != nil {
		key := tokenCacheKey(tokenHash)
		_ = db.SetJSON(ctx, key, token, tokenCacheTTL)
	}
}

// InvalidateCachedToken 吊销/删除 token 缓存
func InvalidateCachedToken(ctx context.Context, tokenHash string) {
	ensureTokenCacheListener()

	tokenRAM.Invalidate(tokenHash)
	if db.Redis != nil {
		key := tokenCacheKey(tokenHash)
		_ = db.Redis.Del(ctx, db.PrefixedKey(key)).Err()
		publishTokenRAMInvalidation(ctx, tokenHash)
	}
}

// GetCachedUser 获取缓存的 User
func GetCachedUser(ctx context.Context, userID uint64) (*model.User, error) {
	ensureUserCacheListener()

	if val, ok := userRAM.GetIfPresent(userID); ok {
		return val, nil
	}

	if db.Redis != nil {
		var u model.User
		key := userCacheKey(userID)
		if err := db.GetJSON(ctx, key, &u); err == nil {
			// Write back to local cache
			userRAM.Set(userID, &u)
			return &u, nil
		}
	}
	return nil, fmt.Errorf("cache miss")
}

// SetCachedUser 设置 User 缓存
func SetCachedUser(ctx context.Context, userID uint64, u *model.User) {
	ensureUserCacheListener()

	userRAM.Set(userID, u)
	if db.Redis != nil {
		key := userCacheKey(userID)
		_ = db.SetJSON(ctx, key, u, userCacheTTL)
	}
}

// InvalidateCachedUser 吊销/失效 User 缓存
func InvalidateCachedUser(ctx context.Context, userID uint64) {
	ensureUserCacheListener()

	userRAM.Invalidate(userID)
	if db.Redis != nil {
		key := userCacheKey(userID)
		_ = db.Redis.Del(ctx, db.PrefixedKey(key)).Err()
		publishUserRAMInvalidation(ctx, userID)
	}
}

// StopOauthCacheListener stops both token and user Redis Pub/Sub subscription listeners and resets the sync.Once guards.
func StopOauthCacheListener() {
	if tokenListenerCancel != nil {
		tokenListenerCancel()
		tokenListenerCancel = nil
	}
	tokenListenerOnce = sync.Once{}

	if userListenerCancel != nil {
		userListenerCancel()
		userListenerCancel = nil
	}
	userListenerOnce = sync.Once{}
}

// ResetOauthRAMCacheForTest clears only the process-local RAM cache.
func ResetOauthRAMCacheForTest() {
	tokenRAM.InvalidateAll()
	userRAM.InvalidateAll()
}
