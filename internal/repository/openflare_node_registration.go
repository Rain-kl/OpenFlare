// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm/clause"
)

// CreateOrGetOpenFlareDiscoveryNode inserts an enrollment once, without changing an existing identity.
func CreateOrGetOpenFlareDiscoveryNode(ctx context.Context, node *model.OpenFlareNode) (*model.OpenFlareNode, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	if err := conn.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "node_id"}},
		DoNothing: true,
	}).Create(node).Error; err != nil {
		return nil, err
	}
	return GetOpenFlareNodeByNodeID(ctx, node.NodeID)
}
