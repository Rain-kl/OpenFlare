// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package routeidentity

import (
	"testing"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeDomainsNormalizesCaseAndOrder(t *testing.T) {
	domains, err := DecodeDomains(`["WWW.Example.COM","example.com"]`, "fallback.example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"www.example.com", "example.com"}, domains)
}

func TestResolveSiteNamePrefersExplicitValue(t *testing.T) {
	route := &model.ProxyRoute{SiteName: "stored-name", Domain: "example.com"}
	assert.Equal(t, "custom", ResolveSiteName(route, "custom", "example.com"))
	assert.Equal(t, "stored-name", ResolveSiteName(route, "", "example.com"))

	routeWithoutSiteName := &model.ProxyRoute{Domain: "example.com"}
	assert.Equal(t, "example.com", ResolveSiteName(routeWithoutSiteName, "", "example.com"))
}

func TestResolveFromRoute(t *testing.T) {
	route := &model.ProxyRoute{
		Domain:  "Example.COM",
		Domains: `["example.com","www.example.com"]`,
	}
	siteName, domains, err := ResolveFromRoute(route)
	require.NoError(t, err)
	assert.Equal(t, "example.com", siteName)
	assert.Equal(t, []string{"example.com", "www.example.com"}, domains)
}
