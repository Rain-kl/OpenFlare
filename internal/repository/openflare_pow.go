// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm/clause"
)

// ListOpenFlarePoWPolicies returns independently configured site policies.
func ListOpenFlarePoWPolicies(ctx context.Context) ([]model.OpenFlarePoWPolicy, error) {
	var policies []model.OpenFlarePoWPolicy
	err := db.DB(ctx).Order("route_id").Find(&policies).Error
	return policies, err
}

// SaveOpenFlarePoWPolicy replaces one site's policy.
func SaveOpenFlarePoWPolicy(ctx context.Context, policy model.OpenFlarePoWPolicy) error {
	return db.DB(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "route_id"}}, DoUpdates: clause.AssignmentColumns([]string{"enabled", "config"})}).Create(&policy).Error
}
