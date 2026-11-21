// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package pow manages site proof-of-work policies independently of WAF.
package pow

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
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

// GetPolicy returns a site's policy or its disabled defaults.
func GetPolicy(ctx context.Context, routeID uint) (*Policy, error) {
	if _, err := repository.GetProxyRouteByID(ctx, routeID); err != nil {
		return nil, err
	}
	row, err := repository.GetOpenFlarePoWPolicy(ctx, routeID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &Policy{RouteID: routeID, Config: edgeconfig.DefaultPoWConfig()}, nil
	}
	if err != nil {
		return nil, err
	}
	var config edgeconfig.PoWConfig
	if err := json.Unmarshal([]byte(row.Config), &config); err != nil {
		return nil, err
	}
	return &Policy{RouteID: routeID, Enabled: row.Enabled, Config: config}, nil
}

// SavePolicy validates and replaces a site's independent PoW configuration.
func SavePolicy(ctx context.Context, routeID uint, input Policy) error {
	return SetPolicy(ctx, routeID, input.Enabled, &input.Config)
}

// SetPolicy preserves existing parameters when config is omitted.
func SetPolicy(ctx context.Context, routeID uint, enabled bool, config *edgeconfig.PoWConfig) error {
	value := edgeconfig.DefaultPoWConfig()
	if config != nil {
		value = *config
	}
	if err := edgeconfig.ValidatePoWConfig(value); err != nil {
		return &ValidationError{Err: err}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return repository.SetOpenFlarePoWPolicy(ctx, model.OpenFlarePoWPolicy{RouteID: routeID, Enabled: enabled, Config: string(raw)}, config == nil)
}

// ValidationError carries safe policy validation feedback.
type ValidationError struct{ Err error }

func (e *ValidationError) Error() string { return e.Err.Error() }
