// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"testing"

	openrestyrender "github.com/Rain-kl/Wavelet/internal/openflare/openresty"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeProxyCachePathForSnapshot(t *testing.T) {
	assert.Equal(t, "/var/cache/openresty", normalizeProxyCachePathForSnapshot(false, "/var/cache/openresty"))
	assert.Equal(t, openrestyrender.ProxyCachePathPlaceholder, normalizeProxyCachePathForSnapshot(true, "/var/cache/openresty"))
	assert.Equal(t, openrestyrender.ProxyCachePathPlaceholder, normalizeProxyCachePathForSnapshot(true, ""))
	assert.Equal(t, "/data/var/cache/custom", normalizeProxyCachePathForSnapshot(true, "/data/var/cache/custom"))
}
