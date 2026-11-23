// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"testing"

	oftls "github.com/Rain-kl/Wavelet/internal/apps/openflare/tls"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshSiteSnapshotCreatesAlignedPingoraPayload(t *testing.T) {
	cleanup := setupConfigVersionTestDB(t)
	defer cleanup()
	ctx := context.Background()
	require.NoError(t, db.DB(ctx).AutoMigrate(&model.TLSCertificate{}))

	oldSecret := config.Config.App.SessionSecret
	config.Config.App.SessionSecret = "pingora-payload-test-secret"
	t.Cleanup(func() { config.Config.App.SessionSecret = oldSecret })

	certPEM, keyPEM := generateTestCertKeyPairForSnapshotForDomain(t, "pingora.example.com")
	certificate, err := oftls.CreateCertificate(ctx, oftls.CertificateInput{
		Name: "pingora-cert", CertPEM: certPEM, KeyPEM: keyPEM,
	})
	require.NoError(t, err)

	route := &model.ProxyRoute{
		SiteName: "pingora-site", OriginURL: "http://origin.ping.example:8080",
		Upstreams: `["http://origin.ping.example:8080"]`, Enabled: true,
		EnableHTTPS: true, EnableHTTP2: true, RedirectHTTP: true,
	}
	require.NoError(t, repository.CreateProxyRouteRecord(ctx, route))
	zone := &model.Zone{Domain: "ping.example.com"}
	require.NoError(t, db.DB(ctx).Create(zone).Error)
	require.NoError(t, db.DB(ctx).Create(&model.ZoneDomain{
		ZoneID: zone.ID, ProxyRouteID: &route.ID, Domain: "pingora.example.com", CertID: &certificate.ID,
	}).Error)

	require.NoError(t, db.DB(ctx).Create(&model.SystemConfig{
		Key: model.ConfigKeyPoWPage, Value: `{"template":"shield","widget":"turnstile"}`, Type: "business", Visibility: 0,
		Description: "PoW challenge page",
	}).Error)

	require.NoError(t, RefreshSiteSnapshot(ctx))
	pingoraVersion, err := repository.GetActivePingoraConfigVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), pingoraVersion.ReleaseSequence)

	require.Empty(t, pingoraVersion.SourceVersion)
	require.NotEmpty(t, pingoraVersion.SnapshotJSON)
	assert.NotContains(t, pingoraVersion.SupportFilesJSON, "PRIVATE KEY")
	assert.NotContains(t, pingoraVersion.SupportFilesJSON, ".key")

	payloadJSON, err := edgeconfig.Encode(pingoraVersion.SnapshotJSON, pingoraVersion.SupportFilesJSON)
	require.NoError(t, err)
	assert.NotContains(t, payloadJSON, "PRIVATE KEY")
	assert.NotContains(t, payloadJSON, "key_pem")

	var payload edgeconfig.Envelope
	require.NoError(t, json.Unmarshal([]byte(payloadJSON), &payload))
	assert.Equal(t, edgeconfig.ContractVersion, payload.ContractVersion)
	assert.JSONEq(t, pingoraVersion.SnapshotJSON, string(payload.Snapshot))
	var snapshot snapshotDocument
	require.NoError(t, json.Unmarshal(payload.Snapshot, &snapshot))
	require.Len(t, snapshot.Routes, 1)
	assert.Equal(t, []string{"tls-sni-h2-v1", "policy-runtime-v1", "shared-snapshot-v1", "tls-memory-keys-v1"}, payload.RequiredCapabilities)
	assert.Equal(t, "shield", snapshot.EdgeSettings.PoWPage.Template)
	assert.Equal(t, "turnstile", snapshot.EdgeSettings.PoWPage.Widget)
	certAsset, ok := payload.Assets.Certificates[strconv.FormatUint(uint64(certificate.ID), 10)]
	require.True(t, ok)
	assert.Equal(t, edgeconfig.CertificatePEMDigest(certPEM), certAsset.CertSHA256)

	digest := sha256.Sum256([]byte(payloadJSON))
	assert.Equal(t, hex.EncodeToString(digest[:]), pingoraVersion.PayloadChecksum)

}
