// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cap

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	pkgcap "github.com/Rain-kl/Wavelet/pkg/cap"
)

func TestCapEndpointsAndMiddleware(t *testing.T) {
	sqliteDB, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	r := testhelper.NewTestGinEngine()

	// Mount CAPTCHA API endpoints
	capGroup := r.Group("/api/cap")
	{
		capGroup.POST("/challenge", Challenge)
		capGroup.POST("/redeem", Redeem)
	}

	r.POST("/api/v1/user/login", VerifyMiddleware(GetDefaultManager(), "login"), func(c *gin.Context) {
		c.JSON(http.StatusOK, response.OK("login success"))
	})

	// 1. Test challenge generation
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/cap/challenge", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
	}

	var challengeResp pkgcap.ChallengeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &challengeResp); err != nil {
		t.Fatalf("failed to unmarshal challenge response: %v", err)
	}

	if challengeResp.Token == "" {
		t.Fatalf("expected token in challenge response")
	}

	// 2. Test login with CAPTCHA disabled (should pass)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/user/login", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK when CAPTCHA is disabled, got %d. Body: %s", w.Code, w.Body.String())
	}

	// 3. Enable CAPTCHA in DB and invalidate runtime snapshot
	err := sqliteDB.Model(&model.SystemConfig{}).Where("key = ?", model.ConfigKeyCapLoginEnabled).Update("value", "true").Error
	if err != nil {
		t.Fatalf("failed to enable cap_login_enabled in DB: %v", err)
	}
	if err := repository.InvalidateSystemConfigCache(context.Background(), model.ConfigKeyCapLoginEnabled); err != nil {
		t.Fatalf("InvalidateSystemConfigCache() error = %v", err)
	}
	InvalidateRuntimeSettings()

	// 4. Test login with CAPTCHA enabled but no header (should be blocked)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/user/login", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d. Body: %s", w.Code, w.Body.String())
	}

	// 5. Solve the challenge
	solutions := pkgcap.Solve(challengeResp.Token, challengeResp.Challenge.C, challengeResp.Challenge.S, challengeResp.Challenge.D)

	// 6. Redeem solutions
	redeemReqPayload := redeemRequest{
		Token:     challengeResp.Token,
		Solutions: solutions,
	}
	bodyBytes, _ := json.Marshal(redeemReqPayload)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/cap/redeem", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for redeem, got %d. Body: %s", w.Code, w.Body.String())
	}

	var redeemResp RedeemResponse
	if err := json.Unmarshal(w.Body.Bytes(), &redeemResp); err != nil {
		t.Fatalf("failed to unmarshal redeem response: %v", err)
	}

	if !redeemResp.Success || redeemResp.Token == "" {
		t.Fatalf("redeem failed or returned empty token: %+v", redeemResp)
	}

	// 7. Login with valid redeem token (should pass)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/user/login", nil)
	req.Header.Set("X-Cap-Token", redeemResp.Token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with valid cap token, got %d. Body: %s", w.Code, w.Body.String())
	}

	// 8. Replay attack: Login with the same redeem token again (should be blocked as it is single-use)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/user/login", nil)
	req.Header.Set("X-Cap-Token", redeemResp.Token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized on replayed token, got %d. Body: %s", w.Code, w.Body.String())
	}
}
