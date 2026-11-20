// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
)

// SupportFile is an immutable configuration asset.
type SupportFile = edgeconfig.SupportFile

type renderedSnapshot struct {
	SupportFilesJSON string
	Checksum         string
}

func renderSnapshotConfig(sourceJSON string, certificateFiles []SupportFile) (*renderedSnapshot, error) {
	var document edgeconfig.Document
	if err := json.Unmarshal([]byte(sourceJSON), &document); err != nil {
		return nil, err
	}
	if err := validateSnapshotCertificates(document.Routes, certificateFiles); err != nil {
		return nil, err
	}
	files := append([]SupportFile{}, certificateFiles...)
	settings := document.Settings
	if settings.OriginErrorPageEnabled {
		files = append(files, SupportFile{
			Path:    edgeconfig.OriginErrorPageSupportPath,
			Content: edgeconfig.EffectiveOriginErrorPageHTML(settings),
		})
	}
	if settings.SWOfflineEnabled && len(settings.SWOfflineDomains) > 0 {
		files = append(files, edgeconfig.ServiceWorkerSupportFiles(settings)...)
	}
	encoded, err := json.Marshal(files)
	if err != nil {
		return nil, err
	}
	payload, err := edgeconfig.Encode(sourceJSON, string(encoded))
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(payload))
	return &renderedSnapshot{
		SupportFilesJSON: string(encoded),
		Checksum:         hex.EncodeToString(digest[:]),
	}, nil
}

func validateSnapshotCertificates(routes []edgeconfig.Route, files []SupportFile) error {
	byPath := make(map[string]string, len(files))
	for _, file := range files {
		byPath[file.Path] = file.Content
	}
	certificates := make(map[uint]*x509.Certificate)
	for _, route := range routes {
		for index, id := range route.DomainCertIDs {
			if id == 0 || index >= len(route.Domains) {
				continue
			}
			cert := certificates[id]
			if cert == nil {
				name := strconv.FormatUint(uint64(id), 10)
				pair, err := tls.X509KeyPair([]byte(byPath[name+".crt"]), []byte(byPath[name+".key"]))
				if err != nil {
					return fmt.Errorf("invalid certificate %d for site %s: %w", id, route.SiteName, err)
				}
				cert = pair.Leaf
				certificates[id] = cert
			}
			if err := cert.VerifyHostname(route.Domains[index]); err != nil {
				return fmt.Errorf("certificate %d does not cover %s: %w", id, route.Domains[index], err)
			}
		}
	}
	return nil
}
