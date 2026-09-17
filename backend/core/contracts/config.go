// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package contracts

import (
	"context"
	"time"
)

// SystemConfigDTO represents a system configuration key-value entry.
type SystemConfigDTO struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Type        string    `json:"type"`
	Visibility  int       `json:"visibility"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// SystemConfigService defines the unified contract for querying and mutating system configurations.
type SystemConfigService interface {
	GetByKey(ctx context.Context, key string) (SystemConfigDTO, error)
	ListByKeys(ctx context.Context, keys []string) (map[string]SystemConfigDTO, error)
	ListVisible(ctx context.Context) ([]SystemConfigDTO, error)
	ListByType(ctx context.Context, configType string) ([]SystemConfigDTO, error)
	GetIntByKey(ctx context.Context, key string) (int, error)
	GetBoolByKey(ctx context.Context, key string) (bool, error)
	SaveOrUpdate(ctx context.Context, key, value string) error
	InvalidateCache(ctx context.Context, key string) error
	InvalidateAllCaches(ctx context.Context) error
}
