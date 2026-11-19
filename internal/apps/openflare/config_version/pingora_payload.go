// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

const (
	pingoraEngineName         = "pingora"
	pingoraContractVersion    = 1
	pingoraVersionPrefix      = "pingora-"
	pingoraRequiredCapability = "tls-sni-h2-v1"
	pingoraVersionParts       = 2
)

type pingoraConfigPayload struct {
	SchemaVersion        int            `json:"schema_version"`
	ContractVersion      int            `json:"contract_version"`
	RequiredCapabilities []string       `json:"required_capabilities"`
	Routes               []pingoraRoute `json:"routes"`
	Assets               pingoraAssets  `json:"assets"`
}

type pingoraRoute struct {
	ID            uint     `json:"id"`
	SiteName      string   `json:"site_name"`
	Domains       []string `json:"domains"`
	DomainCertIDs []uint   `json:"domain_cert_ids"`
	OriginURL     string   `json:"origin_url"`
	OriginHost    string   `json:"origin_host,omitempty"`
	Upstreams     []string `json:"upstreams"`
	Enabled       bool     `json:"enabled"`
	EnableHTTPS   bool     `json:"enable_https"`
	EnableHTTP2   bool     `json:"enable_http2"`
	RedirectHTTP  bool     `json:"redirect_http"`
}

type pingoraAssets struct {
	Certificates map[string]pingoraCertificateAsset `json:"certificates"`
}

type pingoraCertificateAsset struct {
	CertPEM string `json:"cert_pem"`
	KeyPEM  string `json:"key_pem"`
}

// buildPingoraConfigVersion derives a Pingora release from one business snapshot.
func buildPingoraConfigVersion(ctx context.Context, bundle *configBundle, openRestyVersion *model.ConfigVersion) (*model.ConfigVersion, error) {
	if bundle == nil || openRestyVersion == nil {
		return nil, nil
	}
	enabled, err := repository.GetBoolByKey(ctx, model.ConfigKeyPingoraEnabled)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, nil
	}

	routes := make([]pingoraRoute, 0, len(bundle.SnapshotRoutes))
	for _, route := range bundle.SnapshotRoutes {
		routes = append(routes, pingoraRoute{
			ID:            route.ID,
			SiteName:      route.SiteName,
			Domains:       route.Domains,
			DomainCertIDs: route.DomainCertIDs,
			OriginURL:     route.OriginURL,
			OriginHost:    route.OriginHost,
			Upstreams:     route.Upstreams,
			Enabled:       route.Enabled,
			EnableHTTPS:   route.EnableHTTPS,
			EnableHTTP2:   route.EnableHTTP2,
			RedirectHTTP:  route.RedirectHTTP,
		})
	}
	certificates, err := buildPingoraCertificateAssets(ctx, routes)
	if err != nil {
		return nil, err
	}
	payload := pingoraConfigPayload{
		SchemaVersion:        1,
		ContractVersion:      pingoraContractVersion,
		RequiredCapabilities: []string{pingoraRequiredCapability},
		Routes:               routes,
		Assets: pingoraAssets{
			Certificates: certificates,
		},
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if err := validatePingoraPayload(payload); err != nil {
		return nil, err
	}
	releaseSequence, err := pingoraReleaseSequence(openRestyVersion.Version)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(payloadJSON)
	return &model.ConfigVersion{
		Version:          pingoraVersionPrefix + openRestyVersion.Version,
		SnapshotJSON:     bundle.SnapshotJSON,
		MainConfig:       openRestyVersion.MainConfig,
		RenderedConfig:   openRestyVersion.RenderedConfig,
		SupportFilesJSON: openRestyVersion.SupportFilesJSON,
		Checksum:         openRestyVersion.Checksum,
		IsActive:         true,
		Engine:           pingoraEngineName,
		ReleaseSequence:  releaseSequence,
		PayloadJSON:      string(payloadJSON),
		PayloadChecksum:  hex.EncodeToString(digest[:]),
		CreatedBy:        openRestyVersion.CreatedBy,
	}, nil
}

func buildPingoraCertificateAssets(ctx context.Context, routes []pingoraRoute) (map[string]pingoraCertificateAsset, error) {
	certIDSet := make(map[uint]struct{})
	for _, route := range routes {
		for _, certID := range route.DomainCertIDs {
			if certID != 0 {
				certIDSet[certID] = struct{}{}
			}
		}
	}
	assets := make(map[string]pingoraCertificateAsset, len(certIDSet))
	for certID := range certIDSet {
		files, err := buildCertificateSupportFiles(ctx, []snapshotRoute{{DomainCertIDs: []uint{certID}}})
		if err != nil {
			return nil, err
		}
		cert, key, err := pingoraCertificatePair(files, certID)
		if err != nil {
			return nil, err
		}
		assets[strconv.FormatUint(uint64(certID), 10)] = pingoraCertificateAsset{CertPEM: cert, KeyPEM: key}
	}
	return assets, nil
}

func pingoraCertificatePair(files []SupportFile, certID uint) (string, string, error) {
	var cert, key string
	for _, file := range files {
		if file.Path == certificateCertFileName(certID) {
			cert = file.Content
		}
		if file.Path == certificateKeyFileName(certID) {
			key = file.Content
		}
	}
	if strings.TrimSpace(cert) == "" || strings.TrimSpace(key) == "" {
		return "", "", fmt.Errorf("certificate %d is incomplete", certID)
	}
	return cert, key, nil
}

func validatePingoraPayload(payload pingoraConfigPayload) error {
	if payload.SchemaVersion != 1 || payload.ContractVersion != pingoraContractVersion {
		return errors.New("invalid Pingora contract version")
	}
	for _, route := range payload.Routes {
		if !route.Enabled {
			continue
		}
		if len(route.Domains) == 0 {
			return fmt.Errorf("enabled route %s has no domains", route.SiteName)
		}
		if len(route.Upstreams) == 0 && strings.TrimSpace(route.OriginURL) == "" {
			return fmt.Errorf("enabled route %s has no upstream", route.SiteName)
		}
		if len(route.DomainCertIDs) != len(route.Domains) {
			return fmt.Errorf("route %s has inconsistent domain certificate references", route.SiteName)
		}
		for index, certID := range route.DomainCertIDs {
			if certID == 0 {
				continue
			}
			if _, ok := payload.Assets.Certificates[strconv.FormatUint(uint64(certID), 10)]; !ok {
				return fmt.Errorf("certificate %d for route %s domain %s is missing", certID, route.SiteName, route.Domains[index])
			}
		}
	}
	return nil
}

func pingoraReleaseSequence(version string) (int64, error) {
	parts := strings.Split(version, "-")
	if len(parts) != pingoraVersionParts {
		return 0, fmt.Errorf("invalid OpenResty config version %q", version)
	}
	sequence, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid OpenResty config sequence %q: %w", version, err)
	}
	return sequence, nil
}
