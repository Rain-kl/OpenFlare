// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package pow manages site proof-of-work policies independently of WAF.
package pow

import (
	"context"
	"encoding/json"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

// Policy is the administrator-facing site configuration.
type Policy struct {
	RouteID uint                 `json:"route_id"`
	Enabled bool                 `json:"enabled"`
	Config  edgeconfig.PoWConfig `json:"config"`
}

// ListPolicies lists configured policies; unconfigured sites use disabled defaults.
func ListPolicies(ctx context.Context) ([]Policy, error) {
	rows, err := repository.ListOpenFlarePoWPolicies(ctx)
	if err != nil {
		return nil, err
	}
	policies := make([]Policy, 0, len(rows))
	for _, row := range rows {
		var config edgeconfig.PoWConfig
		if err := json.Unmarshal([]byte(row.Config), &config); err != nil {
			return nil, err
		}
		policies = append(policies, Policy{RouteID: row.RouteID, Enabled: row.Enabled, Config: config})
	}
	return policies, nil
}

// SavePolicy validates and replaces a site's independent PoW configuration.
func SavePolicy(ctx context.Context, routeID uint, input Policy) error {
	if _, err := repository.GetProxyRouteByID(ctx, routeID); err != nil {
		return err
	}
	if err := edgeconfig.ValidatePoWConfig(input.Config); err != nil {
		return &ValidationError{Err: err}
	}
	raw, err := json.Marshal(input.Config)
	if err != nil {
		return err
	}
	return repository.SaveOpenFlarePoWPolicy(ctx, model.OpenFlarePoWPolicy{RouteID: routeID, Enabled: input.Enabled, Config: string(raw)})
}

// ValidationError carries safe policy validation feedback.
type ValidationError struct{ Err error }

func (e *ValidationError) Error() string { return e.Err.Error() }
