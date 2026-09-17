// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"

	"Wavelet/openflare/plugins/server/kernel/model"

	"gorm.io/gorm"
)

const singletonCFConnectionID uint = 1

// CFPointingMemberContext contains all local state needed to reconcile one member.
type CFPointingMemberContext struct {
	Member model.CFPointingMember
	Group  model.CFPointingGroup
	Domain model.ZoneDomain
	Zone   model.Zone
	Node   model.OpenFlareNode
}

// GetCFConnection returns the global Cloudflare connection.
func GetCFConnection(ctx context.Context) (*model.CFConnection, error) {
	conn := DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var item model.CFConnection
	if err := conn.First(&item, singletonCFConnectionID).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// UpsertCFConnection creates or replaces the global Cloudflare connection.
func UpsertCFConnection(ctx context.Context, item *model.CFConnection) error {
	conn := DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	item.ID = singletonCFConnectionID
	return conn.Save(item).Error
}

// DeleteCFConnection clears the global Cloudflare connection.
func DeleteCFConnection(ctx context.Context) error {
	conn := DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Delete(&model.CFConnection{}, singletonCFConnectionID).Error
}

// ListCFPointingGroups lists Cloudflare pointing groups newest first.
func ListCFPointingGroups(ctx context.Context) ([]model.CFPointingGroup, error) {
	var items []model.CFPointingGroup
	if err := DB(ctx).Order("id desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetCFPointingGroup returns a group by ID.
func GetCFPointingGroup(ctx context.Context, id uint) (*model.CFPointingGroup, error) {
	var item model.CFPointingGroup
	if err := DB(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// CreateCFPointingGroup creates a group.
func CreateCFPointingGroup(ctx context.Context, item *model.CFPointingGroup) error {
	return DB(ctx).Create(item).Error
}

// SaveCFPointingGroup persists a group.
func SaveCFPointingGroup(ctx context.Context, item *model.CFPointingGroup) error {
	return DB(ctx).Save(item).Error
}

// DeleteCFPointingGroup deletes an empty group.
func DeleteCFPointingGroup(ctx context.Context, id uint) error {
	return DB(ctx).Delete(&model.CFPointingGroup{}, id).Error
}

// CountCFPointingMembersByGroupID counts members in a group.
func CountCFPointingMembersByGroupID(ctx context.Context, groupID uint) (int64, error) {
	var count int64
	err := DB(ctx).Table("of_cf_pointing_members AS members").
		Joins("JOIN of_zone_domains AS domains ON domains.id = members.zone_domain_id").
		Where("members.group_id = ?", groupID).Count(&count).Error
	return count, err
}

// ListCFPointingMembersByGroupID lists members by group.
func ListCFPointingMembersByGroupID(ctx context.Context, groupID uint) ([]model.CFPointingMember, error) {
	var items []model.CFPointingMember
	if err := DB(ctx).Where("group_id = ?", groupID).Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListCFPointingMembersByActiveNodeID lists members whose group currently targets a node.
func ListCFPointingMembersByActiveNodeID(ctx context.Context, nodeID uint) ([]model.CFPointingMember, error) {
	var items []model.CFPointingMember
	err := DB(ctx).Table("of_cf_pointing_members AS members").
		Select("members.*").
		Joins("JOIN of_cf_pointing_groups AS groups ON groups.id = members.group_id").
		Where("groups.active_node_id = ? AND groups.enabled = ?", nodeID, true).
		Order("members.id asc").Scan(&items).Error
	return items, err
}

// GetCFPointingMember returns a member scoped to its group.
func GetCFPointingMember(ctx context.Context, groupID, memberID uint) (*model.CFPointingMember, error) {
	var item model.CFPointingMember
	if err := DB(ctx).Where("id = ? AND group_id = ?", memberID, groupID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// GetCFPointingMemberByID returns a member by ID.
func GetCFPointingMemberByID(ctx context.Context, id uint) (*model.CFPointingMember, error) {
	var item model.CFPointingMember
	if err := DB(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// GetCFPointingMemberByZoneDomainID returns the member managing a ZoneDomain.
func GetCFPointingMemberByZoneDomainID(ctx context.Context, zoneDomainID uint) (*model.CFPointingMember, error) {
	var item model.CFPointingMember
	if err := DB(ctx).Where("zone_domain_id = ?", zoneDomainID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// CreateCFPointingMember creates a member.
func CreateCFPointingMember(ctx context.Context, item *model.CFPointingMember) error {
	return DB(ctx).Create(item).Error
}

// SaveCFPointingMember persists a member.
func SaveCFPointingMember(ctx context.Context, item *model.CFPointingMember) error {
	return DB(ctx).Save(item).Error
}

// UpdateCFPointingMemberColumns updates selected member fields.
func UpdateCFPointingMemberColumns(ctx context.Context, id uint, changes map[string]any) error {
	return DB(ctx).Model(&model.CFPointingMember{}).Where("id = ?", id).Updates(changes).Error
}

// DeleteCFPointingMember deletes a member.
func DeleteCFPointingMember(ctx context.Context, item *model.CFPointingMember) error {
	return DB(ctx).Delete(item).Error
}

// ListAvailableCFZoneDomains returns ZoneDomains not already managed by Cloudflare pointing.
func ListAvailableCFZoneDomains(ctx context.Context) ([]model.ZoneDomain, error) {
	var items []model.ZoneDomain
	err := DB(ctx).Where(`NOT EXISTS (
		SELECT 1 FROM of_cf_pointing_members AS members
		WHERE members.zone_domain_id = of_zone_domains.id
	)`).Order("domain asc").Find(&items).Error
	return items, err
}

// GetCFPointingMemberContext loads one member and all referenced local objects.
func GetCFPointingMemberContext(ctx context.Context, memberID uint) (*CFPointingMemberContext, error) {
	member, err := GetCFPointingMemberByID(ctx, memberID)
	if err != nil {
		return nil, err
	}
	group, err := GetCFPointingGroup(ctx, member.GroupID)
	if err != nil {
		return nil, err
	}
	domain, err := GetZoneDomainByID(ctx, member.ZoneDomainID)
	if err != nil {
		return nil, err
	}
	zone, err := GetZoneByID(ctx, domain.ZoneID)
	if err != nil {
		return nil, err
	}
	node, err := GetOpenFlareNodeByID(ctx, group.ActiveNodeID)
	if err != nil {
		return nil, err
	}
	return &CFPointingMemberContext{Member: *member, Group: *group, Domain: *domain, Zone: *zone, Node: *node}, nil
}

// GetZoneDomainByID returns a ZoneDomain by primary key.
func GetZoneDomainByID(ctx context.Context, id uint) (*model.ZoneDomain, error) {
	var item model.ZoneDomain
	if err := DB(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// MarkCFPointingGroupMembersPending resets every member after target changes.
func MarkCFPointingGroupMembersPending(ctx context.Context, groupID uint) error {
	return DB(ctx).Model(&model.CFPointingMember{}).Where("group_id = ?", groupID).
		Updates(map[string]any{"sync_status": model.CFMemberSyncPending, "last_error": ""}).Error
}

// DeleteCFPointingGroupAndMembers removes a group after its remote records are deleted.
func DeleteCFPointingGroupAndMembers(ctx context.Context, groupID uint) error {
	return DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", groupID).Delete(&model.CFPointingMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.CFPointingGroup{}, groupID).Error
	})
}
