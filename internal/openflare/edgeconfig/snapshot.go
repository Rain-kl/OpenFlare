// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package edgeconfig transports the shared business snapshot and its assets to edge nodes.
package edgeconfig

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	openrestyrender "github.com/Rain-kl/Wavelet/internal/openflare/openresty"
)

const (
	// ContractVersion is the Edge configuration contract supported by this controller.
	ContractVersion           = 3
	pingoraBaseCapability     = "tls-sni-h2-v1"
	pingoraPolicyCapability   = "policy-runtime-v1"
	pingoraSnapshotCapability = "shared-snapshot-v1"
)

// Envelope carries the exact shared business snapshot and its immutable assets.
type Envelope struct {
	SchemaVersion        int             `json:"schema_version"`
	ContractVersion      int             `json:"contract_version"`
	RequiredCapabilities []string        `json:"required_capabilities"`
	Snapshot             json.RawMessage `json:"snapshot"`
	Assets               Assets          `json:"assets"`
}

// Assets contains resources captured with the shared snapshot.
type Assets struct {
	Certificates             map[string]CertificateAsset `json:"certificates"`
	StaticCacheExtensions    []string                    `json:"static_cache_extensions"`
	OriginErrorHTML          string                      `json:"origin_error_html,omitempty"`
	ServiceWorkerScript      string                      `json:"service_worker_script,omitempty"`
	ServiceWorkerOfflineHTML string                      `json:"service_worker_offline_html,omitempty"`
}

// CertificateAsset carries a published certificate chain and its private key.
type CertificateAsset struct {
	CertPEM string `json:"cert_pem"`
	KeyPEM  string `json:"key_pem"`
}

// Encode encodes an immutable published snapshot for a node.
// All assets are read from the published release, including certificates that have since rotated.
func Encode(snapshotJSON, supportFilesJSON string) (string, error) {
	var snapshot openrestyrender.Document
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil {
		return "", fmt.Errorf("decode shared edge snapshot: %w", err)
	}
	var files []openrestyrender.SupportFile
	if err := json.Unmarshal([]byte(supportFilesJSON), &files); err != nil {
		return "", fmt.Errorf("decode shared edge assets: %w", err)
	}
	assets, err := sharedEdgeAssets(snapshot, files)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(Envelope{
		SchemaVersion: 1, ContractVersion: ContractVersion,
		RequiredCapabilities: []string{pingoraBaseCapability, pingoraPolicyCapability, pingoraSnapshotCapability},
		Snapshot:             json.RawMessage(snapshotJSON), Assets: assets,
	})
	return string(data), err
}

func sharedEdgeAssets(snapshot openrestyrender.Document, files []openrestyrender.SupportFile) (Assets, error) {
	byPath := make(map[string]string, len(files))
	for _, file := range files {
		byPath[file.Path] = file.Content
	}
	assets := Assets{
		Certificates:             make(map[string]CertificateAsset),
		StaticCacheExtensions:    snapshot.OpenRestyConfig.CacheStaticExtensions,
		OriginErrorHTML:          byPath[openrestyrender.OriginErrorPageSupportPath],
		ServiceWorkerScript:      byPath["sw/sw.js"],
		ServiceWorkerOfflineHTML: byPath["sw/offline.html"],
	}
	if snapshot.OpenRestyConfig.OriginErrorPageEnabled && assets.OriginErrorHTML == "" {
		return Assets{}, fmt.Errorf("missing published origin error page asset")
	}
	if snapshot.OpenRestyConfig.SWOfflineEnabled && len(snapshot.OpenRestyConfig.SWOfflineDomains) > 0 &&
		(assets.ServiceWorkerScript == "" || assets.ServiceWorkerOfflineHTML == "") {
		return Assets{}, fmt.Errorf("missing published Service Worker assets")
	}
	for _, route := range snapshot.Routes {
		if !route.Enabled {
			continue
		}
		for _, certID := range route.DomainCertIDs {
			if certID == 0 {
				continue
			}
			id := strconv.FormatUint(uint64(certID), 10)
			if _, ok := assets.Certificates[id]; ok {
				continue
			}
			cert := byPath[id+".crt"]
			key := byPath[id+".key"]
			if strings.TrimSpace(cert) == "" || strings.TrimSpace(key) == "" {
				return Assets{}, fmt.Errorf("missing published certificate asset %s", id)
			}
			assets.Certificates[id] = CertificateAsset{CertPEM: cert, KeyPEM: key}
		}
	}
	return assets, nil
}

// ContainsPagesProject reports whether a published snapshot references a Pages project.
func ContainsPagesProject(snapshotJSON string, projectID uint) (bool, error) {
	var snapshot openrestyrender.Document
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil {
		return false, err
	}
	for _, route := range snapshot.Routes {
		if route.Enabled && route.PagesDeployment != nil && route.PagesDeployment.ProjectID == projectID {
			return true, nil
		}
	}
	return false, nil
}
