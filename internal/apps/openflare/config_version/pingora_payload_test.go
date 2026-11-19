// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"strconv"
	"testing"

	oftls "github.com/Rain-kl/Wavelet/internal/apps/openflare/tls"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/openresty"
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

	require.NoError(t, db.DB(ctx).Create(&model.SystemConfig{
		Key: model.ConfigKeyPingoraEnabled, Value: "true", Type: "business", Visibility: 0,
		Description: "Pingora 独立发布组开关",
	}).Error)

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

	openRestyVersion, err := PublishConfigVersion(ctx, "tester", false)
	require.NoError(t, err)
	pingoraVersion, err := repository.GetActivePingoraConfigVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, "pingora-"+openRestyVersion.Version, pingoraVersion.Version)
	require.Equal(t, int64(1), pingoraVersion.ReleaseSequence)

	require.Equal(t, openRestyVersion.Version, pingoraVersion.SourceVersion)
	require.Empty(t, pingoraVersion.SnapshotJSON)
	payloadJSON, err := edgeconfig.Encode(openRestyVersion.SnapshotJSON, openRestyVersion.SupportFilesJSON)
	require.NoError(t, err)
	var payload edgeconfig.Envelope
	require.NoError(t, json.Unmarshal([]byte(payloadJSON), &payload))
	assert.JSONEq(t, openRestyVersion.SnapshotJSON, string(payload.Snapshot))
	var snapshot snapshotDocument
	require.NoError(t, json.Unmarshal(payload.Snapshot, &snapshot))
	require.Len(t, snapshot.Routes, 1)
	assert.Equal(t, []string{"tls-sni-h2-v1", "policy-runtime-v1", "shared-snapshot-v1"}, payload.RequiredCapabilities)
	assert.Equal(t, "shield", snapshot.OpenRestyConfig.PoWPage.Template)
	assert.Equal(t, "turnstile", snapshot.OpenRestyConfig.PoWPage.Widget)
	var supportFiles []SupportFile
	require.NoError(t, json.Unmarshal([]byte(openRestyVersion.SupportFilesJSON), &supportFiles))
	var openRestyPoWPage openresty.PoWPageConfig
	found := false
	for _, file := range supportFiles {
		if file.Path != openresty.PoWPageSupportPath {
			continue
		}
		found = true
		require.NoError(t, json.Unmarshal([]byte(file.Content), &openRestyPoWPage))
	}
	require.True(t, found, "OpenResty PoW page support file was not rendered")
	assert.Equal(t, openRestyPoWPage.Template, snapshot.OpenRestyConfig.PoWPage.Template)
	assert.Equal(t, openRestyPoWPage.Widget, snapshot.OpenRestyConfig.PoWPage.Widget)
	// Both engines resolve the built-in shield template locally; Pingora embeds the same source file.
	assert.Equal(t, openresty.DefaultPoWPageHTML(snapshot.OpenRestyConfig.PoWPage.Template), openRestyPoWPage.HTML)
	assert.Contains(t, payload.Assets.Certificates, strconv.FormatUint(uint64(certificate.ID), 10))

	digest := sha256.Sum256([]byte(payloadJSON))
	assert.Equal(t, hex.EncodeToString(digest[:]), pingoraVersion.PayloadChecksum)

	next, err := PublishConfigVersion(ctx, "tester", true)
	require.NoError(t, err)
	nextEdge, err := repository.GetActivePingoraConfigVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), nextEdge.ReleaseSequence)
	rolledBack, err := ActivateConfigVersion(ctx, pingoraVersion.Version)
	require.NoError(t, err)
	assert.Equal(t, int64(3), rolledBack.ReleaseSequence)
	assert.Equal(t, openRestyVersion.Version, rolledBack.SourceVersion)
	assert.Equal(t, pingoraVersion.PayloadChecksum, rolledBack.PayloadChecksum)
	currentOpenResty, err := repository.GetActiveConfigVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, next.Version, currentOpenResty.Version)
	_, err = repository.DeleteConfigVersionsByVersions(ctx, []string{openRestyVersion.Version, pingoraVersion.Version})
	require.NoError(t, err)
	_, err = repository.GetConfigVersionByVersion(ctx, openRestyVersion.Version)
	require.NoError(t, err, "a retained Edge release protects its source snapshot")

}

func TestBuildPingoraConfigVersionRespectsEnabledSetting(t *testing.T) {
	cleanup := setupConfigVersionTestDB(t)
	defer cleanup()
	ctx := context.Background()
	require.NoError(t, db.DB(ctx).Create(&model.SystemConfig{
		Key: model.ConfigKeyPingoraEnabled, Value: "false", Type: "business", Visibility: 0,
		Description: "Pingora 独立发布组开关",
	}).Error)

	version, err := buildPingoraConfigVersion(ctx, &model.ConfigVersion{
		Version: "20261119-001", Engine: "openresty", ReleaseSequence: 1,
	})
	require.NoError(t, err)
	assert.Nil(t, version)
}
