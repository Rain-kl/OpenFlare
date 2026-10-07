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
	"gorm.io/gorm"
)

const (
	memberLockStripeCount  = 64
	memberLastErrorColumn  = "last_error"
	memberSyncStatusColumn = "sync_status"
)

var errCachedRecordUnavailable = errors.New("cached Cloudflare record unavailable")

var memberLocks [memberLockStripeCount]sync.Mutex

// ReconcileMember makes one Cloudflare DNS record match the local desired state.
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
	input, err := desiredRecordInput(state)
	if err != nil {
		return err
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
	input.Name = state.Domain.Domain
	input.Proxied = state.Member.Proxied
	input.TTL = 300
	if input.Proxied {
		input.TTL = 1
	}
	recordID := state.Member.CFRecordID
	if recordID != "" {
		completed, cachedErr := reconcileCachedRecord(ctx, client, memberID, zoneID, recordID, input)
		if errors.Is(cachedErr, errCachedRecordUnavailable) {
			cachedErr = nil
		}
		if cachedErr != nil {
			return cachedErr
		}
		if completed {
			return nil
		}
	}
	records, err := client.ListRecords(ctx, zoneID, state.Domain.Domain, input.Type)
	if err != nil {
		return err
	}
	var record *DNSRecord
	switch len(records) {
	case 0:
		record, err = client.CreateRecord(ctx, zoneID, input)
	case 1:
		record, err = client.UpdateRecord(ctx, zoneID, records[0].ID, input)
	default:
		return errors.New(errMultipleDNSRecords)
	}
	if err != nil {
		return err
	}
	return markMemberSynced(ctx, memberID, zoneID, record.ID, input.Content)
}

func reconcileCachedRecord(
	ctx context.Context,
	client Client,
	memberID uint,
	zoneID, recordID string,
	input RecordInput,
) (bool, error) {
	current, err := client.GetRecord(ctx, zoneID, recordID)
	if err != nil {
		return false, errors.Join(errCachedRecordUnavailable, err)
	}
	if current.Type != input.Type {
		return false, client.DeleteRecord(ctx, zoneID, recordID)
	}
	record, err := client.UpdateRecord(ctx, zoneID, recordID, input)
	if err != nil {
		return false, err
	}
	return true, markMemberSynced(ctx, memberID, zoneID, record.ID, input.Content)
}

func desiredRecordInput(state *repository.CFPointingMemberContext) (RecordInput, error) {
	if state.Group.TargetMode == model.CFPointingTargetModeCustom {
		recordType := strings.ToUpper(strings.TrimSpace(state.Group.RecordType))
		content := strings.TrimSpace(state.Group.RecordContent)
		if !validCustomRecordType(recordType) || !validCustomRecordContent(recordType, content) {
			return RecordInput{}, errors.New(errRecordContentInvalid)
		}
		return RecordInput{Type: recordType, Content: content}, nil
	}
	ip := strings.TrimSpace(state.Node.IP)
	if net.ParseIP(ip).To4() == nil {
		return RecordInput{}, errors.New(errNodeIPv4Required)
	}
	return RecordInput{Type: "A", Content: ip}, nil
}

func markMemberSynced(ctx context.Context, memberID uint, zoneID, recordID, ip string) error {
	now := time.Now()
	return repository.UpdateCFPointingMemberColumns(ctx, memberID, map[string]any{
		"cf_zone_id": zoneID, "cf_record_id": recordID, "desired_ip": ip,
		memberSyncStatusColumn: model.CFMemberSyncOK, memberLastErrorColumn: "", "synced_at": &now,
	})
}

// DeleteManagedRecord deletes the cached or uniquely discoverable managed record.
func DeleteManagedRecord(ctx context.Context, memberID uint) error {
	state, err := repository.GetCFPointingMemberContext(ctx, memberID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
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
	recordType := "A"
	if state.Group.TargetMode == model.CFPointingTargetModeCustom {
		recordType = state.Group.RecordType
		if recordType == "" {
			recordType = "A"
		}
	}
	records, err := client.ListRecords(ctx, zoneID, state.Domain.Domain, recordType)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	if len(records) > 1 {
		return errors.New(errMultipleDNSRecords)
	}
	return client.DeleteRecord(ctx, zoneID, records[0].ID)
}
