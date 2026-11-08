// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package sync

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/state"
	openrestyrender "github.com/Rain-kl/Wavelet/pkg/render/openresty"
)

func TestSyncWildcardConfig(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"*.s3.example.com"}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, publicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	files := []protocol.SupportFile{
		{Path: "1.crt", Content: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))},
		{Path: "1.key", Content: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))},
	}
	for _, https := range []bool{false, true} {
		t.Run(map[bool]string{false: "HTTP", true: "HTTPS"}[https], func(t *testing.T) {
			doc := openrestyrender.Document{Routes: []openrestyrender.Route{{
				ID: 1, SiteName: "wildcard", Domains: []string{"*.s3.example.com"},
				OriginURL: "http://127.0.0.1:8080", EnableHTTPS: https, RedirectHTTP: https,
				DomainCertIDs: []uint{1}, Enabled: true,
			}}}
			source, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			client := &fakeClient{config: protocol.ActiveConfigResponse{
				Version: "wildcard-v1", Checksum: "wildcard-checksum", SourceConfigJSON: string(source), SupportFiles: files,
			}}
			dir := t.TempDir()
			store := state.NewStore(filepath.Join(dir, "state.json"))
			if _, err := store.EnsureNodeID(); err != nil {
				t.Fatal(err)
			}
			manager := &fakeManager{}
			service := New(client, manager, store)
			if err := service.SyncOnce(context.Background(), &protocol.ActiveConfigMeta{Version: client.config.Version, Checksum: client.config.Checksum}); err != nil {
				t.Fatalf("SyncOnce(wildcard, HTTPS=%t) error = %v", https, err)
			}
			if len(manager.applyRouteContents) != 1 {
				t.Fatalf("agent apply calls = %d, want 1", len(manager.applyRouteContents))
			}
			content := manager.applyRouteContents[0]
			if !strings.Contains(content, "server_name *.s3.example.com;") {
				t.Errorf("agent apply config missing wildcard server_name: %s", content)
			}
			if https && !strings.Contains(content, "listen 443 ssl;") {
				t.Error("agent apply config missing wildcard HTTPS server")
			}
			if len(client.reports) != 1 || client.reports[0].Result != ApplyResultSuccess {
				t.Errorf("SyncOnce(wildcard) apply reports = %+v, want success", client.reports)
			}
			snapshot, err := store.Load()
			if err != nil || snapshot.CurrentVersion != client.config.Version {
				t.Errorf("agent applied version = %+v, error=%v, want %s", snapshot, err, client.config.Version)
			}
		})
	}
}
