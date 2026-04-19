package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"openflare/model"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

var proxyHeaderKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var proxyRouteLimitRatePattern = regexp.MustCompile(`^\d+(?:[kKmM])?$`)

const (
	proxyRouteCachePolicyURL        = "url"
	proxyRouteCachePolicySuffix     = "suffix"
	proxyRouteCachePolicyPathPrefix = "path_prefix"
	proxyRouteCachePolicyPathExact  = "path_exact"
)

type ProxyRouteCustomHeaderInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ProxyRouteInput struct {
	SiteName           string                        `json:"site_name"`
	Domain             string                        `json:"domain"`
	Domains            []string                      `json:"domains"`
	OriginID           *uint                         `json:"origin_id"`
	OriginURL          string                        `json:"origin_url"`
	OriginScheme       string                        `json:"origin_scheme"`
	OriginAddress      string                        `json:"origin_address"`
	OriginPort         string                        `json:"origin_port"`
	OriginURI          string                        `json:"origin_uri"`
	OriginHost         string                        `json:"origin_host"`
	Upstreams          []string                      `json:"upstreams"`
	Enabled            bool                          `json:"enabled"`
	EnableHTTPS        bool                          `json:"enable_https"`
	CertID             *uint                         `json:"cert_id"`
	CertIDs            []uint                        `json:"cert_ids"`
	DomainCertIDs      []uint                        `json:"domain_cert_ids"`
	RedirectHTTP       bool                          `json:"redirect_http"`
	LimitConnPerServer int                           `json:"limit_conn_per_server"`
	LimitConnPerIP     int                           `json:"limit_conn_per_ip"`
	LimitRate          string                        `json:"limit_rate"`
	CacheEnabled       bool                          `json:"cache_enabled"`
	CachePolicy        string                        `json:"cache_policy"`
	CacheRules         []string                      `json:"cache_rules"`
	CustomHeaders      []ProxyRouteCustomHeaderInput `json:"custom_headers"`
	PoWEnabled         bool                          `json:"pow_enabled"`
	PoWConfig          string                        `json:"pow_config"`
	Remark             string                        `json:"remark"`
}

type ProxyRouteView struct {
	ID                 uint                          `json:"id"`
	SiteName           string                        `json:"site_name"`
	Domain             string                        `json:"domain"`
	Domains            []string                      `json:"domains"`
	PrimaryDomain      string                        `json:"primary_domain"`
	DomainCount        int                           `json:"domain_count"`
	OriginID           *uint                         `json:"origin_id"`
	OriginURL          string                        `json:"origin_url"`
	OriginHost         string                        `json:"origin_host"`
	Upstreams          string                        `json:"upstreams"`
	UpstreamList       []string                      `json:"upstream_list"`
	Enabled            bool                          `json:"enabled"`
	EnableHTTPS        bool                          `json:"enable_https"`
	CertID             *uint                         `json:"cert_id"`
	CertIDs            []uint                        `json:"cert_ids"`
	DomainCertIDs      []uint                        `json:"domain_cert_ids"`
	RedirectHTTP       bool                          `json:"redirect_http"`
	LimitConnPerServer int                           `json:"limit_conn_per_server"`
	LimitConnPerIP     int                           `json:"limit_conn_per_ip"`
	LimitRate          string                        `json:"limit_rate"`
	CacheEnabled       bool                          `json:"cache_enabled"`
	CachePolicy        string                        `json:"cache_policy"`
	CacheRules         string                        `json:"cache_rules"`
	CacheRuleList      []string                      `json:"cache_rule_list"`
	CustomHeaders      string                        `json:"custom_headers"`
	CustomHeaderList   []ProxyRouteCustomHeaderInput `json:"custom_header_list"`
	PoWEnabled         bool                          `json:"pow_enabled"`
	PoWConfig          *ProxyRoutePoWConfig          `json:"pow_config"`
	Remark             string                        `json:"remark"`
	CreatedAt          time.Time                     `json:"created_at"`
	UpdatedAt          time.Time                     `json:"updated_at"`
}

func ListProxyRoutes() ([]*ProxyRouteView, error) {
	routes, err := model.ListProxyRoutes()
	if err != nil {
		return nil, err
	}
	return buildProxyRouteViews(routes)
}

func GetProxyRoute(id uint) (*ProxyRouteView, error) {
	route, err := model.GetProxyRouteByID(id)
	if err != nil {
		return nil, err
	}
	return buildProxyRouteView(route)
}

func CreateProxyRoute(input ProxyRouteInput) (*ProxyRouteView, error) {
	route, err := buildProxyRoute(nil, input)
	if err != nil {
		return nil, err
	}
	if err = route.Insert(); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("proxy route identity already exists")
		}
		return nil, err
	}
	return buildProxyRouteView(route)
}

func UpdateProxyRoute(id uint, input ProxyRouteInput) (*ProxyRouteView, error) {
	route, err := model.GetProxyRouteByID(id)
	if err != nil {
		return nil, err
	}
	route, err = buildProxyRoute(route, input)
	if err != nil {
		return nil, err
	}
	if err = route.Update(); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("proxy route identity already exists")
		}
		return nil, err
	}
	return buildProxyRouteView(route)
}

func DeleteProxyRoute(id uint) error {
	route, err := model.GetProxyRouteByID(id)
	if err != nil {
		return err
	}
	return route.Delete()
}

func buildProxyRoute(route *model.ProxyRoute, input ProxyRouteInput) (*model.ProxyRoute, error) {
	domains, err := normalizeProxyRouteDomainsInput(route, input.Domain, input.Domains)
	if err != nil {
		return nil, err
	}
	domain := domains[0]
	siteName := normalizeProxyRouteSiteNameInput(route, input.SiteName, domain)

	originURL, originID, err := resolveProxyRoutePrimaryOrigin(input)
	if err != nil {
		return nil, err
	}
	originHost := strings.TrimSpace(input.OriginHost)
	remark := strings.TrimSpace(input.Remark)
	upstreams, err := normalizeUpstreams(originURL, input.Upstreams)
	if err != nil {
		return nil, err
	}
	cachePolicy := strings.TrimSpace(input.CachePolicy)
	cacheRules, err := normalizeCacheRules(input.CacheEnabled, cachePolicy, input.CacheRules)
	if err != nil {
		return nil, err
	}
	customHeaders, err := normalizeCustomHeaders(input.CustomHeaders)
	if err != nil {
		return nil, err
	}
	limitConnPerServer, err := normalizeProxyRouteLimitConnValue(input.LimitConnPerServer, "limit_conn_per_server")
	if err != nil {
		return nil, err
	}
	limitConnPerIP, err := normalizeProxyRouteLimitConnValue(input.LimitConnPerIP, "limit_conn_per_ip")
	if err != nil {
		return nil, err
	}
	limitRate, err := normalizeProxyRouteLimitRate(input.LimitRate)
	if err != nil {
		return nil, err
	}

	cacheRulesJSON, err := json.Marshal(cacheRules)
	if err != nil {
		return nil, err
	}
	upstreamsJSON, err := json.Marshal(upstreams)
	if err != nil {
		return nil, err
	}
	customHeadersJSON, err := json.Marshal(customHeaders)
	if err != nil {
		return nil, err
	}

	powConfig, err := normalizePoWConfig(input.PoWEnabled, input.PoWConfig)
	if err != nil {
		return nil, err
	}
	powConfigJSON, err := json.Marshal(powConfig)
	if err != nil {
		return nil, err
	}

	if !input.EnableHTTPS {
		input.RedirectHTTP = false
		input.CertID = nil
		input.CertIDs = nil
		input.DomainCertIDs = nil
	}
	domainCertIDs, certIDs, primaryCertID, err := normalizeProxyRouteDomainCertificateIDs(
		domains,
		input.EnableHTTPS,
		input.DomainCertIDs,
		input.CertID,
		input.CertIDs,
	)
	if err != nil {
		return nil, err
	}
	if err := validateProxyRouteDomainCertificateCoverage(domains, domainCertIDs); err != nil {
		return nil, err
	}
	certIDsJSON, err := json.Marshal(certIDs)
	if err != nil {
		return nil, err
	}
	domainCertIDsJSON, err := json.Marshal(domainCertIDs)
	if err != nil {
		return nil, err
	}
	domainsJSON, err := json.Marshal(domains)
	if err != nil {
		return nil, err
	}

	if err := validateProxyRouteSiteName(siteName); err != nil {
		return nil, err
	}
	if err := validateProxyRouteIdentityUniqueness(route, siteName, domains); err != nil {
		return nil, err
	}
	if err := validateOriginHost(originHost); err != nil {
		return nil, err
	}
	input.DomainCertIDs = domainCertIDs
	input.CertIDs = certIDs
	input.CertID = primaryCertID
	if input.RedirectHTTP && !input.EnableHTTPS {
		return nil, errors.New("redirect_http requires enable_https")
	}

	if route == nil {
		route = &model.ProxyRoute{}
	}
	route.SiteName = siteName
	route.Domain = domain
	route.Domains = string(domainsJSON)
	route.OriginID = originID
	route.OriginURL = upstreams[0]
	route.OriginHost = originHost
	route.Upstreams = string(upstreamsJSON)
	route.Enabled = input.Enabled
	route.EnableHTTPS = input.EnableHTTPS
	route.CertID = input.CertID
	route.CertIDs = string(certIDsJSON)
	route.DomainCertIDs = string(domainCertIDsJSON)
	route.RedirectHTTP = input.RedirectHTTP
	route.LimitConnPerServer = limitConnPerServer
	route.LimitConnPerIP = limitConnPerIP
	route.LimitRate = limitRate
	route.CacheEnabled = input.CacheEnabled
	route.CachePolicy = normalizeCachePolicy(input.CacheEnabled, cachePolicy)
	route.CacheRules = string(cacheRulesJSON)
	route.CustomHeaders = string(customHeadersJSON)
	route.PoWEnabled = input.PoWEnabled
	route.PoWConfig = string(powConfigJSON)
	route.Remark = remark
	return route, nil
}

func buildProxyRouteViews(routes []*model.ProxyRoute) ([]*ProxyRouteView, error) {
	views := make([]*ProxyRouteView, 0, len(routes))
	for _, route := range routes {
		view, err := buildProxyRouteView(route)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func buildProxyRouteView(route *model.ProxyRoute) (*ProxyRouteView, error) {
	if route == nil {
		return nil, errors.New("proxy route is nil")
	}
	domains, err := decodeStoredDomains(route.Domains, route.Domain)
	if err != nil {
		return nil, err
	}
	upstreams, err := decodeStoredUpstreams(route.Upstreams, route.OriginURL)
	if err != nil {
		return nil, err
	}
	cacheRules, err := decodeStoredCacheRules(route.CacheRules)
	if err != nil {
		return nil, err
	}
	customHeaders, err := decodeStoredCustomHeaders(route.CustomHeaders)
	if err != nil {
		return nil, err
	}
	powConfig, err := decodeStoredPoWConfig(route.PoWEnabled, route.PoWConfig)
	if err != nil {
		return nil, err
	}
	certIDs, err := decodeStoredCertIDs(route.CertIDs, route.CertID)
	if err != nil {
		return nil, err
	}
	domainCertIDs, err := resolveProxyRouteDomainCertIDs(route, domains, certIDs)
	if err != nil {
		return nil, err
	}
	var certID *uint
	if len(certIDs) > 0 {
		certID = &certIDs[0]
	}
	primaryDomain := domains[0]
	return &ProxyRouteView{
		ID:                 route.ID,
		SiteName:           normalizeProxyRouteSiteNameInput(route, route.SiteName, primaryDomain),
		Domain:             primaryDomain,
		Domains:            domains,
		PrimaryDomain:      primaryDomain,
		DomainCount:        len(domains),
		OriginID:           route.OriginID,
		OriginURL:          route.OriginURL,
		OriginHost:         route.OriginHost,
		Upstreams:          route.Upstreams,
		UpstreamList:       upstreams,
		Enabled:            route.Enabled,
		EnableHTTPS:        route.EnableHTTPS,
		CertID:             certID,
		CertIDs:            certIDs,
		DomainCertIDs:      domainCertIDs,
		RedirectHTTP:       route.RedirectHTTP,
		LimitConnPerServer: route.LimitConnPerServer,
		LimitConnPerIP:     route.LimitConnPerIP,
		LimitRate:          route.LimitRate,
		CacheEnabled:       route.CacheEnabled,
		CachePolicy:        route.CachePolicy,
		CacheRules:         route.CacheRules,
		CacheRuleList:      cacheRules,
		CustomHeaders:      route.CustomHeaders,
		CustomHeaderList:   customHeaders,
		PoWEnabled:         route.PoWEnabled,
		PoWConfig:          powConfig,
		Remark:             route.Remark,
		CreatedAt:          route.CreatedAt,
		UpdatedAt:          route.UpdatedAt,
	}, nil
}

func normalizeProxyRouteSiteNameInput(route *model.ProxyRoute, raw string, primaryDomain string) string {
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

func normalizeProxyRouteDomainsInput(route *model.ProxyRoute, rawDomain string, rawDomains []string) ([]string, error) {
	if len(rawDomains) > 0 {
		domains, err := normalizeProxyRouteDomains(rawDomains)
		if err != nil {
			return nil, err
		}
		domain := normalizeProxyRouteDomainValue(rawDomain)
		if domain != "" && domain != domains[0] {
			return nil, errors.New("domain must match domains[0]")
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

func normalizeProxyRouteDomains(rawDomains []string) ([]string, error) {
	normalized := make([]string, 0, len(rawDomains))
	seen := make(map[string]struct{}, len(rawDomains))
	for _, rawDomain := range rawDomains {
		domain := normalizeProxyRouteDomainValue(rawDomain)
		if domain == "" {
			continue
		}
		if strings.Contains(domain, "://") || strings.Contains(domain, "/") {
			return nil, errors.New("domain format is invalid")
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		normalized = append(normalized, domain)
	}
	if len(normalized) == 0 {
		return nil, errors.New("at least one domain is required")
	}
	return normalized, nil
}

func validateProxyRouteSiteName(siteName string) error {
	if strings.TrimSpace(siteName) == "" {
		return errors.New("site_name cannot be empty")
	}
	return nil
}

func validateProxyRouteIdentityUniqueness(route *model.ProxyRoute, siteName string, domains []string) error {
	routes, err := model.ListProxyRoutes()
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
			return errors.New("site_name already exists")
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
				return fmt.Errorf("domain %s already exists", domain)
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

func normalizeProxyRouteCertificateIDs(enableHTTPS bool, certID *uint, certIDs []uint) ([]uint, error) {
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
		if _, err := model.GetTLSCertificateByID(item); err != nil {
			return nil, errors.New("selected certificate does not exist")
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	if len(normalized) == 0 {
		return nil, errors.New("must select a certificate when HTTPS is enabled")
	}
	return normalized, nil
}

func normalizeProxyRouteDomainCertificateIDs(
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
			return nil, nil, nil, errors.New("domain_cert_ids must match domains length")
		}

		normalizedDomainCertIDs := make([]uint, len(rawDomainCertIDs))
		uniqueCertIDs := make([]uint, 0, len(rawDomainCertIDs))
		seen := make(map[uint]struct{}, len(rawDomainCertIDs))
		hasAssignedCertificate := false
		for index, item := range rawDomainCertIDs {
			if item == 0 {
				continue
			}
			if _, err := model.GetTLSCertificateByID(item); err != nil {
				return nil, nil, nil, errors.New("selected certificate does not exist")
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
			return nil, nil, nil, errors.New("must select a certificate when HTTPS is enabled")
		}

		primaryCertID := &uniqueCertIDs[0]
		return normalizedDomainCertIDs, uniqueCertIDs, primaryCertID, nil
	}

	normalizedCertIDs, err := normalizeProxyRouteCertificateIDs(
		enableHTTPS,
		certID,
		certIDs,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	switch {
	case len(normalizedCertIDs) == 0:
		return nil, nil, nil, errors.New("must select a certificate when HTTPS is enabled")
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
		domainCertIDs, err := deriveDomainCertIDsFromCertificateSet(
			domains,
			normalizedCertIDs,
		)
		if err != nil {
			return nil, nil, nil, err
		}
		primaryCertID := &normalizedCertIDs[0]
		return domainCertIDs, normalizedCertIDs, primaryCertID, nil
	}
}

func validateProxyRouteDomainCertificateCoverage(
	domains []string,
	domainCertIDs []uint,
) error {
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
		certificate, err := model.GetTLSCertificateByID(certID)
		if err != nil {
			return errors.New("selected certificate does not exist")
		}
		if err := validateCertificateCoverage(certificate, assignedDomains); err != nil {
			return err
		}
	}
	return nil
}

func deriveDomainCertIDsFromCertificateSet(
	domains []string,
	certIDs []uint,
) ([]uint, error) {
	certificates, err := loadTLSCertificates(certIDs)
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

func resolveProxyRouteDomainCertIDs(
	route *model.ProxyRoute,
	domains []string,
	certIDs []uint,
) ([]uint, error) {
	domainCertIDs, err := decodeStoredDomainCertIDs(route.DomainCertIDs, len(domains))
	if err != nil {
		return nil, err
	}
	if len(domainCertIDs) > 0 || len(certIDs) == 0 {
		return domainCertIDs, nil
	}
	return deriveDomainCertIDsFromCertificateSet(domains, certIDs)
}

func normalizeProxyRouteLimitRate(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" || normalized == "0" {
		return "", nil
	}
	if !proxyRouteLimitRatePattern.MatchString(normalized) {
		return "", errors.New("limit_rate must be a number or use the 512k / 1m format")
	}
	if strings.TrimRight(normalized, "km") == "" {
		return "", nil
	}
	return normalized, nil
}

func resolveProxyRoutePrimaryOrigin(input ProxyRouteInput) (string, *uint, error) {
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
			origin, err := model.GetOriginByID(*input.OriginID)
			if err != nil {
				return "", nil, errors.New("selected origin does not exist")
			}
			originURL, err := buildOriginURLFromParts(
				scheme,
				origin.Address,
				port,
				uri,
			)
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
		origin, err := getOrCreateOriginByAddress(address)
		if err != nil {
			return "", nil, err
		}
		return originURL, &origin.ID, nil
	}

	originURL := strings.TrimSpace(input.OriginURL)
	if originURL == "" {
		return "", nil, errors.New("origin_url cannot be empty")
	}
	address, err := extractOriginAddress(originURL)
	if err != nil {
		return "", nil, err
	}
	origin, findErr := model.GetOriginByAddress(address)
	if findErr == nil {
		return originURL, &origin.ID, nil
	}
	if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return "", nil, findErr
	}
	return originURL, nil, nil
}

func hasStructuredOriginInput(input ProxyRouteInput) bool {
	return (input.OriginID != nil && *input.OriginID != 0) ||
		strings.TrimSpace(input.OriginScheme) != "" ||
		strings.TrimSpace(input.OriginAddress) != "" ||
		strings.TrimSpace(input.OriginPort) != "" ||
		strings.TrimSpace(input.OriginURI) != ""
}

func normalizeCustomHeaders(headers []ProxyRouteCustomHeaderInput) ([]ProxyRouteCustomHeaderInput, error) {
	if len(headers) == 0 {
		return []ProxyRouteCustomHeaderInput{}, nil
	}
	normalized := make([]ProxyRouteCustomHeaderInput, 0, len(headers))
	for _, header := range headers {
		key := strings.TrimSpace(header.Key)
		value := strings.TrimSpace(header.Value)
		if key == "" && value == "" {
			continue
		}
		if key == "" {
			return nil, errors.New("custom header key cannot be empty")
		}
		if !proxyHeaderKeyPattern.MatchString(key) {
			return nil, errors.New("custom header key format is invalid")
		}
		if strings.ContainsAny(key, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return nil, errors.New("custom headers cannot contain newlines")
		}
		normalized = append(normalized, ProxyRouteCustomHeaderInput{
			Key:   key,
			Value: value,
		})
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
	unique := make([]string, 0, len(trimmed))
	seen := make(map[string]struct{}, len(trimmed))
	for _, item := range trimmed {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		unique = append(unique, item)
	}
	normalized := make([]string, 0, len(unique))
	var scheme string
	multiUpstream := len(unique) > 1
	for _, item := range unique {
		if err := validateOriginURL(item); err != nil {
			return nil, err
		}
		parsed, err := url.ParseRequestURI(item)
		if err != nil {
			return nil, errors.New("origin URL format is invalid")
		}
		if multiUpstream && parsed.Path != "" && parsed.Path != "/" {
			return nil, errors.New("multi-upstream mode does not support origin paths")
		}
		if multiUpstream && parsed.RawQuery != "" {
			return nil, errors.New("multi-upstream mode does not support origin query strings")
		}
		if scheme == "" {
			scheme = parsed.Scheme
		} else if scheme != parsed.Scheme {
			return nil, errors.New("all upstreams must use the same scheme")
		}
		normalized = append(normalized, item)
	}
	if len(normalized) == 0 {
		return nil, errors.New("at least one upstream is required")
	}
	return normalized, nil
}

func decodeStoredCustomHeaders(raw string) ([]ProxyRouteCustomHeaderInput, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return []ProxyRouteCustomHeaderInput{}, nil
	}
	var headers []ProxyRouteCustomHeaderInput
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
		return nil, errors.New("cache policy is not supported")
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
			return nil, errors.New("cache suffix format is invalid")
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	if len(normalized) == 0 {
		return nil, errors.New("at least one suffix is required")
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
			return nil, errors.New("cache path rule format is invalid")
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
			return nil, errors.New("at least one path prefix is required")
		}
		return nil, errors.New("at least one exact path is required")
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
		return errors.New("origin URL cannot be empty")
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return errors.New("origin URL format is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("origin URL must start with http:// or https://")
	}
	if parsed.Host == "" {
		return errors.New("origin URL format is invalid")
	}
	return nil
}

func validateOriginHost(raw string) error {
	if raw == "" {
		return nil
	}
	if strings.ContainsAny(raw, "/\\ \t\r\n") || strings.Contains(raw, "://") {
		return errors.New("origin_host format is invalid")
	}
	parsed, err := url.Parse("//" + raw)
	if err != nil || parsed.Host == "" || parsed.Host != raw {
		return errors.New("origin_host format is invalid")
	}
	if parsed.Hostname() == "" {
		return errors.New("origin_host format is invalid")
	}
	return nil
}

func isUniqueConstraintError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique")
}

// PoW configuration types and validation

type ProxyRoutePoWListConfig struct {
	IPs         []string `json:"ips"`
	IPCidrs     []string `json:"ip_cidrs"`
	Paths       []string `json:"paths"`
	PathRegexes []string `json:"path_regexes"`
	UserAgents  []string `json:"user_agents"`
}

type ProxyRoutePoWConfig struct {
	Difficulty   int                     `json:"difficulty"`
	Algorithm    string                  `json:"algorithm"`
	SessionTTL   int                     `json:"session_ttl"`
	ChallengeTTL int                     `json:"challenge_ttl"`
	Whitelist    ProxyRoutePoWListConfig `json:"whitelist"`
	Blacklist    ProxyRoutePoWListConfig `json:"blacklist"`
}

var powAlgorithmValues = map[string]bool{"fast": true, "slow": true}

func defaultPoWConfig() ProxyRoutePoWConfig {
	return ProxyRoutePoWConfig{
		Difficulty:   4,
		Algorithm:    "fast",
		SessionTTL:   86400,
		ChallengeTTL: 300,
		Whitelist:    ProxyRoutePoWListConfig{IPs: []string{}, IPCidrs: []string{}, Paths: []string{}, PathRegexes: []string{}, UserAgents: []string{}},
		Blacklist:    ProxyRoutePoWListConfig{IPs: []string{}, IPCidrs: []string{}, Paths: []string{}, PathRegexes: []string{}, UserAgents: []string{}},
	}
}

func normalizePoWConfig(enabled bool, raw string) (ProxyRoutePoWConfig, error) {
	if !enabled {
		return defaultPoWConfig(), nil
	}

	cfg := defaultPoWConfig()
	text := strings.TrimSpace(raw)
	if text != "" && text != "{}" {
		if err := json.Unmarshal([]byte(text), &cfg); err != nil {
			return cfg, errors.New("pow_config 格式无效")
		}
	}

	if cfg.Difficulty < 1 || cfg.Difficulty > 16 {
		return cfg, errors.New("pow_config.difficulty 必须在 1-16 之间")
	}
	if !powAlgorithmValues[cfg.Algorithm] {
		return cfg, errors.New("pow_config.algorithm 必须为 fast 或 slow")
	}
	if cfg.SessionTTL < 60 {
		return cfg, errors.New("pow_config.session_ttl 不能小于 60 秒")
	}
	if cfg.ChallengeTTL < 30 {
		return cfg, errors.New("pow_config.challenge_ttl 不能小于 30 秒")
	}

	for _, cidr := range cfg.Whitelist.IPCidrs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return cfg, fmt.Errorf("pow_config 白名单 IP CIDR 格式无效: %s", cidr)
		}
	}
	for _, cidr := range cfg.Blacklist.IPCidrs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return cfg, fmt.Errorf("pow_config 黑名单 IP CIDR 格式无效: %s", cidr)
		}
	}

	for _, re := range cfg.Whitelist.PathRegexes {
		if _, err := regexp.Compile(re); err != nil {
			return cfg, fmt.Errorf("pow_config 白名单路径正则格式无效: %s", re)
		}
	}
	for _, re := range cfg.Blacklist.PathRegexes {
		if _, err := regexp.Compile(re); err != nil {
			return cfg, fmt.Errorf("pow_config 黑名单路径正则格式无效: %s", re)
		}
	}

	for _, ip := range cfg.Whitelist.IPs {
		if net.ParseIP(ip) == nil {
			return cfg, fmt.Errorf("pow_config 白名单 IP 格式无效: %s", ip)
		}
	}
	for _, ip := range cfg.Blacklist.IPs {
		if net.ParseIP(ip) == nil {
			return cfg, fmt.Errorf("pow_config 黑名单 IP 格式无效: %s", ip)
		}
	}

	type dimension struct {
		name string
		wl   []string
		bl   []string
	}
	dimensions := []dimension{
		{"IP", cfg.Whitelist.IPs, cfg.Blacklist.IPs},
		{"IP CIDR", cfg.Whitelist.IPCidrs, cfg.Blacklist.IPCidrs},
		{"路径", cfg.Whitelist.Paths, cfg.Blacklist.Paths},
		{"路径正则", cfg.Whitelist.PathRegexes, cfg.Blacklist.PathRegexes},
		{"User-Agent", cfg.Whitelist.UserAgents, cfg.Blacklist.UserAgents},
	}
	for _, dim := range dimensions {
		if len(dim.wl) > 0 && len(dim.bl) > 0 {
			return cfg, fmt.Errorf("pow_config %s 不能同时配置白名单和黑名单", dim.name)
		}
	}

	return cfg, nil
}

func decodeStoredPoWConfig(enabled bool, raw string) (*ProxyRoutePoWConfig, error) {
	if !enabled {
		cfg := defaultPoWConfig()
		return &cfg, nil
	}
	text := strings.TrimSpace(raw)
	if text == "" || text == "{}" {
		cfg := defaultPoWConfig()
		return &cfg, nil
	}
	var cfg ProxyRoutePoWConfig
	if err := json.Unmarshal([]byte(text), &cfg); err != nil {
		return nil, errors.New("pow_config 格式无效")
	}
	return &cfg, nil
}
