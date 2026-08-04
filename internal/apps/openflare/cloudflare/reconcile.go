// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	memberLockStripeCount  = 64
	memberLastErrorColumn  = "last_error"
	memberSyncStatusColumn = "sync_status"
)

var memberLocks [memberLockStripeCount]sync.Mutex

// ReconcileMember makes one Cloudflare A record match the local desired state.
func ReconcileMember(ctx context.Context, memberID uint) error {
	lock := &memberLocks[memberID%memberLockStripeCount]
	lock.Lock()
	defer lock.Unlock()

	if err := repository.UpdateCFPointingMemberColumns(ctx, memberID, map[string]any{memberSyncStatusColumn: model.CFMemberSyncing, memberLastErrorColumn: ""}); err != nil {
		return err
	}
	if err := reconcileMember(ctx, memberID); err != nil {
		if updateErr := repository.UpdateCFPointingMemberColumns(ctx, memberID, map[string]any{memberSyncStatusColumn: model.CFMemberSyncError, memberLastErrorColumn: err.Error()}); updateErr != nil {
			logger.ErrorF(ctx, "[Cloudflare] persist member sync error failed: member_id=%d error=%v", memberID, updateErr)
		}
		return err
	}
	return nil
}

func reconcileMember(ctx context.Context, memberID uint) error {
	state, err := repository.GetCFPointingMemberContext(ctx, memberID)
	if err != nil {
		return err
	}
	if !state.Group.Enabled {
		return errors.New(errGroupDisabled)
	}
	ip := strings.TrimSpace(state.Node.IP)
	if net.ParseIP(ip).To4() == nil {
		return errors.New(errNodeIPv4Required)
	}
	connection, err := repository.GetCFConnection(ctx)
	if err != nil || connection.Status != model.CFConnectionStatusReady {
		return errors.New(errConnectionNotConfigured)
	}
	token, err := resolveToken(ctx, connection)
	if err != nil {
		return err
	}
	client := clientFactory(token)
	zoneID := state.Member.CFZoneID
	if zoneID == "" {
		zone, findErr := client.FindZone(ctx, state.Zone.Domain)
		if findErr != nil {
			return findErr
		}
		zoneID = zone.ID
	}
	input := RecordInput{Type: "A", Name: state.Domain.Domain, Content: ip, Proxied: state.Member.Proxied, TTL: 300}
	if input.Proxied {
		input.TTL = 1
	}
	recordID := state.Member.CFRecordID
	if recordID != "" {
		if _, getErr := client.GetRecord(ctx, zoneID, recordID); getErr == nil {
			record, updateErr := client.UpdateARecord(ctx, zoneID, recordID, input)
			if updateErr != nil {
				return updateErr
			}
			return markMemberSynced(ctx, memberID, zoneID, record.ID, ip)
		}
	}
	records, err := client.ListARecords(ctx, zoneID, state.Domain.Domain)
	if err != nil {
		return err
	}
	var record *DNSRecord
	switch len(records) {
	case 0:
		record, err = client.CreateARecord(ctx, zoneID, input)
	case 1:
		record, err = client.UpdateARecord(ctx, zoneID, records[0].ID, input)
	default:
		return errors.New(errMultipleARecords)
	}
	if err != nil {
		return err
	}
	return markMemberSynced(ctx, memberID, zoneID, record.ID, ip)
}

func markMemberSynced(ctx context.Context, memberID uint, zoneID, recordID, ip string) error {
	now := time.Now()
	return repository.UpdateCFPointingMemberColumns(ctx, memberID, map[string]any{
		"cf_zone_id": zoneID, "cf_record_id": recordID, "desired_ip": ip,
		memberSyncStatusColumn: model.CFMemberSyncOK, memberLastErrorColumn: "", "synced_at": &now,
	})
}

// DeleteManagedRecord deletes the cached or uniquely discoverable A record.
func DeleteManagedRecord(ctx context.Context, memberID uint) error {
	state, err := repository.GetCFPointingMemberContext(ctx, memberID)
	if err != nil {
		return err
	}
	connection, err := repository.GetCFConnection(ctx)
	if err != nil {
		return err
	}
	token, err := resolveToken(ctx, connection)
	if err != nil {
		return err
	}
	client := clientFactory(token)
	zoneID := state.Member.CFZoneID
	if zoneID == "" {
		zone, findErr := client.FindZone(ctx, state.Zone.Domain)
		if findErr != nil {
			return findErr
		}
		zoneID = zone.ID
	}
	if state.Member.CFRecordID != "" {
		if deleteErr := client.DeleteRecord(ctx, zoneID, state.Member.CFRecordID); deleteErr == nil {
			return nil
		}
	}
	records, err := client.ListARecords(ctx, zoneID, state.Domain.Domain)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	if len(records) > 1 {
		return errors.New(errMultipleARecords)
	}
	return client.DeleteRecord(ctx, zoneID, records[0].ID)
}
