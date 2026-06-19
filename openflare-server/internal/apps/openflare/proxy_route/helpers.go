// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package proxy_route

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

var proxyHeaderKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var proxyRouteLimitRatePattern = regexp.MustCompile(`^\d+[kKmM]?$`)

const (
	proxyRouteCachePolicyURL        = "url"
	proxyRouteCachePolicySuffix     = "suffix"
	proxyRouteCachePolicyPathPrefix = "path_prefix"
	proxyRouteCachePolicyPathExact  = "path_exact"
)

type tlsCertificateRow struct {
	ID      uint   `gorm:"column:id;primaryKey"`
	CertPEM string `gorm:"column:cert_pem"`
}

func (tlsCertificateRow) TableName() string {
	return "of_tls_certificates"
}

type tunnelNodeRow struct {
	ID       uint   `gorm:"column:id;primaryKey"`
	NodeType string `gorm:"column:node_type"`
}

func (tunnelNodeRow) TableName() string {
	return "of_nodes"
}

type pagesProjectRow struct {
	ID                 uint  `gorm:"column:id;primaryKey"`
	Enabled            bool  `gorm:"column:enabled"`
	ActiveDeploymentID *uint `gorm:"column:active_deployment_id"`
}

func (pagesProjectRow) TableName() string {
	return "of_pages_projects"
}

func uniqueStrings(items []string) []string {
	if len(items) == 0 {
		return items
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}

func normalizeOriginAddress(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func validateOriginAddress(address string) error {
	if address == "" {
		return errors.New(errProxyRouteOriginEmpty)
	}
	if strings.Contains(address, "://") || strings.ContainsAny(address, "/?#") {
		return errors.New(errProxyRouteOriginInvalid)
	}
	if strings.HasPrefix(address, "[") || strings.HasSuffix(address, "]") {
		return errors.New(errProxyRouteOriginInvalid)
	}
	if ip := net.ParseIP(address); ip != nil {
		return nil
	}
	if len(address) > 253 {
		return errors.New(errProxyRouteOriginInvalid)
	}
	labels := strings.Split(address, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return errors.New(errProxyRouteOriginInvalid)
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return errors.New(errProxyRouteOriginInvalid)
		}
		for _, r := range label {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
				continue
			}
			return errors.New(errProxyRouteOriginInvalid)
		}
	}
	return nil
}

func normalizeOriginPort(raw string) (string, error) {
	port := strings.TrimSpace(raw)
	if port == "" {
		return "", errors.New(errProxyRouteOriginPortEmpty)
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return "", errors.New(errProxyRouteOriginPort)
	}
	return strconv.Itoa(value), nil
}

func normalizeOriginScheme(raw string) (string, error) {
	scheme := strings.ToLower(strings.TrimSpace(raw))
	switch scheme {
	case "http", "https":
		return scheme, nil
	default:
		return "", errors.New(errProxyRouteOriginSchemeOnly)
	}
}

func normalizeOriginURI(raw string) (string, error) {
	uri := strings.TrimSpace(raw)
	if uri == "" {
		return "", nil
	}
	if strings.Contains(uri, "://") {
		return "", errors.New(errProxyRouteOriginURIProto)
	}
	if !strings.HasPrefix(uri, "/") && !strings.HasPrefix(uri, "?") {
		return "", errors.New(errProxyRouteOriginURI)
	}
	return uri, nil
}

func formatOriginHost(address string, port string) string {
	return net.JoinHostPort(address, port)
}

func buildOriginURLFromParts(scheme, address, port, uri string) (string, error) {
	normalizedScheme, err := normalizeOriginScheme(scheme)
	if err != nil {
		return "", err
	}
	normalizedAddress := normalizeOriginAddress(address)
	if err := validateOriginAddress(normalizedAddress); err != nil {
		return "", err
	}
	normalizedPort, err := normalizeOriginPort(port)
	if err != nil {
		return "", err
	}
	normalizedURI, err := normalizeOriginURI(uri)
	if err != nil {
		return "", err
	}

	parsed := &url.URL{
		Scheme: normalizedScheme,
		Host:   formatOriginHost(normalizedAddress, normalizedPort),
	}
	if normalizedURI != "" {
		if strings.HasPrefix(normalizedURI, "?") {
			parsed.RawQuery = strings.TrimPrefix(normalizedURI, "?")
		} else {
			pathQuery := strings.SplitN(normalizedURI, "?", 2)
			parsed.Path = pathQuery[0]
			if len(pathQuery) > 1 {
				parsed.RawQuery = pathQuery[1]
			}
		}
	}
	return parsed.String(), nil
}

func extractOriginAddress(rawURL string) (string, error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("%s: %w", errProxyRouteOriginInvalid, err)
	}
	address := normalizeOriginAddress(parsed.Hostname())
	if err := validateOriginAddress(address); err != nil {
		return "", err
	}
	return address, nil
}

func getOrCreateOriginByAddress(ctx context.Context, address string) (*model.Origin, error) {
	normalizedAddress := normalizeOriginAddress(address)
	if err := validateOriginAddress(normalizedAddress); err != nil {
		return nil, err
	}
	existing, err := model.GetOriginByAddress(ctx, normalizedAddress)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	origin := &model.Origin{
		Name:    normalizedAddress,
		Address: normalizedAddress,
		Remark:  "",
	}
	if err := model.CreateOriginRecord(ctx, origin); err != nil {
		if isUniqueConstraintError(err) {
			return model.GetOriginByAddress(ctx, normalizedAddress)
		}
		return nil, err
	}
	return origin, nil
}

func lookupTLSCertificateByID(ctx context.Context, id uint) (*tlsCertificateRow, error) {
	if !db.DB(ctx).Migrator().HasTable(&tlsCertificateRow{}) {
		return nil, gorm.ErrRecordNotFound
	}
	var certificate tlsCertificateRow
	if err := db.DB(ctx).First(&certificate, id).Error; err != nil {
		return nil, err
	}
	return &certificate, nil
}

func lookupTunnelNodeByID(ctx context.Context, id uint) (*tunnelNodeRow, error) {
	if !db.DB(ctx).Migrator().HasTable(&tunnelNodeRow{}) {
		return nil, gorm.ErrRecordNotFound
	}
	var node tunnelNodeRow
	if err := db.DB(ctx).First(&node, id).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

func lookupPagesProjectByID(ctx context.Context, id uint) (*pagesProjectRow, error) {
	if !db.DB(ctx).Migrator().HasTable(&pagesProjectRow{}) {
		return nil, gorm.ErrRecordNotFound
	}
	var project pagesProjectRow
	if err := db.DB(ctx).First(&project, id).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

func parseLeafCertificate(certPEM string) (*x509.Certificate, error) {
	certPEMBlock, _ := pem.Decode([]byte(certPEM))
	if certPEMBlock == nil {
		return nil, errors.New(errProxyRouteCertNotFound)
	}
	leaf, err := x509.ParseCertificate(certPEMBlock.Bytes)
	if err != nil {
		return nil, err
	}
	return leaf, nil
}

func validateCertificateCoverage(certificate *tlsCertificateRow, domains []string) error {
	if certificate == nil {
		return errors.New(errProxyRouteCertNotFound)
	}
	leaf, err := parseLeafCertificate(certificate.CertPEM)
	if err != nil {
		return err
	}
	for _, domain := range domains {
		if err := leaf.VerifyHostname(domain); err != nil {
			return fmt.Errorf("certificate does not cover domain %s", domain)
		}
	}
	return nil
}

func loadTLSCertificates(ctx context.Context, certIDs []uint) ([]*tlsCertificateRow, error) {
	certificates := make([]*tlsCertificateRow, 0, len(certIDs))
	for _, certID := range certIDs {
		certificate, err := lookupTLSCertificateByID(ctx, certID)
		if err != nil {
			return nil, err
		}
		certificates = append(certificates, certificate)
	}
	return certificates, nil
}

func normalizeProxyRouteSiteNameInput(route *model.ProxyRoute, raw, primaryDomain string) string {
	siteName := strings.TrimSpace(raw)
	if siteName != "" {
		return siteName
	}
	if route != nil && strings.TrimSpace(route.SiteName) != "" {
		return strings.TrimSpace(route.SiteName)
	}
	return primaryDomain
}

func normalizeProxyRouteDomainValue(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeProxyRouteDomains(rawDomains []string) ([]string, error) {
	normalized := make([]string, 0, len(rawDomains))
	for _, rawDomain := range rawDomains {
		domain := normalizeProxyRouteDomainValue(rawDomain)
		if domain == "" {
			continue
		}
		if strings.Contains(domain, "://") || strings.Contains(domain, "/") {
			return nil, errors.New(errProxyRouteDomainInvalid)
		}
		normalized = append(normalized, domain)
	}
	normalized = uniqueStrings(normalized)
	if len(normalized) == 0 {
		return nil, errors.New(errProxyRouteDomainRequired)
	}
	return normalized, nil
}

func normalizeProxyRouteDomainsInput(route *model.ProxyRoute, rawDomain string, rawDomains []string) ([]string, error) {
	if len(rawDomains) > 0 {
		domains, err := normalizeProxyRouteDomains(rawDomains)
		if err != nil {
			return nil, err
		}
		domain := normalizeProxyRouteDomainValue(rawDomain)
		if domain != "" && domain != domains[0] {
			return nil, errors.New(errProxyRouteDomainMismatch)
		}
		return domains, nil
	}

	if route != nil {
		existingDomains, err := decodeStoredDomains(route.Domains, route.Domain)
		if err == nil && len(existingDomains) > 0 {
			domain := normalizeProxyRouteDomainValue(rawDomain)
			if domain == "" || domain == existingDomains[0] {
				return existingDomains, nil
			}
		}
	}

	return normalizeProxyRouteDomains([]string{rawDomain})
}

func validateProxyRouteSiteName(siteName string) error {
	if strings.TrimSpace(siteName) == "" {
		return errors.New(errProxyRouteSiteNameEmpty)
	}
	return nil
}

func validateProxyRouteIdentityUniqueness(ctx context.Context, route *model.ProxyRoute, siteName string, domains []string) error {
	routes, err := model.ListProxyRoutes(ctx)
	if err != nil {
		return err
	}

	currentID := uint(0)
	if route != nil {
		currentID = route.ID
	}

	for _, item := range routes {
		if item == nil || item.ID == currentID {
			continue
		}
		existingSiteName := normalizeProxyRouteSiteNameInput(item, item.SiteName, item.Domain)
		if existingSiteName == siteName {
			return errors.New(errProxyRouteSiteNameExists)
		}

		existingDomains, err := decodeStoredDomains(item.Domains, item.Domain)
		if err != nil {
			return fmt.Errorf("existing route %d domains are invalid: %w", item.ID, err)
		}
		existingSet := make(map[string]struct{}, len(existingDomains))
		for _, existingDomain := range existingDomains {
			existingSet[existingDomain] = struct{}{}
		}
		for _, domain := range domains {
			if _, ok := existingSet[domain]; ok {
				return fmt.Errorf(errProxyRouteDomainExists, domain)
			}
		}
	}

	return nil
}

func normalizeProxyRouteLimitConnValue(value int, field string) (int, error) {
	if value < 0 {
		return 0, fmt.Errorf("%s must be greater than or equal to 0", field)
	}
	return value, nil
}

func normalizeProxyRouteCertificateIDs(ctx context.Context, enableHTTPS bool, certID *uint, certIDs []uint) ([]uint, error) {
	if !enableHTTPS {
		return []uint{}, nil
	}

	candidates := make([]uint, 0, len(certIDs)+1)
	if certID != nil && *certID != 0 {
		candidates = append(candidates, *certID)
	}
	candidates = append(candidates, certIDs...)

	normalized := make([]uint, 0, len(candidates))
	seen := make(map[uint]struct{}, len(candidates))
	for _, item := range candidates {
		if item == 0 {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		if _, err := lookupTLSCertificateByID(ctx, item); err != nil {
			return nil, errors.New(errProxyRouteCertNotFound)
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	if len(normalized) == 0 {
		return nil, errors.New(errProxyRouteCertRequired)
	}
	return normalized, nil
}

func normalizeProxyRouteDomainCertificateIDs(
	ctx context.Context,
	domains []string,
	enableHTTPS bool,
	rawDomainCertIDs []uint,
	certID *uint,
	certIDs []uint,
) ([]uint, []uint, *uint, error) {
	if !enableHTTPS {
		return []uint{}, []uint{}, nil, nil
	}

	if len(rawDomainCertIDs) > 0 {
		if len(rawDomainCertIDs) != len(domains) {
			return nil, nil, nil, errors.New(errProxyRouteCertDomainLength)
		}

		normalizedDomainCertIDs := make([]uint, len(rawDomainCertIDs))
		uniqueCertIDs := make([]uint, 0, len(rawDomainCertIDs))
		seen := make(map[uint]struct{}, len(rawDomainCertIDs))
		hasAssignedCertificate := false
		for index, item := range rawDomainCertIDs {
			if item == 0 {
				continue
			}
			if _, err := lookupTLSCertificateByID(ctx, item); err != nil {
				return nil, nil, nil, errors.New(errProxyRouteCertNotFound)
			}
			normalizedDomainCertIDs[index] = item
			hasAssignedCertificate = true
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			uniqueCertIDs = append(uniqueCertIDs, item)
		}
		if !hasAssignedCertificate {
			return nil, nil, nil, errors.New(errProxyRouteCertRequired)
		}

		primaryCertID := &uniqueCertIDs[0]
		return normalizedDomainCertIDs, uniqueCertIDs, primaryCertID, nil
	}

	normalizedCertIDs, err := normalizeProxyRouteCertificateIDs(ctx, enableHTTPS, certID, certIDs)
	if err != nil {
		return nil, nil, nil, err
	}

	switch {
	case len(normalizedCertIDs) == 0:
		return nil, nil, nil, errors.New(errProxyRouteCertRequired)
	case len(normalizedCertIDs) == 1:
		domainCertIDs := make([]uint, len(domains))
		for index := range domainCertIDs {
			domainCertIDs[index] = normalizedCertIDs[0]
		}
		primaryCertID := &normalizedCertIDs[0]
		return domainCertIDs, normalizedCertIDs, primaryCertID, nil
	case len(normalizedCertIDs) == len(domains):
		domainCertIDs := make([]uint, len(normalizedCertIDs))
		copy(domainCertIDs, normalizedCertIDs)
		primaryCertID := &normalizedCertIDs[0]
		return domainCertIDs, normalizedCertIDs, primaryCertID, nil
	default:
		domainCertIDs, err := deriveDomainCertIDsFromCertificateSet(ctx, domains, normalizedCertIDs)
		if err != nil {
			return nil, nil, nil, err
		}
		primaryCertID := &normalizedCertIDs[0]
		return domainCertIDs, normalizedCertIDs, primaryCertID, nil
	}
}

func validateProxyRouteDomainCertificateCoverage(ctx context.Context, domains []string, domainCertIDs []uint) error {
	if len(domainCertIDs) == 0 {
		return nil
	}

	domainsByCertID := make(map[uint][]string)
	for index, certID := range domainCertIDs {
		if certID == 0 {
			continue
		}
		domainsByCertID[certID] = append(domainsByCertID[certID], domains[index])
	}

	for certID, assignedDomains := range domainsByCertID {
		certificate, err := lookupTLSCertificateByID(ctx, certID)
		if err != nil {
			return errors.New(errProxyRouteCertNotFound)
		}
		if err := validateCertificateCoverage(certificate, assignedDomains); err != nil {
			return err
		}
	}
	return nil
}

func deriveDomainCertIDsFromCertificateSet(ctx context.Context, domains []string, certIDs []uint) ([]uint, error) {
	certificates, err := loadTLSCertificates(ctx, certIDs)
	if err != nil {
		return nil, err
	}

	result := make([]uint, len(domains))
	for domainIndex, domain := range domains {
		if domainIndex < len(certificates) &&
			certificates[domainIndex] != nil &&
			validateCertificateCoverage(certificates[domainIndex], []string{domain}) == nil {
			result[domainIndex] = certificates[domainIndex].ID
			continue
		}

		assigned := uint(0)
		for _, certificate := range certificates {
			if certificate != nil &&
				validateCertificateCoverage(certificate, []string{domain}) == nil {
				assigned = certificate.ID
				break
			}
		}
		if assigned == 0 {
			return nil, fmt.Errorf("certificate does not cover domain %s", domain)
		}
		result[domainIndex] = assigned
	}
	return result, nil
}

func decodeStoredDomainCertIDs(raw string, domainCount int) ([]uint, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return []uint{}, nil
	}

	var domainCertIDs []uint
	if err := json.Unmarshal([]byte(text), &domainCertIDs); err != nil {
		return nil, errors.New("domain_cert_ids payload is invalid")
	}
	if len(domainCertIDs) == 0 {
		return []uint{}, nil
	}
	if domainCount > 0 && len(domainCertIDs) != domainCount {
		return nil, errors.New("domain_cert_ids length does not match domains")
	}

	normalized := make([]uint, len(domainCertIDs))
	copy(normalized, domainCertIDs)
	return normalized, nil
}

func resolveProxyRouteDomainCertIDs(ctx context.Context, route *model.ProxyRoute, domains []string, certIDs []uint) ([]uint, error) {
	domainCertIDs, err := decodeStoredDomainCertIDs(route.DomainCertIDs, len(domains))
	if err != nil {
		return nil, err
	}
	if len(domainCertIDs) > 0 || len(certIDs) == 0 {
		return domainCertIDs, nil
	}
	return deriveDomainCertIDsFromCertificateSet(ctx, domains, certIDs)
}

func normalizeProxyRouteLimitRate(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" || normalized == "0" {
		return "", nil
	}
	if !proxyRouteLimitRatePattern.MatchString(normalized) {
		return "", errors.New(errProxyRouteLimitRate)
	}
	if strings.TrimRight(normalized, "km") == "" {
		return "", nil
	}
	return normalized, nil
}

func hasStructuredOriginInput(input Input) bool {
	return (input.OriginID != nil && *input.OriginID != 0) ||
		strings.TrimSpace(input.OriginScheme) != "" ||
		strings.TrimSpace(input.OriginAddress) != "" ||
		strings.TrimSpace(input.OriginPort) != "" ||
		strings.TrimSpace(input.OriginURI) != ""
}

func resolveProxyRoutePrimaryOrigin(ctx context.Context, input Input) (string, *uint, error) {
	if hasStructuredOriginInput(input) {
		scheme, err := normalizeOriginScheme(input.OriginScheme)
		if err != nil {
			return "", nil, err
		}
		port, err := normalizeOriginPort(input.OriginPort)
		if err != nil {
			return "", nil, err
		}
		uri, err := normalizeOriginURI(input.OriginURI)
		if err != nil {
			return "", nil, err
		}
		if input.OriginID != nil && *input.OriginID != 0 {
			origin, err := model.GetOriginByID(ctx, *input.OriginID)
			if err != nil {
				return "", nil, errors.New(errProxyRouteOriginNotFound)
			}
			originURL, err := buildOriginURLFromParts(scheme, origin.Address, port, uri)
			if err != nil {
				return "", nil, err
			}
			return originURL, &origin.ID, nil
		}

		address := normalizeOriginAddress(input.OriginAddress)
		if err := validateOriginAddress(address); err != nil {
			return "", nil, err
		}
		originURL, err := buildOriginURLFromParts(scheme, address, port, uri)
		if err != nil {
			return "", nil, err
		}
		origin, err := getOrCreateOriginByAddress(ctx, address)
		if err != nil {
			return "", nil, err
		}
		return originURL, &origin.ID, nil
	}

	originURL := strings.TrimSpace(input.OriginURL)
	if originURL == "" {
		return "", nil, errors.New(errProxyRouteOriginEmpty)
	}
	address, err := extractOriginAddress(originURL)
	if err != nil {
		return "", nil, err
	}
	origin, findErr := model.GetOriginByAddress(ctx, address)
	if findErr == nil {
		return originURL, &origin.ID, nil
	}
	if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return "", nil, findErr
	}
	return originURL, nil, nil
}

func normalizeCustomHeaders(headers []CustomHeaderInput) ([]CustomHeaderInput, error) {
	if len(headers) == 0 {
		return []CustomHeaderInput{}, nil
	}
	normalized := make([]CustomHeaderInput, 0, len(headers))
	for _, header := range headers {
		key := strings.TrimSpace(header.Key)
		value := strings.TrimSpace(header.Value)
		if key == "" && value == "" {
			continue
		}
		if key == "" {
			return nil, errors.New(errProxyRouteHeaderKeyEmpty)
		}
		if !proxyHeaderKeyPattern.MatchString(key) {
			return nil, errors.New(errProxyRouteHeaderKeyInvalid)
		}
		if strings.ContainsAny(key, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return nil, errors.New(errProxyRouteHeaderNewline)
		}
		normalized = append(normalized, CustomHeaderInput{Key: key, Value: value})
	}
	return normalized, nil
}

func normalizeUpstreams(originURL string, upstreams []string) ([]string, error) {
	candidates := make([]string, 0, len(upstreams)+1)
	if strings.TrimSpace(originURL) != "" {
		candidates = append(candidates, originURL)
	}
	candidates = append(candidates, upstreams...)
	trimmed := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		item := strings.TrimSpace(candidate)
		if item == "" {
			continue
		}
		trimmed = append(trimmed, item)
	}
	unique := uniqueStrings(trimmed)
	normalized := make([]string, 0, len(unique))
	var scheme string
	multiUpstream := len(unique) > 1
	for _, item := range unique {
		if err := validateOriginURL(item); err != nil {
			return nil, err
		}
		parsed, err := url.ParseRequestURI(item)
		if err != nil {
			return nil, errors.New(errProxyRouteOriginInvalid)
		}
		if multiUpstream && parsed.Path != "" && parsed.Path != "/" {
			return nil, errors.New(errProxyRouteUpstreamPath)
		}
		if multiUpstream && parsed.RawQuery != "" {
			return nil, errors.New(errProxyRouteUpstreamQuery)
		}
		if scheme == "" {
			scheme = parsed.Scheme
		} else if scheme != parsed.Scheme {
			return nil, errors.New(errProxyRouteUpstreamScheme)
		}
		normalized = append(normalized, item)
	}
	if len(normalized) == 0 {
		return nil, errors.New(errProxyRouteUpstreamRequired)
	}
	return normalized, nil
}

func decodeStoredCustomHeaders(raw string) ([]CustomHeaderInput, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return []CustomHeaderInput{}, nil
	}
	var headers []CustomHeaderInput
	if err := json.Unmarshal([]byte(text), &headers); err != nil {
		return nil, errors.New("custom_headers payload is invalid")
	}
	return normalizeCustomHeaders(headers)
}

func normalizeCachePolicy(enabled bool, raw string) string {
	if !enabled {
		return ""
	}
	policy := strings.TrimSpace(raw)
	if policy == "" {
		return proxyRouteCachePolicyURL
	}
	return policy
}

func normalizeCacheRules(enabled bool, rawPolicy string, rules []string) ([]string, error) {
	if !enabled {
		return []string{}, nil
	}
	policy := normalizeCachePolicy(enabled, rawPolicy)
	switch policy {
	case proxyRouteCachePolicyURL:
		return []string{}, nil
	case proxyRouteCachePolicySuffix:
		return normalizeCacheSuffixRules(rules)
	case proxyRouteCachePolicyPathPrefix:
		return normalizeCachePathRules(rules, true)
	case proxyRouteCachePolicyPathExact:
		return normalizeCachePathRules(rules, false)
	default:
		return nil, errors.New(errProxyRouteCachePolicy)
	}
}

func normalizeCacheSuffixRules(rules []string) ([]string, error) {
	normalized := make([]string, 0, len(rules))
	seen := make(map[string]struct{}, len(rules))
	for _, rule := range rules {
		item := strings.TrimSpace(strings.TrimPrefix(rule, "."))
		if item == "" {
			continue
		}
		if strings.ContainsAny(item, "/\\ \t\r\n") {
			return nil, errors.New(errProxyRouteCacheSuffix)
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	if len(normalized) == 0 {
		return nil, errors.New(errProxyRouteCacheSuffixReq)
	}
	return normalized, nil
}

func normalizeCachePathRules(rules []string, allowPrefix bool) ([]string, error) {
	normalized := make([]string, 0, len(rules))
	seen := make(map[string]struct{}, len(rules))
	for _, rule := range rules {
		item := strings.TrimSpace(rule)
		if item == "" {
			continue
		}
		if !strings.HasPrefix(item, "/") || strings.Contains(item, "://") || strings.ContainsAny(item, " \t\r\n") {
			return nil, errors.New(errProxyRouteCachePath)
		}
		if !allowPrefix && strings.HasSuffix(item, "/") && len(item) > 1 {
			item = strings.TrimRight(item, "/")
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	if len(normalized) == 0 {
		if allowPrefix {
			return nil, errors.New(errProxyRouteCachePrefixReq)
		}
		return nil, errors.New(errProxyRouteCacheExactReq)
	}
	return normalized, nil
}

func decodeStoredCacheRules(raw string) ([]string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return []string{}, nil
	}
	var rules []string
	if err := json.Unmarshal([]byte(text), &rules); err != nil {
		return nil, errors.New("cache_rules payload is invalid")
	}
	normalized := make([]string, 0, len(rules))
	for _, rule := range rules {
		item := strings.TrimSpace(rule)
		if item == "" {
			continue
		}
		normalized = append(normalized, item)
	}
	return normalized, nil
}

func decodeStoredUpstreams(raw string, fallbackOriginURL string) ([]string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return normalizeUpstreams(fallbackOriginURL, nil)
	}
	var upstreams []string
	if err := json.Unmarshal([]byte(text), &upstreams); err != nil {
		return nil, errors.New("upstreams payload is invalid")
	}
	return normalizeUpstreams(fallbackOriginURL, upstreams)
}

func decodeStoredDomains(raw string, fallbackDomain string) ([]string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return normalizeProxyRouteDomains([]string{fallbackDomain})
	}
	var domains []string
	if err := json.Unmarshal([]byte(text), &domains); err != nil {
		return nil, errors.New("domains payload is invalid")
	}
	return normalizeProxyRouteDomains(domains)
}

func decodeStoredCertIDs(raw string, fallbackCertID *uint) ([]uint, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		if fallbackCertID == nil || *fallbackCertID == 0 {
			return []uint{}, nil
		}
		return []uint{*fallbackCertID}, nil
	}
	var certIDs []uint
	if err := json.Unmarshal([]byte(text), &certIDs); err != nil {
		return nil, errors.New("cert_ids payload is invalid")
	}
	normalized := make([]uint, 0, len(certIDs))
	seen := make(map[uint]struct{}, len(certIDs))
	for _, certID := range certIDs {
		if certID == 0 {
			continue
		}
		if _, ok := seen[certID]; ok {
			continue
		}
		seen[certID] = struct{}{}
		normalized = append(normalized, certID)
	}
	if len(normalized) == 0 && fallbackCertID != nil && *fallbackCertID != 0 {
		return []uint{*fallbackCertID}, nil
	}
	return normalized, nil
}

func validateOriginURL(raw string) error {
	if raw == "" {
		return errors.New(errProxyRouteOriginEmpty)
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return errors.New(errProxyRouteOriginInvalid)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New(errProxyRouteOriginScheme)
	}
	if parsed.Host == "" {
		return errors.New(errProxyRouteOriginInvalid)
	}
	return nil
}

func validateOriginHost(raw string) error {
	if raw == "" {
		return nil
	}
	if strings.ContainsAny(raw, "/\\ \t\r\n") || strings.Contains(raw, "://") {
		return errors.New(errProxyRouteOriginHostInvalid)
	}
	parsed, err := url.Parse("//" + raw)
	if err != nil || parsed.Host == "" || parsed.Host != raw {
		return errors.New(errProxyRouteOriginHostInvalid)
	}
	if parsed.Hostname() == "" {
		return errors.New(errProxyRouteOriginHostInvalid)
	}
	return nil
}

func normalizeTunnelNodeID(tunnelNodeID, legacyTunnelID *uint) (*uint, error) {
	if tunnelNodeID != nil && *tunnelNodeID != 0 {
		return tunnelNodeID, nil
	}
	if legacyTunnelID != nil && *legacyTunnelID != 0 {
		return legacyTunnelID, nil
	}
	return nil, errors.New(errProxyRouteTunnelNodeReq)
}

func validateTunnelRouteInput(ctx context.Context, tunnelNodeID *uint, targetAddr, targetProtocol string) error {
	if tunnelNodeID == nil || *tunnelNodeID == 0 {
		return errors.New(errProxyRouteTunnelNodeReq)
	}
	tunnelNode, err := lookupTunnelNodeByID(ctx, *tunnelNodeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(errProxyRouteTunnelNodeMissing)
		}
		return err
	}
	if tunnelNode.NodeType != "tunnel_client" {
		return errors.New(errProxyRouteTunnelNodeType)
	}
	if strings.TrimSpace(targetAddr) == "" {
		return errors.New(errProxyRouteTunnelAddrReq)
	}
	switch strings.ToLower(strings.TrimSpace(targetProtocol)) {
	case "", "http", "https":
		return nil
	default:
		return errors.New(errProxyRouteTunnelProtocol)
	}
}

func validatePagesRouteInput(ctx context.Context, projectID *uint) error {
	if projectID == nil || *projectID == 0 {
		return errors.New(errProxyRoutePagesProjectReq)
	}
	project, err := lookupPagesProjectByID(ctx, *projectID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(errProxyRoutePagesNotFound)
		}
		return err
	}
	if !project.Enabled {
		return errors.New(errProxyRoutePagesDisabled)
	}
	if project.ActiveDeploymentID == nil || *project.ActiveDeploymentID == 0 {
		return errors.New(errProxyRoutePagesNoDeploy)
	}
	return nil
}

func normalizeUpstreamType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "tunnel":
		return "tunnel"
	case "pages":
		return "pages"
	default:
		return "direct"
	}
}

func normalizeTunnelTargetProtocol(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "https":
		return "https"
	default:
		return "http"
	}
}
