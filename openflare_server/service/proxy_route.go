package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"openflare/model"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

var proxyHeaderKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

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
	SiteName      string                        `json:"site_name"`
	Domain        string                        `json:"domain"`
	Domains       []string                      `json:"domains"`
	OriginID      *uint                         `json:"origin_id"`
	OriginURL     string                        `json:"origin_url"`
	OriginScheme  string                        `json:"origin_scheme"`
	OriginAddress string                        `json:"origin_address"`
	OriginPort    string                        `json:"origin_port"`
	OriginURI     string                        `json:"origin_uri"`
	OriginHost    string                        `json:"origin_host"`
	Upstreams     []string                      `json:"upstreams"`
	Enabled       bool                          `json:"enabled"`
	EnableHTTPS   bool                          `json:"enable_https"`
	CertID        *uint                         `json:"cert_id"`
	RedirectHTTP  bool                          `json:"redirect_http"`
	CacheEnabled  bool                          `json:"cache_enabled"`
	CachePolicy   string                        `json:"cache_policy"`
	CacheRules    []string                      `json:"cache_rules"`
	CustomHeaders []ProxyRouteCustomHeaderInput `json:"custom_headers"`
	Remark        string                        `json:"remark"`
}

func ListProxyRoutes() ([]*model.ProxyRoute, error) {
	return model.ListProxyRoutes()
}

func CreateProxyRoute(input ProxyRouteInput) (*model.ProxyRoute, error) {
	route, err := buildProxyRoute(nil, input)
	if err != nil {
		return nil, err
	}
	if err = route.Insert(); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("域名已存在")
		}
		return nil, err
	}
	return route, nil
}

func UpdateProxyRoute(id uint, input ProxyRouteInput) (*model.ProxyRoute, error) {
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
			return nil, errors.New("域名已存在")
		}
		return nil, err
	}
	return route, nil
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
	if !input.EnableHTTPS {
		input.RedirectHTTP = false
		input.CertID = nil
	}
	if input.EnableHTTPS {
		if input.CertID == nil || *input.CertID == 0 {
			return nil, errors.New("启用 HTTPS 时必须选择证书")
		}
		if _, err := model.GetTLSCertificateByID(*input.CertID); err != nil {
			return nil, errors.New("所选证书不存在")
		}
	}
	if input.RedirectHTTP && !input.EnableHTTPS {
		return nil, errors.New("仅启用 HTTPS 后才能开启 HTTP 重定向")
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
	route.RedirectHTTP = input.RedirectHTTP
	route.CacheEnabled = input.CacheEnabled
	route.CachePolicy = normalizeCachePolicy(input.CacheEnabled, cachePolicy)
	route.CacheRules = string(cacheRulesJSON)
	route.CustomHeaders = string(customHeadersJSON)
	route.Remark = remark
	return route, nil
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
			return nil, errors.New("域名格式不合法")
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		normalized = append(normalized, domain)
	}
	if len(normalized) == 0 {
		return nil, errors.New("至少填写一个域名")
	}
	return normalized, nil
}

func validateProxyRouteSiteName(siteName string) error {
	if strings.TrimSpace(siteName) == "" {
		return errors.New("站点标识不能为空")
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
			return errors.New("站点标识已存在")
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
				return fmt.Errorf("域名 %s 已存在", domain)
			}
		}
	}

	return nil
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
				return "", nil, errors.New("所选源站不存在")
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
		return "", nil, errors.New("源站地址不能为空")
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
			return nil, errors.New("自定义请求头名称不能为空")
		}
		if !proxyHeaderKeyPattern.MatchString(key) {
			return nil, errors.New("自定义请求头名称格式不合法")
		}
		if strings.ContainsAny(key, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return nil, errors.New("自定义请求头不能包含换行")
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
			return nil, errors.New("源站地址格式不合法")
		}
		if multiUpstream && parsed.Path != "" && parsed.Path != "/" {
			return nil, errors.New("多上游模式暂不支持带路径的源站地址")
		}
		if multiUpstream && parsed.RawQuery != "" {
			return nil, errors.New("多上游模式暂不支持带查询参数的源站地址")
		}
		if scheme == "" {
			scheme = parsed.Scheme
		} else if scheme != parsed.Scheme {
			return nil, errors.New("同一规则的多个上游必须使用相同协议")
		}
		normalized = append(normalized, item)
	}
	if len(normalized) == 0 {
		return nil, errors.New("至少填写一个上游地址")
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
		return nil, errors.New("自定义请求头配置格式不合法")
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
		return nil, errors.New("缓存策略不支持")
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
			return nil, errors.New("缓存后缀格式不合法")
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	if len(normalized) == 0 {
		return nil, errors.New("按后缀缓存时至少填写一个后缀")
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
			return nil, errors.New("缓存路径规则格式不合法")
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
			return nil, errors.New("按路径前缀缓存时至少填写一个路径")
		}
		return nil, errors.New("按精确路径缓存时至少填写一个路径")
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
		return nil, errors.New("缓存规则格式不合法")
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
		return nil, errors.New("上游配置格式不合法")
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
		return nil, errors.New("域名配置格式不合法")
	}
	return normalizeProxyRouteDomains(domains)
}

func validateOriginURL(raw string) error {
	if raw == "" {
		return errors.New("源站地址不能为空")
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return errors.New("源站地址格式不合法")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("源站地址必须以 http:// 或 https:// 开头")
	}
	if parsed.Host == "" {
		return errors.New("源站地址格式不合法")
	}
	return nil
}

func validateOriginHost(raw string) error {
	if raw == "" {
		return nil
	}
	if strings.ContainsAny(raw, "/\\ \t\r\n") || strings.Contains(raw, "://") {
		return errors.New("回源主机名格式不合法")
	}
	parsed, err := url.Parse("//" + raw)
	if err != nil || parsed.Host == "" || parsed.Host != raw {
		return errors.New("回源主机名格式不合法")
	}
	if parsed.Hostname() == "" {
		return errors.New("回源主机名格式不合法")
	}
	return nil
}

func isUniqueConstraintError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique")
}
