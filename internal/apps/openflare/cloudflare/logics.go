// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/credential"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
)

var clientFactory = func(token string) Client { return NewHTTPClient(token) }

// SetClientFactoryForTest replaces Cloudflare client construction for tests.
func SetClientFactoryForTest(factory func(string) Client) func() {
	previous := clientFactory
	clientFactory = factory
	return func() { clientFactory = previous }
}

// GetConnection returns the global connection state without its token.
func GetConnection(ctx context.Context) (*ConnectionView, error) {
	item, err := repository.GetCFConnection(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &ConnectionView{}, nil
	}
	if err != nil {
		return nil, err
	}
	return connectionView(item), nil
}

// SaveConnection stores a DNS-account or standalone Cloudflare credential source.
func SaveConnection(ctx context.Context, input ConnectionInput) (*ConnectionView, error) {
	source := strings.TrimSpace(input.Source)
	item := &model.CFConnection{Source: source}
	switch source {
	case model.CFConnectionSourceDNSAccount:
		account, err := repository.GetDNSAccountByID(ctx, input.DNSAccountID)
		if err != nil || !strings.EqualFold(strings.TrimSpace(account.Type), "cloudflare") {
			return nil, errors.New(errDNSAccountInvalid)
		}
		item.DNSAccountID = &account.ID
	case model.CFConnectionSourceStandalone:
		token := strings.TrimSpace(input.APIToken)
		if token == "" {
			return nil, errors.New(errStandaloneInputRequired)
		}
		payload, err := json.Marshal(map[string]string{"api_token": token})
		if err != nil {
			return nil, errors.New(errStandaloneInputInvalid)
		}
		sealed, err := credential.Seal(string(payload))
		if err != nil {
			return nil, errors.New(errStandaloneInputInvalid)
		}
		item.Authorization = sealed
	default:
		return nil, errors.New(errConnectionSourceInvalid)
	}
	if err := repository.UpsertCFConnection(ctx, item); err != nil {
		return nil, err
	}
	return connectionView(item), nil
}

// ClearConnection removes the configured Cloudflare credential.
func ClearConnection(ctx context.Context) error {
	return repository.DeleteCFConnection(ctx)
}

// VerifyConnection verifies and marks the configured token ready.
func VerifyConnection(ctx context.Context) (*ConnectionView, error) {
	item, err := repository.GetCFConnection(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errConnectionNotConfigured)
		}
		return nil, err
	}
	token, err := resolveToken(ctx, item)
	if err != nil {
		return nil, err
	}
	if err = clientFactory(token).VerifyToken(ctx); err != nil {
		item.Status = model.CFConnectionStatusError
		item.VerifiedAt = nil
		if persistErr := repository.UpsertCFConnection(ctx, item); persistErr != nil {
			logger.ErrorF(ctx, "[Cloudflare] persist failed verification status failed: error=%v", persistErr)
		}
		return nil, errors.New(errStandaloneInputInvalid)
	}
	now := time.Now()
	item.Status = model.CFConnectionStatusReady
	item.VerifiedAt = &now
	if err = repository.UpsertCFConnection(ctx, item); err != nil {
		return nil, err
	}
	return connectionView(item), nil
}

func connectionView(item *model.CFConnection) *ConnectionView {
	return &ConnectionView{
		Configured: true,
		Ready:      item.Status == model.CFConnectionStatusReady,
		Source:     item.Source, DNSAccountID: item.DNSAccountID,
		Status: item.Status, VerifiedAt: item.VerifiedAt,
	}
}

func resolveToken(ctx context.Context, item *model.CFConnection) (string, error) {
	if item == nil {
		return "", errors.New(errConnectionNotConfigured)
	}
	stored := item.Authorization
	if item.Source == model.CFConnectionSourceDNSAccount {
		if item.DNSAccountID == nil {
			return "", errors.New(errDNSAccountInvalid)
		}
		account, err := repository.GetDNSAccountByID(ctx, *item.DNSAccountID)
		if err != nil || !strings.EqualFold(strings.TrimSpace(account.Type), "cloudflare") {
			return "", errors.New(errDNSAccountInvalid)
		}
		stored = account.Authorization
	} else if item.Source != model.CFConnectionSourceStandalone {
		return "", errors.New(errConnectionSourceInvalid)
	}
	opened, err := credential.Open(stored)
	if err != nil {
		return "", errors.New(errStandaloneInputInvalid)
	}
	var authorization map[string]string
	if err = json.Unmarshal([]byte(opened), &authorization); err != nil || strings.TrimSpace(authorization["api_token"]) == "" {
		return "", errors.New(errStandaloneInputInvalid)
	}
	return strings.TrimSpace(authorization["api_token"]), nil
}

// ListNodeOptions lists edge nodes selectable by pointing groups.
func ListNodeOptions(ctx context.Context) ([]NodeOption, error) {
	nodes, err := repository.ListOpenFlareNodes(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]NodeOption, 0, len(nodes))
	for _, node := range nodes {
		if node.NodeType != "edge_node" {
			continue
		}
		items = append(items, nodeOption(&node))
	}
	return items, nil
}

// ListGroups returns pointing group summaries.
func ListGroups(ctx context.Context) ([]GroupItem, error) {
	groups, err := repository.ListCFPointingGroups(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]GroupItem, 0, len(groups))
	for i := range groups {
		item, buildErr := buildGroupItem(ctx, &groups[i])
		if buildErr != nil {
			return nil, buildErr
		}
		items = append(items, *item)
	}
	return items, nil
}

// CreateGroup creates a pointing group with its primary node active.
func CreateGroup(ctx context.Context, input GroupInput) (*GroupItem, error) {
	group, err := groupFromInput(ctx, nil, input)
	if err != nil {
		return nil, err
	}
	if err = repository.CreateCFPointingGroup(ctx, group); err != nil {
		return nil, err
	}
	return buildGroupItem(ctx, group)
}

// UpdateGroup updates a pointing group and queues reconciliation when enabled.
func UpdateGroup(ctx context.Context, id uint, input GroupInput) (*GroupItem, error) {
	existing, err := repository.GetCFPointingGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	group, err := groupFromInput(ctx, existing, input)
	if err != nil {
		return nil, err
	}
	if err = repository.SaveCFPointingGroup(ctx, group); err != nil {
		return nil, err
	}
	if err = repository.MarkCFPointingGroupMembersPending(ctx, id); err != nil {
		return nil, err
	}
	if group.Enabled {
		if _, err = DispatchGroupSync(ctx, id, "cloudflare_group_update"); err != nil {
			return nil, errors.New(errTaskDispatchFailed)
		}
	}
	return buildGroupItem(ctx, group)
}

func groupFromInput(ctx context.Context, existing *model.CFPointingGroup, input GroupInput) (*model.CFPointingGroup, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New(errGroupNameRequired)
	}
	if input.BackupNodeID != nil && *input.BackupNodeID == input.PrimaryNodeID {
		return nil, errors.New(errGroupNodeSame)
	}
	primary, err := validEdgeNode(ctx, input.PrimaryNodeID, true)
	if err != nil {
		return nil, err
	}
	if input.BackupNodeID != nil {
		if _, err = validEdgeNode(ctx, *input.BackupNodeID, false); err != nil {
			return nil, err
		}
	}
	if existing == nil {
		existing = &model.CFPointingGroup{}
	}
	existing.Name = name
	existing.PrimaryNodeID = primary.ID
	existing.ActiveNodeID = primary.ID
	existing.BackupNodeID = input.BackupNodeID
	existing.DefaultProxied = input.DefaultProxied
	existing.Enabled = input.Enabled
	return existing, nil
}

func validEdgeNode(ctx context.Context, id uint, requireIPv4 bool) (*model.OpenFlareNode, error) {
	node, err := repository.GetOpenFlareNodeByID(ctx, id)
	if err != nil || node.NodeType != "edge_node" {
		return nil, errors.New(errNodeInvalid)
	}
	if requireIPv4 && net.ParseIP(strings.TrimSpace(node.IP)).To4() == nil {
		return nil, errors.New(errNodeIPv4Required)
	}
	return node, nil
}

func buildGroupItem(ctx context.Context, group *model.CFPointingGroup) (*GroupItem, error) {
	primary, err := repository.GetOpenFlareNodeByID(ctx, group.PrimaryNodeID)
	if err != nil {
		return nil, err
	}
	active, err := repository.GetOpenFlareNodeByID(ctx, group.ActiveNodeID)
	if err != nil {
		return nil, err
	}
	count, err := repository.CountCFPointingMembersByGroupID(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	item := &GroupItem{ID: group.ID, Name: group.Name, PrimaryNode: nodeOption(primary), ActiveNode: nodeOption(active), DefaultProxied: group.DefaultProxied, Enabled: group.Enabled, MemberCount: count, CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt}
	if group.BackupNodeID != nil {
		backup, backupErr := repository.GetOpenFlareNodeByID(ctx, *group.BackupNodeID)
		if backupErr != nil {
			return nil, backupErr
		}
		option := nodeOption(backup)
		item.BackupNode = &option
	}
	return item, nil
}

func nodeOption(node *model.OpenFlareNode) NodeOption {
	return NodeOption{ID: node.ID, Name: node.Name, IP: node.IP}
}

// GetGroup returns a group and its members.
func GetGroup(ctx context.Context, id uint) (*GroupDetail, error) {
	group, err := repository.GetCFPointingGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	item, err := buildGroupItem(ctx, group)
	if err != nil {
		return nil, err
	}
	members, err := listMemberItems(ctx, id)
	if err != nil {
		return nil, err
	}
	return &GroupDetail{Group: *item, Members: members}, nil
}

// CreateMember adds a ZoneDomain and queues its first synchronization.
func CreateMember(ctx context.Context, groupID uint, input MemberCreateInput) (*MemberItem, error) {
	group, err := repository.GetCFPointingGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	domain, err := repository.GetZoneDomainByID(ctx, input.ZoneDomainID)
	if err != nil {
		return nil, err
	}
	if _, err = repository.GetCFPointingMemberByZoneDomainID(ctx, domain.ID); err == nil {
		return nil, errors.New(errMemberExists)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	proxied := group.DefaultProxied
	if input.Proxied != nil {
		proxied = *input.Proxied
	}
	member := &model.CFPointingMember{GroupID: groupID, ZoneDomainID: domain.ID, Proxied: proxied, SyncStatus: model.CFMemberSyncPending}
	if err = repository.CreateCFPointingMember(ctx, member); err != nil {
		return nil, err
	}
	if group.Enabled {
		if _, err = DispatchMemberSync(ctx, member.ID, "cloudflare_member_create"); err != nil {
			return nil, errors.New(errTaskDispatchFailed)
		}
	}
	return memberItem(member, domain), nil
}

// UpdateMember updates orange-cloud state and queues reconciliation.
func UpdateMember(ctx context.Context, groupID, memberID uint, input MemberUpdateInput) (*MemberItem, error) {
	member, err := repository.GetCFPointingMember(ctx, groupID, memberID)
	if err != nil {
		return nil, err
	}
	member.Proxied = input.Proxied
	member.SyncStatus = model.CFMemberSyncPending
	member.LastError = ""
	if err = repository.SaveCFPointingMember(ctx, member); err != nil {
		return nil, err
	}
	group, err := repository.GetCFPointingGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if group.Enabled {
		if _, err = DispatchMemberSync(ctx, member.ID, "cloudflare_member_update"); err != nil {
			return nil, errors.New(errTaskDispatchFailed)
		}
	}
	domain, err := repository.GetZoneDomainByID(ctx, member.ZoneDomainID)
	if err != nil {
		return nil, err
	}
	return memberItem(member, domain), nil
}

// RemoveMember deletes the managed remote A record before removing local state.
func RemoveMember(ctx context.Context, groupID, memberID uint) error {
	member, err := repository.GetCFPointingMember(ctx, groupID, memberID)
	if err != nil {
		return err
	}
	if err = DeleteManagedRecord(ctx, member.ID); err != nil {
		return errors.New(errDeleteRemoteFailed)
	}
	return repository.DeleteCFPointingMember(ctx, member)
}

// DeleteGroup removes every managed remote A record and then local state.
func DeleteGroup(ctx context.Context, groupID uint) error {
	if _, err := repository.GetCFPointingGroup(ctx, groupID); err != nil {
		return err
	}
	members, err := repository.ListCFPointingMembersByGroupID(ctx, groupID)
	if err != nil {
		return err
	}
	for _, member := range members {
		if err = DeleteManagedRecord(ctx, member.ID); err != nil {
			return errors.New(errDeleteRemoteFailed)
		}
	}
	return repository.DeleteCFPointingGroupAndMembers(ctx, groupID)
}

// ListAvailableDomains returns ZoneDomains not yet assigned to a group.
func ListAvailableDomains(ctx context.Context) ([]AvailableDomain, error) {
	domains, err := repository.ListAvailableCFZoneDomains(ctx)
	if err != nil {
		return nil, err
	}
	zones, err := repository.ListZones(ctx)
	if err != nil {
		return nil, err
	}
	zoneRoots := make(map[uint]string, len(zones))
	for i := range zones {
		zoneRoots[zones[i].ID] = zones[i].Domain
	}
	items := make([]AvailableDomain, 0, len(domains))
	for _, domain := range domains {
		items = append(items, AvailableDomain{
			ID:         domain.ID,
			ZoneID:     domain.ZoneID,
			Domain:     domain.Domain,
			ZoneDomain: zoneRoots[domain.ZoneID],
		})
	}
	return items, nil
}

func listMemberItems(ctx context.Context, groupID uint) ([]MemberItem, error) {
	members, err := repository.ListCFPointingMembersByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	items := make([]MemberItem, 0, len(members))
	for i := range members {
		domain, domainErr := repository.GetZoneDomainByID(ctx, members[i].ZoneDomainID)
		if domainErr != nil {
			return nil, domainErr
		}
		items = append(items, *memberItem(&members[i], domain))
	}
	return items, nil
}

func memberItem(member *model.CFPointingMember, domain *model.ZoneDomain) *MemberItem {
	return &MemberItem{ID: member.ID, GroupID: member.GroupID, ZoneDomainID: member.ZoneDomainID, Domain: domain.Domain, ZoneID: domain.ZoneID, Proxied: member.Proxied, DesiredIP: member.DesiredIP, SyncStatus: member.SyncStatus, LastError: member.LastError, SyncedAt: member.SyncedAt}
}

// GetOverview returns readiness and aggregate sync counts.
func GetOverview(ctx context.Context) (*Overview, error) {
	connection, err := GetConnection(ctx)
	if err != nil {
		return nil, err
	}
	groups, err := repository.ListCFPointingGroups(ctx)
	if err != nil {
		return nil, err
	}
	overview := &Overview{Connection: *connection, GroupCount: len(groups)}
	for _, group := range groups {
		members, listErr := repository.ListCFPointingMembersByGroupID(ctx, group.ID)
		if listErr != nil {
			return nil, listErr
		}
		for _, member := range members {
			overview.MemberCount++
			switch member.SyncStatus {
			case model.CFMemberSyncOK:
				overview.OKCount++
			case model.CFMemberSyncError:
				overview.ErrorCount++
			default:
				overview.PendingCount++
			}
		}
	}
	return overview, nil
}
