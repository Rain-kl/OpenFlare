// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
)

var poolKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// PoolInput creates or updates a tunnel relay pool.
type PoolInput struct {
	PoolKey       string         `json:"pool_key"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Region        string         `json:"region"`
	IsDefault     bool           `json:"is_default"`
	RoutingPolicy map[string]any `json:"routing_policy"`
}

// PoolView is the console API representation of a tunnel relay pool.
type PoolView struct {
	ID            uint           `json:"id"`
	PoolKey       string         `json:"pool_key"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Region        string         `json:"region"`
	IsDefault     bool           `json:"is_default"`
	RoutingPolicy map[string]any `json:"routing_policy"`
	RelayCount    int64          `json:"relay_count"`
	TunnelCount   int64          `json:"tunnel_count"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// ListPools returns all relay pools with attached relay and tunnel counts.
func ListPools(ctx context.Context) ([]PoolView, error) {
	pools, err := repository.ListTunnelRelayPools(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]PoolView, 0, len(pools))
	for i := range pools {
		view, err := buildPoolView(ctx, &pools[i])
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

// GetPool returns one relay pool by ID.
func GetPool(ctx context.Context, id uint) (*PoolView, error) {
	pool, err := repository.GetTunnelRelayPoolByID(ctx, id)
	if err != nil {
		return nil, err
	}
	view, err := buildPoolView(ctx, pool)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

// CreatePool creates a new relay pool.
func CreatePool(ctx context.Context, input PoolInput) (*PoolView, error) {
	poolKey := strings.ToLower(strings.TrimSpace(input.PoolKey))
	if poolKey == "" {
		return nil, errors.New(errPoolKeyRequired)
	}
	if !poolKeyPattern.MatchString(poolKey) {
		return nil, errors.New(errPoolKeyInvalid)
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New(errPoolNameRequired)
	}
	if _, err := repository.GetTunnelRelayPoolByKey(ctx, poolKey); err == nil {
		return nil, errors.New(errPoolKeyExists)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	policyJSON, err := encodeJSONObject(input.RoutingPolicy)
	if err != nil {
		return nil, errors.New(errPoolPolicyInvalid)
	}

	pool := &model.TunnelRelayPool{
		PoolKey:           poolKey,
		Name:              name,
		Description:       strings.TrimSpace(input.Description),
		Region:            strings.TrimSpace(input.Region),
		IsDefault:         input.IsDefault,
		RoutingPolicyJSON: policyJSON,
	}

	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New("database not initialized")
	}
	if err := conn.Transaction(func(tx *gorm.DB) error {
		if pool.IsDefault {
			if err := tx.Model(&model.TunnelRelayPool{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(pool).Error
	}); err != nil {
		return nil, err
	}

	websocket.NotifyTunnelPoolsChanged(ctx)
	view, err := buildPoolView(ctx, pool)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

// UpdatePool updates an existing relay pool.
func UpdatePool(ctx context.Context, id uint, input PoolInput) (*PoolView, error) {
	pool, err := repository.GetTunnelRelayPoolByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New(errPoolNameRequired)
	}
	policyJSON := pool.RoutingPolicyJSON
	if input.RoutingPolicy != nil {
		policyJSON, err = encodeJSONObject(input.RoutingPolicy)
		if err != nil {
			return nil, errors.New(errPoolPolicyInvalid)
		}
	}

	pool.Name = name
	pool.Description = strings.TrimSpace(input.Description)
	pool.Region = strings.TrimSpace(input.Region)
	pool.RoutingPolicyJSON = policyJSON

	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New("database not initialized")
	}
	if err := conn.Transaction(func(tx *gorm.DB) error {
		if input.IsDefault && !pool.IsDefault {
			if err := tx.Model(&model.TunnelRelayPool{}).Where("id <> ? AND is_default = ?", pool.ID, true).Update("is_default", false).Error; err != nil {
				return err
			}
			pool.IsDefault = true
		}
		return tx.Model(pool).Select("name", "description", "region", "is_default", "routing_policy_json").Updates(pool).Error
	}); err != nil {
		return nil, err
	}

	websocket.NotifyTunnelPoolsChanged(ctx)
	view, err := buildPoolView(ctx, pool)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

// DeletePool deletes a non-default empty relay pool.
func DeletePool(ctx context.Context, id uint) error {
	pool, err := repository.GetTunnelRelayPoolByID(ctx, id)
	if err != nil {
		return err
	}
	if pool.IsDefault {
		return errors.New(errPoolDefaultCannotDelete)
	}
	relayCount, err := repository.CountTunnelRelaysInPool(ctx, id)
	if err != nil {
		return err
	}
	tunnelCount, err := repository.CountTunnelsInPool(ctx, id)
	if err != nil {
		return err
	}
	if relayCount > 0 || tunnelCount > 0 {
		return errors.New(errPoolHasRelaysOrTunnels)
	}
	return repository.DeleteTunnelRelayPool(ctx, id)
}

func buildPoolView(ctx context.Context, pool *model.TunnelRelayPool) (PoolView, error) {
	relayCount, err := repository.CountTunnelRelaysInPool(ctx, pool.ID)
	if err != nil {
		return PoolView{}, err
	}
	tunnelCount, err := repository.CountTunnelsInPool(ctx, pool.ID)
	if err != nil {
		return PoolView{}, err
	}
	return PoolView{
		ID:            pool.ID,
		PoolKey:       pool.PoolKey,
		Name:          pool.Name,
		Description:   pool.Description,
		Region:        pool.Region,
		IsDefault:     pool.IsDefault,
		RoutingPolicy: decodeJSONObject(ctx, pool.RoutingPolicyJSON),
		RelayCount:    relayCount,
		TunnelCount:   tunnelCount,
		CreatedAt:     pool.CreatedAt,
		UpdatedAt:     pool.UpdatedAt,
	}, nil
}

func resolvePoolIDForMutation(ctx context.Context, requestedPoolID uint, poolKey string) (uint, error) {
	if requestedPoolID > 0 {
		pool, err := repository.GetTunnelRelayPoolByID(ctx, requestedPoolID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, errors.New(errPoolNotFound)
			}
			return 0, err
		}
		return pool.ID, nil
	}
	if trimmedKey := strings.ToLower(strings.TrimSpace(poolKey)); trimmedKey != "" {
		pool, err := repository.GetTunnelRelayPoolByKey(ctx, trimmedKey)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, errors.New(errPoolNotFound)
			}
			return 0, err
		}
		return pool.ID, nil
	}
	pool, err := repository.GetDefaultTunnelRelayPool(ctx)
	if err == nil {
		return pool.ID, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	return 0, err
}

func encodeJSONObject(values map[string]any) (string, error) {
	if len(values) == 0 {
		return "{}", nil
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func decodeJSONObject(ctx context.Context, raw string) map[string]any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" {
		return map[string]any{}
	}
	out := make(map[string]any)
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		logger.WarnF(ctx, "decode json object failed: %v", err)
		return map[string]any{}
	}
	return out
}

func encodeJSONStringSlice(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func decodeJSONStringSlice(ctx context.Context, raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "[]" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		logger.WarnF(ctx, "decode json string slice failed: %v", err)
		return []string{}
	}
	if out == nil {
		return []string{}
	}
	return out
}
