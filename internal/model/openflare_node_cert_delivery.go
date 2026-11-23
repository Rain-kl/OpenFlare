// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import "time"

// OpenFlareNodeCertDelivery records when a TLS certificate's private key was delivered to an edge node's memory.
type OpenFlareNodeCertDelivery struct {
	ID               uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID           string    `json:"node_id" gorm:"size:64;not null;uniqueIndex:idx_of_node_cert_deliveries_unique;index:idx_of_node_cert_deliveries_node_id"`
	CertificateID    uint      `json:"certificate_id" gorm:"not null;uniqueIndex:idx_of_node_cert_deliveries_unique;index:idx_of_node_cert_deliveries_cert_id"`
	CertSHA256       string    `json:"cert_sha256" gorm:"size:64;not null;default:'';uniqueIndex:idx_of_node_cert_deliveries_unique"`
	ReleaseSequence  int64     `json:"release_sequence" gorm:"not null;default:0"`
	DeliveryCount    int64     `json:"delivery_count" gorm:"not null;default:1"`
	FirstDeliveredAt time.Time `json:"first_delivered_at" gorm:"not null"`
	LastDeliveredAt  time.Time `json:"last_delivered_at" gorm:"not null"`
}

// TableName returns the GORM table name.
func (OpenFlareNodeCertDelivery) TableName() string {
	return "of_node_cert_deliveries"
}
