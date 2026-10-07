// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"regexp"
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
	targetMode := strings.TrimSpace(input.TargetMode)
	if targetMode == "" {
		targetMode = model.CFPointingTargetModeNode
	}
	if targetMode != model.CFPointingTargetModeNode && targetMode != model.CFPointingTargetModeCustom {
		return nil, errors.New(errGroupTargetModeInvalid)
	}
	primaryNodeID := uint(0)
	var backupNodeID *uint
	recordType := strings.ToUpper(strings.TrimSpace(input.RecordType))
	if recordType == "" {
		recordType = "A"
	}
	recordContent := strings.TrimSpace(input.RecordContent)
	if targetMode == model.CFPointingTargetModeCustom && !validCustomRecordType(recordType) {
		return nil, errors.New(errRecordTypeInvalid)
	}
	if targetMode == model.CFPointingTargetModeCustom && !validCustomRecordContent(recordType, recordContent) {
		return nil, errors.New(errRecordContentInvalid)
	}
	if targetMode == model.CFPointingTargetModeNode {
		resolvedPrimary, resolvedBackup, err := resolveGroupNodes(ctx, input)
		if err != nil {
			return nil, err
		}
		primaryNodeID = resolvedPrimary
		backupNodeID = resolvedBackup
	}
	activeNodeID := primaryNodeID
	if existing != nil && targetMode == model.CFPointingTargetModeNode && existing.TargetMode == targetMode && existing.PrimaryNodeID == primaryNodeID && equalOptionalUint(existing.BackupNodeID, backupNodeID) {
		activeNodeID = existing.ActiveNodeID
	}
	if existing == nil {
		existing = &model.CFPointingGroup{}
	}
	existing.Name = name
	existing.TargetMode = targetMode
	existing.RecordType = recordType
	existing.RecordContent = recordContent
	existing.PrimaryNodeID = primaryNodeID
	existing.ActiveNodeID = activeNodeID
	existing.BackupNodeID = backupNodeID
	existing.DefaultProxied = input.DefaultProxied
	existing.Enabled = input.Enabled
	return existing, nil
}

func equalOptionalUint(left, right *uint) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func resolveGroupNodes(ctx context.Context, input GroupInput) (uint, *uint, error) {
	if input.BackupNodeID != nil && *input.BackupNodeID == input.PrimaryNodeID {
		return 0, nil, errors.New(errGroupNodeSame)
	}
	primary, err := validEdgeNode(ctx, input.PrimaryNodeID, true)
	if err != nil {
		return 0, nil, err
	}
	if input.BackupNodeID != nil {
		if _, err = validEdgeNode(ctx, *input.BackupNodeID, false); err != nil {
			return 0, nil, err
		}
	}
	return primary.ID, input.BackupNodeID, nil
}

func validCustomRecordType(recordType string) bool {
	return recordType == "CNAME" || recordType == "A" || recordType == "AAAA"
}

func validCustomRecordContent(recordType, content string) bool {
	switch recordType {
	case "A":
		return isIPv4(content)
	case "AAAA":
		return isIPv6(content)
	case "CNAME":
		return validCNAMEContent(content)
	default:
		return false
	}
}

var cnameHostPattern = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(?:\.(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?))*$`)

func isIPv4(content string) bool {
	ip := net.ParseIP(content)
	return ip != nil && ip.To4() != nil
}

func isIPv6(content string) bool {
	ip := net.ParseIP(content)
	return ip != nil && ip.To4() == nil
}

func validCNAMEContent(content string) bool {
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(content)), ".")
	return len(host) > 0 && len(host) <= 253 && net.ParseIP(host) == nil && cnameHostPattern.MatchString(host)
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

// CheckNodeFailover switches enabled node groups to healthy backup nodes and fails back after primary recovery.
func CheckNodeFailover(ctx context.Context) (int, error) {
	groups, err := repository.ListCFPointingGroups(ctx)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	changed := 0
	var dispatchErrors []error
	for i := range groups {
		group := &groups[i]
		if !group.Enabled || group.TargetMode == model.CFPointingTargetModeCustom || group.BackupNodeID == nil {
			continue
		}
		primary, primaryErr := repository.GetOpenFlareNodeByID(ctx, group.PrimaryNodeID)
		if primaryErr != nil && !errors.Is(primaryErr, gorm.ErrRecordNotFound) {
			return changed, primaryErr
		}
		primaryHealthy := primaryErr == nil && isNodeHealthyForFailover(primary, now)
		targetNodeID := group.ActiveNodeID
		if primaryHealthy {
			targetNodeID = group.PrimaryNodeID
		} else {
			backup, backupErr := repository.GetOpenFlareNodeByID(ctx, *group.BackupNodeID)
			if backupErr != nil && !errors.Is(backupErr, gorm.ErrRecordNotFound) {
				return changed, backupErr
			}
			if backupErr == nil && isNodeHealthyForFailover(backup, now) {
				targetNodeID = *group.BackupNodeID
			}
		}
		if targetNodeID != group.ActiveNodeID {
			if err = repository.UpdateCFPointingGroupTarget(ctx, group.ID, map[string]any{"active_node_id": targetNodeID}); err != nil {
				return changed, err
			}
			if err = repository.MarkCFPointingGroupMembersPending(ctx, group.ID); err != nil {
				return changed, err
			}
			changed++
		}
		pending, pendingErr := repository.HasPendingCFPointingGroupMembers(ctx, group.ID)
		if pendingErr != nil {
			return changed, pendingErr
		}
		if pending {
			if _, err = DispatchGroupSync(ctx, group.ID, "cloudflare_node_failover"); err != nil {
				logger.WarnF(ctx, "[Cloudflare] dispatch failover sync failed: group_id=%d error=%v", group.ID, err)
				dispatchErrors = append(dispatchErrors, err)
			}
		}
	}
	return changed, errors.Join(dispatchErrors...)
}

func isNodeHealthyForFailover(node *model.OpenFlareNode, now time.Time) bool {
	if node == nil || node.Status == "offline" || node.OpenrestyStatus == "unhealthy" || strings.TrimSpace(node.LastError) != "" {
		return false
	}
	return node.LastSeenAt != nil && now.Sub(*node.LastSeenAt) <= 60*time.Second
}

func buildGroupItem(ctx context.Context, group *model.CFPointingGroup) (*GroupItem, error) {
	targetMode := group.TargetMode
	if targetMode == "" {
		targetMode = model.CFPointingTargetModeNode
	}
	primary := (*model.OpenFlareNode)(nil)
	active := (*model.OpenFlareNode)(nil)
	var err error
	if targetMode == model.CFPointingTargetModeNode {
		primary, err = lookupGroupNode(ctx, group.ID, group.PrimaryNodeID)
		if err != nil {
			return nil, err
		}
		active, err = lookupGroupNode(ctx, group.ID, group.ActiveNodeID)
		if err != nil {
			return nil, err
		}
	}
	count, err := repository.CountCFPointingMembersByGroupID(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	item := &GroupItem{ID: group.ID, Name: group.Name, TargetMode: targetMode, RecordType: group.RecordType, RecordContent: group.RecordContent, PrimaryNode: nodeOptionForID(group.PrimaryNodeID, primary), ActiveNode: nodeOptionForID(group.ActiveNodeID, active), DefaultProxied: group.DefaultProxied, Enabled: group.Enabled, MemberCount: count, CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt}
	if group.BackupNodeID != nil {
		backup, backupErr := lookupGroupNode(ctx, group.ID, *group.BackupNodeID)
		if backupErr != nil {
			return nil, backupErr
		}
		option := nodeOptionForID(*group.BackupNodeID, backup)
		item.BackupNode = &option
	}
	return item, nil
}

func lookupGroupNode(ctx context.Context, groupID, nodeID uint) (*model.OpenFlareNode, error) {
	node, err := repository.GetOpenFlareNodeByID(ctx, nodeID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logger.WarnF(ctx, "[Cloudflare] pointing group references missing node: group_id=%d node_id=%d", groupID, nodeID)
		return nil, nil
	}
	return node, err
}

func nodeOptionForID(id uint, node *model.OpenFlareNode) NodeOption {
	if node == nil {
		return NodeOption{ID: id}
	}
	return nodeOption(node)
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

// RemoveMember deletes the managed remote DNS record before removing local state.
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

// MoveMember transfers a member from sourceGroupID to targetGroupID.
func MoveMember(ctx context.Context, sourceGroupID, memberID, targetGroupID uint) (*MemberItem, error) {
	if targetGroupID == 0 || targetGroupID == sourceGroupID {
		return nil, errors.New(errTargetGroupSame)
	}
	targetGroup, err := repository.GetCFPointingGroup(ctx, targetGroupID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errTargetGroupInvalid)
		}
		return nil, err
	}
	member, err := repository.GetCFPointingMember(ctx, sourceGroupID, memberID)
	if err != nil {
		return nil, err
	}
	member.GroupID = targetGroupID
	member.SyncStatus = model.CFMemberSyncPending
	member.LastError = ""
	if err = repository.SaveCFPointingMember(ctx, member); err != nil {
		return nil, err
	}
	if targetGroup.Enabled {
		if _, err = DispatchMemberSync(ctx, member.ID, "cloudflare_member_move"); err != nil {
			logger.WarnF(ctx, "[Cloudflare] dispatch move sync failed: member_id=%d error=%v", member.ID, err)
		}
	}
	domain, err := repository.GetZoneDomainByID(ctx, member.ZoneDomainID)
	if err != nil {
		return nil, err
	}
	return memberItem(member, domain), nil
}

// BatchMoveMembers transfers multiple members from sourceGroupID to targetGroupID.
func BatchMoveMembers(ctx context.Context, sourceGroupID uint, input MemberBatchMoveInput) error {
	if len(input.MemberIDs) == 0 {
		return errors.New(errNoMembersSelected)
	}
	if input.TargetGroupID == 0 || input.TargetGroupID == sourceGroupID {
		return errors.New(errTargetGroupSame)
	}
	targetGroup, err := repository.GetCFPointingGroup(ctx, input.TargetGroupID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(errTargetGroupInvalid)
		}
		return err
	}
	for _, memberID := range uniqueIDs(input.MemberIDs) {
		member, getErr := repository.GetCFPointingMember(ctx, sourceGroupID, memberID)
		if getErr != nil {
			continue
		}
		member.GroupID = input.TargetGroupID
		member.SyncStatus = model.CFMemberSyncPending
		member.LastError = ""
		if saveErr := repository.SaveCFPointingMember(ctx, member); saveErr != nil {
			logger.ErrorF(ctx, "[Cloudflare] batch move save member failed: member_id=%d error=%v", memberID, saveErr)
			continue
		}
		if targetGroup.Enabled {
			if _, syncErr := DispatchMemberSync(ctx, member.ID, "cloudflare_member_move"); syncErr != nil {
				logger.WarnF(ctx, "[Cloudflare] dispatch batch move sync failed: member_id=%d error=%v", member.ID, syncErr)
			}
		}
	}
	return nil
}

// BatchRemoveMembers deletes multiple members and their remote DNS records.
func BatchRemoveMembers(ctx context.Context, sourceGroupID uint, input MemberBatchRemoveInput) error {
	if len(input.MemberIDs) == 0 {
		return errors.New(errNoMembersSelected)
	}
	for _, memberID := range uniqueIDs(input.MemberIDs) {
		member, err := repository.GetCFPointingMember(ctx, sourceGroupID, memberID)
		if err != nil {
			continue
		}
		if delErr := DeleteManagedRecord(ctx, member.ID); delErr != nil {
			logger.WarnF(ctx, "[Cloudflare] delete remote record failed during batch remove: member_id=%d error=%v", member.ID, delErr)
		}
		if err = repository.DeleteCFPointingMember(ctx, member); err != nil {
			logger.ErrorF(ctx, "[Cloudflare] delete member failed during batch remove: member_id=%d error=%v", member.ID, err)
		}
	}
	return nil
}

// BatchEnableProxy enables orange-cloud proxy for selected members.
func BatchEnableProxy(ctx context.Context, groupID uint, input MemberBatchProxyInput) error {
	if len(input.MemberIDs) == 0 {
		return errors.New(errNoMembersSelected)
	}
	group, err := repository.GetCFPointingGroup(ctx, groupID)
	if err != nil {
		return err
	}
	var firstErr error
	for _, memberID := range uniqueIDs(input.MemberIDs) {
		member, getErr := repository.GetCFPointingMember(ctx, groupID, memberID)
		if getErr != nil {
			if firstErr == nil {
				firstErr = getErr
			}
			continue
		}
		member.Proxied = true
		member.SyncStatus = model.CFMemberSyncPending
		member.LastError = ""
		if saveErr := repository.SaveCFPointingMember(ctx, member); saveErr != nil {
			logger.ErrorF(ctx, "[Cloudflare] batch enable proxy save member failed: member_id=%d error=%v", memberID, saveErr)
			if firstErr == nil {
				firstErr = saveErr
			}
			continue
		}
		if group.Enabled {
			if _, syncErr := DispatchMemberSync(ctx, member.ID, "cloudflare_member_batch_proxy"); syncErr != nil {
				logger.WarnF(ctx, "[Cloudflare] dispatch batch enable proxy sync failed: member_id=%d error=%v", member.ID, syncErr)
				if firstErr == nil {
					firstErr = syncErr
				}
			}
		}
	}
	return firstErr
}

func uniqueIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return ids
	}
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}
	return result
}

// DeleteGroup removes every managed remote DNS record and then local state.
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
			if errors.Is(domainErr, gorm.ErrRecordNotFound) {
				logger.WarnF(ctx, "[Cloudflare] cleaning up orphaned pointing member: member_id=%d zone_domain_id=%d", members[i].ID, members[i].ZoneDomainID)
				if delErr := repository.DeleteCFPointingMember(ctx, &members[i]); delErr != nil {
					logger.ErrorF(ctx, "[Cloudflare] delete orphaned member failed: member_id=%d error=%v", members[i].ID, delErr)
				}
				continue
			}
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
