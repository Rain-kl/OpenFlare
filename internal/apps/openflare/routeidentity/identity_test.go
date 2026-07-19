// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package routeidentity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeDomainsNormalizesCaseAndOrder(t *testing.T) {
	domains, err := DecodeDomains(`["WWW.Example.COM","example.com"]`, "fallback.example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"www.example.com", "example.com"}, domains)
}
