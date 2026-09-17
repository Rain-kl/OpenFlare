// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package publicconfig implements contracts.PublicConfigProvider for OpenFlare.
package publicconfig

import (
	"context"

	"Wavelet/core"
	"Wavelet/openflare/plugins/server/kernel/repository"
)

// Provider returns visibility=1 system configs as a flat key/value map,
// matching gold GetPublicConfig.
type Provider struct{}

// New constructs a PublicConfigProvider. ctx is accepted for future binding
// but the payload is loaded from the shared system config store.
func New(_ *core.Context) *Provider {
	return &Provider{}
}

// PublicConfig returns visibility=1 keys as map[string]string.
func (p *Provider) PublicConfig(ctx context.Context) (map[string]string, error) {
	configs, err := repository.ListVisibleSystemConfigs(ctx)
	if err != nil {
		return nil, err
	}
	resp := make(map[string]string, len(configs))
	for _, config := range configs {
		resp[config.Key] = config.Value
	}
	return resp, nil
}
