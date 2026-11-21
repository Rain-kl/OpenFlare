// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/observability"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

// SiteTrafficTotals counts logged requests, distinct source IPs, bytes, and HTTP errors.
type SiteTrafficTotals struct {
	Requests     int64 `json:"requests"`
	UniqueIPs    int64 `json:"unique_ips"`
	BytesSent    int64 `json:"bytes_sent"`
	ClientErrors int64 `json:"http_4xx"`
	ServerErrors int64 `json:"http_5xx"`
}

// SiteOverviewItem is a compact inventory entry with traffic across all its hostnames.
type SiteOverviewItem struct {
	SiteID       string            `json:"site_id"`
	SiteName     string            `json:"site_name"`
	Hostnames    []string          `json:"hostnames"`
	Enabled      bool              `json:"enabled"`
	HTTPSEnabled bool              `json:"https_enabled"`
	UpstreamType string            `json:"upstream_type"`
	Traffic      SiteTrafficTotals `json:"traffic"`
}

// SiteOverview binds the entire response to a single access-log window.
type SiteOverview struct {
	Since             time.Time                        `json:"since"`
	Until             time.Time                        `json:"until"`
	SiteCount         int                              `json:"site_count"`
	EnabledSiteCount  int                              `json:"enabled_site_count"`
	SitesWithRequests int                              `json:"sites_with_requests"`
	TotalTraffic      SiteTrafficTotals                `json:"total_traffic"`
	UnmatchedRequests int64                            `json:"unmatched_requests"`
	StatusCodes       []observability.DistributionItem `json:"status_codes"`
	Sites             []SiteOverviewItem               `json:"sites"`
	NextOffset        *int                             `json:"next_offset,omitempty"`
}

// @ID openflare_site_overview
// @Summary List configured websites with recent traffic in one call
// @Description Start here for the current site inventory and recent traffic. Returns instance-wide totals, common statuses and sites ranked by requests, including enabled, disabled and zero-traffic sites. Site traffic combines all hostnames using current bindings. Instance totals include unmatched hostnames. Use site_list for upstream configuration, metrics_traffic for a time series, or logs_list for individual requests.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param hours query integer false "Lookback in hours, ending at query time" minimum(1) maximum(720) default(24)
// @Param until query string false "Exclusive RFC3339 window end; default is now. Reuse the returned until when paginating." maxlength(35)
// @Param node_id query string false "Optional node registration identifier for traffic; the site inventory is instance-wide" maxlength(64)
// @Param limit query integer false "Maximum number of sites returned, descending by request count" minimum(1) maximum(100) default(50)
// @Param offset query integer false "Site offset; continue with next_offset and the same filters" minimum(0) maximum(2147483647) default(0)
// @Success 200 {object} OperationResponse{data=SiteOverview}
// @Failure 400,401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"site overview","scope":"sites:read","requires":["metrics:read"],"effect":"read"}
// @Router /api/v1/automation/sites/overview [get]
func siteOverview(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	until := time.Now().UTC()
	if value, supplied := input["until"]; supplied {
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return nil, invalidArguments(errors.New("until must be an RFC3339 timestamp"))
		}
		until = parsed.UTC()
	}
	query, _ := automationTrafficWindow(input, until)
	routes, err := repository.ListProxyRoutesWithDomains(ctx)
	if err != nil {
		return nil, operationFailed(ctx, "list site inventory", err)
	}
	totals, statuses, err := readTrafficTotals(ctx, query)
	if err != nil {
		return nil, operationFailed(ctx, "read site overview totals", err)
	}
	var bindings []model.OpenFlareAccessLogSiteBinding
	for _, route := range routes {
		for _, domain := range route.ZoneDomains {
			bindings = append(bindings, model.OpenFlareAccessLogSiteBinding{Hostname: domain.Domain, SiteID: route.ID})
		}
	}
	rows, err := repository.SiteTrafficOpenFlareAccessLogs(ctx, query, bindings)
	if err != nil {
		return nil, operationFailed(ctx, "group traffic by site", err)
	}
	return assembleSiteOverview(query, routes, rows, totals, statuses, input), nil
}

func assembleSiteOverview(
	query model.OpenFlareAccessLogQuery,
	routes []*model.ProxyRoute,
	rows []model.OpenFlareAccessLogSiteTraffic,
	totals model.OpenFlareAccessLogTrafficSummary,
	statuses []observability.DistributionItem,
	input map[string]string,
) SiteOverview {
	result := SiteOverview{
		Since: query.Since, Until: query.Until, SiteCount: len(routes), StatusCodes: statuses,
		TotalTraffic: SiteTrafficTotals{totals.RequestCount, totals.UniqueIPCount, totals.BytesSent, totals.Status4xxCount, totals.ErrorCount},
	}
	bySite := make(map[uint]SiteTrafficTotals, len(rows))
	for _, row := range rows {
		if row.SiteID == 0 {
			result.UnmatchedRequests = row.Requests
			continue
		}
		bySite[row.SiteID] = SiteTrafficTotals{row.Requests, row.UniqueIPs, row.BytesSent, row.ClientErrors, row.ServerErrors}
	}
	sites := make([]SiteOverviewItem, 0, len(routes))
	for _, route := range routes {
		item := SiteOverviewItem{
			SiteID: decimalID(route.ID), SiteName: route.SiteName, Enabled: route.Enabled,
			HTTPSEnabled: route.EnableHTTPS, UpstreamType: route.UpstreamType,
			Hostnames: make([]string, 0, len(route.ZoneDomains)), Traffic: bySite[route.ID],
		}
		for _, domain := range route.ZoneDomains {
			item.Hostnames = append(item.Hostnames, domain.Domain)
		}
		if item.Enabled {
			result.EnabledSiteCount++
		}
		if item.Traffic.Requests > 0 {
			result.SitesWithRequests++
		}
		sites = append(sites, item)
	}
	slices.SortStableFunc(sites, func(a, b SiteOverviewItem) int {
		return cmp.Compare(b.Traffic.Requests, a.Traffic.Requests)
	})
	limit := inputInt(input, "limit")
	if limit == 0 {
		limit = 50
	}
	start := min(len(sites), inputInt(input, "offset"))
	end := min(len(sites), start+limit)
	if end < len(sites) {
		result.NextOffset = &end
	}
	result.Sites = sites[start:end]
	return result
}
