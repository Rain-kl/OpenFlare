// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package origin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

// Input 源站创建/更新请求。
type Input struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Remark  string `json:"remark"`
}

// RouteSummary 源站详情中的代理规则摘要。
type RouteSummary struct {
	ID        uint   `json:"id"`
	Domain    string `json:"domain"`
	OriginURL string `json:"origin_url"`
	Enabled   bool   `json:"enabled"`
	UpdatedAt string `json:"updated_at"`
}

// View 源站列表项。
type View struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	Address    string `json:"address"`
	Remark     string `json:"remark"`
	RouteCount int64  `json:"route_count"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// DetailView 源站详情。
type DetailView struct {
	View
	Routes []RouteSummary `json:"routes"`
}

// ListOrigins 列出全部源站。
func ListOrigins(ctx context.Context) ([]View, error) {
	origins, err := model.ListOrigins(ctx)
	if err != nil {
		return nil, err
	}
	return buildOriginViews(ctx, origins)
}

// GetOriginDetail 获取源站详情。
func GetOriginDetail(ctx context.Context, id uint) (*DetailView, error) {
	origin, err := model.GetOriginByID(ctx, id)
	if err != nil {
		return nil, err
	}
	views, err := buildOriginViews(ctx, []model.Origin{*origin})
	if err != nil {
		return nil, err
	}
	routes, err := model.ListProxyRoutesByOriginID(ctx, id)
	if err != nil {
		return nil, err
	}
	items := make([]RouteSummary, 0, len(routes))
	for _, route := range routes {
		items = append(items, RouteSummary{
			ID:        route.ID,
			Domain:    route.Domain,
			OriginURL: route.OriginURL,
			Enabled:   route.Enabled,
			UpdatedAt: route.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Domain < items[j].Domain
	})
	return &DetailView{
		View:   views[0],
		Routes: items,
	}, nil
}

// CreateOrigin 创建源站。
func CreateOrigin(ctx context.Context, input Input) (*model.Origin, error) {
	origin, err := buildOrigin(nil, input)
	if err != nil {
		return nil, err
	}
	if err = model.CreateOriginRecord(ctx, origin); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errOriginAddressExists)
		}
		return nil, err
	}
	return origin, nil
}

// UpdateOrigin 更新源站。
func UpdateOrigin(ctx context.Context, id uint, input Input) (*model.Origin, error) {
	origin, err := model.GetOriginByID(ctx, id)
	if err != nil {
		return nil, err
	}
	previousAddress := origin.Address
	nextOrigin, err := buildOrigin(origin, input)
	if err != nil {
		return nil, err
	}
	err = db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(nextOrigin).Error; err != nil {
			if isUniqueConstraintError(err) {
				return errors.New(errOriginAddressExists)
			}
			return err
		}
		if previousAddress == nextOrigin.Address {
			return nil
		}
		return updateRoutesForOriginAddress(ctx, tx, nextOrigin.ID, nextOrigin.Address)
	})
	if err != nil {
		return nil, err
	}
	return nextOrigin, nil
}

// DeleteOrigin 删除源站。
func DeleteOrigin(ctx context.Context, id uint) error {
	count, err := model.CountProxyRoutesByOriginID(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New(errOriginDeleteReferenced)
	}
	if _, err = model.GetOriginByID(ctx, id); err != nil {
		return err
	}
	return model.DeleteOriginRecord(ctx, id)
}

func buildOrigin(existing *model.Origin, input Input) (*model.Origin, error) {
	address := normalizeOriginAddress(input.Address)
	if err := validateOriginAddress(address); err != nil {
		return nil, err
	}
	if existing == nil {
		existing = &model.Origin{}
	}
	existing.Address = address
	existing.Name = normalizeOriginName(input.Name, address)
	existing.Remark = strings.TrimSpace(input.Remark)
	return existing, nil
}

func buildOriginViews(ctx context.Context, origins []model.Origin) ([]View, error) {
	countRows, err := model.ListOriginRouteCounts(ctx)
	if err != nil {
		return nil, err
	}
	countMap := make(map[uint]int64, len(countRows))
	for _, row := range countRows {
		countMap[row.OriginID] = row.RouteCount
	}
	views := make([]View, 0, len(origins))
	for _, origin := range origins {
		views = append(views, View{
			ID:         origin.ID,
			Name:       origin.Name,
			Address:    origin.Address,
			Remark:     origin.Remark,
			RouteCount: countMap[origin.ID],
			CreatedAt:  origin.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:  origin.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return views, nil
}

func updateRoutesForOriginAddress(ctx context.Context, tx *gorm.DB, originID uint, address string) error {
	if !model.HasProxyRoutesTable(ctx) {
		return nil
	}
	var routes []model.OriginProxyRoute
	if err := tx.Where("origin_id = ?", originID).Order("id asc").Find(&routes).Error; err != nil {
		return fmt.Errorf("query routes for origin update failed: %w", err)
	}
	for _, route := range routes {
		rewrittenOriginURL, err := rewriteOriginURLAddress(route.OriginURL, address)
		if err != nil {
			return fmt.Errorf("rewrite route %d origin failed: %w", route.ID, err)
		}
		upstreams := make([]string, 0)
		if strings.TrimSpace(route.Upstreams) != "" {
			if err := json.Unmarshal([]byte(route.Upstreams), &upstreams); err != nil {
				return fmt.Errorf("decode route %d upstreams failed: %w", route.ID, err)
			}
		}
		if len(upstreams) == 0 {
			upstreams = append(upstreams, rewrittenOriginURL)
		} else {
			upstreams[0] = rewrittenOriginURL
		}
		upstreamsJSON, err := json.Marshal(upstreams)
		if err != nil {
			return fmt.Errorf("encode route %d upstreams failed: %w", route.ID, err)
		}
		if err := tx.Model(&model.OriginProxyRoute{}).
			Where("id = ?", route.ID).
			Updates(map[string]any{
				"origin_url": rewrittenOriginURL,
				"upstreams":  string(upstreamsJSON),
			}).Error; err != nil {
			return fmt.Errorf("update route %d origin address failed: %w", route.ID, err)
		}
	}
	return nil
}
