// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package routeidentity

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDecodeDomainsNormalizesCaseAndOrder(t *testing.T) {
	domains, err := DecodeDomains(`["WWW.Example.COM","example.com"]`, "fallback.example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"www.example.com", "example.com"}, domains)
}
