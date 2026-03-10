package router_test

import (
	"atsflare/common"
	"atsflare/router"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"net/http"
	"testing"
)

func TestPhase2ManagedDomainLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	token := prepareRootToken(t)
	wildcardCertPEM, wildcardKeyPEM := generateCertificatePairForRouterTest(t, []string{"*.example.com"})
	exactCertPEM, exactKeyPEM := generateCertificatePairForRouterTest(t, []string{"api.example.com"})

	wildcardResp := performJSONRequest(t, engine, token, http.MethodPost, "/api/tls-certificates/", map[string]any{
		"name":     "wildcard-cert",
		"cert_pem": wildcardCertPEM,
		"key_pem":  wildcardKeyPEM,
	})
	var wildcardCertificate map[string]any
	decodeResponseData(t, wildcardResp, &wildcardCertificate)

	exactResp := performJSONRequest(t, engine, token, http.MethodPost, "/api/tls-certificates/", map[string]any{
		"name":     "exact-cert",
		"cert_pem": exactCertPEM,
		"key_pem":  exactKeyPEM,
	})
	var exactCertificate map[string]any
	decodeResponseData(t, exactResp, &exactCertificate)

	wildcardID := uint(wildcardCertificate["id"].(float64))
	exactID := uint(exactCertificate["id"].(float64))

	createWildcard := performJSONRequest(t, engine, token, http.MethodPost, "/api/managed-domains/", map[string]any{
		"domain":  "*.example.com",
		"cert_id": wildcardID,
		"enabled": true,
		"remark":  "wildcard binding",
	})
	var wildcardDomain map[string]any
	decodeResponseData(t, createWildcard, &wildcardDomain)

	createExact := performJSONRequest(t, engine, token, http.MethodPost, "/api/managed-domains/", map[string]any{
		"domain":  "api.example.com",
		"cert_id": exactID,
		"enabled": true,
		"remark":  "exact binding",
	})
	var exactDomain map[string]any
	decodeResponseData(t, createExact, &exactDomain)

	listResp := performJSONRequest(t, engine, token, http.MethodGet, "/api/managed-domains/", nil)
	var domains []map[string]any
	decodeResponseData(t, listResp, &domains)
	if len(domains) != 2 {
		t.Fatalf("expected 2 managed domains, got %d", len(domains))
	}

	matchResp := performJSONRequest(t, engine, token, http.MethodGet, "/api/managed-domains/match?domain=api.example.com", nil)
	var matchResult map[string]any
	decodeResponseData(t, matchResp, &matchResult)
	if matched, ok := matchResult["matched"].(bool); !ok || !matched {
		t.Fatalf("expected exact domain to be matched, got %#v", matchResult)
	}
	candidate, ok := matchResult["candidate"].(map[string]any)
	if !ok {
		t.Fatalf("expected candidate payload, got %#v", matchResult["candidate"])
	}
	if candidate["match_type"] != "exact" {
		t.Fatalf("expected exact match type, got %#v", candidate["match_type"])
	}
	if uint(candidate["certificate_id"].(float64)) != exactID {
		t.Fatalf("expected exact certificate id %d, got %#v", exactID, candidate["certificate_id"])
	}

	updateResp := performJSONRequest(t, engine, token, http.MethodPut, "/api/managed-domains/"+toString(uint(exactDomain["id"].(float64))), map[string]any{
		"domain":  "api.example.com",
		"cert_id": exactID,
		"enabled": false,
		"remark":  "disabled exact binding",
	})
	decodeResponseData(t, updateResp, &exactDomain)

	matchResp = performJSONRequest(t, engine, token, http.MethodGet, "/api/managed-domains/match?domain=api.example.com", nil)
	decodeResponseData(t, matchResp, &matchResult)
	candidate, ok = matchResult["candidate"].(map[string]any)
	if !ok {
		t.Fatalf("expected wildcard fallback candidate, got %#v", matchResult["candidate"])
	}
	if candidate["match_type"] != "wildcard" {
		t.Fatalf("expected wildcard fallback, got %#v", candidate["match_type"])
	}
	if uint(candidate["certificate_id"].(float64)) != wildcardID {
		t.Fatalf("expected wildcard certificate id %d, got %#v", wildcardID, candidate["certificate_id"])
	}

	deleteResp := performJSONRequest(t, engine, token, http.MethodDelete, "/api/managed-domains/"+toString(uint(wildcardDomain["id"].(float64))), nil)
	if !deleteResp.Success {
		t.Fatalf("expected delete success, got %s", deleteResp.Message)
	}
}
