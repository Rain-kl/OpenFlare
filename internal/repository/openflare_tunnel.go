// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ListTunnelRelayPools returns all relay pools ordered by default first then id asc.
func ListTunnelRelayPools(ctx context.Context) ([]model.TunnelRelayPool, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var pools []model.TunnelRelayPool
	if err := conn.Order("is_default desc, id asc").Find(&pools).Error; err != nil {
		return nil, err
	}
	return pools, nil
}

// GetTunnelRelayPoolByID returns a relay pool by primary key.
func GetTunnelRelayPoolByID(ctx context.Context, id uint) (*model.TunnelRelayPool, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var pool model.TunnelRelayPool
	if err := conn.First(&pool, id).Error; err != nil {
		return nil, err
	}
	return &pool, nil
}

// GetTunnelRelayPoolByKey returns a relay pool by pool_key.
func GetTunnelRelayPoolByKey(ctx context.Context, key string) (*model.TunnelRelayPool, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var pool model.TunnelRelayPool
	if err := conn.Where("pool_key = ?", key).First(&pool).Error; err != nil {
		return nil, err
	}
	return &pool, nil
}

// GetDefaultTunnelRelayPool returns the default relay pool.
func GetDefaultTunnelRelayPool(ctx context.Context) (*model.TunnelRelayPool, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var pool model.TunnelRelayPool
	err := conn.Where("is_default = ?", true).First(&pool).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = conn.Order("id asc").First(&pool).Error
	}
	if err != nil {
		return nil, err
	}
	return &pool, nil
}

// CreateTunnelRelayPool inserts a new relay pool.
func CreateTunnelRelayPool(ctx context.Context, pool *model.TunnelRelayPool) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if pool.IsDefault {
			if err := tx.Model(&model.TunnelRelayPool{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(pool).Error
	})
}

// UpdateTunnelRelayPoolFields updates selected fields of a relay pool.
func UpdateTunnelRelayPoolFields(ctx context.Context, pool *model.TunnelRelayPool, fields ...string) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if pool.IsDefault {
			if err := tx.Model(&model.TunnelRelayPool{}).Where("id <> ? AND is_default = ?", pool.ID, true).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		if len(fields) == 0 {
			return tx.Save(pool).Error
		}
		return tx.Model(pool).Select(fields).Updates(pool).Error
	})
}

// DeleteTunnelRelayPool deletes a relay pool by ID.
func DeleteTunnelRelayPool(ctx context.Context, id uint) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Delete(&model.TunnelRelayPool{}, id).Error
}

// CountTunnelRelaysInPool counts relays assigned to a pool.
func CountTunnelRelaysInPool(ctx context.Context, poolID uint) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}
	var count int64
	err := conn.Model(&model.TunnelRelay{}).Where("pool_id = ?", poolID).Count(&count).Error
	return count, err
}

// CountTunnelsInPool counts logical tunnels assigned to a pool.
func CountTunnelsInPool(ctx context.Context, poolID uint) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}
	var count int64
	err := conn.Model(&model.Tunnel{}).Where("pool_id = ?", poolID).Count(&count).Error
	return count, err
}

// ListTunnelRelays returns all tunnel relays ordered by id desc.
func ListTunnelRelays(ctx context.Context) ([]model.TunnelRelay, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var relays []model.TunnelRelay
	if err := conn.Order("id desc").Find(&relays).Error; err != nil {
		return nil, err
	}
	return relays, nil
}

// GetTunnelRelayByID returns a tunnel relay by primary key.
func GetTunnelRelayByID(ctx context.Context, id uint) (*model.TunnelRelay, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var relay model.TunnelRelay
	if err := conn.First(&relay, id).Error; err != nil {
		return nil, err
	}
	return &relay, nil
}

// GetTunnelRelayByRelayID returns a tunnel relay by relay_id.
func GetTunnelRelayByRelayID(ctx context.Context, relayID string) (*model.TunnelRelay, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var relay model.TunnelRelay
	if err := conn.Where("relay_id = ?", relayID).First(&relay).Error; err != nil {
		return nil, err
	}
	return &relay, nil
}

// GetTunnelRelayByAccessToken returns a tunnel relay by access_token.
func GetTunnelRelayByAccessToken(ctx context.Context, token string) (*model.TunnelRelay, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var relay model.TunnelRelay
	if err := conn.Where("access_token = ?", token).First(&relay).Error; err != nil {
		return nil, err
	}
	return &relay, nil
}

// CreateTunnelRelay inserts a new tunnel relay.
func CreateTunnelRelay(ctx context.Context, relay *model.TunnelRelay) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Create(relay).Error
}

// CreateOrGetDiscoveryTunnelRelay idempotently creates or loads a discovery-registered relay.
func CreateOrGetDiscoveryTunnelRelay(ctx context.Context, relay *model.TunnelRelay) (*model.TunnelRelay, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	if err := conn.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "relay_id"}},
		DoNothing: true,
	}).Create(relay).Error; err != nil {
		return nil, err
	}
	return GetTunnelRelayByRelayID(ctx, relay.RelayID)
}

// UpdateTunnelRelayFields updates selected fields of a tunnel relay.
func UpdateTunnelRelayFields(ctx context.Context, relay *model.TunnelRelay, fields ...string) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	if len(fields) == 0 {
		return conn.Save(relay).Error
	}
	return conn.Model(relay).Select(fields).Updates(relay).Error
}

// UpdateTunnelRelayColumns updates relay columns from a map.
func UpdateTunnelRelayColumns(ctx context.Context, relay *model.TunnelRelay, changes map[string]any) error {
	if relay == nil || len(changes) == 0 {
		return nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Model(relay).Updates(changes).Error
}

// DeleteTunnelRelay deletes a relay and its attachments in one transaction.
func DeleteTunnelRelay(ctx context.Context, id uint) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("relay_id = ?", id).Delete(&model.TunnelAttachment{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.TunnelService{}).Where("relay_id = ?", id).Update("relay_id", 0).Error; err != nil {
			return err
		}
		return tx.Delete(&model.TunnelRelay{}, id).Error
	})
}

// ListAvailableRelaysByPool returns candidate relays for a pool (or all pools when poolID == 0)
// that are non-quarantined, non-draining, and seen after minSeenAt.
func ListAvailableRelaysByPool(ctx context.Context, poolID uint, minSeenAt time.Time) ([]model.TunnelRelay, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	q := conn.Where("quarantined_at IS NULL AND draining = ? AND last_seen_at IS NOT NULL AND last_seen_at >= ?", false, minSeenAt)
	if poolID > 0 {
		q = q.Where("pool_id = ?", poolID)
	}
	var relays []model.TunnelRelay
	if err := q.Order("weight desc, id asc").Find(&relays).Error; err != nil {
		return nil, err
	}
	return relays, nil
}

// ListTunnels returns all logical tunnels ordered by id desc.
func ListTunnels(ctx context.Context) ([]model.Tunnel, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var tunnels []model.Tunnel
	if err := conn.Order("id desc").Find(&tunnels).Error; err != nil {
		return nil, err
	}
	return tunnels, nil
}

// GetTunnelByID returns a tunnel by primary key.
func GetTunnelByID(ctx context.Context, id uint) (*model.Tunnel, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var tunnel model.Tunnel
	if err := conn.First(&tunnel, id).Error; err != nil {
		return nil, err
	}
	return &tunnel, nil
}

// GetTunnelByTunnelID returns a tunnel by tunnel_id.
func GetTunnelByTunnelID(ctx context.Context, tunnelID string) (*model.Tunnel, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var tunnel model.Tunnel
	if err := conn.Where("tunnel_id = ?", tunnelID).First(&tunnel).Error; err != nil {
		return nil, err
	}
	return &tunnel, nil
}

// GetTunnelByAccessToken returns a tunnel by access_token.
func GetTunnelByAccessToken(ctx context.Context, token string) (*model.Tunnel, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var tunnel model.Tunnel
	if err := conn.Where("access_token = ?", token).First(&tunnel).Error; err != nil {
		return nil, err
	}
	return &tunnel, nil
}

// CreateTunnel inserts a new logical tunnel.
func CreateTunnel(ctx context.Context, tunnel *model.Tunnel) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Create(tunnel).Error
}

// UpdateTunnelFields updates selected fields of a tunnel.
func UpdateTunnelFields(ctx context.Context, tunnel *model.Tunnel, fields ...string) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	if len(fields) == 0 {
		return conn.Save(tunnel).Error
	}
	return conn.Model(tunnel).Select(fields).Updates(tunnel).Error
}

// UpdateTunnelColumns updates tunnel columns from a map.
func UpdateTunnelColumns(ctx context.Context, tunnel *model.Tunnel, changes map[string]any) error {
	if tunnel == nil || len(changes) == 0 {
		return nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Model(tunnel).Updates(changes).Error
}

// IncrementTunnelConfigRevision bumps config_revision atomically so connectors pull updated ingress rules.
func IncrementTunnelConfigRevision(ctx context.Context, tunnelID uint) error {
	if tunnelID == 0 {
		return nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Model(&model.Tunnel{}).
		Where("id = ?", tunnelID).
		UpdateColumn("config_revision", gorm.Expr("config_revision + 1")).Error
}

// CountProxyRoutesByTunnelID counts proxy routes referencing a tunnel.
func CountProxyRoutesByTunnelID(ctx context.Context, tunnelID uint) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}
	var count int64
	err := conn.Model(&model.ProxyRoute{}).
		Where("upstream_type = ? AND tunnel_id = ?", "tunnel", tunnelID).
		Count(&count).Error
	return count, err
}

// DeleteTunnel deletes a tunnel, its connectors, attachments, and L4 services in one transaction.
func DeleteTunnel(ctx context.Context, id uint) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tunnel_id = ?", id).Delete(&model.TunnelAttachment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tunnel_id = ?", id).Delete(&model.TunnelConnector{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tunnel_id = ?", id).Delete(&model.TunnelService{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Tunnel{}, id).Error
	})
}

// ListTunnelConnectors returns all connectors (optionally filtered by tunnelID when > 0).
func ListTunnelConnectors(ctx context.Context, tunnelID uint) ([]model.TunnelConnector, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	q := conn.Order("last_seen_at desc, id desc")
	if tunnelID > 0 {
		q = q.Where("tunnel_id = ?", tunnelID)
	}
	var connectors []model.TunnelConnector
	if err := q.Find(&connectors).Error; err != nil {
		return nil, err
	}
	return connectors, nil
}

// GetTunnelConnectorByID returns a connector by primary key.
func GetTunnelConnectorByID(ctx context.Context, id uint) (*model.TunnelConnector, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var connector model.TunnelConnector
	if err := conn.First(&connector, id).Error; err != nil {
		return nil, err
	}
	return &connector, nil
}

// GetTunnelConnectorByConnectorID returns a connector by connector_id.
func GetTunnelConnectorByConnectorID(ctx context.Context, connectorID string) (*model.TunnelConnector, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var connector model.TunnelConnector
	if err := conn.Where("connector_id = ?", connectorID).First(&connector).Error; err != nil {
		return nil, err
	}
	return &connector, nil
}

// UpsertTunnelConnector inserts or updates a connector record by connector_id.
func UpsertTunnelConnector(ctx context.Context, connector *model.TunnelConnector) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "connector_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"tunnel_id",
			"hostname",
			"os_arch",
			"public_ip",
			"version",
			"engine_version",
			"contract_version",
			"capabilities_json",
			"applied_revision",
			"status",
			"last_error",
			"connected_relays_json",
			"last_seen_at",
			"updated_at",
		}),
	}).Create(connector).Error
}

// UpdateTunnelConnectorColumns updates connector columns by ID.
func UpdateTunnelConnectorColumns(ctx context.Context, id uint, changes map[string]any) error {
	if len(changes) == 0 {
		return nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Model(&model.TunnelConnector{}).Where("id = ?", id).Updates(changes).Error
}

// CountActiveTunnelConnectors counts non-revoked connectors seen after minSeenAt for a tunnel.
func CountActiveTunnelConnectors(ctx context.Context, tunnelID uint, minSeenAt time.Time, excludeConnectorID string) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}
	q := conn.Model(&model.TunnelConnector{}).
		Where("tunnel_id = ? AND revoked_at IS NULL AND last_seen_at IS NOT NULL AND last_seen_at >= ?", tunnelID, minSeenAt)
	if excludeConnectorID != "" {
		q = q.Where("connector_id <> ?", excludeConnectorID)
	}
	var count int64
	err := q.Count(&count).Error
	return count, err
}

// ListTunnelAttachments returns attachments filtered by optional tunnelID, relayID, or connectorID.
func ListTunnelAttachments(ctx context.Context, tunnelID uint, relayID uint, connectorID string) ([]model.TunnelAttachment, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	q := conn.Order("rtt_ms asc, id asc")
	if tunnelID > 0 {
		q = q.Where("tunnel_id = ?", tunnelID)
	}
	if relayID > 0 {
		q = q.Where("relay_id = ?", relayID)
	}
	if connectorID != "" {
		q = q.Where("connector_id = ?", connectorID)
	}
	var attachments []model.TunnelAttachment
	if err := q.Find(&attachments).Error; err != nil {
		return nil, err
	}
	return attachments, nil
}

// ListActiveVerifiedAttachments returns relay-verified attachments whose lease has not expired.
func ListActiveVerifiedAttachments(ctx context.Context, now time.Time) ([]model.TunnelAttachment, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var attachments []model.TunnelAttachment
	if err := conn.
		Where("state IN ? AND lease_expires_at IS NOT NULL AND lease_expires_at > ?",
			[]string{model.TunnelAttachmentStateActive, model.TunnelAttachmentStateDraining},
			now,
		).
		Order("tunnel_id asc, rtt_ms asc, id asc").
		Find(&attachments).Error; err != nil {
		return nil, err
	}
	return attachments, nil
}

// ListRelayAuthorizedAttachments returns non-stale, non-expired attachments for a specific relay.
func ListRelayAuthorizedAttachments(ctx context.Context, relayID uint, now time.Time) ([]model.TunnelAttachment, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var attachments []model.TunnelAttachment
	if err := conn.
		Where("relay_id = ? AND state IN ? AND (lease_expires_at IS NULL OR lease_expires_at > ?)",
			relayID,
			[]string{
				model.TunnelAttachmentStateConnecting,
				model.TunnelAttachmentStateActive,
				model.TunnelAttachmentStateDraining,
			},
			now,
		).
		Order("id asc").
		Find(&attachments).Error; err != nil {
		return nil, err
	}
	return attachments, nil
}

// ReconcileConnectorAttachments upserts negotiated attachments for a connector and marks deselected ones draining.
func ReconcileConnectorAttachments(ctx context.Context, connectorID string, desired []model.TunnelAttachment) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		keepRelayIDs := make([]uint, 0, len(desired))
		for i := range desired {
			item := &desired[i]
			keepRelayIDs = append(keepRelayIDs, item.RelayID)
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "connector_id"}, {Name: "relay_id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"tunnel_id",
					"service_name",
					"session_token",
					"transport",
					"state",
					"rtt_ms",
					"lease_expires_at",
					"updated_at",
				}),
			}).Create(item).Error; err != nil {
				return err
			}
		}

		q := tx.Model(&model.TunnelAttachment{}).Where("connector_id = ?", connectorID)
		if len(keepRelayIDs) > 0 {
			q = q.Where("relay_id NOT IN ?", keepRelayIDs)
		}
		return q.Updates(map[string]any{
			"state":      model.TunnelAttachmentStateDraining,
			"updated_at": time.Now().UTC(),
		}).Error
	})
}

// VerifiedAttachmentProof is a relay-reported liveness proof for an active connector tunnel.
type VerifiedAttachmentProof struct {
	ConnectorID   string
	ServiceName   string
	RTTMs         int
	ActiveStreams int
}

// ReconcileRelayVerifiedAttachments renews leases for relay-verified attachments and marks missing active ones stale.
// It returns true when the set of active routing attachments changed.
func ReconcileRelayVerifiedAttachments(
	ctx context.Context,
	relayID uint,
	proofs []VerifiedAttachmentProof,
	now time.Time,
	leaseTTL time.Duration,
) (bool, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return false, errors.New(errDatabaseNotInitialized)
	}
	changed := false
	err := conn.Transaction(func(tx *gorm.DB) error {
		var existing []model.TunnelAttachment
		if err := tx.Where("relay_id = ?", relayID).Find(&existing).Error; err != nil {
			return err
		}
		byConnector := make(map[string]model.TunnelAttachment, len(existing))
		for _, att := range existing {
			byConnector[att.ConnectorID] = att
		}

		expiresAt := now.Add(leaseTTL)
		verifiedConnectors := make(map[string]struct{}, len(proofs))
		for _, proof := range proofs {
			verifiedConnectors[proof.ConnectorID] = struct{}{}
			att, ok := byConnector[proof.ConnectorID]
			if !ok {
				continue
			}
			if att.State != model.TunnelAttachmentStateActive {
				changed = true
			}
			nextState := att.State
			if nextState == model.TunnelAttachmentStateConnecting || nextState == model.TunnelAttachmentStateStale {
				nextState = model.TunnelAttachmentStateActive
			}
			if err := tx.Model(&model.TunnelAttachment{}).Where("id = ?", att.ID).Updates(map[string]any{
				"state":             nextState,
				"rtt_ms":            proof.RTTMs,
				"active_streams":    proof.ActiveStreams,
				"relay_verified_at": now,
				"lease_expires_at":  expiresAt,
				"updated_at":        now,
			}).Error; err != nil {
				return err
			}
		}

		for _, att := range existing {
			if _, ok := verifiedConnectors[att.ConnectorID]; ok {
				continue
			}
			if att.State == model.TunnelAttachmentStateActive || att.State == model.TunnelAttachmentStateDraining {
				changed = true
				if err := tx.Model(&model.TunnelAttachment{}).Where("id = ?", att.ID).Updates(map[string]any{
					"state":      model.TunnelAttachmentStateStale,
					"updated_at": now,
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	return changed, err
}

// DeleteTunnelAttachmentsByConnector removes all attachments belonging to a connector.
func DeleteTunnelAttachmentsByConnector(ctx context.Context, connectorID string) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Where("connector_id = ?", connectorID).Delete(&model.TunnelAttachment{}).Error
}

// DeleteTunnelAttachmentsByTunnel removes all attachments belonging to a tunnel.
func DeleteTunnelAttachmentsByTunnel(ctx context.Context, tunnelID uint) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Where("tunnel_id = ?", tunnelID).Delete(&model.TunnelAttachment{}).Error
}

// ListTunnelServices returns L4 services filtered by optional tunnelID or relayID.
func ListTunnelServices(ctx context.Context, tunnelID uint, relayID uint) ([]model.TunnelService, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	q := conn.Order("id asc")
	if tunnelID > 0 {
		q = q.Where("tunnel_id = ?", tunnelID)
	}
	if relayID > 0 {
		q = q.Where("relay_id = ?", relayID)
	}
	var services []model.TunnelService
	if err := q.Find(&services).Error; err != nil {
		return nil, err
	}
	return services, nil
}

// GetTunnelServiceByID returns a tunnel L4 service by ID.
func GetTunnelServiceByID(ctx context.Context, id uint) (*model.TunnelService, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var service model.TunnelService
	if err := conn.First(&service, id).Error; err != nil {
		return nil, err
	}
	return &service, nil
}

// CreateTunnelService inserts a new tunnel L4 service.
func CreateTunnelService(ctx context.Context, service *model.TunnelService) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Create(service).Error
}

// UpdateTunnelServiceFields updates selected fields of a tunnel L4 service.
func UpdateTunnelServiceFields(ctx context.Context, service *model.TunnelService, fields ...string) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	if len(fields) == 0 {
		return conn.Save(service).Error
	}
	return conn.Model(service).Select(fields).Updates(service).Error
}

// DeleteTunnelService deletes a tunnel L4 service by ID.
func DeleteTunnelService(ctx context.Context, id uint) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Delete(&model.TunnelService{}, id).Error
}
