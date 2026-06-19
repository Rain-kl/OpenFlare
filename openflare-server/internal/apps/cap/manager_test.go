// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cap

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pkgcap "github.com/Rain-kl/Wavelet/pkg/cap"
)

func installTestManagerSettings(t *testing.T) func() {
	t.Helper()
	return InstallTestRuntimeSettings(RuntimeSettings{
		ChallengeCount:      3,
		ChallengeSize:       32,
		ChallengeDifficulty: 3,
		ChallengeTTL:        5 * time.Second,
		TokenTTL:            10 * time.Second,
	})
}

func TestCapFullFlow(t *testing.T) {
	cleanup := installTestManagerSettings(t)
	defer cleanup()

	secret := []byte("a-very-long-secret-key-at-least-16-bytes")
	store := pkgcap.NewMemoryStore(1 * time.Minute)
	manager := NewManager(secret, store)

	scope := "test-scope"
	ctx := context.Background()
	resp, err := manager.Generate(ctx, scope)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if resp.Challenge.C != 3 {
		t.Fatalf("Generate().Challenge.C = %d, want %d", resp.Challenge.C, 3)
	}

	solutions := pkgcap.Solve(resp.Token, resp.Challenge.C, resp.Challenge.S, resp.Challenge.D)

	redeemResp, err := manager.Redeem(ctx, resp.Token, solutions, scope)
	if err != nil {
		t.Fatalf("Redeem() error = %v", err)
	}
	if !redeemResp.Success {
		t.Fatalf("Redeem().Success = false, error = %s", redeemResp.Error)
	}
	if redeemResp.Token == "" {
		t.Fatal("Redeem().Token is empty")
	}

	valid, err := manager.VerifyToken(ctx, redeemResp.Token, scope)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	if !valid {
		t.Fatal("VerifyToken() = false, want true")
	}

	validAgain, err := manager.VerifyToken(ctx, redeemResp.Token, scope)
	if err != nil {
		t.Fatalf("VerifyToken() second call error = %v", err)
	}
	if validAgain {
		t.Fatal("VerifyToken() second call = true, want false")
	}
}

func TestRedeemConcurrentRace(t *testing.T) {
	const goroutines = 50

	cleanup := installTestManagerSettings(t)
	defer cleanup()

	secret := []byte("race-test-secret-key-at-least-16-bytes")
	store := pkgcap.NewMemoryStore(1 * time.Minute)
	manager := NewManager(secret, store)

	ctx := context.Background()
	resp, err := manager.Generate(ctx, "login")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	solutions := pkgcap.Solve(resp.Token, resp.Challenge.C, resp.Challenge.S, resp.Challenge.D)

	var (
		wg      sync.WaitGroup
		success atomic.Int32
		barrier = make(chan struct{})
	)

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-barrier
			r, _ := manager.Redeem(ctx, resp.Token, solutions, "login")
			if r != nil && r.Success {
				success.Add(1)
			}
		}()
	}
	close(barrier)
	wg.Wait()

	if got := success.Load(); got != 1 {
		t.Fatalf("successful Redeem count = %d, want %d", got, 1)
	}
}

func TestVerifyTokenConcurrentRace(t *testing.T) {
	const goroutines = 50

	cleanup := installTestManagerSettings(t)
	defer cleanup()

	secret := []byte("race-test-secret-key-at-least-16-bytes")
	store := pkgcap.NewMemoryStore(1 * time.Minute)
	manager := NewManager(secret, store)

	ctx := context.Background()
	resp, err := manager.Generate(ctx, "login")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	solutions := pkgcap.Solve(resp.Token, resp.Challenge.C, resp.Challenge.S, resp.Challenge.D)
	redeemResp, err := manager.Redeem(ctx, resp.Token, solutions, "login")
	if err != nil || !redeemResp.Success {
		t.Fatalf("Redeem() error = %v, resp = %+v", err, redeemResp)
	}

	var (
		wg      sync.WaitGroup
		success atomic.Int32
		barrier = make(chan struct{})
	)

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-barrier
			ok, _ := manager.VerifyToken(ctx, redeemResp.Token, "login")
			if ok {
				success.Add(1)
			}
		}()
	}
	close(barrier)
	wg.Wait()

	if got := success.Load(); got != 1 {
		t.Fatalf("successful VerifyToken count = %d, want %d", got, 1)
	}
}
