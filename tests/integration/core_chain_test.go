// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package integration

import (
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
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/config_version"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/migrator"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/platform/bootstrap"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	adminUserID   = uint64(1001)
	adminUsername = "openflare-admin"
)

type adminSeed struct {
	User      model.User
	Token     string
	TokenHash string
}

func setupCoreChainTest(t *testing.T) (*gin.Engine, adminSeed, func()) {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	sqlDB, err := sqliteDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	db.SetDB(sqliteDB)
	require.True(t, migrator.Migrate().Applied)
	previousSecret := config.Config.App.SessionSecret
	config.Config.App.SessionSecret = "integration-core-chain-session-secret"
	bootstrap.RegisterSecurityDomainEvents()
	agent.ResetAuthCacheForTest()

	seed, err := seedAdminWithAccessToken(sqliteDB)
	require.NoError(t, err)

	engine := testhelper.NewTestGinEngine()
	mountOpenFlareTestRoutes(engine)

	cleanup := func() {
		db.SetDB(nil)
		config.Config.App.SessionSecret = previousSecret
		agent.ResetAuthCacheForTest()
	}

	return engine, seed, cleanup
}

func seedAdminWithAccessToken(conn *gorm.DB) (adminSeed, error) {
	now := time.Now().UTC()
	admin := model.User{
		ID:          adminUserID,
		Username:    adminUsername,
		Nickname:    "OpenFlare Admin",
		IsActive:    true,
		IsAdmin:     true,
		LastLoginAt: now,
	}
	if err := conn.Create(&admin).Error; err != nil {
		return adminSeed{}, err
	}

	token, err := model.GenerateTokenString()
	if err != nil {
		return adminSeed{}, err
	}
	tokenHash := model.HashToken(token)
	tokenRecord := model.AccessToken{
		UserID:      adminUserID,
		Name:        "integration-admin-token",
		TokenHash:   tokenHash,
		MaskedToken: model.MaskTokenString(token),
		IsAdmin:     true,
	}
	if err := conn.Create(&tokenRecord).Error; err != nil {
		return adminSeed{}, err
	}

	return adminSeed{
		User:      admin,
		Token:     token,
		TokenHash: tokenHash,
	}, nil
}

func TestCoreChainMigrationFlow(t *testing.T) {
	engine, seed, cleanup := setupCoreChainTest(t)
	defer cleanup()

	ctx := context.Background()
	stop := config_version.StartRefreshLoop(ctx)
	defer func() {
		stopCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		require.NoError(t, stop(stopCtx))
	}()

	var initial *model.ConfigVersion
	require.Eventually(t, func() bool {
		var err error
		initial, err = repository.GetActivePingoraConfigVersion(ctx)
		return err == nil
	}, time.Second, 10*time.Millisecond)
	require.Contains(t, initial.SnapshotJSON, `"routes":[]`)

	var (
		releaseSequence int64
		originID        uint
		proxyRouteID    uint
		configVersion   string
		configChecksum  string
		nodeID          uint
		nodePublicID    string
		agentToken      string
	)

	t.Run("create origin", func(t *testing.T) {
		rec := performJSONRequest(t, engine, http.MethodPost, apiPath("/origins/"), map[string]any{
			"name":    "Primary Origin",
			"address": "origin.core-chain.internal",
			"remark":  "integration upstream",
		}, map[string]string{
			"X-Access-Token": seed.Token,
		})
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		originID = uint(data["id"].(float64))
		assert.NotZero(t, originID)
		assert.Equal(t, "Primary Origin", data["name"])
		assert.Equal(t, "origin.core-chain.internal", data["address"])
	})

	t.Run("create proxy route linked to origin", func(t *testing.T) {
		// Create Zone and ZoneDomain directly in the DB
		zone := model.Zone{Domain: "example.com"}
		require.NoError(t, db.DB(context.Background()).Create(&zone).Error)
		zoneDomain := model.ZoneDomain{
			ZoneID: zone.ID,
			Domain: "core-chain.example.com",
		}
		require.NoError(t, db.DB(context.Background()).Create(&zoneDomain).Error)

		rec := performJSONRequest(t, engine, http.MethodPost, apiPath("/proxy-routes/"), map[string]any{
			"site_name":       "core-chain-site",
			"zone_domain_ids": []uint{zoneDomain.ID},
			"origin_id":       originID,
			"origin_scheme":   "http",
			"origin_port":     "8080",
			"enabled":         true,
		}, map[string]string{
			"X-Access-Token": seed.Token,
		})
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		proxyRouteID = uint(data["id"].(float64))
		assert.NotZero(t, proxyRouteID)
		assert.Equal(t, "core-chain-site", data["site_name"])
		assert.NotEmpty(t, data["zone_domains"])
		zoneDomains := data["zone_domains"].([]any)
		assert.Len(t, zoneDomains, 1)
		assert.Equal(t, "core-chain.example.com", zoneDomains[0].(map[string]any)["domain"])
		assert.InDelta(t, float64(originID), data["origin_id"], 1e-9)
		assert.Equal(t, "http://origin.core-chain.internal:8080", data["origin_url"])
	})

	t.Run("assemble saved configuration", func(t *testing.T) {
		var active *model.ConfigVersion
		require.Eventually(t, func() bool {
			var err error
			active, err = repository.GetActivePingoraConfigVersion(ctx)
			return err == nil && active.Version != initial.Version
		}, time.Second, 10*time.Millisecond)
		configVersion, configChecksum, releaseSequence = active.Version, active.Checksum, active.ReleaseSequence
		assert.NotEmpty(t, configVersion)
		assert.NotEmpty(t, configChecksum)
		assert.Contains(t, active.SnapshotJSON, "core-chain.example.com")
		assert.Greater(t, active.ReleaseSequence, initial.ReleaseSequence)

		require.NoError(t, config_version.RefreshSiteSnapshot(ctx))
		unchanged, err := repository.GetActivePingoraConfigMetadata(ctx)
		require.NoError(t, err)
		assert.Equal(t, configVersion, unchanged.Version)
	})

	t.Run("create node", func(t *testing.T) {
		rec := performJSONRequest(t, engine, http.MethodPost, apiPath("/nodes/"), map[string]any{
			"name":                "edge-core-chain",
			"ip":                  "10.10.0.1",
			"auto_update_enabled": true,
		}, map[string]string{
			"X-Access-Token": seed.Token,
		})
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		nodeID = uint(data["id"].(float64))
		nodePublicID, _ = data["node_id"].(string)
		agentToken, _ = data["access_token"].(string)
		assert.NotZero(t, nodeID)
		assert.NotEmpty(t, nodePublicID)
		assert.Len(t, agentToken, 32)
	})

	t.Run("fetch the site snapshot selected by the heartbeat", func(t *testing.T) {
		headers := map[string]string{"X-Agent-Token": agentToken}
		heartbeat := func(cursor string) protocol.ConfigResource {
			t.Helper()
			rec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/nodes/heartbeat", protocol.NodePayload{
				Engine: "pingora", ContractVersion: edgeconfig.ContractVersion, ConfigSyncVersion: protocol.EdgeConfigSyncVersion,
				LatestConfigVersion: cursor, Version: "test",
			}, headers)
			var data struct{ Config protocol.ConfigManifest }
			unmarshalAPIData(t, requireAPIOK(t, rec).Data, &data)
			return data.Config.Resources["sites.v1"]
		}
		resource := heartbeat(initial.Version)
		require.NotNil(t, resource.Metadata)
		assert.Equal(t, configVersion, resource.Revision)
		assert.Equal(t, configChecksum, resource.SHA256)
		assert.Empty(t, resource.InlineJSON)
		known := heartbeat(configVersion)
		assert.Nil(t, known.Metadata)
		assert.Equal(t, resource.SHA256, known.SHA256)

		pull := "/api/v1/agent/config/pull?" + url.Values{
			"key": {"sites.v1"}, "sha256": {known.SHA256},
		}.Encode()
		reply := performJSONRequest(t, engine, http.MethodGet, pull, nil, headers)
		require.Equal(t, http.StatusOK, reply.Code, reply.Body.String())
		assert.Contains(t, reply.Body.String(), "core-chain.example.com")
	})

	t.Run("create apply log for node", func(t *testing.T) {
		rec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/apply-logs", map[string]any{
			"version":               configVersion,
			"result":                "success",
			"message":               "config applied",
			"checksum":              configChecksum,
			"engine":                "pingora",
			"release_sequence":      releaseSequence,
			"payload_checksum":      configChecksum,
			"main_config_checksum":  "main-checksum",
			"route_config_checksum": "route-checksum",
			"support_file_count":    2,
		}, map[string]string{
			"X-Agent-Token": agentToken,
		})
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		assert.Equal(t, nodePublicID, data["node_id"])
		assert.Equal(t, configVersion, data["version"])
		assert.Equal(t, "success", data["result"])
		assert.Equal(t, configChecksum, data["checksum"])
	})

	t.Run("verify apply log listing and node metadata", func(t *testing.T) {
		listRec := performJSONRequest(
			t,
			engine,
			http.MethodGet,
			apiPath("/apply-logs/?node_id="+nodePublicID+"&pageNo=1&pageSize=10"),
			nil,
			map[string]string{
				"X-Access-Token": seed.Token,
			},
		)
		require.Equal(t, http.StatusOK, listRec.Code)

		listResp := requireAPIOK(t, listRec)
		listData := unmarshalAPIMap(t, listResp.Data)
		assert.InDelta(t, float64(1), listData["total"], 1e-9)

		rows, ok := listData["rows"].([]any)
		require.True(t, ok)
		require.Len(t, rows, 1)
		row, ok := rows[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, nodePublicID, row["node_id"])
		assert.Equal(t, configVersion, row["version"])
		assert.Equal(t, "success", row["result"])

		nodeRec := performJSONRequest(t, engine, http.MethodGet, apiPath("/nodes/"), nil, map[string]string{
			"X-Access-Token": seed.Token,
		})
		require.Equal(t, http.StatusOK, nodeRec.Code)
		nodeResp := requireAPIOK(t, nodeRec)

		nodes := unmarshalAPISlice(t, nodeResp.Data)
		require.Len(t, nodes, 1)
		nodeView, ok := nodes[0].(map[string]any)
		require.True(t, ok)
		assert.InDelta(t, float64(nodeID), nodeView["id"], 1e-9)
		assert.Equal(t, nodePublicID, nodeView["node_id"])
		assert.Equal(t, "success", nodeView["latest_apply_result"])
		assert.Equal(t, configChecksum, nodeView["latest_apply_checksum"])
		assert.InDelta(t, float64(2), nodeView["latest_support_file_count"], 1e-9)
	})
	t.Run("retain the last snapshot if compilation fails", func(t *testing.T) {
		var route model.ProxyRoute
		require.NoError(t, db.DB(ctx).Take(&route, proxyRouteID).Error)
		require.NoError(t, db.DB(ctx).Model(&model.ProxyRoute{}).Where("id = ?", proxyRouteID).UpdateColumn("upstreams", "invalid JSON").Error)
		require.Error(t, config_version.RefreshSiteSnapshot(ctx))
		retained, err := repository.GetActivePingoraConfigMetadata(ctx)
		require.NoError(t, err)
		assert.Equal(t, configVersion, retained.Version)

		require.NoError(t, db.DB(ctx).Model(&model.ProxyRoute{}).Where("id = ?", proxyRouteID).UpdateColumn("upstreams", route.Upstreams).Error)
		require.NoError(t, config_version.RefreshSiteSnapshot(ctx))
		retained, err = repository.GetActivePingoraConfigMetadata(ctx)
		require.NoError(t, err)
		assert.Equal(t, configVersion, retained.Version)
	})

	t.Run("automatically clear the last route", func(t *testing.T) {
		require.NoError(t, db.DB(ctx).Model(&model.ProxyRoute{}).Where("id = ?", proxyRouteID).Update("site_name", "updated-site").Error)
		state, err := repository.GetEdgeSnapshotState(ctx)
		require.NoError(t, err)
		require.NoError(t, db.DB(ctx).Model(&model.ProxyRoute{}).Where("id = ?", proxyRouteID).Update("enabled", false).Error)
		stale, err := repository.StoreEdgeSnapshot(ctx, state.SourceRevision, &model.ConfigVersion{PayloadChecksum: "stale"})
		require.NoError(t, err)
		require.False(t, stale)

		var empty *model.ConfigVersion
		require.Eventually(t, func() bool {
			var err error
			empty, err = repository.GetActivePingoraConfigVersion(ctx)
			return err == nil && empty.Version != configVersion
		}, 3*time.Second, 20*time.Millisecond)
		assert.Contains(t, empty.SnapshotJSON, `"routes":[]`)
	})

}

func generateIntegrationTestCert(t *testing.T, cn string) (string, string) {
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

func TestMemoryOnlyTLSKeysAndQuarantineFlow(t *testing.T) {
	engine, seed, cleanup := setupCoreChainTest(t)
	defer cleanup()

	ctx := context.Background()
	adminHeaders := map[string]string{"X-Access-Token": seed.Token}

	certPEM, keyPEM := generateIntegrationTestCert(t, "secure.example.com")
	certResp := performJSONRequest(t, engine, http.MethodPost, apiPath("/tls-certificates/"), map[string]any{
		"name":     "secure-cert",
		"cert_pem": certPEM,
		"key_pem":  keyPEM,
	}, adminHeaders)
	require.Equal(t, http.StatusOK, certResp.Code, certResp.Body.String())
	certData := unmarshalAPIMap(t, requireAPIOK(t, certResp).Data)
	certID := uint(certData["id"].(float64))
	certSHA := edgeconfig.CertificatePEMDigest(certPEM)

	originResp := performJSONRequest(t, engine, http.MethodPost, apiPath("/origins/"), map[string]any{
		"name":    "Secure Origin",
		"address": "origin.secure.internal",
	}, adminHeaders)
	require.Equal(t, http.StatusOK, originResp.Code)
	originID := uint(unmarshalAPIMap(t, requireAPIOK(t, originResp).Data)["id"].(float64))

	zone := model.Zone{Domain: "example.com"}
	require.NoError(t, db.DB(ctx).Create(&zone).Error)
	zoneDomain := model.ZoneDomain{ZoneID: zone.ID, Domain: "secure.example.com", CertID: &certID}
	require.NoError(t, db.DB(ctx).Create(&zoneDomain).Error)

	routeResp := performJSONRequest(t, engine, http.MethodPost, apiPath("/proxy-routes/"), map[string]any{
		"site_name":       "secure-site",
		"zone_domain_ids": []uint{zoneDomain.ID},
		"origin_id":       originID,
		"origin_scheme":   "http",
		"origin_port":     "8080",
		"enable_https":    true,
		"enabled":         true,
	}, adminHeaders)
	require.Equal(t, http.StatusOK, routeResp.Code, routeResp.Body.String())

	require.NoError(t, db.DB(ctx).Exec("UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1").Error)
	require.NoError(t, config_version.RefreshSiteSnapshot(ctx))
	active, err := repository.GetActivePingoraConfigVersion(ctx)
	require.NoError(t, err)
	assert.Contains(t, active.SnapshotJSON, "secure.example.com")
	assert.NotContains(t, active.SupportFilesJSON, "PRIVATE KEY")
	assert.NotContains(t, active.SupportFilesJSON, ".key")

	nodeResp := performJSONRequest(t, engine, http.MethodPost, apiPath("/nodes/"), map[string]any{
		"name": "edge-secure-1",
		"ip":   "198.51.100.42",
	}, adminHeaders)
	require.Equal(t, http.StatusOK, nodeResp.Code)
	nodeData := unmarshalAPIMap(t, requireAPIOK(t, nodeResp).Data)
	nodeID := uint(nodeData["id"].(float64))
	nodePublicID := nodeData["node_id"].(string)
	oldToken := nodeData["access_token"].(string)

	agentHeaders := map[string]string{"X-Agent-Token": oldToken}
	hbResp := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/nodes/heartbeat", protocol.NodePayload{
		Engine: "pingora", ContractVersion: edgeconfig.ContractVersion, ConfigSyncVersion: protocol.EdgeConfigSyncVersion,
		Version: "test",
	}, agentHeaders)
	require.Equal(t, http.StatusOK, hbResp.Code)
	var hbData struct{ Config protocol.ConfigManifest }
	unmarshalAPIData(t, requireAPIOK(t, hbResp).Data, &hbData)
	siteRes := hbData.Config.Resources["sites.v1"]

	pullPath := "/api/v1/agent/config/pull?" + url.Values{"key": {"sites.v1"}, "sha256": {siteRes.SHA256}}.Encode()
	pullResp := performJSONRequest(t, engine, http.MethodGet, pullPath, nil, agentHeaders)
	require.Equal(t, http.StatusOK, pullResp.Code)
	var env edgeconfig.Envelope
	require.NoError(t, json.Unmarshal(pullResp.Body.Bytes(), &env))
	assert.Equal(t, 4, env.ContractVersion)
	assert.Contains(t, env.RequiredCapabilities, "tls-memory-keys-v1")
	assert.NotContains(t, pullResp.Body.String(), "PRIVATE KEY")
	assert.NotContains(t, pullResp.Body.String(), "key_pem")

	keysResp := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/tls/keys", protocol.FetchTLSKeysRequest{
		ReleaseSequence: active.ReleaseSequence,
		Certificates: []protocol.TLSKeyRequestItem{
			{CertificateID: certID, CertSHA256: certSHA},
		},
	}, agentHeaders)
	require.Equal(t, http.StatusOK, keysResp.Code, keysResp.Body.String())
	assert.Contains(t, keysResp.Header().Get("Cache-Control"), "no-store")
	var keysOut protocol.FetchTLSKeysResponse
	unmarshalAPIData(t, requireAPIOK(t, keysResp).Data, &keysOut)
	assert.Equal(t, strings.TrimSpace(keyPEM)+"\n", keysOut.Keys[json.Number("1").String()])

	deliveriesResp := performJSONRequest(t, engine, http.MethodGet, apiPath("/nodes/"+json.Number("1").String()+"/certificate-deliveries"), nil, adminHeaders)
	if nodeID != 1 {
		deliveriesResp = performJSONRequest(t, engine, http.MethodGet, apiPath("/nodes/"+strconvUint(nodeID)+"/certificate-deliveries"), nil, adminHeaders)
	}
	require.Equal(t, http.StatusOK, deliveriesResp.Code)
	var deliveries []map[string]any
	unmarshalAPIData(t, requireAPIOK(t, deliveriesResp).Data, &deliveries)
	require.Len(t, deliveries, 1)
	assert.InDelta(t, float64(certID), deliveries[0]["certificate_id"], 1e-9)
	assert.Equal(t, true, deliveries[0]["matches_current"])
	assert.InDelta(t, float64(1), deliveries[0]["delivery_count"], 1e-9)

	quarantineResp := performJSONRequest(t, engine, http.MethodPost, apiPath("/nodes/"+strconvUint(nodeID)+"/quarantine"), map[string]any{
		"reason":            "suspected host intrusion",
		"rotate_acme_certs": false,
	}, adminHeaders)
	require.Equal(t, http.StatusOK, quarantineResp.Code, quarantineResp.Body.String())
	quarantineData := unmarshalAPIMap(t, requireAPIOK(t, quarantineResp).Data)
	qNode := quarantineData["node"].(map[string]any)
	assert.Equal(t, "quarantined", qNode["status"])
	assert.Empty(t, qNode["access_token"])
	manualIDs := quarantineData["manual_action_certificate_ids"].([]any)
	require.Len(t, manualIDs, 1)

	// Cached token must be invalidated immediately across agent endpoints.
	staleHB := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/nodes/heartbeat", protocol.NodePayload{
		Engine: "pingora", ContractVersion: edgeconfig.ContractVersion, ConfigSyncVersion: protocol.EdgeConfigSyncVersion,
		Version: "test",
	}, agentHeaders)
	assert.Equal(t, http.StatusUnauthorized, staleHB.Code)
	assert.Equal(t, "invalid_token", staleHB.Header().Get("X-OpenFlare-Auth-Error"))

	staleKeys := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/tls/keys", protocol.FetchTLSKeysRequest{
		ReleaseSequence: active.ReleaseSequence,
		Certificates: []protocol.TLSKeyRequestItem{
			{CertificateID: certID, CertSHA256: certSHA},
		},
	}, agentHeaders)
	assert.Equal(t, http.StatusUnauthorized, staleKeys.Code)
	assert.Equal(t, "invalid_token", staleKeys.Header().Get("X-OpenFlare-Auth-Error"))

	unqResp := performJSONRequest(t, engine, http.MethodPost, apiPath("/nodes/"+strconvUint(nodeID)+"/unquarantine"), nil, adminHeaders)
	require.Equal(t, http.StatusOK, unqResp.Code, unqResp.Body.String())
	unqData := unmarshalAPIMap(t, requireAPIOK(t, unqResp).Data)
	newToken := unqData["access_token"].(string)
	assert.Equal(t, "pending", unqData["status"])
	assert.NotEmpty(t, newToken)
	assert.NotEqual(t, oldToken, newToken)
	assert.Equal(t, nodePublicID, unqData["node_id"])

	freshHB := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/nodes/heartbeat", protocol.NodePayload{
		Engine: "pingora", ContractVersion: edgeconfig.ContractVersion, ConfigSyncVersion: protocol.EdgeConfigSyncVersion,
		Version: "test",
	}, map[string]string{"X-Agent-Token": newToken})
	assert.Equal(t, http.StatusOK, freshHB.Code, freshHB.Body.String())
}

func strconvUint(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}
