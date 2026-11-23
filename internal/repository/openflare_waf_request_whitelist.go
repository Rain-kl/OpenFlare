// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"

	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

// ListOpenFlareWAFWhitelist returns global whitelist entries in creation order.
func ListOpenFlareWAFWhitelist(ctx context.Context) ([]model.OpenFlareWAFWhitelistEntry, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]model.OpenFlareWAFWhitelistEntry, 0)
	err = conn.Order("id ASC").Find(&entries).Error
	return entries, err
}

// SaveOpenFlareWAFWhitelistEntry creates an entry or updates one at the supplied ID.
func SaveOpenFlareWAFWhitelistEntry(ctx context.Context, entry *model.OpenFlareWAFWhitelistEntry) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	if entry.ID == 0 {
		return conn.Create(entry).Error
	}
	result := conn.Model(entry).Select("Domain", "PathMatch", "Path").Updates(entry)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteOpenFlareWAFWhitelistEntry removes an entry from the global whitelist.
func DeleteOpenFlareWAFWhitelistEntry(ctx context.Context, id uint) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Delete(&model.OpenFlareWAFWhitelistEntry{}, id).Error
}
