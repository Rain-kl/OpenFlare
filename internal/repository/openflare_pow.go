// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ListOpenFlarePoWPolicies returns independently configured site policies.
func ListOpenFlarePoWPolicies(ctx context.Context) ([]model.OpenFlarePoWPolicy, error) {
	var policies []model.OpenFlarePoWPolicy
	err := db.DB(ctx).Order("route_id").Find(&policies).Error
	return policies, err
}

// GetOpenFlarePoWPolicy returns a site's stored policy, if configured.
func GetOpenFlarePoWPolicy(ctx context.Context, id uint) (*model.OpenFlarePoWPolicy, error) {
	var policy model.OpenFlarePoWPolicy
	err := db.DB(ctx).Where("route_id = ?", id).Take(&policy).Error
	return &policy, err
}

// SaveOpenFlarePoWPolicy replaces a site's policy.
func SaveOpenFlarePoWPolicy(ctx context.Context, policy model.OpenFlarePoWPolicy) error {
	return SetOpenFlarePoWPolicy(ctx, policy, false)
}

// SetOpenFlarePoWPolicy optionally preserves existing parameters when toggling a policy.
func SetOpenFlarePoWPolicy(ctx context.Context, policy model.OpenFlarePoWPolicy, preserveConfig bool) error {
	return WithProxyRouteTx(ctx, func(tx *gorm.DB) error {
		if _, err := LockProxyRouteTx(tx, policy.RouteID); err != nil {
			return err
		}
		columns := []string{"enabled"}
		if !preserveConfig {
			columns = append(columns, "config")
		}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "route_id"}}, DoUpdates: clause.AssignmentColumns(columns)}).Create(&policy).Error
	})
}
