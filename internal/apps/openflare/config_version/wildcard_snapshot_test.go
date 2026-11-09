// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	oftls "github.com/Rain-kl/Wavelet/internal/apps/openflare/tls"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/zone"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	openrestyrender "github.com/Rain-kl/Wavelet/pkg/render/openresty"
)

func TestWildcardDomainConfigBundle(t *testing.T) {
	t.Cleanup(setupConfigVersionTestDB(t))
	ctx := context.Background()
	if err := db.DB(ctx).AutoMigrate(&model.TLSCertificate{}); err != nil {
		t.Fatal(err)
	}
	oldSecret := config.Config.App.SessionSecret
	config.Config.App.SessionSecret = "wildcard-snapshot-test-secret"
	t.Cleanup(func() { config.Config.App.SessionSecret = oldSecret })
	certPEM, keyPEM := generateTestCertKeyPairForSnapshotForDomain(t, "*.s3.example.com")
	cert, err := oftls.CreateCertificate(ctx, oftls.CertificateInput{Name: "wildcard", CertPEM: certPEM, KeyPEM: keyPEM})
	if err != nil {
		t.Fatal(err)
	}
	root, err := zone.Create(ctx, zone.Input{Domain: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	domain, err := zone.CreateDomain(ctx, root.ID, zone.DomainInput{Domain: "*.s3.example.com", CertID: &cert.ID})
	if err != nil {
		t.Fatalf("CreateDomain(wildcard) error = %v", err)
	}
	route := &model.ProxyRoute{SiteName: "wildcard-site", OriginURL: "http://origin:8080", Upstreams: `["http://origin:8080"]`, Enabled: true, EnableHTTPS: true, RedirectHTTP: true}
	if err := repository.CreateProxyRouteRecord(ctx, route); err != nil {
		t.Fatal(err)
	}
	if err := repository.ReplaceZoneDomainRouteBindings(ctx, route.ID, []uint{domain.ID}); err != nil {
		t.Fatal(err)
	}
	bundle, err := buildCurrentConfigBundle(ctx, true)
	if err != nil {
		t.Fatalf("buildCurrentConfigBundle(wildcard) error = %v", err)
	}
	var doc openrestyrender.Document
	if err := json.Unmarshal([]byte(bundle.SnapshotJSON), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Routes) != 1 || !slices.Equal(doc.Routes[0].Domains, []string{domain.Domain}) || !slices.Equal(doc.Routes[0].DomainCertIDs, []uint{cert.ID}) {
		t.Fatalf("snapshot routes = %+v, want wildcard domain and its certificate", doc.Routes)
	}
	if !strings.Contains(bundle.RouteConfig, "server_name *.s3.example.com;") || !strings.Contains(bundle.RouteConfig, "listen 443 ssl;") {
		t.Errorf("wildcard bundle missing HTTPS server: %s", bundle.RouteConfig)
	}
}
