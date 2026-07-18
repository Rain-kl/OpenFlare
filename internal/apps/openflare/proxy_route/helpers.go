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

	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

var proxyHeaderKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var proxyRouteLimitRatePattern = regexp.MustCompile(`^\d+[kKmM]?$`)

const (
	proxyRouteCachePolicyStatic     = "static"
	proxyRouteCachePolicyAll        = "all"
	proxyRouteCachePolicyURL        = "url" // legacy alias of all
	proxyRouteCachePolicySuffix     = "suffix"
	proxyRouteCachePolicyPathPrefix = "path_prefix"
	proxyRouteCachePolicyPathExact  = "path_exact"
	proxyRouteSchemeHTTP            = "http"
	proxyRouteSchemeHTTPS           = "https"
	proxyRouteUpstreamTypeTunnel    = "tunnel"
	proxyRouteUpstreamTypePages     = "pages"

	maxOriginHostnameLength = 253
	originURIPathQueryParts = 2
)

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
	if len(address) > maxOriginHostnameLength {
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
	case proxyRouteSchemeHTTP, proxyRouteSchemeHTTPS:
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
			pathQuery := strings.SplitN(normalizedURI, "?", originURIPathQueryParts)
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

func lookupTLSCertificateByID(ctx context.Context, id uint) (*model.TLSCertificate, error) {
	return model.GetTLSCertificateByID(ctx, id)
}

func lookupTunnelNodeByID(ctx context.Context, id uint) (*model.OpenFlareNode, error) {
	return model.GetOpenFlareNodeByID(ctx, id)
}

func lookupPagesProjectByID(ctx context.Context, id uint) (*model.PagesProject, error) {
	return model.GetPagesProjectByID(ctx, id)
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

func validateCertificateCoverage(certificate *model.TLSCertificate, domains []string) error {
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

func loadProxyRouteZoneDomains(ctx context.Context, ids []uint) ([]model.ZoneDomain, error) {
	if len(ids) == 0 {
		return nil, errors.New(errProxyRouteZoneDomainsRequired)
	}
	seen := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			return nil, errors.New(errProxyRouteZoneDomainNotFound)
		}
		if _, ok := seen[id]; ok {
			return nil, errors.New(errProxyRouteZoneDomainDuplicate)
		}
		seen[id] = struct{}{}
	}
	domains, err := model.ListZoneDomainsByIDs(ctx, ids)
	if err != nil {
		return nil, errors.New(errProxyRouteZoneDomainNotFound)
	}
	return domains, nil
}

func validateProxyRouteSiteName(siteName string) error {
	if strings.TrimSpace(siteName) == "" {
		return errors.New(errProxyRouteSiteNameEmpty)
	}
	return nil
}

func validateProxyRouteSiteNameUniqueness(ctx context.Context, route *model.ProxyRoute, siteName string) error {
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
		if item.SiteName == siteName {
			return errors.New(errProxyRouteSiteNameExists)
		}
	}

	return nil
}

func validateProxyRouteZoneDomainCertificates(ctx context.Context, domains []model.ZoneDomain, enableHTTPS bool) error {
	if !enableHTTPS {
		return nil
	}
	for _, domain := range domains {
		if domain.CertID == nil || *domain.CertID == 0 {
			return errors.New(errProxyRouteCertRequired)
		}
		certificate, err := lookupTLSCertificateByID(ctx, *domain.CertID)
		if err != nil {
			return errors.New(errProxyRouteCertNotFound)
		}
		if err := validateCertificateCoverage(certificate, []string{domain.Domain}); err != nil {
			return err
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

// normalizeCachePolicy stores API write values.
// When enabling with empty/url policy, keep legacy "all" semantics so old rows
// and clients that omit policy do not silently narrow cache to static extensions.
// New UI should send policy=static explicitly when choosing the recommended default.
func normalizeCachePolicy(enabled bool, raw string) string {
	if !enabled {
		return ""
	}
	policy := strings.TrimSpace(strings.ToLower(raw))
	switch policy {
	case "", proxyRouteCachePolicyURL, proxyRouteCachePolicyAll:
		return proxyRouteCachePolicyAll
	case proxyRouteCachePolicyStatic:
		return proxyRouteCachePolicyStatic
	case proxyRouteCachePolicySuffix, proxyRouteCachePolicyPathPrefix, proxyRouteCachePolicyPathExact:
		return policy
	default:
		return policy
	}
}

// displayCachePolicy normalizes values for API list/get (and UI).
func displayCachePolicy(enabled bool, raw string) string {
	if !enabled {
		return ""
	}
	return normalizeCachePolicy(true, raw)
}

func normalizeCacheRules(enabled bool, rawPolicy string, rules []string) ([]string, error) {
	if !enabled {
		return []string{}, nil
	}
	policy := normalizeCachePolicy(enabled, rawPolicy)
	switch policy {
	case proxyRouteCachePolicyStatic, proxyRouteCachePolicyAll, proxyRouteCachePolicyURL:
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

func validateOriginURL(raw string) error {
	if raw == "" {
		return errors.New(errProxyRouteOriginEmpty)
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return errors.New(errProxyRouteOriginInvalid)
	}
	if parsed.Scheme != proxyRouteSchemeHTTP && parsed.Scheme != proxyRouteSchemeHTTPS {
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
	case "", proxyRouteSchemeHTTP, proxyRouteSchemeHTTPS:
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
	case proxyRouteUpstreamTypeTunnel:
		return proxyRouteUpstreamTypeTunnel
	case proxyRouteUpstreamTypePages:
		return proxyRouteUpstreamTypePages
	default:
		return "direct"
	}
}

func normalizeTunnelTargetProtocol(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case proxyRouteSchemeHTTPS:
		return proxyRouteSchemeHTTPS
	default:
		return proxyRouteSchemeHTTP
	}
}
