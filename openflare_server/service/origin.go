package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"openflare/model"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

type OriginInput struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Remark  string `json:"remark"`
}

type OriginRouteSummary struct {
	ID        uint   `json:"id"`
	Domain    string `json:"domain"`
	OriginURL string `json:"origin_url"`
	Enabled   bool   `json:"enabled"`
	UpdatedAt string `json:"updated_at"`
}

type OriginView struct {
	ID         uint      `json:"id"`
	Name       string    `json:"name"`
	Address    string    `json:"address"`
	Remark     string    `json:"remark"`
	RouteCount int64     `json:"route_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type OriginDetailView struct {
	OriginView
	Routes []OriginRouteSummary `json:"routes"`
}

func ListOrigins() ([]OriginView, error) {
	origins, err := model.ListOrigins()
	if err != nil {
		return nil, err
	}
	return buildOriginViews(origins)
}

func GetOriginDetail(id uint) (*OriginDetailView, error) {
	origin, err := model.GetOriginByID(id)
	if err != nil {
		return nil, err
	}
	views, err := buildOriginViews([]*model.Origin{origin})
	if err != nil {
		return nil, err
	}
	routes, err := model.ListProxyRoutesByOriginID(id)
	if err != nil {
		return nil, err
	}
	items := make([]OriginRouteSummary, 0, len(routes))
	for _, route := range routes {
		items = append(items, OriginRouteSummary{
			ID:        route.ID,
			Domain:    route.Domain,
			OriginURL: route.OriginURL,
			Enabled:   route.Enabled,
			UpdatedAt: route.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	sort.Slice(items, func(i int, j int) bool {
		return items[i].Domain < items[j].Domain
	})
	detail := &OriginDetailView{
		OriginView: views[0],
		Routes:     items,
	}
	return detail, nil
}

func CreateOrigin(input OriginInput) (*model.Origin, error) {
	origin, err := buildOrigin(nil, input)
	if err != nil {
		return nil, err
	}
	if err = origin.Insert(); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("源站地址已存在")
		}
		return nil, err
	}
	return origin, nil
}

func UpdateOrigin(id uint, input OriginInput) (*model.Origin, error) {
	origin, err := model.GetOriginByID(id)
	if err != nil {
		return nil, err
	}
	previousAddress := origin.Address
	nextOrigin, err := buildOrigin(origin, input)
	if err != nil {
		return nil, err
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(nextOrigin).Error; err != nil {
			if isUniqueConstraintError(err) {
				return errors.New("源站地址已存在")
			}
			return err
		}
		if previousAddress == nextOrigin.Address {
			return nil
		}
		return updateRoutesForOriginAddress(tx, nextOrigin.ID, nextOrigin.Address)
	})
	if err != nil {
		return nil, err
	}
	return nextOrigin, nil
}

func DeleteOrigin(id uint) error {
	routes, err := model.ListProxyRoutesByOriginID(id)
	if err != nil {
		return err
	}
	if len(routes) > 0 {
		return errors.New("该源站仍被规则引用，无法删除")
	}
	origin, err := model.GetOriginByID(id)
	if err != nil {
		return err
	}
	return origin.Delete()
}

func buildOrigin(existing *model.Origin, input OriginInput) (*model.Origin, error) {
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

func getOrCreateOriginByAddress(address string) (*model.Origin, error) {
	normalizedAddress := normalizeOriginAddress(address)
	if err := validateOriginAddress(normalizedAddress); err != nil {
		return nil, err
	}
	existing, err := model.GetOriginByAddress(normalizedAddress)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	origin := &model.Origin{
		Name:    normalizedAddress,
		Address: normalizedAddress,
		Remark:  "",
	}
	if err := origin.Insert(); err != nil {
		if isUniqueConstraintError(err) {
			return model.GetOriginByAddress(normalizedAddress)
		}
		return nil, err
	}
	return origin, nil
}

func updateRoutesForOriginAddress(tx *gorm.DB, originID uint, address string) error {
	var routes []*model.ProxyRoute
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
		if err := tx.Model(&model.ProxyRoute{}).
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

func buildOriginViews(origins []*model.Origin) ([]OriginView, error) {
	countRows, err := model.ListOriginRouteCounts()
	if err != nil {
		return nil, err
	}
	countMap := make(map[uint]int64, len(countRows))
	for _, row := range countRows {
		countMap[row.OriginID] = row.RouteCount
	}
	views := make([]OriginView, 0, len(origins))
	for _, origin := range origins {
		views = append(views, OriginView{
			ID:         origin.ID,
			Name:       origin.Name,
			Address:    origin.Address,
			Remark:     origin.Remark,
			RouteCount: countMap[origin.ID],
			CreatedAt:  origin.CreatedAt,
			UpdatedAt:  origin.UpdatedAt,
		})
	}
	return views, nil
}
