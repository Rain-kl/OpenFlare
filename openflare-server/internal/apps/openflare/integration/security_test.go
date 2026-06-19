// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/option"
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSecurityTest(t *testing.T) (*gin.Engine, adminSeed, func()) {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.User{},
		&model.AccessToken{},
		&model.Origin{},
		&model.ProxyRoute{},
		&model.OpenFlareWAFRuleGroup{},
		&model.OpenFlareWAFRuleGroupBinding{},
		&model.OpenFlareWAFIPGroup{},
		&model.TLSCertificate{},
		&model.ManagedDomain{},
		&model.DNSAccount{},
		&model.AcmeAccount{},
	))

	db.SetDB(sqliteDB)
	option.ResetInitializationForTest()

	seed, err := seedAdminWithAccessToken(sqliteDB)
	require.NoError(t, err)

	oldSecret := config.Config.App.SessionSecret
	config.Config.App.SessionSecret = "test_session_secret_for_security_integration"

	engine := testhelper.NewTestGinEngine()
	mountOpenFlareTestRoutes(engine)

	cleanup := func() {
		config.Config.App.SessionSecret = oldSecret
		db.SetDB(nil)
		option.ResetInitializationForTest()
	}

	return engine, seed, cleanup
}

func generateSelfSignedCertificatePair(t *testing.T, dnsNames []string) (string, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: dnsNames[0],
		},
		DNSNames:    dnsNames,
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	return string(certPEM), string(keyPEM)
}

func TestSecurityWAFTLSMigrationFlow(t *testing.T) {
	engine, seed, cleanup := setupSecurityTest(t)
	defer cleanup()

	var (
		ruleGroupID  uint
		ipGroupID    uint
		proxyRouteID uint
		certID       uint
		domainID     uint
		dnsAccountID uint
	)

	t.Run("WAF rule group create", func(t *testing.T) {
		rec := performJSONRequest(t, engine, http.MethodPost, apiPath("/waf/rule-groups"), map[string]any{
			"name":              "edge-security",
			"enabled":           true,
			"block_status_code": 403,
			"ip_whitelist":      []string{"192.0.2.1"},
			"ip_blacklist":      []string{"203.0.113.10"},
			"country_blacklist": []string{"CN"},
			"remark":            "integration rule group",
		}, adminAuthHeaders(seed.Token))
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		ruleGroupID = uint(data["id"].(float64))
		assert.NotZero(t, ruleGroupID)
		assert.Equal(t, "edge-security", data["name"])
		assert.Equal(t, false, data["is_global"])
		assert.Equal(t, float64(403), data["block_status_code"])
	})

	t.Run("WAF rule group list includes global and custom groups", func(t *testing.T) {
		rec := performJSONRequest(t, engine, http.MethodGet, apiPath("/waf/rule-groups"), nil, adminAuthHeaders(seed.Token))
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		groups := unmarshalAPISlice(t, resp.Data)
		require.GreaterOrEqual(t, len(groups), 2)

		foundCustom := false
		foundGlobal := false
		for _, item := range groups {
			group, ok := item.(map[string]any)
			require.True(t, ok)
			if group["is_global"] == true {
				foundGlobal = true
			}
			if uint(group["id"].(float64)) == ruleGroupID {
				foundCustom = true
				assert.Equal(t, "edge-security", group["name"])
			}
		}
		assert.True(t, foundGlobal)
		assert.True(t, foundCustom)
	})

	t.Run("WAF rule group get detail", func(t *testing.T) {
		rec := performJSONRequest(
			t,
			engine,
			http.MethodGet,
			fmt.Sprintf("%s/waf/rule-groups/%d", apiPath(""), ruleGroupID),
			nil,
			adminAuthHeaders(seed.Token),
		)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		assert.Equal(t, float64(ruleGroupID), data["id"])
		assert.Equal(t, "edge-security", data["name"])
	})

	t.Run("WAF rule group update", func(t *testing.T) {
		rec := performJSONRequest(
			t,
			engine,
			http.MethodPost,
			fmt.Sprintf("%s/waf/rule-groups/%d/update", apiPath(""), ruleGroupID),
			map[string]any{
				"name":              "edge-security-updated",
				"enabled":           true,
				"block_status_code": 451,
				"remark":            "updated by integration test",
			},
			adminAuthHeaders(seed.Token),
		)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		assert.Equal(t, "edge-security-updated", data["name"])
		assert.Equal(t, float64(451), data["block_status_code"])
	})

	t.Run("WAF IP group create", func(t *testing.T) {
		rec := performJSONRequest(t, engine, http.MethodPost, apiPath("/waf/ip-groups"), map[string]any{
			"name":    "blocked-ips",
			"type":    "manual",
			"enabled": true,
			"ip_list": []string{"203.0.113.0/24", "198.51.100.10"},
			"remark":  "manual deny list",
		}, adminAuthHeaders(seed.Token))
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		ipGroupID = uint(data["id"].(float64))
		assert.NotZero(t, ipGroupID)
		assert.Equal(t, "blocked-ips", data["name"])
		assert.Equal(t, "manual", data["type"])
	})

	t.Run("create proxy route for WAF binding", func(t *testing.T) {
		rec := performJSONRequest(t, engine, http.MethodPost, apiPath("/proxy-routes/"), map[string]any{
			"site_name":  "security-site",
			"domain":     "security.example.com",
			"origin_url": "http://origin.security.internal:8080",
			"enabled":    true,
		}, adminAuthHeaders(seed.Token))
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		proxyRouteID = uint(data["id"].(float64))
		assert.NotZero(t, proxyRouteID)
		assert.Equal(t, "security.example.com", data["domain"])
	})

	t.Run("bind WAF rule group to proxy route", func(t *testing.T) {
		rec := performJSONRequest(
			t,
			engine,
			http.MethodPost,
			fmt.Sprintf("%s/waf/sites/%d/rule-groups", apiPath(""), proxyRouteID),
			map[string]any{
				"ids": []uint{ruleGroupID},
			},
			adminAuthHeaders(seed.Token),
		)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		assert.Equal(t, float64(proxyRouteID), data["route_id"])

		appliedIDs, ok := data["applied_ids"].([]any)
		require.True(t, ok)
		require.Len(t, appliedIDs, 1)
		assert.Equal(t, float64(ruleGroupID), appliedIDs[0])
	})

	t.Run("verify site rule groups binding", func(t *testing.T) {
		rec := performJSONRequest(
			t,
			engine,
			http.MethodGet,
			fmt.Sprintf("%s/waf/sites/%d/rule-groups", apiPath(""), proxyRouteID),
			nil,
			adminAuthHeaders(seed.Token),
		)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		assert.NotNil(t, data["global_rule_group"])

		appliedGroups, ok := data["applied_rule_groups"].([]any)
		require.True(t, ok)
		require.Len(t, appliedGroups, 1)
		group, ok := appliedGroups[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, float64(ruleGroupID), group["id"])
	})

	t.Run("create TLS certificate with PEM", func(t *testing.T) {
		certPEM, keyPEM := generateSelfSignedCertificatePair(t, []string{"security.example.com"})

		rec := performJSONRequest(t, engine, http.MethodPost, apiPath("/tls-certificates/"), map[string]any{
			"name":     "security-cert",
			"cert_pem": certPEM,
			"key_pem":  keyPEM,
			"remark":   "self-signed integration cert",
		}, adminAuthHeaders(seed.Token))
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		certID = uint(data["id"].(float64))
		assert.NotZero(t, certID)
		assert.Equal(t, "security-cert", data["name"])
		assert.Equal(t, "upload", data["provider"])
	})

	t.Run("create managed domain", func(t *testing.T) {
		rec := performJSONRequest(t, engine, http.MethodPost, apiPath("/managed-domains/"), map[string]any{
			"domain":  "security.example.com",
			"cert_id": certID,
			"enabled": true,
			"remark":  "primary security domain",
		}, adminAuthHeaders(seed.Token))
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		domainID = uint(data["id"].(float64))
		assert.NotZero(t, domainID)
		assert.Equal(t, "security.example.com", data["domain"])
		assert.Equal(t, float64(certID), data["cert_id"])
		assert.Equal(t, true, data["enabled"])
	})

	t.Run("create DNS account", func(t *testing.T) {
		rec := performJSONRequest(t, engine, http.MethodPost, apiPath("/dns-accounts/"), map[string]any{
			"name":          "cloudflare-dns",
			"type":          "cloudflare",
			"authorization": "test-api-token-value",
		}, adminAuthHeaders(seed.Token))
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		dnsAccountID = uint(data["id"].(float64))
		assert.NotZero(t, dnsAccountID)
		assert.Equal(t, "cloudflare-dns", data["name"])
		assert.Equal(t, "cloudflare", data["type"])
		// API 响应会脱敏 authorization，不应回显明文凭证。
		if auth, ok := data["authorization"]; ok {
			assert.NotEqual(t, "test-api-token-value", auth)
		}
	})

	t.Run("WAF rule group delete", func(t *testing.T) {
		rec := performJSONRequest(
			t,
			engine,
			http.MethodPost,
			fmt.Sprintf("%s/waf/rule-groups/%d/delete", apiPath(""), ruleGroupID),
			nil,
			adminAuthHeaders(seed.Token),
		)
		require.Equal(t, http.StatusOK, rec.Code)
		requireAPIOK(t, rec)

		detailRec := performJSONRequest(
			t,
			engine,
			http.MethodGet,
			fmt.Sprintf("%s/waf/rule-groups/%d", apiPath(""), ruleGroupID),
			nil,
			adminAuthHeaders(seed.Token),
		)
		require.Equal(t, http.StatusNotFound, detailRec.Code)
		detailResp := decodeAPIResponse(t, detailRec)
		assert.NotEmpty(t, detailResp.ErrorMsg)
	})

	_ = ipGroupID
	_ = domainID
	_ = dnsAccountID
}