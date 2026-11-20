// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package edgeconfig

import (
	"encoding/json"
	"fmt"
)

// PagesDirPlaceholder anchors the local Pages project path.
const PagesDirPlaceholder = "__OPENFLARE_PAGES_DIR__"

// DefaultStaticCacheExtensions is the built-in suffix allowlist for cache_policy=static.
// HTML and JSON are excluded (Cloudflare default). map/mjs/wasm are intentional extras.
var DefaultStaticCacheExtensions = []string{
	"css", "js", "mjs", "map",
	"ico", "cur", "gif", "jpg", "jpeg", "png", "webp", "avif", "svg", "svgz",
	"ttf", "otf", "woff", "woff2", "eot",
	"mp3", "mp4", "webm", "ogg", "flac",
	"wasm", "pdf",
	"zip", "7z", "gz", "tar",
}

// PoWListConfig holds the IP, CIDR, path, and user-agent lists used by the
// Proof-of-Work whitelist or blacklist filter.
type PoWListConfig struct {
	IPs         []string `json:"ips"`
	IPCidrs     []string `json:"ip_cidrs"`
	Paths       []string `json:"paths"`
	PathRegexes []string `json:"path_regexes"`
	UserAgents  []string `json:"user_agents"`
}

// PoWConfig holds the full Proof-of-Work challenge parameters for a route,
// including difficulty, algorithm, TTLs, and allow/block lists.
type PoWConfig struct {
	Difficulty   int           `json:"difficulty"`
	Algorithm    string        `json:"algorithm"`
	SessionTTL   int           `json:"session_ttl"`
	ChallengeTTL int           `json:"challenge_ttl"`
	Whitelist    PoWListConfig `json:"whitelist"`
	Blacklist    PoWListConfig `json:"blacklist"`
	PathRules    *PoWPathRules `json:"path_rules,omitempty"`
}

// PoWPathRules selects normalized paths, with whitelist bypasses taking precedence.
type PoWPathRules struct {
	Mode      string         `json:"mode"`
	Whitelist []PoWPathMatch `json:"whitelist,omitempty"`
	Blacklist []PoWPathMatch `json:"blacklist,omitempty"`
}

// PoWPathMatch matches a path using exact, prefix, wildcard or regex semantics.
type PoWPathMatch struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// DefaultPoWConfig returns the canonical PoW defaults used when pow_enabled is
// true but no explicit pow_config payload is available.
func DefaultPoWConfig() PoWConfig {
	return PoWConfig{
		Difficulty:   4,
		Algorithm:    "fast",
		SessionTTL:   600,
		ChallengeTTL: 300,
		Whitelist:    PoWListConfig{IPs: []string{}, IPCidrs: []string{}, Paths: []string{}, PathRegexes: []string{}, UserAgents: []string{}},
		Blacklist:    PoWListConfig{IPs: []string{}, IPCidrs: []string{}, Paths: []string{}, PathRegexes: []string{}, UserAgents: []string{}},
	}
}

// Route describes a single proxy or pages site entry in the OpenFlare config
// document, including upstream, TLS, caching, rate-limiting and WAF settings.
type Route struct {
	ID                 uint             `json:"id,omitempty"`
	SiteName           string           `json:"site_name,omitempty"`
	Domains            []string         `json:"domains,omitempty"`
	OriginURL          string           `json:"origin_url"`
	OriginHost         string           `json:"origin_host,omitempty"`
	Upstreams          []string         `json:"upstreams,omitempty"`
	Enabled            bool             `json:"enabled"`
	EnableHTTPS        bool             `json:"enable_https"`
	EnableHTTP2        bool             `json:"enable_http2"`
	DomainCertIDs      []uint           `json:"domain_cert_ids,omitempty"`
	RedirectHTTP       bool             `json:"redirect_http"`
	LimitConnPerServer int              `json:"limit_conn_per_server,omitempty"`
	LimitConnPerIP     int              `json:"limit_conn_per_ip,omitempty"`
	LimitRate          string           `json:"limit_rate,omitempty"`
	LimitReqPerIP      string           `json:"limit_req_per_ip,omitempty"`
	CacheEnabled       bool             `json:"cache_enabled"`
	CachePolicy        string           `json:"cache_policy,omitempty"`
	CacheRules         []string         `json:"cache_rules,omitempty"`
	CustomHeaders      []CustomHeader   `json:"custom_headers,omitempty"`
	PoWEnabled         bool             `json:"pow_enabled,omitempty"`
	PoWConfig          *PoWConfig       `json:"pow_config,omitempty"`
	BasicAuthEnabled   bool             `json:"basic_auth_enabled,omitempty"`
	BasicAuthUsername  string           `json:"basic_auth_username,omitempty"`
	BasicAuthPassword  string           `json:"basic_auth_password,omitempty"`
	SiteAuthTTLHours   int              `json:"site_auth_ttl_hours,omitempty"`
	SiteAuthKey        string           `json:"site_auth_key,omitempty"`
	UpstreamType       string           `json:"upstream_type,omitempty"`
	PagesDeployment    *PagesDeployment `json:"pages_deployment,omitempty"`
}

// PagesDeployment holds the static-site deployment parameters for a Pages-type
// route, including local root, entry file, SPA fallback, and API proxy options.
//
// LocalRoot is anchored on ProjectID (projects/{id}/current), not a specific
// deployment ID, so Agents can switch active packages without re-publishing
// the site snapshot.
type PagesDeployment struct {
	ProjectID          uint   `json:"project_id"`
	ProjectSlug        string `json:"project_slug"`
	DeploymentID       uint   `json:"deployment_id"`
	DeploymentNumber   int    `json:"deployment_number"`
	Checksum           string `json:"checksum"`
	EntryFile          string `json:"entry_file"`
	SPAFallbackEnabled bool   `json:"spa_fallback_enabled"`
	SPAFallbackPath    string `json:"spa_fallback_path"`
	APIProxyEnabled    bool   `json:"api_proxy_enabled"`
	APIProxyPath       string `json:"api_proxy_path"`
	APIProxyPass       string `json:"api_proxy_pass"`
	APIProxyRewrite    string `json:"api_proxy_rewrite"`
	LocalRoot          string `json:"local_root"`
}

// PagesProjectLocalRoot returns the Agent-local root for a Pages project.
func PagesProjectLocalRoot(projectID uint) string {
	if projectID == 0 {
		return PagesDirPlaceholder
	}
	return fmt.Sprintf("%s/projects/%d/current", PagesDirPlaceholder, projectID)
}

// WAFRuleGraph is the compact graph executed by the edge WAF.
type WAFRuleGraph struct {
	Entry string                 `json:"entry"`
	Nodes map[string]WAFRuleNode `json:"nodes"`
}

// WAFRuleNode contains one compiled node and its handle-to-target edges.
type WAFRuleNode struct {
	Type   string            `json:"type"`
	Config json.RawMessage   `json:"config,omitempty"`
	Next   map[string]string `json:"next,omitempty"`
}

// WAFRuleGroup defines one enabled runtime graph. Legacy flattened fields are
// retained only for decoding older stored snapshots during rolling upgrades.
type WAFRuleGroup struct {
	ID                uint         `json:"id"`
	Name              string       `json:"name"`
	Enabled           bool         `json:"enabled"`
	IsGlobal          bool         `json:"is_global"`
	BlockStatusCode   int          `json:"block_status_code"`
	BlockResponseBody string       `json:"block_response_body,omitempty"`
	IPWhitelist       []string     `json:"ip_whitelist,omitempty"`
	IPBlacklist       []string     `json:"ip_blacklist,omitempty"`
	IPWhitelistGroups []uint       `json:"ip_whitelist_group_ids,omitempty"`
	IPBlacklistGroups []uint       `json:"ip_blacklist_group_ids,omitempty"`
	CountryWhitelist  []string     `json:"country_whitelist,omitempty"`
	CountryBlacklist  []string     `json:"country_blacklist,omitempty"`
	RegionWhitelist   []string     `json:"region_whitelist,omitempty"`
	RegionBlacklist   []string     `json:"region_blacklist,omitempty"`
	PoWEnabled        bool         `json:"pow_enabled,omitempty"`
	PoWConfig         *PoWConfig   `json:"pow_config,omitempty"`
	Graph             WAFRuleGraph `json:"graph"`
}

// WAFIPGroup is a named, reusable list of IP addresses or CIDRs that can be
// referenced by multiple WAF rule groups as a whitelist or blacklist.
type WAFIPGroup struct {
	ID      uint     `json:"id"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Enabled bool     `json:"enabled"`
	IPList  []string `json:"ip_list,omitempty"`
}

// WAFBinding associates a route (by site name) with the WAF rule groups that
// should be enforced for that site.
type WAFBinding struct {
	RouteID      uint   `json:"route_id"`
	SiteName     string `json:"site_name"`
	RuleGroupIDs []uint `json:"rule_group_ids"`
}

// WAFDocument is the top-level WAF configuration snapshot containing rule
// groups, IP groups, and per-site bindings.
type WAFDocument struct {
	RuleGroups []WAFRuleGroup `json:"rule_groups"`
	IPGroups   []WAFIPGroup   `json:"ip_groups,omitempty"`
	Bindings   []WAFBinding   `json:"bindings"`
}

// ConfigSnapshot contains site-wide policy and response settings.
type ConfigSnapshot struct {
	TrustedProxyCIDRs          []string      `json:"trusted_proxy_cidrs"`
	CacheStaticExtensions      []string      `json:"cache_static_extensions,omitempty"`
	DefaultLimitConnPerServer  int           `json:"default_limit_conn_per_server,omitempty"`
	DefaultLimitConnPerIP      int           `json:"default_limit_conn_per_ip,omitempty"`
	DefaultLimitRate           string        `json:"default_limit_rate,omitempty"`
	DefaultLimitReqPerIP       string        `json:"default_limit_req_per_ip,omitempty"`
	OriginErrorPageEnabled     bool          `json:"origin_error_page_enabled"`
	OriginErrorPageStatusCodes []string      `json:"origin_error_page_status_codes,omitempty"`
	OriginErrorPageHTML        string        `json:"origin_error_page_html,omitempty"`
	OriginErrorPageGetOnly     bool          `json:"origin_error_page_get_only,omitempty"`
	SWOfflineEnabled           bool          `json:"sw_offline_enabled,omitempty"`
	SWOfflineHTML              string        `json:"sw_offline_html,omitempty"`
	SWOfflineDomains           []string      `json:"sw_offline_domains,omitempty"`
	PoWPage                    PoWPageConfig `json:"pow_page,omitempty"`
	SiteAuthHTML               string        `json:"site_auth_html,omitempty"`
}

// DefaultTrustedProxyCIDRs returns Cloudflare's published IPv4 and IPv6
// networks. Callers can override the list, including with an explicit empty
// slice to disable trusted-proxy processing.
func DefaultTrustedProxyCIDRs() []string {
	return []string{
		"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
		"141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20",
		"197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
		"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
		"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32",
		"2405:8100::/32", "2a06:98c0::/29", "2c0f:f248::/32",
	}
}

// Document carries the site routing and policy snapshot.
type Document struct {
	Routes   []Route        `json:"routes"`
	Settings ConfigSnapshot `json:"edge_config"`
	WAF      WAFDocument    `json:"waf"`
}

// UnmarshalJSON reads shared site snapshots already referenced by published versions.
func (d *Document) UnmarshalJSON(data []byte) error {
	type document Document
	disk := struct {
		*document
		Settings   *ConfigSnapshot `json:"edge_config"`
		Historical *ConfigSnapshot `json:"openresty_config"`
	}{document: (*document)(d)}
	if err := json.Unmarshal(data, &disk); err != nil {
		return err
	}
	if disk.Settings != nil {
		d.Settings = *disk.Settings
	} else if disk.Historical != nil {
		d.Settings = *disk.Historical
	}
	return nil
}

// SupportFile is an immutable release asset.
type SupportFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// CustomHeader holds an origin request header.
type CustomHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
