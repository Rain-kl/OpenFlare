// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

// PushHistoryListFilter filters push history pagination queries.
type PushHistoryListFilter struct {
	EventKey string
	Status   string
	Page     int
	PageSize int
}

// ListPushHistories returns paginated push history records.
func ListPushHistories(ctx context.Context, filter PushHistoryListFilter) (int64, []model.PushHistory, error) {
	query := db.DB(ctx).Model(&model.PushHistory{}).Order("created_at DESC")
	if filter.EventKey != "" {
		query = query.Where("event_key = ?", filter.EventKey)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	var results []model.PushHistory
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Offset(offset).Limit(filter.PageSize).Find(&results).Error; err != nil {
		return 0, nil, err
	}

	return total, results, nil
}

// CreatePushHistory persists a push history audit record.
func CreatePushHistory(ctx context.Context, history *model.PushHistory) error {
	return db.DB(ctx).Create(history).Error
}

// PushHistoryQuery returns a scoped query builder for push histories.
func PushHistoryQuery(ctx context.Context) *gorm.DB {
	return db.DB(ctx).Model(&model.PushHistory{})
}
