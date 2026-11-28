// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
)

// EnsureEmbeddedDNSInstance upserts the controller-managed embedded SmartDNS instance record.
func EnsureEmbeddedDNSInstance(ctx context.Context, instance *model.OpenFlareDNSInstance) (*model.OpenFlareDNSInstance, error) {
	if instance == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var out model.OpenFlareDNSInstance
	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where("instance_id = ?", instance.InstanceID).First(&out).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			out = *instance
			return tx.Create(&out).Error
		}
		if err != nil {
			return err
		}
		updates := map[string]any{
			colName:         instance.Name,
			"mode":          instance.Mode,
			"listen_addr":   instance.ListenAddr,
			"public_addr":   instance.PublicAddr,
			"geo_name":      instance.GeoName,
			"geo_latitude":  instance.GeoLatitude,
			"geo_longitude": instance.GeoLongitude,
			"version":       instance.Version,
		}
		if strings.TrimSpace(out.AccessToken) == "" && strings.TrimSpace(instance.AccessToken) != "" {
			updates["access_token"] = instance.AccessToken
		}
		if err := tx.Model(&out).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Where("instance_id = ?", instance.InstanceID).First(&out).Error
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ListDNSInstances returns all SmartDNS instances ordered with embedded instances first.
func ListDNSInstances(ctx context.Context) ([]model.OpenFlareDNSInstance, error) {
	var items []model.OpenFlareDNSInstance
	if err := db.DB(ctx).Order("mode ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetDNSInstanceByID returns a SmartDNS instance by database ID.
func GetDNSInstanceByID(ctx context.Context, id uint) (*model.OpenFlareDNSInstance, error) {
	var item model.OpenFlareDNSInstance
	if err := db.DB(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// GetDNSInstanceByInstanceID returns a SmartDNS instance by its logical instance_id.
func GetDNSInstanceByInstanceID(ctx context.Context, instanceID string) (*model.OpenFlareDNSInstance, error) {
	var item model.OpenFlareDNSInstance
	if err := db.DB(ctx).Where("instance_id = ?", strings.TrimSpace(instanceID)).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// GetDNSInstanceByAccessToken looks up a SmartDNS instance by its bearer access token.
func GetDNSInstanceByAccessToken(ctx context.Context, token string) (*model.OpenFlareDNSInstance, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var item model.OpenFlareDNSInstance
	if err := db.DB(ctx).Where("access_token = ?", trimmed).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// CreateDNSInstance persists a new SmartDNS instance record.
func CreateDNSInstance(ctx context.Context, item *model.OpenFlareDNSInstance) error {
	return db.DB(ctx).Create(item).Error
}

// SaveDNSInstance persists full updates to an existing SmartDNS instance record.
func SaveDNSInstance(ctx context.Context, item *model.OpenFlareDNSInstance) error {
	return db.DB(ctx).Save(item).Error
}

// UpdateDNSInstanceFields updates explicit columns of a SmartDNS instance.
func UpdateDNSInstanceFields(ctx context.Context, item *model.OpenFlareDNSInstance, fields ...string) error {
	if len(fields) == 0 {
		return db.DB(ctx).Save(item).Error
	}
	return db.DB(ctx).Model(item).Select(fields).Updates(item).Error
}

// UpdateDNSInstanceRuntime updates runtime status/telemetry columns for a SmartDNS instance.
func UpdateDNSInstanceRuntime(ctx context.Context, instanceID string, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	res := db.DB(ctx).Model(&model.OpenFlareDNSInstance{}).
		Where("instance_id = ?", strings.TrimSpace(instanceID)).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteDNSInstance deletes a SmartDNS instance by database ID.
func DeleteDNSInstance(ctx context.Context, id uint) error {
	res := db.DB(ctx).Where("id = ?", id).Delete(&model.OpenFlareDNSInstance{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListDNSPolicies returns all SmartDNS routing policies ordered by priority and ID.
func ListDNSPolicies(ctx context.Context) ([]model.OpenFlareDNSPolicy, error) {
	var items []model.OpenFlareDNSPolicy
	if err := db.DB(ctx).Order("priority ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListEnabledDNSPolicies returns enabled SmartDNS routing policies ordered by priority and ID.
func ListEnabledDNSPolicies(ctx context.Context) ([]model.OpenFlareDNSPolicy, error) {
	var items []model.OpenFlareDNSPolicy
	if err := db.DB(ctx).Where("enabled = ?", true).Order("priority ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetDNSPolicyByID returns a SmartDNS routing policy by primary key.
func GetDNSPolicyByID(ctx context.Context, id uint) (*model.OpenFlareDNSPolicy, error) {
	var item model.OpenFlareDNSPolicy
	if err := db.DB(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// GetDefaultDNSPolicy returns the default SmartDNS routing policy.
func GetDefaultDNSPolicy(ctx context.Context) (*model.OpenFlareDNSPolicy, error) {
	var item model.OpenFlareDNSPolicy
	if err := db.DB(ctx).Where("is_default = ?", true).Order("id ASC").First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// CreateDNSPolicy creates a new SmartDNS routing policy, clearing other defaults when IsDefault is true.
func CreateDNSPolicy(ctx context.Context, item *model.OpenFlareDNSPolicy) error {
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if item.IsDefault {
			if err := tx.Model(&model.OpenFlareDNSPolicy{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(item).Error
	})
}

// SaveDNSPolicy persists updates to a SmartDNS routing policy, maintaining single-default invariant.
func SaveDNSPolicy(ctx context.Context, item *model.OpenFlareDNSPolicy) error {
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if item.IsDefault {
			if err := tx.Model(&model.OpenFlareDNSPolicy{}).
				Where("id <> ? AND is_default = ?", item.ID, true).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Save(item).Error
	})
}

// SetDefaultDNSPolicy promotes the specified policy to the sole enabled default policy.
func SetDefaultDNSPolicy(ctx context.Context, id uint) error {
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.OpenFlareDNSPolicy
		if err := tx.First(&existing, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.OpenFlareDNSPolicy{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.OpenFlareDNSPolicy{}).
			Where("id = ?", id).
			Updates(map[string]any{
				"is_default": true,
				"enabled":    true,
			}).Error
	})
}

// DeleteDNSPolicy removes a SmartDNS routing policy by ID.
func DeleteDNSPolicy(ctx context.Context, id uint) error {
	res := db.DB(ctx).Where("id = ?", id).Delete(&model.OpenFlareDNSPolicy{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CountPointingGroupsBySmartDNSPolicyID returns how many Cloudflare pointing groups reference the given policy.
func CountPointingGroupsBySmartDNSPolicyID(ctx context.Context, policyID uint) (int64, error) {
	var count int64
	if err := db.DB(ctx).Model(&model.CFPointingGroup{}).
		Where("target_mode = ? AND smartdns_policy_id = ?", model.CFPointingTargetModeSmartDNS, policyID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// InsertOpenFlareDNSMetricSnapshot queues or inserts a SmartDNS metric snapshot through active logstore hooks.
func InsertOpenFlareDNSMetricSnapshot(ctx context.Context, record *model.OpenFlareDNSMetricSnapshot) error {
	s, err := logstore.Active(ctx)
	if err != nil {
		return err
	}
	return s.Observability.InsertDNSMetricSnapshot(ctx, record)
}

// ErrDNSUpdateUnavailable reports that at least one selected instance could not accept the request.
var ErrDNSUpdateUnavailable = errors.New("selected DNS instances cannot accept an update")

// QueueDNSInstanceUpdates atomically queues the channel on every selected eligible instance.
func QueueDNSInstanceUpdates(ctx context.Context, ids []uint, requestID, channel string, now, onlineSince, expiry time.Time) error {
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.OpenFlareDNSInstance{}).
			Where("id IN ? AND mode = ? AND quarantined_at IS NULL", ids, model.DNSInstanceModeStandalone).
			Where("remote_update_supported = ? AND last_seen_at >= ?", true, onlineSince).
			Where("(update_status NOT IN ? OR update_requested_at <= ?)", []string{model.DNSUpdateQueued, model.DNSUpdateRunning}, expiry).
			Updates(map[string]any{"update_request_id": requestID, "update_channel": channel, "update_status": model.DNSUpdateQueued, "update_requested_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(ids)) {
			return ErrDNSUpdateUnavailable
		}
		return nil
	})
}

// AcknowledgeDNSInstanceUpdate applies a receipt only to the current, nonterminal request.
func AcknowledgeDNSInstanceUpdate(ctx context.Context, instanceID, requestID, status string) error {
	return db.DB(ctx).Model(&model.OpenFlareDNSInstance{}).
		Where("instance_id = ? AND update_request_id = ? AND update_status IN ?", instanceID, requestID, []string{model.DNSUpdateQueued, model.DNSUpdateRunning}).
		Update("update_status", status).Error
}
