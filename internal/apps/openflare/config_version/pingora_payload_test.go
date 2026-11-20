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

func TestPublishConfigVersionCreatesAlignedPingoraPayload(t *testing.T) {
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

	published, err := PublishConfigVersion(ctx, "tester", false)
	require.NoError(t, err)
	pingoraVersion, err := repository.GetActivePingoraConfigVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, published.Version, pingoraVersion.Version)
	require.Equal(t, int64(1), pingoraVersion.ReleaseSequence)

	require.Empty(t, pingoraVersion.SourceVersion)
	require.NotEmpty(t, pingoraVersion.SnapshotJSON)
	payloadJSON, err := edgeconfig.Encode(published.SnapshotJSON, published.SupportFilesJSON)
	require.NoError(t, err)
	var payload edgeconfig.Envelope
	require.NoError(t, json.Unmarshal([]byte(payloadJSON), &payload))
	assert.JSONEq(t, published.SnapshotJSON, string(payload.Snapshot))
	var snapshot snapshotDocument
	require.NoError(t, json.Unmarshal(payload.Snapshot, &snapshot))
	require.Len(t, snapshot.Routes, 1)
	assert.Equal(t, []string{"tls-sni-h2-v1", "policy-runtime-v1", "shared-snapshot-v1"}, payload.RequiredCapabilities)
	assert.Equal(t, "shield", snapshot.EdgeSettings.PoWPage.Template)
	assert.Equal(t, "turnstile", snapshot.EdgeSettings.PoWPage.Widget)
	assert.Contains(t, payload.Assets.Certificates, strconv.FormatUint(uint64(certificate.ID), 10))

	digest := sha256.Sum256([]byte(payloadJSON))
	assert.Equal(t, hex.EncodeToString(digest[:]), pingoraVersion.PayloadChecksum)

	_, err = PublishConfigVersion(ctx, "tester", true)
	require.NoError(t, err)
	nextEdge, err := repository.GetActivePingoraConfigVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), nextEdge.ReleaseSequence)
	rolledBack, err := ActivateConfigVersion(ctx, pingoraVersion.Version)
	require.NoError(t, err)
	assert.Equal(t, int64(3), rolledBack.ReleaseSequence)
	assert.Empty(t, rolledBack.SourceVersion)
	assert.Equal(t, published.SnapshotJSON, rolledBack.SnapshotJSON)
	assert.Equal(t, pingoraVersion.PayloadChecksum, rolledBack.PayloadChecksum)
	active, err := repository.GetActiveConfigVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, rolledBack.Version, active.Version)
}

func TestPromotePublishedSiteSnapshotOnce(t *testing.T) {
	t.Cleanup(setupConfigVersionTestDB(t))
	ctx := context.Background()
	historical := model.ConfigVersion{
		Version: "20261119-001", Engine: "openresty", IsActive: true,
		SnapshotJSON:     `{"routes":[],"openresty_config":{"worker_connections":999},"waf":{}}`,
		SupportFilesJSON: "[]",
		Checksum:         "historical-digest",
	}
	require.NoError(t, db.DB(ctx).Create(&historical).Error)
	require.NoError(t, EnsureActivePingoraRelease(ctx))
	require.NoError(t, EnsureActivePingoraRelease(ctx))
	active, err := repository.GetActiveConfigVersion(ctx)
	require.NoError(t, err)
	assert.Empty(t, active.SourceVersion)
	assert.Contains(t, active.SnapshotJSON, `"edge_config"`)
	assert.NotContains(t, active.SnapshotJSON, "worker_connections")
	assert.Equal(t, int64(1), active.ReleaseSequence)
	assert.NotEmpty(t, active.PayloadChecksum)
}
