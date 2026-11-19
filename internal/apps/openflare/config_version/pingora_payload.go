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
	openrestyrender "github.com/Rain-kl/Wavelet/internal/openflare/openresty"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

const (
	pingoraEngineName       = "pingora"
	pingoraContractVersion  = 2
	pingoraVersionPrefix    = "pingora-"
	pingoraBaseCapability   = "tls-sni-h2-v1"
	pingoraPolicyCapability = "policy-runtime-v1"
	pingoraVersionParts     = 2
)

type pingoraConfigPayload struct {
	SchemaVersion        int                           `json:"schema_version"`
	ContractVersion      int                           `json:"contract_version"`
	RequiredCapabilities []string                      `json:"required_capabilities"`
	Routes               []pingoraRoute                `json:"routes"`
	WAF                  snapshotWAFDocument           `json:"waf"`
	Assets               pingoraAssets                 `json:"assets"`
	PoWPage              openrestyrender.PoWPageConfig `json:"pow_page,omitempty"`
}

type pingoraRoute struct {
	ID                uint                             `json:"id"`
	SiteName          string                           `json:"site_name"`
	Domains           []string                         `json:"domains"`
	DomainCertIDs     []uint                           `json:"domain_cert_ids"`
	OriginURL         string                           `json:"origin_url"`
	OriginHost        string                           `json:"origin_host,omitempty"`
	Upstreams         []string                         `json:"upstreams"`
	Enabled           bool                             `json:"enabled"`
	EnableHTTPS       bool                             `json:"enable_https"`
	EnableHTTP2       bool                             `json:"enable_http2"`
	RedirectHTTP      bool                             `json:"redirect_http"`
	Limit             pingoraLimitConfig               `json:"limit,omitempty"`
	PoWEnabled        bool                             `json:"pow_enabled,omitempty"`
	PoW               *openrestyrender.PoWConfig       `json:"pow,omitempty"`
	BasicAuthEnabled  bool                             `json:"basic_auth_enabled,omitempty"`
	BasicAuthUsername string                           `json:"basic_auth_username,omitempty"`
	BasicAuthPassword string                           `json:"basic_auth_password,omitempty"`
	SiteAuthTTLHours  int                              `json:"site_auth_ttl_hours,omitempty"`
	SiteAuthKey       string                           `json:"site_auth_key,omitempty"`
	UpstreamType      string                           `json:"upstream_type,omitempty"`
	Pages             *openrestyrender.PagesDeployment `json:"pages,omitempty"`
}

type pingoraLimitConfig struct {
	ConnPerServer int    `json:"conn_per_server,omitempty"`
	ConnPerIP     int    `json:"conn_per_ip,omitempty"`
	Rate          string `json:"rate,omitempty"`
	ReqPerIP      string `json:"req_per_ip,omitempty"`
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
			ID:                route.ID,
			SiteName:          route.SiteName,
			Domains:           route.Domains,
			DomainCertIDs:     route.DomainCertIDs,
			OriginURL:         route.OriginURL,
			OriginHost:        route.OriginHost,
			Upstreams:         route.Upstreams,
			Enabled:           route.Enabled,
			EnableHTTPS:       route.EnableHTTPS,
			EnableHTTP2:       route.EnableHTTP2,
			RedirectHTTP:      route.RedirectHTTP,
			Limit:             mergePingoraLimitConfig(route, bundle.OpenRestyConfig),
			PoWEnabled:        route.PoWEnabled,
			PoW:               route.PoWConfig,
			BasicAuthEnabled:  route.BasicAuthEnabled,
			BasicAuthUsername: route.BasicAuthUsername,
			BasicAuthPassword: route.BasicAuthPassword,
			SiteAuthTTLHours:  route.SiteAuthTTLHours,
			SiteAuthKey:       route.SiteAuthKey,
			UpstreamType:      route.UpstreamType,
			Pages:             route.PagesDeployment,
		})
	}
	certificates, err := buildPingoraCertificateAssets(ctx, routes)
	if err != nil {
		return nil, err
	}
	payload := pingoraConfigPayload{
		SchemaVersion:        1,
		ContractVersion:      pingoraContractVersion,
		RequiredCapabilities: []string{pingoraBaseCapability, pingoraPolicyCapability},
		Routes:               routes,
		WAF:                  bundle.WAFSnapshot,
		Assets: pingoraAssets{
			Certificates: certificates,
		},
		PoWPage: bundle.OpenRestyConfig.PoWPage,
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

func mergePingoraLimitConfig(route snapshotRoute, cfg openRestyConfigSnapshot) pingoraLimitConfig {
	return pingoraLimitConfig{
		ConnPerServer: mergePingoraLimitInt(route.LimitConnPerServer, cfg.DefaultLimitConnPerServer),
		ConnPerIP:     mergePingoraLimitInt(route.LimitConnPerIP, cfg.DefaultLimitConnPerIP),
		Rate:          mergePingoraLimitRate(route.LimitRate, cfg.DefaultLimitRate),
		ReqPerIP:      mergePingoraLimitRate(route.LimitReqPerIP, cfg.DefaultLimitReqPerIP),
	}
}

func mergePingoraLimitInt(route, def int) int {
	if route == -1 || route == 0 {
		if def > 0 {
			return def
		}
		return 0
	}
	return route
}

func mergePingoraLimitRate(route, def string) string {
	value := strings.ToLower(strings.TrimSpace(route))
	if value == "" || value == "-1" || value == "0" {
		value = strings.ToLower(strings.TrimSpace(def))
	}
	if value == "0" {
		return ""
	}
	return value
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
	for _, group := range payload.WAF.RuleGroups {
		if !group.Enabled {
			continue
		}
		if group.Graph.Entry == "" || group.Graph.Nodes == nil {
			return fmt.Errorf("WAF rule %s has an invalid graph", group.Name)
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
