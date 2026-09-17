// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pow

import (
	"context"
	"testing"
	"time"
)

func TestPowChallengeFlow(t *testing.T) {
	secret := []byte("test-secret-key-1234567890123456")
	conf := ChallengeConfig{
		Count:      2,
		Size:       16,
		Difficulty: 1,
		Expires:    1 * time.Minute,
	}
	scope := "login"

	resp, err := GenerateChallenge(secret, conf, scope)
	if err != nil {
		t.Fatalf("GenerateChallenge failed: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if resp.Challenge.C != 2 {
		t.Fatalf("expected count 2, got %d", resp.Challenge.C)
	}

	sigHex := JwtSigHex(resp.Token)
	if sigHex == "" {
		t.Fatal("expected non-empty sigHex")
	}

	solutions := Solve(resp.Token, resp.Challenge.C, resp.Challenge.S, resp.Challenge.D)
	if len(solutions) != 2 {
		t.Fatalf("expected 2 solutions, got %d", len(solutions))
	}

	payload, err := VerifyChallengeSolutions(resp.Token, solutions, secret, scope)
	if err != nil {
		t.Fatalf("VerifyChallengeSolutions failed: %v", err)
	}
	if payload.Scope != scope {
		t.Fatalf("expected scope %s, got %s", scope, payload.Scope)
	}

	// Scope mismatch test
	_, err = VerifyChallengeSolutions(resp.Token, solutions, secret, "other_scope")
	if err == nil {
		t.Fatal("expected scope mismatch error")
	}

	// Invalid solutions test
	_, err = VerifyChallengeSolutions(resp.Token, []int{9999999, 9999999}, secret, scope)
	if err == nil {
		t.Fatal("expected invalid solution error")
	}

	// Invalid token test
	_, err = VerifyChallengeSolutions("invalid.jwt.token", solutions, secret, scope)
	if err == nil {
		t.Fatal("expected invalid token error")
	}
}

func TestMemoryStore(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(100 * time.Millisecond)

	// Set and Get
	err := store.Set(ctx, "k1", "v1", 200*time.Millisecond)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	val, ok, err := store.Get(ctx, "k1")
	if err != nil || !ok || val != "v1" {
		t.Fatalf("Get failed: val=%s, ok=%v, err=%v", val, ok, err)
	}

	// SetNX
	set, err := store.SetNX(ctx, "k1", "v2", 200*time.Millisecond)
	if err != nil || set {
		t.Fatalf("SetNX should have failed because key exists: set=%v, err=%v", set, err)
	}

	set, err = store.SetNX(ctx, "k2", "v2", 200*time.Millisecond)
	if err != nil || !set {
		t.Fatalf("SetNX should have succeeded: set=%v, err=%v", set, err)
	}

	// GetAndDelete
	val, ok, err = store.GetAndDelete(ctx, "k2")
	if err != nil || !ok || val != "v2" {
		t.Fatalf("GetAndDelete failed: val=%s, ok=%v, err=%v", val, ok, err)
	}
	_, ok, _ = store.Get(ctx, "k2")
	if ok {
		t.Fatal("k2 should be deleted")
	}

	// Delete
	_ = store.Delete(ctx, "k1")
	_, ok, _ = store.Get(ctx, "k1")
	if ok {
		t.Fatal("k1 should be deleted")
	}
}
