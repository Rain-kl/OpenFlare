// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/credential"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func generateAgentTestCertPair(t *testing.T, cn string) (string, string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{cn},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	keyBytes, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}))
	return certPEM, keyPEM
}

func setupTLSKeysTest(t *testing.T) (*gin.Engine, *model.OpenFlareNode, model.TLSCertificate, model.TLSCertificate, string, func()) {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	sqlDB, err := sqliteDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.OpenFlareNode{},
		&model.ConfigVersion{},
		&model.TLSCertificate{},
		&model.OpenFlareNodeCertDelivery{},
	))

	previousSecret := config.Config.App.SessionSecret
	config.Config.App.SessionSecret = "test_session_secret_for_agent_tls_keys"
	db.SetDB(sqliteDB)
	ResetAuthCacheForTest()

	cert1PEM, key1PEM := generateAgentTestCertPair(t, "active.example.com")
	sealed1, err := credential.Seal(key1PEM)
	require.NoError(t, err)
	cert1 := model.TLSCertificate{
		Name:          "Active Cert",
		Provider:      "upload",
		PrimaryDomain: "active.example.com",
		CertPEM:       cert1PEM,
		KeyPEM:        sealed1,
		ApplyStatus:   "ready",
	}
	require.NoError(t, sqliteDB.Create(&cert1).Error)

	cert2PEM, key2PEM := generateAgentTestCertPair(t, "unused.example.com")
	sealed2, err := credential.Seal(key2PEM)
	require.NoError(t, err)
	cert2 := model.TLSCertificate{
		Name:          "Unused Cert",
		Provider:      "upload",
		PrimaryDomain: "unused.example.com",
		CertPEM:       cert2PEM,
		KeyPEM:        sealed2,
		ApplyStatus:   "ready",
	}
	require.NoError(t, sqliteDB.Create(&cert2).Error)

	doc := edgeconfig.Document{
		Routes: []edgeconfig.Route{
			{
				ID:            1,
				SiteName:      "active-site",
				Domains:       []string{"active.example.com"},
				DomainCertIDs: []uint{cert1.ID},
				Enabled:       true,
				EnableHTTPS:   true,
			},
		},
	}
	snapshotBytes, err := json.Marshal(doc)
	require.NoError(t, err)
	files := []edgeconfig.SupportFile{
		{
			Path:    strconv.FormatUint(uint64(cert1.ID), 10) + ".crt",
			Content: cert1PEM,
		},
	}
	supportBytes, err := json.Marshal(files)
	require.NoError(t, err)

	version := model.ConfigVersion{
		Version:          "v4-test-1",
		ReleaseSequence:  1,
		SnapshotJSON:     string(snapshotBytes),
		SupportFilesJSON: string(supportBytes),
		Checksum:         "checksum-1",
		PayloadChecksum:  "payload-checksum-1",
		IsActive:         true,
	}
	require.NoError(t, sqliteDB.Create(&version).Error)

	node := &model.OpenFlareNode{
		NodeID:      "node-tls-1",
		Name:        "Edge TLS 1",
		AccessToken: "token-tls-1-00000000000000000000",
		Status:      "online",
		NodeType:    nodeTypeEdge,
		Engine:      enginePingora,
	}
	require.NoError(t, sqliteDB.Create(node).Error)

	router := testhelper.NewTestGinEngine()
	group := router.Group("/api/v1/agent")
	group.Use(Auth())
	group.POST("/tls/keys", FetchTLSKeysHandler)
	group.POST("/nodes/heartbeat", HeartbeatHandler)

	cleanup := func() {
		ResetAuthCacheForTest()
		db.SetDB(nil)
		config.Config.App.SessionSecret = previousSecret
		_ = sqlDB.Close()
	}
	return router, node, cert1, cert2, strings.TrimSpace(key1PEM) + "\n", cleanup
}

func TestFetchTLSKeysEndpointAndQuarantineGuards(t *testing.T) {
	router, node, cert1, cert2, expectedKey1PEM, cleanup := setupTLSKeysTest(t)
	defer cleanup()

	cert1SHA := edgeconfig.CertificatePEMDigest(cert1.CertPEM)
	cert2SHA := edgeconfig.CertificatePEMDigest(cert2.CertPEM)
	ctx := context.Background()

	postKeys := func(token string, body protocol.FetchTLSKeysRequest) *httptest.ResponseRecorder {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/tls/keys", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set(agentTokenHeader, token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	// 1. Authorized fetch returns key, no-store headers, and records delivery audit ledger.
	resp := postKeys(node.AccessToken, protocol.FetchTLSKeysRequest{
		ReleaseSequence: 1,
		Certificates: []protocol.TLSKeyRequestItem{
			{CertificateID: cert1.ID, CertSHA256: cert1SHA},
		},
	})
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "private, no-store, no-cache, must-revalidate", resp.Header().Get("Cache-Control"))
	assert.Equal(t, "no-cache", resp.Header().Get("Pragma"))

	var envelope struct {
		ErrorMsg string                        `json:"error_msg"`
		Data     protocol.FetchTLSKeysResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &envelope))
	assert.Empty(t, envelope.ErrorMsg)
	assert.Equal(t, int64(1), envelope.Data.ReleaseSequence)
	assert.Equal(t, expectedKey1PEM, envelope.Data.Keys[strconv.FormatUint(uint64(cert1.ID), 10)])

	// Second fetch increments delivery_count to 2.
	resp2 := postKeys(node.AccessToken, protocol.FetchTLSKeysRequest{
		ReleaseSequence: 1,
		Certificates: []protocol.TLSKeyRequestItem{
			{CertificateID: cert1.ID, CertSHA256: cert1SHA},
		},
	})
	require.Equal(t, http.StatusOK, resp2.Code)
	deliveries, err := repository.ListNodeCertDeliveriesByNodeID(ctx, node.NodeID)
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	assert.Equal(t, int64(2), deliveries[0].DeliveryCount)
	assert.Equal(t, cert1SHA, deliveries[0].CertSHA256)

	// 2. Certificate outside active release -> 403 Forbidden.
	forbidden := postKeys(node.AccessToken, protocol.FetchTLSKeysRequest{
		ReleaseSequence: 1,
		Certificates: []protocol.TLSKeyRequestItem{
			{CertificateID: cert2.ID, CertSHA256: cert2SHA},
		},
	})
	assert.Equal(t, http.StatusForbidden, forbidden.Code)

	// 3. Certificate digest mismatch -> 409 Conflict.
	conflict := postKeys(node.AccessToken, protocol.FetchTLSKeysRequest{
		ReleaseSequence: 1,
		Certificates: []protocol.TLSKeyRequestItem{
			{CertificateID: cert1.ID, CertSHA256: strings.Repeat("0", 64)},
		},
	})
	assert.Equal(t, http.StatusConflict, conflict.Code)

	// 4. Invalid/revoked token -> 401 with explicit auth-error header.
	unauth := postKeys("invalid-or-rotated-token", protocol.FetchTLSKeysRequest{
		ReleaseSequence: 1,
		Certificates: []protocol.TLSKeyRequestItem{
			{CertificateID: cert1.ID, CertSHA256: cert1SHA},
		},
	})
	assert.Equal(t, http.StatusUnauthorized, unauth.Code)
	assert.Equal(t, agentAuthErrorInvalidToken, unauth.Header().Get(agentAuthErrorHeader))

	// 5. Quarantined node: FetchNodeTLSKeys primary-DB check blocks even when auth cache is warm.
	quarantinedAt := time.Now().UTC()
	require.NoError(t, db.DB(ctx).Model(&model.OpenFlareNode{}).Where("id = ?", node.ID).Updates(map[string]any{
		"quarantined_at":    &quarantinedAt,
		"quarantine_reason": "test quarantine",
		"status":            "quarantined",
	}).Error)

	quarantinedResp := postKeys(node.AccessToken, protocol.FetchTLSKeysRequest{
		ReleaseSequence: 1,
		Certificates: []protocol.TLSKeyRequestItem{
			{CertificateID: cert1.ID, CertSHA256: cert1SHA},
		},
	})
	assert.Equal(t, http.StatusForbidden, quarantinedResp.Code)
	assert.Equal(t, agentQuarantinedHeaderTrue, quarantinedResp.Header().Get(agentQuarantinedHeader))

	// Cache invalidation causes Auth() middleware to reject with quarantine header as well.
	InvalidateNodeAccessTokenCache(node.NodeID)
	quarantinedMW := postKeys(node.AccessToken, protocol.FetchTLSKeysRequest{
		ReleaseSequence: 1,
		Certificates: []protocol.TLSKeyRequestItem{
			{CertificateID: cert1.ID, CertSHA256: cert1SHA},
		},
	})
	assert.Equal(t, http.StatusForbidden, quarantinedMW.Code)
	assert.Equal(t, agentQuarantinedHeaderTrue, quarantinedMW.Header().Get(agentQuarantinedHeader))
}
