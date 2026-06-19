// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package apply_log

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
)

const (
	defaultApplyLogPageSize  = 20
	maxApplyLogPageSize      = 200
	maxApplyLogRetentionDays = 3650
)

// ListQuery filters apply logs for paginated listing.
type ListQuery struct {
	NodeID   string `json:"node_id"`
	PageNo   int    `json:"pageNo"`
	PageSize int    `json:"pageSize"`
}

// ListResult is the paginated apply log list response.
type ListResult struct {
	Rows      []*model.OpenFlareApplyLog `json:"rows"`
	Current   int                        `json:"current"`
	Total     int                        `json:"total"`
	TotalPage int                        `json:"totalPage"`
}

// CleanupInput controls apply log cleanup behavior.
type CleanupInput struct {
	DeleteAll     bool `json:"delete_all"`
	RetentionDays int  `json:"retention_days"`
}

// CleanupResult reports apply log cleanup outcome.
type CleanupResult struct {
	DeleteAll     bool       `json:"delete_all"`
	RetentionDays int        `json:"retention_days"`
	DeletedCount  int64      `json:"deleted_count"`
	Cutoff        *time.Time `json:"cutoff,omitempty"`
}

// ListPage returns paginated apply logs with optional node_id filter.
func ListPage(ctx context.Context, input ListQuery) (*ListResult, error) {
	pageNo := normalizePageNo(input.PageNo)
	pageSize := normalizePageSize(input.PageSize)
	nodeID := strings.TrimSpace(input.NodeID)

	rows, err := model.ListOpenFlareApplyLogs(ctx, model.OpenFlareApplyLogQuery{
		NodeID:   nodeID,
		PageNo:   pageNo,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, err
	}

	total, err := model.CountOpenFlareApplyLogs(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	totalPage := 0
	if total > 0 {
		totalPage = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return &ListResult{
		Rows:      rows,
		Current:   pageNo,
		Total:     int(total),
		TotalPage: totalPage,
	}, nil
}

// Cleanup removes old apply logs or deletes all records.
func Cleanup(ctx context.Context, input CleanupInput) (*CleanupResult, error) {
	if input.DeleteAll {
		deleted, err := model.DeleteAllOpenFlareApplyLogs(ctx)
		if err != nil {
			return nil, err
		}
		return &CleanupResult{
			DeleteAll:    true,
			DeletedCount: deleted,
		}, nil
	}

	if input.RetentionDays <= 0 || input.RetentionDays > maxApplyLogRetentionDays {
		return nil, errors.New(errRetentionDaysOutOfRange)
	}

	cutoff := time.Now().UTC().Add(-time.Duration(input.RetentionDays) * 24 * time.Hour)
	deleted, err := model.DeleteOpenFlareApplyLogsBefore(ctx, cutoff)
	if err != nil {
		return nil, err
	}

	return &CleanupResult{
		RetentionDays: input.RetentionDays,
		DeletedCount:  deleted,
		Cutoff:        &cutoff,
	}, nil
}

func normalizePageNo(pageNo int) int {
	if pageNo <= 0 {
		return 1
	}
	return pageNo
}

func normalizePageSize(pageSize int) int {
	if pageSize <= 0 {
		return defaultApplyLogPageSize
	}
	if pageSize > maxApplyLogPageSize {
		return maxApplyLogPageSize
	}
	return pageSize
}
