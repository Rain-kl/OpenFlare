package service

import (
	"atsflare/model"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

type ReleaseResult struct {
	Version *model.ConfigVersion `json:"version"`
	Routes  []*model.ProxyRoute  `json:"routes"`
}

type SupportFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type ConfigPreviewResult struct {
	SnapshotJSON   string        `json:"snapshot_json"`
	RenderedConfig string        `json:"rendered_config"`
	SupportFiles   []SupportFile `json:"support_files"`
	Checksum       string        `json:"checksum"`
	RouteCount     int           `json:"route_count"`
}

type ConfigDiffResult struct {
	ActiveVersion   string   `json:"active_version,omitempty"`
	AddedDomains    []string `json:"added_domains"`
	RemovedDomains  []string `json:"removed_domains"`
	ModifiedDomains []string `json:"modified_domains"`
}

type snapshotRoute struct {
	Domain        string                        `json:"domain"`
	OriginURL     string                        `json:"origin_url"`
	Enabled       bool                          `json:"enabled"`
	EnableHTTPS   bool                          `json:"enable_https"`
	CertID        *uint                         `json:"cert_id,omitempty"`
	RedirectHTTP  bool                          `json:"redirect_http"`
	CustomHeaders []ProxyRouteCustomHeaderInput `json:"custom_headers,omitempty"`
	Remark        string                        `json:"remark,omitempty"`
}

type configBundle struct {
	Routes         []*model.ProxyRoute
	SnapshotRoutes []snapshotRoute
	SnapshotJSON   string
	RenderedConfig string
	SupportFiles   []SupportFile
	Checksum       string
}

const nginxCertDirPlaceholder = "__ATSF_CERT_DIR__"

func ListConfigVersions() ([]*model.ConfigVersion, error) {
	return model.ListConfigVersions()
}

func GetActiveConfigVersion() (*model.ConfigVersion, error) {
	return model.GetActiveConfigVersion()
}

func PreviewConfigVersion() (*ConfigPreviewResult, error) {
	bundle, err := buildCurrentConfigBundle(false)
	if err != nil {
		return nil, err
	}
	return &ConfigPreviewResult{
		SnapshotJSON:   bundle.SnapshotJSON,
		RenderedConfig: bundle.RenderedConfig,
		SupportFiles:   bundle.SupportFiles,
		Checksum:       bundle.Checksum,
		RouteCount:     len(bundle.Routes),
	}, nil
}

func DiffConfigVersion() (*ConfigDiffResult, error) {
	bundle, err := buildCurrentConfigBundle(false)
	if err != nil {
		return nil, err
	}
	result := &ConfigDiffResult{
		AddedDomains:    []string{},
		RemovedDomains:  []string{},
		ModifiedDomains: []string{},
	}
	activeVersion, err := model.GetActiveConfigVersion()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			for _, route := range bundle.SnapshotRoutes {
				result.AddedDomains = append(result.AddedDomains, route.Domain)
			}
			return result, nil
		}
		return nil, err
	}
	result.ActiveVersion = activeVersion.Version
	activeRoutes, err := parseSnapshotRoutes(activeVersion.SnapshotJSON)
	if err != nil {
		return nil, err
	}
	currentMap := make(map[string]snapshotRoute, len(bundle.SnapshotRoutes))
	for _, route := range bundle.SnapshotRoutes {
		currentMap[route.Domain] = route
	}
	activeMap := make(map[string]snapshotRoute, len(activeRoutes))
	for _, route := range activeRoutes {
		activeMap[route.Domain] = route
	}
	for domain, currentRoute := range currentMap {
		activeRoute, ok := activeMap[domain]
		if !ok {
			result.AddedDomains = append(result.AddedDomains, domain)
			continue
		}
		if !snapshotRouteConfigEqual(activeRoute, currentRoute) {
			result.ModifiedDomains = append(result.ModifiedDomains, domain)
		}
	}
	for domain := range activeMap {
		if _, ok := currentMap[domain]; !ok {
			result.RemovedDomains = append(result.RemovedDomains, domain)
		}
	}
	sort.Strings(result.AddedDomains)
	sort.Strings(result.RemovedDomains)
	sort.Strings(result.ModifiedDomains)
	return result, nil
}

func PublishConfigVersion(createdBy string) (*ReleaseResult, error) {
	bundle, err := buildCurrentConfigBundle(true)
	if err != nil {
		return nil, err
	}
	if len(bundle.Routes) == 0 {
		return nil, errors.New("没有可发布的启用规则")
	}
	supportFilesJSON, err := json.Marshal(bundle.SupportFiles)
	if err != nil {
		return nil, err
	}
	version, err := nextVersionNumber(time.Now())
	if err != nil {
		return nil, err
	}
	record := &model.ConfigVersion{
		Version:          version,
		SnapshotJSON:     bundle.SnapshotJSON,
		RenderedConfig:   bundle.RenderedConfig,
		SupportFilesJSON: string(supportFilesJSON),
		Checksum:         bundle.Checksum,
		IsActive:         true,
		CreatedBy:        createdBy,
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ConfigVersion{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
			return err
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("版本号生成冲突，请重试")
		}
		return nil, err
	}
	return &ReleaseResult{
		Version: record,
		Routes:  bundle.Routes,
	}, nil
}

func ActivateConfigVersion(id uint) (*model.ConfigVersion, error) {
	version, err := model.GetConfigVersionByID(id)
	if err != nil {
		return nil, err
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ConfigVersion{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
			return err
		}
		if err := tx.Model(version).Update("is_active", true).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	version.IsActive = true
	return version, nil
}

func renderSnapshot(routes []*model.ProxyRoute) (string, error) {
	items, err := buildSnapshotRoutes(routes)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func renderNginxConfig(routes []*model.ProxyRoute) (string, []SupportFile, error) {
	var builder strings.Builder
	builder.WriteString("# This file is generated by ATSFlare. Do not edit manually.\n")
	supportFiles := make([]SupportFile, 0)
	for _, route := range routes {
		customHeaders, err := decodeStoredCustomHeaders(route.CustomHeaders)
		if err != nil {
			return "", nil, fmt.Errorf("路由 %s 自定义请求头无效", route.Domain)
		}
		if !route.EnableHTTPS {
			builder.WriteString(renderHTTPProxyServer(route.Domain, route.OriginURL, customHeaders))
			continue
		}
		if route.CertID == nil || *route.CertID == 0 {
			return "", nil, fmt.Errorf("路由 %s 未配置证书", route.Domain)
		}
		certificate, err := model.GetTLSCertificateByID(*route.CertID)
		if err != nil {
			return "", nil, fmt.Errorf("路由 %s 关联证书不存在", route.Domain)
		}
		supportFiles = append(supportFiles,
			SupportFile{Path: certificateCertFileName(certificate.ID), Content: normalizePEM(certificate.CertPEM)},
			SupportFile{Path: certificateKeyFileName(certificate.ID), Content: normalizePEM(certificate.KeyPEM)},
		)
		if route.RedirectHTTP {
			builder.WriteString(renderHTTPRedirectServer(route.Domain))
		} else {
			builder.WriteString(renderHTTPProxyServer(route.Domain, route.OriginURL, customHeaders))
		}
		builder.WriteString(renderHTTPSServer(route.Domain, route.OriginURL, certificate.ID, customHeaders))
	}
	return builder.String(), dedupeSupportFiles(supportFiles), nil
}

func buildCurrentConfigBundle(requireRoutes bool) (*configBundle, error) {
	routes, err := model.GetEnabledProxyRoutes()
	if err != nil {
		return nil, err
	}
	if requireRoutes && len(routes) == 0 {
		return nil, errors.New("没有可发布的启用规则")
	}
	snapshotRoutes, err := buildSnapshotRoutes(routes)
	if err != nil {
		return nil, err
	}
	snapshotJSON, err := json.Marshal(snapshotRoutes)
	if err != nil {
		return nil, err
	}
	renderedConfig, supportFiles, err := renderNginxConfig(routes)
	if err != nil {
		return nil, err
	}
	return &configBundle{
		Routes:         routes,
		SnapshotRoutes: snapshotRoutes,
		SnapshotJSON:   string(snapshotJSON),
		RenderedConfig: renderedConfig,
		SupportFiles:   supportFiles,
		Checksum:       checksumBundle(renderedConfig, supportFiles),
	}, nil
}

func buildSnapshotRoutes(routes []*model.ProxyRoute) ([]snapshotRoute, error) {
	items := make([]snapshotRoute, 0, len(routes))
	for _, route := range routes {
		customHeaders, err := decodeStoredCustomHeaders(route.CustomHeaders)
		if err != nil {
			return nil, fmt.Errorf("路由 %s 自定义请求头无效", route.Domain)
		}
		items = append(items, snapshotRoute{
			Domain:        route.Domain,
			OriginURL:     route.OriginURL,
			Enabled:       route.Enabled,
			EnableHTTPS:   route.EnableHTTPS,
			CertID:        route.CertID,
			RedirectHTTP:  route.RedirectHTTP,
			CustomHeaders: customHeaders,
			Remark:        route.Remark,
		})
	}
	return items, nil
}

func parseSnapshotRoutes(snapshotJSON string) ([]snapshotRoute, error) {
	text := strings.TrimSpace(snapshotJSON)
	if text == "" {
		return []snapshotRoute{}, nil
	}
	var routes []snapshotRoute
	if err := json.Unmarshal([]byte(text), &routes); err != nil {
		return nil, errors.New("历史版本快照格式不合法")
	}
	for index := range routes {
		normalizedHeaders, err := normalizeCustomHeaders(routes[index].CustomHeaders)
		if err != nil {
			return nil, err
		}
		routes[index].CustomHeaders = normalizedHeaders
	}
	return routes, nil
}

func snapshotRouteConfigEqual(left snapshotRoute, right snapshotRoute) bool {
	if left.Domain != right.Domain || left.OriginURL != right.OriginURL || left.EnableHTTPS != right.EnableHTTPS || left.RedirectHTTP != right.RedirectHTTP || !uintPointerEqual(left.CertID, right.CertID) {
		return false
	}
	if len(left.CustomHeaders) != len(right.CustomHeaders) {
		return false
	}
	for index := range left.CustomHeaders {
		if left.CustomHeaders[index] != right.CustomHeaders[index] {
			return false
		}
	}
	return true
}

func uintPointerEqual(left *uint, right *uint) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func checksum(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func checksumBundle(renderedConfig string, supportFiles []SupportFile) string {
	var builder strings.Builder
	builder.WriteString(renderedConfig)
	builder.WriteString("\n--support-files--\n")
	files := dedupeSupportFiles(supportFiles)
	sort.Slice(files, func(i int, j int) bool {
		return files[i].Path < files[j].Path
	})
	for _, file := range files {
		builder.WriteString(file.Path)
		builder.WriteString("\n")
		builder.WriteString(file.Content)
		builder.WriteString("\n")
	}
	return checksum(builder.String())
}

func nextVersionNumber(now time.Time) (string, error) {
	prefix := now.Format("20060102")
	var count int64
	if err := model.DB.Model(&model.ConfigVersion{}).Where("version LIKE ?", prefix+"-%").Count(&count).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%03d", prefix, count+1), nil
}

func renderHTTPProxyServer(domain string, originURL string, customHeaders []ProxyRouteCustomHeaderInput) string {
	return fmt.Sprintf("server {\n    listen 80;\n    server_name %s;\n\n    location / {\n%s        proxy_pass %s;\n    }\n}\n\n", domain, renderProxyHeaderBlock(customHeaders), originURL)
}

func renderHTTPRedirectServer(domain string) string {
	return fmt.Sprintf("server {\n    listen 80;\n    server_name %s;\n\n    return 301 https://$host$request_uri;\n}\n\n", domain)
}

func renderHTTPSServer(domain string, originURL string, certificateID uint, customHeaders []ProxyRouteCustomHeaderInput) string {
	certPath := fmt.Sprintf("%s/%s", nginxCertDirPlaceholder, certificateCertFileName(certificateID))
	keyPath := fmt.Sprintf("%s/%s", nginxCertDirPlaceholder, certificateKeyFileName(certificateID))
	return fmt.Sprintf("server {\n    listen 443 ssl;\n    server_name %s;\n    ssl_certificate %s;\n    ssl_certificate_key %s;\n\n    location / {\n%s        proxy_pass %s;\n    }\n}\n\n", domain, certPath, keyPath, renderProxyHeaderBlock(customHeaders), originURL)
}

func renderProxyHeaderBlock(customHeaders []ProxyRouteCustomHeaderInput) string {
	var builder strings.Builder
	builder.WriteString("        proxy_set_header Host $host;\n")
	builder.WriteString("        proxy_set_header X-Real-IP $remote_addr;\n")
	builder.WriteString("        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
	builder.WriteString("        proxy_set_header X-Forwarded-Proto $scheme;\n")
	for _, header := range customHeaders {
		builder.WriteString(fmt.Sprintf("        proxy_set_header %s %s;\n", header.Key, quoteNginxHeaderValue(header.Value)))
	}
	return builder.String()
}

func quoteNginxHeaderValue(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return fmt.Sprintf(`"%s"`, escaped)
}

func certificateCertFileName(id uint) string {
	return fmt.Sprintf("%d.crt", id)
}

func certificateKeyFileName(id uint) string {
	return fmt.Sprintf("%d.key", id)
}

func normalizePEM(content string) string {
	return strings.TrimSpace(content) + "\n"
}

func dedupeSupportFiles(files []SupportFile) []SupportFile {
	if len(files) == 0 {
		return nil
	}
	unique := make(map[string]SupportFile, len(files))
	for _, file := range files {
		unique[file.Path] = file
	}
	result := make([]SupportFile, 0, len(unique))
	for _, file := range unique {
		result = append(result, file)
	}
	return result
}
