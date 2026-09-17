// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package service implements domain business services and orchestration for the auth plugin.
package service

import (
	"Wavelet/core/contracts"
	"Wavelet/plugins/domain/auth/consts"
	"Wavelet/plugins/domain/auth/model/dto"
	"Wavelet/plugins/domain/auth/pow"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// CaptchaManager orchestrates challenge generation and solution validation.
type CaptchaManager struct {
	secret      []byte
	store       pow.Store
	settingsMgr *CapSettingsManager
}

// NewCaptchaManager creates a new CAPTCHA Manager.
func NewCaptchaManager(secret []byte, store pow.Store, settingsMgr *CapSettingsManager) *CaptchaManager {
	return &CaptchaManager{
		secret:      secret,
		store:       store,
		settingsMgr: settingsMgr,
	}
}

// SetSecret updates the shared secret used for PoW generation and validation.
func (m *CaptchaManager) SetSecret(secret []byte) {
	m.secret = secret
}

// Generate creates a challenge response.
func (m *CaptchaManager) Generate(ctx context.Context, scope string) (*pow.ChallengeResponse, error) {
	settings, err := m.settingsMgr.Current(ctx)
	if err != nil {
		return nil, err
	}

	challengeConfig := pow.ChallengeConfig{
		Count:      settings.ChallengeCount,
		Size:       settings.ChallengeSize,
		Difficulty: settings.ChallengeDifficulty,
		Expires:    settings.ChallengeTTL,
	}
	return pow.GenerateChallenge(m.secret, challengeConfig, scope)
}

// Redeem verifies PoW solutions and returns a one-time redeem token.
func (m *CaptchaManager) Redeem(ctx context.Context, token string, solutions []int, scope string) (*dto.RedeemResponse, error) {
	sigHex := pow.JwtSigHex(token)
	if sigHex == "" {
		return &dto.RedeemResponse{Success: false, Error: consts.RedeemErrInvalidToken}, nil
	}

	nonceKey := "cap:nonce:" + sigHex

	payload, err := pow.VerifyChallengeSolutions(token, solutions, m.secret, scope)
	if err != nil {
		return &dto.RedeemResponse{Success: false, Error: err.Error()}, nil //nolint:nilerr // validation errors returned as response
	}

	now := time.Now().UnixNano() / int64(time.Millisecond)
	nonceTTL := time.Duration(payload.Expires-now) * time.Millisecond
	if nonceTTL < time.Second {
		nonceTTL = time.Second
	}

	set, err := m.store.SetNX(ctx, nonceKey, "1", nonceTTL)
	if err != nil {
		return &dto.RedeemResponse{Success: false, Error: consts.RedeemErrNonceStoreFailed}, err
	}
	if !set {
		return &dto.RedeemResponse{Success: false, Error: consts.RedeemErrAlreadyRedeemed}, nil
	}

	settings, err := m.settingsMgr.Current(ctx)
	if err != nil {
		return &dto.RedeemResponse{Success: false, Error: consts.RedeemErrSettingsLoad}, err
	}

	id := pow.RandomHex(consts.RedeemTokenIDLength)
	verToken := pow.RandomHex(consts.RedeemVerTokenLength)
	verHashBytes := sha256.Sum256([]byte(verToken))
	verHashHex := hex.EncodeToString(verHashBytes[:])

	tokenKey := "cap:token:" + id + ":" + verHashHex
	tokenExpires := time.Now().Add(settings.TokenTTL)
	storeVal := strconv.FormatInt(tokenExpires.UnixNano(), 10) + "|" + scope

	if err := m.store.Set(ctx, tokenKey, storeVal, settings.TokenTTL); err != nil {
		return &dto.RedeemResponse{Success: false, Error: consts.RedeemErrTokenStoreFailed}, err
	}

	return &dto.RedeemResponse{
		Success: true,
		Token:   id + ":" + verToken,
		Expires: tokenExpires.UnixNano() / int64(time.Millisecond),
	}, nil
}

// VerifyToken validates and consumes the redeem token (single-use).
func (m *CaptchaManager) VerifyToken(ctx context.Context, token, expectedScope string) (bool, error) {
	if token == "" {
		return false, nil
	}
	parts := strings.Split(token, ":")
	if len(parts) != consts.TokenPartsCount {
		return false, nil
	}
	id := parts[0]
	verToken := parts[1]

	verHashBytes := sha256.Sum256([]byte(verToken))
	verHashHex := hex.EncodeToString(verHashBytes[:])

	tokenKey := "cap:token:" + id + ":" + verHashHex

	if m.store == nil {
		return false, nil
	}
	val, exists, err := m.store.GetAndDelete(ctx, tokenKey)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	valParts := strings.Split(val, "|")
	if len(valParts) != consts.ValuePartsCount {
		return false, nil
	}

	expNano, err := strconv.ParseInt(valParts[0], 10, 64)
	if err != nil {
		return false, nil //nolint:nilerr // invalid format is failure
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

// CaptchaServiceImpl implements contracts.CaptchaService.
type CaptchaServiceImpl struct {
	manager          *CaptchaManager
	verifyMiddleware func(scope string) any
	challengeHandler any
	redeemHandler    any
}

// NewCaptchaService creates a new CaptchaServiceImpl.
func NewCaptchaService(mgr *CaptchaManager, verifyMiddleware func(scope string) any, challengeHandler any, redeemHandler any) contracts.CaptchaService {
	return &CaptchaServiceImpl{
		manager:          mgr,
		verifyMiddleware: verifyMiddleware,
		challengeHandler: challengeHandler,
		redeemHandler:    redeemHandler,
	}
}

// VerifyMiddleware returns the captcha verification middleware.
func (s *CaptchaServiceImpl) VerifyMiddleware(scope string) any {
	if s.verifyMiddleware != nil {
		return s.verifyMiddleware(scope)
	}
	return nil
}

// ChallengeHandler returns the challenge HTTP handler.
func (s *CaptchaServiceImpl) ChallengeHandler() any {
	return s.challengeHandler
}

// RedeemHandler returns the redeem HTTP handler.
func (s *CaptchaServiceImpl) RedeemHandler() any {
	return s.redeemHandler
}
