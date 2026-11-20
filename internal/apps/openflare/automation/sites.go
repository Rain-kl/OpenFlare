package automation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/proxy_route"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/zone"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/routeidentity"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

// ZoneSummary identifies a registered root domain.
type ZoneSummary struct {
	ZoneID     string `json:"zone_id"`
	RootDomain string `json:"root_domain"`
}

// ZoneList contains matching registered root domains.
type ZoneList struct {
	Results []ZoneSummary `json:"results"`
}

// DomainSummary identifies a hostname and its bindings.
type DomainSummary struct {
	DomainRecordID string `json:"domain_record_id"`
	ZoneID         string `json:"zone_id"`
	Hostname       string `json:"hostname"`
	SiteID         string `json:"site_id,omitempty"`
	CertificateID  string `json:"certificate_id,omitempty"`
}

// DomainList contains the hostname records in a Zone.
type DomainList struct {
	Results []DomainSummary `json:"results"`
}

// CertificateSummary exposes certificate metadata without PEM data or keys.
type CertificateSummary struct {
	CertificateID string    `json:"certificate_id"`
	Name          string    `json:"name"`
	PrimaryDomain string    `json:"primary_domain"`
	OtherDomains  string    `json:"other_domains"`
	NotAfter      time.Time `json:"not_after"`
	Status        string    `json:"status"`
}

// CertificateList contains selectable certificate references.
type CertificateList struct {
	Results []CertificateSummary `json:"results"`
}

// SiteSummary describes a site without authentication secrets.
type SiteSummary struct {
	SiteID       string          `json:"site_id"`
	SiteName     string          `json:"site_name"`
	Domains      []DomainSummary `json:"domains"`
	UpstreamType string          `json:"upstream_type"`
	Upstreams    []string        `json:"upstreams,omitempty"`
	OriginHost   string          `json:"origin_host,omitempty"`
	HTTPSEnabled bool            `json:"https_enabled"`
	Enabled      bool            `json:"enabled"`
}

// SiteList contains matching sites.
type SiteList struct {
	Results []SiteSummary `json:"results"`
}

// SiteWriteResult confirms a site change saved on the controller.
type SiteWriteResult struct {
	Site  SiteSummary `json:"site"`
	State string      `json:"state"`
}

// DomainBindingResult confirms domain association saved on the controller.
type DomainBindingResult struct {
	Domain DomainSummary `json:"domain"`
	State  string        `json:"state"`
}

// CreateZoneBody registers a root domain.
type CreateZoneBody struct {
	// Registered root domain such as example.com, without scheme, port or path.
	RootDomain string `json:"root_domain" binding:"required" minLength:"1" maxLength:"253"`
}

// CreateDomainBody registers a hostname with an optional certificate.
type CreateDomainBody struct {
	// Exact hostname such as www.example.com, or an explicit wildcard such as *.example.com; omit scheme, port and path.
	Hostname string `json:"hostname" binding:"required" minLength:"1" maxLength:"253"`
	// Certificate record ID from openflare_certificate_list. Required before binding the hostname to an HTTPS site.
	CertificateID string `json:"certificate_id"`
}

// CreateSiteBody defines the initial configuration for a direct site.
type CreateSiteBody struct {
	// Unique display name for the new site.
	SiteName string `json:"site_name" binding:"required" minLength:"1" maxLength:"128"`
	// Unbound domain record IDs from openflare_domain_list. Use decimal strings, not hostnames.
	DomainRecordIDs []string `json:"domain_record_ids" binding:"required"`
	// Ordered direct HTTP(S) URLs including scheme and optional port. No credentials, query strings or fragments. Multiple upstreams must share a scheme and have no path prefix.
	Upstreams []string `json:"upstreams" binding:"required"`
	// Optional upstream Host override. If omitted, use normal edge Host handling.
	OriginHost string `json:"origin_host" maxLength:"253"`
	// True enables browser-facing HTTPS and requires a covering certificate on each domain. False serves HTTP only. Independent of upstream URL schemes.
	EnableHTTPS bool `json:"enable_https" binding:"required"`
}

// SetUpstreamsBody replaces one site's direct upstreams.
type SetUpstreamsBody struct {
	// Complete replacement list of direct HTTP(S) origin URLs without credentials, query strings or fragments. Multiple upstreams must share a scheme and have no path prefix.
	Upstreams []string `json:"upstreams" binding:"required"`
	// New upstream Host override. Omit to preserve the existing override or pass an empty string to clear it.
	OriginHost *string `json:"origin_host" maxLength:"253"`
}

const savedState = "saved_on_controller"

func decimalID(id uint) string { return strconv.FormatUint(uint64(id), 10) }

func summarizeDomain(item model.ZoneDomain) DomainSummary {
	result := DomainSummary{DomainRecordID: decimalID(item.ID), ZoneID: decimalID(item.ZoneID), Hostname: item.Domain}
	if item.ProxyRouteID != nil {
		result.SiteID = decimalID(*item.ProxyRouteID)
	}
	if item.CertID != nil {
		result.CertificateID = decimalID(*item.CertID)
	}
	return result
}

func summarizeSite(view *proxy_route.View) SiteSummary {
	result := SiteSummary{SiteID: decimalID(view.ID), SiteName: view.SiteName, Domains: make([]DomainSummary, 0, len(view.ZoneDomains)), UpstreamType: view.UpstreamType, HTTPSEnabled: view.EnableHTTPS, Enabled: view.Enabled}
	for _, item := range view.ZoneDomains {
		result.Domains = append(result.Domains, summarizeDomain(model.ZoneDomain{ID: item.ID, ZoneID: item.ZoneID, Domain: item.Domain, CertID: item.CertID, ProxyRouteID: &view.ID}))
	}
	if view.UpstreamType == "direct" || view.UpstreamType == "" {
		result.UpstreamType = "direct"
		result.OriginHost = view.OriginHost
		result.Upstreams = safeUpstreamURLs(view.UpstreamList)
	}
	return result
}

// @ID openflare_zone_list
// @Summary List registered root domains
// @Description Resolve a registered root domain to its zone_id before adding hostnames. Omit root_domain to list all zones.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param root_domain query string false "Exact registered root domain; omit to list all zones" maxlength(253)
// @Success 200 {object} OperationResponse{data=ZoneList}
// @Failure 400,401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"zone list","scope":"sites:read","effect":"read"}
// @Router /api/v1/automation/zones [get]
func listZones(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	items, err := zone.List(ctx)
	if err != nil {
		return nil, operationFailed(ctx, "list zones", err)
	}
	result := ZoneList{Results: make([]ZoneSummary, 0)}
	for _, item := range items {
		if input["root_domain"] == "" || strings.EqualFold(item.Domain, input["root_domain"]) {
			result.Results = append(result.Results, ZoneSummary{ZoneID: decimalID(item.ID), RootDomain: item.Domain})
		}
	}
	return result, nil
}

// @ID openflare_zone_create
// @Summary Register a root domain in OpenFlare
// @Description Create a Zone under which hostnames can be registered. This does not buy a domain or change DNS. Use openflare_zone_list to check for an existing Zone.
// @Tags automation
// @Accept json
// @Produce json
// @Security AutomationBearer
// @Param body body CreateZoneBody true "Registered root domain"
// @Success 200 {object} OperationResponse{data=ZoneSummary}
// @Failure 400,401,403,404,409,500 {object} OperationResponse
// @x-automation {"cli":"zone create","scope":"sites:write","effect":"write"}
// @Router /api/v1/automation/zones [post]
func createZone(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	if _, err := routeidentity.NormalizeDomain(input["root_domain"]); err != nil {
		return nil, invalidArguments(errors.New("root_domain must be a registered root domain without a scheme, port or path"))
	}
	item, err := zone.Create(ctx, zone.Input{Domain: input["root_domain"]})
	if err != nil {
		return nil, zoneFailure(ctx, "create zone", err)
	}
	return ZoneSummary{ZoneID: decimalID(item.ID), RootDomain: item.Domain}, nil
}

// @ID openflare_domain_list
// @Summary List hostnames in a Zone
// @Description Return hostname records under the selected Zone, including their certificate and site bindings. Use domain_record_id to bind an unbound hostname or create a site.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param zone_id path string true "Zone record ID from openflare_zone_list"
// @Success 200 {object} OperationResponse{data=DomainList}
// @Failure 400,401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"domain list","scope":"sites:read","effect":"read","parameters":{"zone_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/zones/{zone_id}/domains [get]
func listDomains(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	zoneID, err := recordID(input, "zone_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	item, err := zone.GetOverview(ctx, zoneID)
	if err != nil {
		return nil, resourceFailure(ctx, "list domains", err)
	}
	result := DomainList{Results: make([]DomainSummary, 0, len(item.Domains))}
	for _, domain := range item.Domains {
		result.Results = append(result.Results, summarizeDomain(domain))
	}
	return result, nil
}

// @ID openflare_domain_create
// @Summary Register a hostname under an existing Zone
// @Description Register a hostname in OpenFlare. This does not publish DNS or bind the hostname to a site. For HTTPS, first select an existing covering certificate.
// @Tags automation
// @Accept json
// @Produce json
// @Security AutomationBearer
// @Param zone_id path string true "Zone record ID from openflare_zone_list"
// @Param body body CreateDomainBody true "Hostname and optional certificate"
// @Success 200 {object} OperationResponse{data=DomainSummary}
// @Failure 400,401,403,404,409,500 {object} OperationResponse
// @x-automation {"cli":"domain create","scope":"sites:write","effect":"write","parameters":{"zone_id":{"pattern":"^[1-9][0-9]{0,18}$"},"certificate_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/zones/{zone_id}/domains [post]
func createDomain(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	zoneID, err := recordID(input, "zone_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	params := zone.DomainInput{Domain: input["hostname"]}
	if _, err := routeidentity.NormalizeDomain(params.Domain); err != nil {
		return nil, invalidArguments(errors.New("hostname must be an exact or wildcard DNS hostname without a scheme, port or path"))
	}
	if _, supplied := input["certificate_id"]; supplied {
		id, err := recordID(input, "certificate_id")
		if err != nil {
			return nil, invalidArguments(err)
		}
		params.CertID = &id
	}
	item, err := zone.CreateDomain(ctx, zoneID, params)
	if err != nil {
		return nil, zoneFailure(ctx, "create domain", err)
	}
	return summarizeDomain(*item), nil
}

// @ID openflare_certificate_list
// @Summary List existing TLS certificates
// @Description Return certificate IDs, configured hostnames, expiration and issuance status. Select a ready certificate covering the target hostname. No private keys or certificate files are returned.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Success 200 {object} OperationResponse{data=CertificateList}
// @Failure 401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"certificate list","scope":"sites:read","effect":"read"}
// @Router /api/v1/automation/certificates [get]
func listCertificates(ctx context.Context, _ identity, _ map[string]string) (any, *operationError) {
	items, err := repository.ListTLSCertificates(ctx)
	if err != nil {
		return nil, operationFailed(ctx, "list certificates", err)
	}
	result := CertificateList{Results: make([]CertificateSummary, 0, len(items))}
	for _, item := range items {
		result.Results = append(result.Results, CertificateSummary{CertificateID: decimalID(item.ID), Name: item.Name, PrimaryDomain: item.PrimaryDomain, OtherDomains: item.OtherDomains, NotAfter: item.NotAfter, Status: item.ApplyStatus})
	}
	return result, nil
}

// @ID openflare_site_list
// @Summary List websites and their upstreams
// @Description Resolve an existing site's name or hostname to its site_id and read its domain bindings and upstreams. Omit search to list all sites. Does not test upstream connectivity.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param search query string false "Literal case-insensitive substring of site name or registered hostname" maxlength(253)
// @Success 200 {object} OperationResponse{data=SiteList}
// @Failure 400,401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"site list","scope":"sites:read","effect":"read"}
// @Router /api/v1/automation/sites [get]
func listSites(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	items, err := proxy_route.ListProxyRoutes(ctx)
	if err != nil {
		return nil, operationFailed(ctx, "list sites", err)
	}
	result := SiteList{Results: make([]SiteSummary, 0)}
	search := strings.ToLower(input["search"])
	for _, item := range items {
		match := strings.Contains(strings.ToLower(item.SiteName), search)
		for _, domain := range item.ZoneDomains {
			match = match || strings.Contains(strings.ToLower(domain.Domain), search)
		}
		if match {
			result.Results = append(result.Results, summarizeSite(item))
		}
	}
	return result, nil
}

// @ID openflare_site_create
// @Summary Create an enabled direct reverse-proxy website
// @Description Create one enabled site with the specified unbound domain records and direct upstreams. Resolve domain_record_ids with openflare_domain_list. The result confirms controller storage only; check node status after synchronization. DNS is unchanged.
// @Tags automation
// @Accept json
// @Produce json
// @Security AutomationBearer
// @Param body body CreateSiteBody true "New website settings"
// @Success 200 {object} OperationResponse{data=SiteWriteResult}
// @Failure 400,401,403,404,409,500 {object} OperationResponse
// @x-automation {"cli":"site create","scope":"sites:write","effect":"write"}
// @Router /api/v1/automation/sites [post]
func createSite(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	var ids, upstreams []string
	if err := json.Unmarshal([]byte(input["domain_record_ids"]), &ids); err != nil {
		return nil, invalidArguments(err)
	}
	if err := json.Unmarshal([]byte(input["upstreams"]), &upstreams); err != nil {
		return nil, invalidArguments(err)
	}
	domains := make([]uint, 0, len(ids))
	for _, value := range ids {
		id, err := recordID(map[string]string{"domain_record_id": value}, "domain_record_id")
		if err != nil || value != decimalID(id) {
			return nil, invalidArguments(errors.New("domain_record_ids must contain positive decimal record ID strings"))
		}
		domains = append(domains, id)
	}
	view, err := proxy_route.CreateDirectSite(ctx, proxy_route.Input{
		SiteName: input["site_name"], ZoneDomainIDs: domains, OriginHost: input["origin_host"], Upstreams: upstreams,
		Enabled: true, EnableHTTPS: input["enable_https"] == "true",
	})
	if err != nil {
		return nil, siteFailure(ctx, "create site", err)
	}
	return SiteWriteResult{Site: summarizeSite(view), State: savedState}, nil
}

// @ID openflare_site_bind_domain
// @Summary Add a registered hostname to an existing website
// @Description Bind one domain to a site without removing its other domains. An existing binding to another site is rejected. HTTPS sites require a covering certificate on the domain. DNS is unchanged.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param site_id path string true "Site record ID from openflare_site_list"
// @Param domain_record_id path string true "Hostname record ID from openflare_domain_list"
// @Success 200 {object} OperationResponse{data=DomainBindingResult}
// @Failure 400,401,403,404,409,500 {object} OperationResponse
// @x-automation {"cli":"site bind-domain","scope":"sites:write","effect":"write","parameters":{"site_id":{"pattern":"^[1-9][0-9]{0,18}$"},"domain_record_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/sites/{site_id}/domains/{domain_record_id} [post]
func bindDomain(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	siteID, err := recordID(input, "site_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	domainID, err := recordID(input, "domain_record_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	domain, err := proxy_route.BindDomain(ctx, siteID, domainID)
	if err != nil {
		return nil, siteFailure(ctx, "bind domain", err)
	}
	return DomainBindingResult{Domain: summarizeDomain(*domain), State: savedState}, nil
}

// @ID openflare_site_set_upstreams
// @Summary Replace the upstream URLs of one direct website
// @Description Replace all direct upstream URLs for the selected site. Use openflare_site_list to read current URLs and decide which to retain. This detaches the previous shared origin record without changing other sites. Domain, frontend HTTPS, caching and authentication settings are preserved. Tunnel and Pages sites are rejected.
// @Tags automation
// @Accept json
// @Produce json
// @Security AutomationBearer
// @Param site_id path string true "Site record ID from openflare_site_list"
// @Param body body SetUpstreamsBody true "Replacement direct upstreams"
// @Success 200 {object} OperationResponse{data=SiteWriteResult}
// @Failure 400,401,403,404,409,500 {object} OperationResponse
// @x-automation {"cli":"site set-upstreams","scope":"sites:write","effect":"write","parameters":{"site_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/sites/{site_id}/upstreams [patch]
func setUpstreams(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	siteID, err := recordID(input, "site_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	var upstreams []string
	if err := json.Unmarshal([]byte(input["upstreams"]), &upstreams); err != nil {
		return nil, invalidArguments(err)
	}
	var originHost *string
	if host, exists := input["origin_host"]; exists {
		originHost = &host
	}
	view, err := proxy_route.SetDirectUpstreams(ctx, siteID, upstreams, originHost)
	if err != nil {
		return nil, siteFailure(ctx, "set upstreams", err)
	}
	return SiteWriteResult{Site: summarizeSite(view), State: savedState}, nil
}

func siteFailure(ctx context.Context, operation string, err error) *operationError {
	if errors.Is(err, proxy_route.ErrInvalidDirectSite) {
		return invalidArguments(err)
	}
	if errors.Is(err, proxy_route.ErrNotDirectSite) || errors.Is(err, proxy_route.ErrSiteConflict) {
		return &operationError{http.StatusConflict, "SITE_CONFLICT", err.Error()}
	}
	return resourceFailure(ctx, operation, err)
}

func zoneFailure(ctx context.Context, operation string, err error) *operationError {
	switch {
	case errors.Is(err, zone.ErrDomainExists):
		return &operationError{http.StatusConflict, "DOMAIN_EXISTS", err.Error()}
	case errors.Is(err, zone.ErrInvalidZoneRoot), errors.Is(err, zone.ErrDomainOutsideZone):
		return invalidArguments(err)
	case errors.Is(err, zone.ErrCertificateNotFound):
		return &operationError{http.StatusNotFound, "CERTIFICATE_NOT_FOUND", err.Error()}
	default:
		return resourceFailure(ctx, operation, err)
	}
}

func safeUpstreamURLs(upstreams []string) []string {
	result := make([]string, 0, len(upstreams))
	for _, upstream := range upstreams {
		parsed, err := url.Parse(upstream)
		if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			continue
		}
		parsed.User, parsed.RawQuery, parsed.Fragment, parsed.RawFragment, parsed.ForceQuery = nil, "", "", "", false
		result = append(result, parsed.String())
	}
	return result
}
