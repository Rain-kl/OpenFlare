// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package proxy_route

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

// Errors from direct site changes that can be presented to operators.
var (
	ErrInvalidDirectSite = errors.New("invalid direct site configuration")
	ErrNotDirectSite     = errors.New("the site uses a tunnel or Pages upstream")
	ErrSiteConflict      = errors.New("the site name or domain is already in use")
)

// CreateDirectSite validates and creates a site with direct upstream URLs.
func CreateDirectSite(ctx context.Context, input Input) (*View, error) {
	upstreams, err := validateDirectUpstreams(input.Upstreams, input.OriginHost)
	if err != nil {
		return nil, err
	}
	input.OriginURL, input.Upstreams = upstreams[0], upstreams[1:]
	view, err := CreateProxyRoute(ctx, input)
	if err == nil {
		return view, nil
	}
	switch err.Error() {
	case errProxyRouteSiteNameExists, errProxyRouteIdentityExists, errProxyRouteZoneDomainBound:
		return nil, ErrSiteConflict
	case errProxyRouteZoneDomainNotFound, errProxyRouteCertNotFound:
		return nil, gorm.ErrRecordNotFound
	case errProxyRouteSiteNameEmpty, errProxyRouteZoneDomainsRequired, errProxyRouteZoneDomainDuplicate, errProxyRouteCertRequired:
		return nil, fmt.Errorf("%w: %s", ErrInvalidDirectSite, err)
	}
	if errors.Is(err, errCertificateCoverage) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidDirectSite, err)
	}
	return nil, err
}

func validateDirectUpstreams(upstreams []string, originHost string) ([]string, error) {
	if len(upstreams) == 0 {
		return nil, fmt.Errorf("%w: at least one upstream URL is required", ErrInvalidDirectSite)
	}
	for _, upstream := range upstreams {
		parsed, err := url.Parse(upstream)
		if err != nil || parsed.User != nil || parsed.Hostname() == "" || strings.Contains(upstream, "#") || parsed.RawQuery != "" || parsed.ForceQuery {
			return nil, fmt.Errorf("%w: upstreams must be HTTP(S) URLs without credentials, query strings or fragments", ErrInvalidDirectSite)
		}
		if port := parsed.Port(); port != "" {
			value, err := strconv.ParseUint(port, 10, 16)
			if err != nil || value == 0 {
				return nil, fmt.Errorf("%w: an explicit upstream port must be between 1 and 65535", ErrInvalidDirectSite)
			}
		} else if strings.HasSuffix(parsed.Host, ":") {
			return nil, fmt.Errorf("%w: an explicit upstream port must not be empty", ErrInvalidDirectSite)
		}
	}
	normalized, err := normalizeUpstreams("", upstreams)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidDirectSite, err)
	}
	if err := validateOriginHost(originHost); err != nil || strings.ContainsAny(originHost, "@?#") {
		return nil, fmt.Errorf("%w: origin_host must be a hostname or address with an optional port", ErrInvalidDirectSite)
	}
	return normalized, nil
}

// SetDirectUpstreams replaces only the upstream fields of an existing direct site.
func SetDirectUpstreams(ctx context.Context, id uint, upstreams []string, originHost *string) (*View, error) {
	host := ""
	if originHost != nil {
		host = *originHost
	}
	normalized, err := validateDirectUpstreams(upstreams, host)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	err = repository.WithProxyRouteTx(ctx, func(tx *gorm.DB) error {
		route, err := repository.LockProxyRouteTx(tx, id)
		if err != nil {
			return err
		}
		if normalizeUpstreamType(route.UpstreamType) != "direct" {
			return ErrNotDirectSite
		}
		route.OriginURL, route.Upstreams = normalized[0], string(encoded)
		if originHost != nil {
			route.OriginHost = host
		}
		route.OriginID = nil
		return repository.UpdateDirectRouteUpstreamsTx(tx, route)
	})
	if err != nil {
		return nil, err
	}
	return GetProxyRoute(ctx, id)
}

// BindDomain adds a hostname to one site without removing existing bindings.
func BindDomain(ctx context.Context, routeID, domainID uint) (*model.ZoneDomain, error) {
	var domain *model.ZoneDomain
	err := repository.WithProxyRouteTx(ctx, func(tx *gorm.DB) error {
		route, err := repository.LockProxyRouteTx(tx, routeID)
		if err != nil {
			return err
		}
		domain, err = repository.LockZoneDomainTx(tx, domainID)
		if err != nil {
			return err
		}
		if route.EnableHTTPS {
			if domain.CertID == nil {
				return fmt.Errorf("%w: %s", ErrInvalidDirectSite, errProxyRouteCertRequired)
			}
			certificate, err := repository.GetTLSCertificateByIDTx(tx, *domain.CertID)
			if err != nil {
				return err
			}
			if err := validateCertificateCoverage(certificate, []string{domain.Domain}); err != nil {
				return fmt.Errorf("%w: the selected certificate must cover the domain", ErrInvalidDirectSite)
			}
		}
		if err := repository.BindZoneDomainTx(tx, domain, routeID); err != nil {
			return err
		}
		domain.ProxyRouteID = &routeID
		return nil
	})
	return domain, err
}
