// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package edgeconfig transports the shared business snapshot and its assets to edge nodes.
package edgeconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	// ContractVersion is the Edge configuration contract supported by this controller.
	ContractVersion             = 4
	pingoraBaseCapability       = "tls-sni-h2-v1"
	pingoraPolicyCapability     = "policy-runtime-v1"
	pingoraSnapshotCapability   = "shared-snapshot-v1"
	pingoraMemoryKeysCapability = "tls-memory-keys-v1"
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

// CertificateAsset carries a published public certificate chain and its normalized SHA-256 digest.
// Private keys are never embedded in configuration snapshots.
type CertificateAsset struct {
	CertPEM    string `json:"cert_pem"`
	CertSHA256 string `json:"cert_sha256"`
}

// CertificatePEMDigest returns the SHA-256 hex digest of a normalized certificate PEM chain.
func CertificatePEMDigest(certPEM string) string {
	trimmed := strings.TrimSpace(certPEM)
	if trimmed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(trimmed + "\n"))
	return hex.EncodeToString(sum[:])
}

// Encode encodes an immutable published snapshot for a node.
// Only public certificate chains are embedded; private keys are fetched separately in memory.
func Encode(snapshotJSON, supportFilesJSON string) (string, error) {
	var snapshot Document
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil {
		return "", fmt.Errorf("decode shared edge snapshot: %w", err)
	}
	var files []SupportFile
	if err := json.Unmarshal([]byte(supportFilesJSON), &files); err != nil {
		return "", fmt.Errorf("decode shared edge assets: %w", err)
	}
	assets, err := sharedEdgeAssets(snapshot, files)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(Envelope{
		SchemaVersion: 1, ContractVersion: ContractVersion,
		RequiredCapabilities: []string{pingoraBaseCapability, pingoraPolicyCapability, pingoraSnapshotCapability, pingoraMemoryKeysCapability},
		Snapshot:             json.RawMessage(snapshotJSON), Assets: assets,
	})
	return string(data), err
}

func sharedEdgeAssets(snapshot Document, files []SupportFile) (Assets, error) {
	byPath := make(map[string]string, len(files))
	for _, file := range files {
		byPath[file.Path] = file.Content
	}
	assets := Assets{
		Certificates:             make(map[string]CertificateAsset),
		StaticCacheExtensions:    snapshot.Settings.CacheStaticExtensions,
		OriginErrorHTML:          byPath[OriginErrorPageSupportPath],
		ServiceWorkerScript:      byPath["sw/sw.js"],
		ServiceWorkerOfflineHTML: byPath["sw/offline.html"],
	}
	if snapshot.Settings.OriginErrorPageEnabled && assets.OriginErrorHTML == "" {
		return Assets{}, fmt.Errorf("missing published origin error page asset")
	}
	if snapshot.Settings.SWOfflineEnabled && len(snapshot.Settings.SWOfflineDomains) > 0 &&
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
			rawCert := strings.TrimSpace(byPath[id+".crt"])
			if rawCert == "" {
				return Assets{}, fmt.Errorf("missing published certificate asset %s", id)
			}
			cert := rawCert + "\n"
			assets.Certificates[id] = CertificateAsset{
				CertPEM:    cert,
				CertSHA256: CertificatePEMDigest(cert),
			}
		}
	}
	return assets, nil
}

// ActiveCertificateDigests returns certificate IDs and public PEM SHA-256 digests
// referenced by enabled HTTPS routes in a published release.
func ActiveCertificateDigests(snapshotJSON, supportFilesJSON string) (map[uint]string, error) {
	var snapshot Document
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil {
		return nil, fmt.Errorf("decode shared edge snapshot: %w", err)
	}
	var files []SupportFile
	if err := json.Unmarshal([]byte(supportFilesJSON), &files); err != nil {
		return nil, fmt.Errorf("decode shared edge assets: %w", err)
	}
	byPath := make(map[string]string, len(files))
	for _, file := range files {
		byPath[file.Path] = file.Content
	}
	digests := make(map[uint]string)
	for _, route := range snapshot.Routes {
		if !route.Enabled || !route.EnableHTTPS {
			continue
		}
		for _, certID := range route.DomainCertIDs {
			if certID == 0 {
				continue
			}
			if _, ok := digests[certID]; ok {
				continue
			}
			id := strconv.FormatUint(uint64(certID), 10)
			digest := CertificatePEMDigest(byPath[id+".crt"])
			if digest == "" {
				return nil, fmt.Errorf("missing published certificate asset %s", id)
			}
			digests[certID] = digest
		}
	}
	return digests, nil
}

// ContainsPagesProject reports whether a published snapshot references a Pages project.
func ContainsPagesProject(snapshotJSON string, projectID uint) (bool, error) {
	var snapshot Document
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
