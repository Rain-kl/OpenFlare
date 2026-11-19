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

	var snapshot snapshotDocument
	require.NoError(t, json.Unmarshal([]byte(pingoraVersion.SnapshotJSON), &snapshot))
	var payload pingoraConfigPayload
	require.NoError(t, json.Unmarshal([]byte(pingoraVersion.PayloadJSON), &payload))
	require.Len(t, payload.Routes, 1)
	require.Len(t, snapshot.Routes, 1)
	assert.Equal(t, snapshot.Routes[0].SiteName, payload.Routes[0].SiteName)
	assert.Equal(t, snapshot.Routes[0].Domains, payload.Routes[0].Domains)
	assert.Equal(t, snapshot.Routes[0].DomainCertIDs, payload.Routes[0].DomainCertIDs)
	assert.Equal(t, snapshot.Routes[0].Upstreams, payload.Routes[0].Upstreams)
	assert.Equal(t, snapshot.Routes[0].EnableHTTPS, payload.Routes[0].EnableHTTPS)
	assert.Equal(t, snapshot.Routes[0].EnableHTTP2, payload.Routes[0].EnableHTTP2)
	assert.Equal(t, snapshot.Routes[0].RedirectHTTP, payload.Routes[0].RedirectHTTP)
	assert.Equal(t, snapshot.WAF, payload.WAF)
	assert.Equal(t, []string{"tls-sni-h2-v1", "policy-runtime-v1"}, payload.RequiredCapabilities)
	assert.Equal(t, "shield", payload.PoWPage.Template)
	assert.Equal(t, "turnstile", payload.PoWPage.Widget)
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
	assert.Equal(t, openRestyPoWPage.Template, payload.PoWPage.Template)
	assert.Equal(t, openRestyPoWPage.Widget, payload.PoWPage.Widget)
	// Both engines resolve the built-in shield template locally; Pingora embeds the same source file.
	assert.Equal(t, openresty.DefaultPoWPageHTML(payload.PoWPage.Template), openRestyPoWPage.HTML)
	assert.Contains(t, payload.Assets.Certificates, strconv.FormatUint(uint64(certificate.ID), 10))

	digest := sha256.Sum256([]byte(pingoraVersion.PayloadJSON))
	assert.Equal(t, hex.EncodeToString(digest[:]), pingoraVersion.PayloadChecksum)
}

func TestBuildPingoraConfigVersionRespectsEnabledSetting(t *testing.T) {
	cleanup := setupConfigVersionTestDB(t)
	defer cleanup()
	ctx := context.Background()
	require.NoError(t, db.DB(ctx).Create(&model.SystemConfig{
		Key: model.ConfigKeyPingoraEnabled, Value: "false", Type: "business", Visibility: 0,
		Description: "Pingora 独立发布组开关",
	}).Error)

	version, err := buildPingoraConfigVersion(ctx, &configBundle{}, &model.ConfigVersion{
		Version: "20261119-001", Engine: "openresty", ReleaseSequence: 1,
	})
	require.NoError(t, err)
	assert.Nil(t, version)
}
