// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cap provides CAPTCHA and proof-of-work (PoW) verification services.
package cap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db"
	pkgcap "github.com/Rain-kl/Wavelet/pkg/cap"
)

const (
	redeemTokenIDLength  = 8  // 兑换 Token ID 字节长度
	redeemVerTokenLength = 15 // 兑换验证 Token 字节长度
	tokenPartsCount      = 2  // 兑换 Token 由两部分组成
	valuePartsCount      = 2  // 存储值由 scope 和过期时间组成
)

// Manager orchestrates challenge generation and solution validation.
type Manager struct {
	secret []byte
	store  pkgcap.Store
}

// NewManager creates a new CAPTCHA Manager.
func NewManager(secret []byte, store pkgcap.Store) *Manager {
	return &Manager{
		secret: secret,
		store:  store,
	}
}

// Generate creates a challenge response.
func (m *Manager) Generate(ctx context.Context, scope string) (*pkgcap.ChallengeResponse, error) {
	settings, err := CurrentSettings(ctx)
	if err != nil {
		return nil, err
	}

	challengeConfig := pkgcap.ChallengeConfig{
		Count:      settings.ChallengeCount,
		Size:       settings.ChallengeSize,
		Difficulty: settings.ChallengeDifficulty,
		Expires:    settings.ChallengeTTL,
	}
	return pkgcap.GenerateChallenge(m.secret, challengeConfig, scope)
}

// RedeemResponse is returned to the client on redeem.
type RedeemResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Expires int64  `json:"expires,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Redeem verifies PoW solutions and returns a one-time redeem token.
func (m *Manager) Redeem(ctx context.Context, token string, solutions []int, scope string) (*RedeemResponse, error) {
	sigHex := pkgcap.JwtSigHex(token)
	if sigHex == "" {
		return &RedeemResponse{Success: false, Error: "invalid_token"}, nil
	}

	nonceKey := "cap:nonce:" + sigHex

	payload, err := pkgcap.VerifyChallengeSolutions(token, solutions, m.secret, scope)
	if err != nil {
		return &RedeemResponse{Success: false, Error: err.Error()}, nil //nolint:nilerr // validation errors are returned as response, not system errors
	}

	now := time.Now().UnixNano() / int64(time.Millisecond)
	nonceTTL := time.Duration(payload.Expires-now) * time.Millisecond
	if nonceTTL < time.Second {
		nonceTTL = time.Second
	}

	set, err := m.store.SetNX(ctx, nonceKey, "1", nonceTTL)
	if err != nil {
		return &RedeemResponse{Success: false, Error: "nonce_store_error"}, err
	}
	if !set {
		return &RedeemResponse{Success: false, Error: "already_redeemed"}, nil
	}

	settings, err := CurrentSettings(ctx)
	if err != nil {
		return &RedeemResponse{Success: false, Error: "settings_load_error"}, err
	}

	id := pkgcap.RandomHex(redeemTokenIDLength)
	verToken := pkgcap.RandomHex(redeemVerTokenLength)
	verHashBytes := sha256.Sum256([]byte(verToken))
	verHashHex := hex.EncodeToString(verHashBytes[:])

	tokenKey := "cap:token:" + id + ":" + verHashHex
	tokenExpires := time.Now().Add(settings.TokenTTL)
	storeVal := strconv.FormatInt(tokenExpires.UnixNano(), 10) + "|" + scope

	if err := m.store.Set(ctx, tokenKey, storeVal, settings.TokenTTL); err != nil {
		return &RedeemResponse{Success: false, Error: "token_store_error"}, err
	}

	return &RedeemResponse{
		Success: true,
		Token:   id + ":" + verToken,
		Expires: tokenExpires.UnixNano() / int64(time.Millisecond),
	}, nil
}

// VerifyToken validates and consumes the redeem token (single-use).
func (m *Manager) VerifyToken(ctx context.Context, token string, expectedScope string) (bool, error) {
	if token == "" {
		return false, nil
	}
	parts := strings.Split(token, ":")
	if len(parts) != tokenPartsCount {
		return false, nil
	}
	id := parts[0]
	verToken := parts[1]

	verHashBytes := sha256.Sum256([]byte(verToken))
	verHashHex := hex.EncodeToString(verHashBytes[:])

	tokenKey := "cap:token:" + id + ":" + verHashHex

	val, exists, err := sGetAndDelete(ctx, m.store, tokenKey)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	valParts := strings.Split(val, "|")
	if len(valParts) != valuePartsCount {
		return false, nil
	}

	expNano, err := strconv.ParseInt(valParts[0], 10, 64)
	if err != nil {
		return false, nil //nolint:nilerr // invalid format is treated as validation failure
	}
	tokenScope := valParts[1]

	if expectedScope != "" && tokenScope != expectedScope {
		return false, nil
	}

	if time.Now().UnixNano() > expNano {
		return false, nil
	}

	return true, nil
}

func sGetAndDelete(ctx context.Context, store pkgcap.Store, key string) (string, bool, error) {
	if store == nil {
		return "", false, nil
	}
	return store.GetAndDelete(ctx, key)
}

var (
	defaultManager *Manager
	once           sync.Once
)

// GetDefaultManager yields the global singleton CAPTCHA manager.
func GetDefaultManager() *Manager {
	once.Do(func() {
		secret := []byte("default-captcha-secret-key-at-least-16-bytes")
		if config.Config != nil && config.Config.App.SessionSecret != "" {
			secret = []byte(config.Config.App.SessionSecret)
		}

		var store pkgcap.Store
		if config.Config != nil && config.Config.Redis.Enabled && db.Redis != nil {
			store = pkgcap.NewRedisStore(db.Redis)
		} else {
			store = pkgcap.NewMemoryStore(1 * time.Minute)
		}

		defaultManager = NewManager(secret, store)
	})
	return defaultManager
}
