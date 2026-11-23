// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UpsertNodeCertDeliveries persists memory-key delivery audit records for an edge node.
func UpsertNodeCertDeliveries(ctx context.Context, deliveries []model.OpenFlareNodeCertDelivery) error {
	if len(deliveries) == 0 {
		return nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "node_id"},
			{Name: "certificate_id"},
			{Name: "cert_sha256"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"release_sequence":  gorm.Expr("excluded.release_sequence"),
			"last_delivered_at": gorm.Expr("excluded.last_delivered_at"),
			"delivery_count":    gorm.Expr("of_node_cert_deliveries.delivery_count + 1"),
		}),
	}).Create(&deliveries).Error
}

// ListNodeCertDeliveriesByNodeID returns all certificate delivery audit rows for one node.
func ListNodeCertDeliveriesByNodeID(ctx context.Context, nodeID string) ([]model.OpenFlareNodeCertDelivery, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var deliveries []model.OpenFlareNodeCertDelivery
	if err := conn.Where("node_id = ?", nodeID).Order("last_delivered_at desc, id desc").Find(&deliveries).Error; err != nil {
		return nil, err
	}
	return deliveries, nil
}

// ListNodeCertDeliveriesByCertificateID returns all node delivery audit rows for one certificate.
func ListNodeCertDeliveriesByCertificateID(ctx context.Context, certID uint) ([]model.OpenFlareNodeCertDelivery, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var deliveries []model.OpenFlareNodeCertDelivery
	if err := conn.Where("certificate_id = ?", certID).Order("last_delivered_at desc, id desc").Find(&deliveries).Error; err != nil {
		return nil, err
	}
	return deliveries, nil
}
