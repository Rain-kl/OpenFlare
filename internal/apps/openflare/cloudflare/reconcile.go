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

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/dnsaccount"
	"github.com/Rain-kl/Wavelet/internal/openflare/dnsprovider"

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
	target, err := resolveMemberDNS(ctx, state)
	if err != nil {
		return err
	}
	client, zoneID := target.client, target.zoneID
	input.Name = state.Domain.Domain
	input.Proxied = state.Member.Proxied
	if !target.supportsProxy || state.Group.TargetMode == model.CFPointingTargetModeSmartDNS {
		input.Proxied = false
	}
	input.TTL = groupTTL(&state.Group)
	if input.Proxied {
		input.TTL = automaticTTL
	}
	recordID := state.Member.CFRecordID
	if recordID != "" && sameDNSOwner(&state.Member, target) {
		completed, cachedErr := reconcileCachedRecord(ctx, target, memberID, recordID, input)
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
	return markMemberSynced(ctx, memberID, target, record.ID, input.Content)
}

func reconcileCachedRecord(
	ctx context.Context,
	target memberDNSTarget,
	memberID uint,
	recordID string,
	input RecordInput,
) (bool, error) {
	client, zoneID := target.client, target.zoneID
	current, err := client.GetRecord(ctx, zoneID, recordID)
	if err != nil {
		return false, errors.Join(errCachedRecordUnavailable, err)
	}
	if current.Name != "" && !strings.EqualFold(strings.TrimSuffix(current.Name, "."), strings.TrimSuffix(input.Name, ".")) {
		return false, errCachedRecordUnavailable
	}
	if current.Type != input.Type {
		return false, client.DeleteRecord(ctx, zoneID, recordID)
	}
	record, err := client.UpdateRecord(ctx, zoneID, recordID, input)
	if err != nil {
		return false, err
	}
	return true, markMemberSynced(ctx, memberID, target, record.ID, input.Content)
}

func desiredRecordInput(state *repository.CFPointingMemberContext) (RecordInput, error) {
	if state.Group.TargetMode == model.CFPointingTargetModeSmartDNS {
		host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(state.Group.SmartDNSTargetHost)), ".")
		if !validCNAMEContent(host) {
			return RecordInput{}, errors.New(errSmartDNSHostInvalid)
		}
		return RecordInput{Type: recordTypeCNAME, Content: host}, nil
	}
	if state.Group.TargetMode == model.CFPointingTargetModeCustom {
		recordType := strings.ToUpper(strings.TrimSpace(state.Group.RecordType))
		content := strings.TrimSpace(state.Group.RecordContent)
		if !validCustomRecordType(recordType) || !validCustomRecordContent(recordType, content) {
			return RecordInput{}, errors.New(errRecordContentInvalid)
		}
		return RecordInput{Type: recordType, Content: content}, nil
	}
	ip := strings.TrimSpace(state.Node.IP)
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil || parsed.IsUnspecified() {
		return RecordInput{}, errors.New(errNodeIPv4Required)
	}
	return RecordInput{Type: "A", Content: ip}, nil
}

func markMemberSynced(ctx context.Context, memberID uint, target memberDNSTarget, recordID, ip string) error {
	now := time.Now()
	columns := map[string]any{
		"cf_zone_id": target.zoneID, "cf_record_id": recordID, "dns_account_id": target.accountID, "desired_ip": ip,
		memberSyncStatusColumn: model.CFMemberSyncOK, memberLastErrorColumn: "", "synced_at": &now,
	}
	if !target.supportsProxy {
		columns["proxied"] = false
	}
	return repository.UpdateCFPointingMemberColumns(ctx, memberID, columns)
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
	target, err := resolveMemberDNS(ctx, state)
	if state.Member.DNSAccountID != nil && state.Member.CFRecordID != "" && (err != nil || !sameDNSOwner(&state.Member, target)) {
		account, accountErr := repository.GetDNSAccountByID(ctx, *state.Member.DNSAccountID)
		if accountErr != nil {
			return accountErr
		}
		client, accountErr := dnsaccount.OpenAdapter(account)
		if accountErr != nil {
			return accountErr
		}
		target = memberDNSTarget{client: client, accountID: &account.ID, zoneID: state.Member.CFZoneID}
		err = nil
	}
	if err != nil {
		return err
	}
	client, zoneID := target.client, target.zoneID
	if state.Member.CFRecordID != "" && sameDNSOwner(&state.Member, target) {
		if deleteErr := client.DeleteRecord(ctx, zoneID, state.Member.CFRecordID); deleteErr == nil {
			return nil
		}
	}
	recordType := "A"
	switch state.Group.TargetMode {
	case model.CFPointingTargetModeSmartDNS:
		recordType = recordTypeCNAME
	case model.CFPointingTargetModeCustom:
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

type memberDNSTarget struct {
	client        Client
	accountID     *uint
	zoneID        string
	supportsProxy bool
}

func sameDNSOwner(member *model.CFPointingMember, target memberDNSTarget) bool {
	return equalOptionalUint(member.DNSAccountID, target.accountID) && member.CFZoneID == target.zoneID
}

func resolveMemberDNS(ctx context.Context, state *repository.CFPointingMemberContext) (memberDNSTarget, error) {
	accounts, err := repository.ListDNSAccounts(ctx)
	if err != nil {
		return memberDNSTarget{}, err
	}
	legacy, err := repository.GetCFConnection(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return memberDNSTarget{}, err
	}
	var preferred []uint
	if state.Member.DNSAccountID != nil {
		preferred = append(preferred, *state.Member.DNSAccountID)
	}
	if legacy != nil && legacy.DNSAccountID != nil {
		preferred = append(preferred, *legacy.DNSAccountID)
	}
	route, err := dnsaccount.Resolve(accounts, state.Domain.Domain, false, preferred...)
	if err != nil {
		return memberDNSTarget{}, err
	}
	if route != nil {
		client, openErr := route.Open()
		return memberDNSTarget{client: client, accountID: &route.Account.ID, zoneID: route.Zone.ID, supportsProxy: dnsprovider.SupportsProxy(route.Account.Type)}, openErr
	}
	if legacy == nil || legacy.Status != model.CFConnectionStatusReady {
		return memberDNSTarget{}, errors.New(errDNSProviderScopeUnavailable)
	}
	// A previously verified Cloudflare connection remains usable until its DNS account has been probed.
	if legacy.DNSAccountID != nil {
		for _, account := range accounts {
			if account.ID == *legacy.DNSAccountID && account.LastCheckedAt != nil {
				return memberDNSTarget{}, errors.New(errDNSProviderScopeUnavailable)
			}
		}
	}
	token, err := resolveToken(ctx, legacy)
	if err != nil {
		return memberDNSTarget{}, err
	}
	client := clientFactory(token)
	zoneID := state.Member.CFZoneID
	if zoneID == "" || !equalOptionalUint(state.Member.DNSAccountID, legacy.DNSAccountID) {
		zone, findErr := client.FindZone(ctx, state.Zone.Domain)
		if findErr != nil {
			return memberDNSTarget{}, findErr
		}
		zoneID = zone.ID
	}
	return memberDNSTarget{client: client, accountID: legacy.DNSAccountID, zoneID: zoneID, supportsProxy: true}, nil
}
