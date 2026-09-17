// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package dao provides data access objects and caching for the auth domain plugin.
package dao

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/cache/ram"
	"Wavelet/plugins/domain/auth/consts"
	"Wavelet/plugins/domain/auth/model/do"
	"context"
	"fmt"
)

var (
	tokenRAM = ram.MustNew[string, *do.CachedToken](ram.Options{MaximumSize: 2048})
	userRAM  = ram.MustNew[uint64, *contracts.UserDTO](ram.Options{MaximumSize: 2048})
)

func tokenCacheKey(tokenHash string) string {
	return "oauth:token:" + tokenHash
}

func userCacheKey(userID uint64) string {
	return fmt.Sprintf("oauth:user:%d", userID)
}

// GetCachedToken 获取缓存的 Token
//
//nolint:dupl // token and user cache lookup pattern
func (d *DAO) GetCachedToken(ctx context.Context, tokenHash string) (*do.CachedToken, error) {
	if val, ok := tokenRAM.GetIfPresent(tokenHash); ok {
		return val, nil
	}

	if cache := d.Cache(); cache != nil {
		var token do.CachedToken
		key := tokenCacheKey(tokenHash)
		if err := cache.Get(ctx, key, &token); err == nil {
			tokenRAM.Set(tokenHash, &token)
			return &token, nil
		}
	}
	return nil, fmt.Errorf("cache miss")
}

// SetCachedToken 设置 Token 缓存
func (d *DAO) SetCachedToken(ctx context.Context, tokenHash string, token *do.CachedToken) {
	tokenRAM.Set(tokenHash, token)
	if cache := d.Cache(); cache != nil {
		key := tokenCacheKey(tokenHash)
		_ = cache.Set(ctx, key, token, consts.TokenCacheTTL)
	}
}

// InvalidateCachedToken 吊销/删除 token 缓存
func (d *DAO) InvalidateCachedToken(ctx context.Context, tokenHash string) {
	tokenRAM.Invalidate(tokenHash)
	if cache := d.Cache(); cache != nil {
		key := tokenCacheKey(tokenHash)
		_ = cache.Delete(ctx, key)
	}
}

// GetCachedUser 获取缓存的 UserDTO
//
//nolint:dupl // token and user cache lookup pattern
func (d *DAO) GetCachedUser(ctx context.Context, userID uint64) (*contracts.UserDTO, error) {
	if val, ok := userRAM.GetIfPresent(userID); ok {
		return val, nil
	}

	if cache := d.Cache(); cache != nil {
		var u contracts.UserDTO
		key := userCacheKey(userID)
		if err := cache.Get(ctx, key, &u); err == nil {
			userRAM.Set(userID, &u)
			return &u, nil
		}
	}
	return nil, fmt.Errorf("cache miss")
}

// SetCachedUser 设置 UserDTO 缓存
func (d *DAO) SetCachedUser(ctx context.Context, userID uint64, u *contracts.UserDTO) {
	userRAM.Set(userID, u)
	if cache := d.Cache(); cache != nil {
		key := userCacheKey(userID)
		_ = cache.Set(ctx, key, u, consts.UserCacheTTL)
	}
}

// InvalidateCachedUser 吊销/失效 UserDTO 缓存
func (d *DAO) InvalidateCachedUser(ctx context.Context, userID uint64) {
	userRAM.Invalidate(userID)
	if cache := d.Cache(); cache != nil {
		key := userCacheKey(userID)
		_ = cache.Delete(ctx, key)
	}
}

// ResetRAMCacheForTest clears only the process-local RAM cache.
func ResetRAMCacheForTest() {
	tokenRAM.InvalidateAll()
	userRAM.InvalidateAll()
}
