package openresty

import (
	"fmt"
	"strings"
)

type routeCertPartition struct {
	httpOnlyDomains []string
	domainsByCertID map[uint][]string
}

func partitionRouteDomainsByCert(domains []string, certIDs, domainCertIDs []uint) routeCertPartition {
	httpOnlyDomains := make([]string, 0, len(domains))
	domainsByCertID := make(map[uint][]string, len(certIDs))
	for index, domain := range domains {
		if index >= len(domainCertIDs) || domainCertIDs[index] == 0 {
			httpOnlyDomains = append(httpOnlyDomains, domain)
			continue
		}
		domainsByCertID[domainCertIDs[index]] = append(domainsByCertID[domainCertIDs[index]], domain)
	}
	return routeCertPartition{
		httpOnlyDomains: httpOnlyDomains,
		domainsByCertID: domainsByCertID,
	}
}

func validateRouteCertificates(route Route, displayName string, certIDs []uint, partition routeCertPartition, certificates map[uint]string) error {
	for _, certID := range certIDs {
		assignedDomains := partition.domainsByCertID[certID]
		if len(assignedDomains) == 0 {
			continue
		}
		certPEM, ok := certificates[certID]
		if !ok {
			return fmt.Errorf("route %s certificate %d does not exist", route.SiteName, certID)
		}
		if err := validateCertificateCoverage(certPEM, assignedDomains); err != nil {
			return fmt.Errorf("site %s certificate validation failed: %w", displayName, err)
		}
	}
	return nil
}

func renderPagesRouteHTTPS(
	builder *strings.Builder,
	serverNames, displayName string,
	route Route,
	partition routeCertPartition,
	certIDs []uint,
	limitConfig routeLimitConfig,
	powEnabled bool,
	cfg ConfigSnapshot,
) {
	if route.RedirectHTTP {
		if len(partition.httpOnlyDomains) > 0 {
			builder.WriteString(renderHTTPPagesServer(renderServerNames(partition.httpOnlyDomains), displayName, route.PagesDeployment, limitConfig, powEnabled, route.BasicAuthEnabled, route.BasicAuthUsername, route.BasicAuthPassword))
		}
		for _, certID := range certIDs {
			if assignedDomains := partition.domainsByCertID[certID]; len(assignedDomains) > 0 {
				builder.WriteString(renderHTTPRedirectServer(renderServerNames(assignedDomains)))
			}
		}
	} else {
		builder.WriteString(renderHTTPPagesServer(serverNames, displayName, route.PagesDeployment, limitConfig, powEnabled, route.BasicAuthEnabled, route.BasicAuthUsername, route.BasicAuthPassword))
	}
	for _, certID := range certIDs {
		if assignedDomains := partition.domainsByCertID[certID]; len(assignedDomains) > 0 {
			builder.WriteString(renderHTTPSPagesServer(renderServerNames(assignedDomains), displayName, certID, route.PagesDeployment, limitConfig, powEnabled, route.BasicAuthEnabled, route.BasicAuthUsername, route.BasicAuthPassword, cfg))
		}
	}
}

func renderProxyRouteHTTPS(
	builder *strings.Builder,
	serverNames, displayName string,
	route Route,
	partition routeCertPartition,
	certIDs []uint,
	cacheConfig routeCacheConfig,
	limitConfig routeLimitConfig,
	upstreamConfig routeUpstreamConfig,
	powEnabled bool,
	cfg ConfigSnapshot,
) {
	if route.RedirectHTTP {
		if len(partition.httpOnlyDomains) > 0 {
			builder.WriteString(renderHTTPProxyServer(renderServerNames(partition.httpOnlyDomains), displayName, route.OriginURL, route.OriginHost, route.CustomHeaders, cacheConfig, limitConfig, upstreamConfig, powEnabled, route.BasicAuthEnabled, route.BasicAuthUsername, route.BasicAuthPassword, cfg))
		}
		for _, certID := range certIDs {
			if assignedDomains := partition.domainsByCertID[certID]; len(assignedDomains) > 0 {
				builder.WriteString(renderHTTPRedirectServer(renderServerNames(assignedDomains)))
			}
		}
	} else {
		builder.WriteString(renderHTTPProxyServer(serverNames, displayName, route.OriginURL, route.OriginHost, route.CustomHeaders, cacheConfig, limitConfig, upstreamConfig, powEnabled, route.BasicAuthEnabled, route.BasicAuthUsername, route.BasicAuthPassword, cfg))
	}
	for _, certID := range certIDs {
		if assignedDomains := partition.domainsByCertID[certID]; len(assignedDomains) > 0 {
			builder.WriteString(renderHTTPSServer(renderServerNames(assignedDomains), displayName, route.OriginURL, route.OriginHost, certID, route.CustomHeaders, cacheConfig, limitConfig, upstreamConfig, powEnabled, route.BasicAuthEnabled, route.BasicAuthUsername, route.BasicAuthPassword, cfg))
		}
	}
}

func renderPagesRoute(builder *strings.Builder, route Route, displayName, serverNames string, certificates map[uint]string, limitConfig routeLimitConfig, powEnabled bool, cfg ConfigSnapshot) error {
	if route.PagesDeployment == nil {
		return fmt.Errorf("route %s pages deployment is missing", route.SiteName)
	}
	if !route.EnableHTTPS {
		builder.WriteString(renderHTTPPagesServer(serverNames, displayName, route.PagesDeployment, limitConfig, powEnabled, route.BasicAuthEnabled, route.BasicAuthUsername, route.BasicAuthPassword))
		return nil
	}
	certIDs := certificateIDsFromDomainCertIDs(route.DomainCertIDs)
	domainCertIDs := route.DomainCertIDs
	if len(certIDs) == 0 {
		return fmt.Errorf("路由 %s 未配置证书", route.SiteName)
	}
	partition := partitionRouteDomainsByCert(normalizedRouteDomains(route), certIDs, domainCertIDs)
	if err := validateRouteCertificates(route, displayName, certIDs, partition, certificates); err != nil {
		return err
	}
	renderPagesRouteHTTPS(builder, serverNames, displayName, route, partition, certIDs, limitConfig, powEnabled, cfg)
	return nil
}

func renderProxyRoute(builder *strings.Builder, route Route, displayName, serverNames string, certificates map[uint]string, cacheConfig routeCacheConfig, limitConfig routeLimitConfig, powEnabled bool, cfg ConfigSnapshot) error {
	upstreams := route.Upstreams
	if len(upstreams) == 0 && strings.TrimSpace(route.OriginURL) != "" {
		upstreams = []string{route.OriginURL}
	}
	upstreamConfig := buildRouteUpstreamConfig(route, upstreams)
	if upstreamConfig.UsesNamedUpstream {
		builder.WriteString(renderNamedUpstreamBlock(upstreamConfig))
	}
	if !route.EnableHTTPS {
		builder.WriteString(renderHTTPProxyServer(serverNames, displayName, route.OriginURL, route.OriginHost, route.CustomHeaders, cacheConfig, limitConfig, upstreamConfig, powEnabled, route.BasicAuthEnabled, route.BasicAuthUsername, route.BasicAuthPassword, cfg))
		return nil
	}
	certIDs := certificateIDsFromDomainCertIDs(route.DomainCertIDs)
	domainCertIDs := route.DomainCertIDs
	if len(certIDs) == 0 {
		return fmt.Errorf("路由 %s 未配置证书", route.SiteName)
	}
	partition := partitionRouteDomainsByCert(normalizedRouteDomains(route), certIDs, domainCertIDs)
	if err := validateRouteCertificates(route, displayName, certIDs, partition, certificates); err != nil {
		return err
	}
	renderProxyRouteHTTPS(builder, serverNames, displayName, route, partition, certIDs, cacheConfig, limitConfig, upstreamConfig, powEnabled, cfg)
	return nil
}
