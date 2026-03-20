package model

import "time"

type ProxyRoute struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	Domain        string    `json:"domain" gorm:"uniqueIndex;size:255;not null"`
	OriginID      *uint     `json:"origin_id" gorm:"index"`
	OriginURL     string    `json:"origin_url" gorm:"size:2048;not null"`
	OriginHost    string    `json:"origin_host" gorm:"size:255"`
	Upstreams     string    `json:"upstreams" gorm:"type:text;not null;default:'[]'"`
	Enabled       bool      `json:"enabled" gorm:"not null;default:true"`
	EnableHTTPS   bool      `json:"enable_https" gorm:"column:enable_https;not null;default:false"`
	CertID        *uint     `json:"cert_id"`
	RedirectHTTP  bool      `json:"redirect_http" gorm:"not null;default:false"`
	CacheEnabled  bool      `json:"cache_enabled" gorm:"not null;default:false"`
	CachePolicy   string    `json:"cache_policy" gorm:"size:32;not null;default:''"`
	CacheRules    string    `json:"cache_rules" gorm:"type:text;not null;default:'[]'"`
	CustomHeaders string    `json:"custom_headers" gorm:"type:text;not null;default:'[]'"`
	Remark        string    `json:"remark" gorm:"size:255"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func ListProxyRoutes() (routes []*ProxyRoute, err error) {
	err = DB.Order("id desc").Find(&routes).Error
	return routes, err
}

func GetEnabledProxyRoutes() (routes []*ProxyRoute, err error) {
	err = DB.Where("enabled = ?", true).Order("domain asc").Find(&routes).Error
	return routes, err
}

func GetProxyRouteByID(id uint) (*ProxyRoute, error) {
	route := &ProxyRoute{}
	err := DB.First(route, id).Error
	return route, err
}

func ListProxyRoutesByOriginID(originID uint) (routes []*ProxyRoute, err error) {
	err = DB.Where("origin_id = ?", originID).Order("id desc").Find(&routes).Error
	return routes, err
}

func (route *ProxyRoute) Insert() error {
	return DB.Create(route).Error
}

func (route *ProxyRoute) Update() error {
	return DB.Model(&ProxyRoute{}).Where("id = ?", route.ID).Updates(map[string]any{
		"domain":         route.Domain,
		"origin_id":      route.OriginID,
		"origin_url":     route.OriginURL,
		"origin_host":    route.OriginHost,
		"upstreams":      route.Upstreams,
		"enabled":        route.Enabled,
		"enable_https":   route.EnableHTTPS,
		"cert_id":        route.CertID,
		"redirect_http":  route.RedirectHTTP,
		"cache_enabled":  route.CacheEnabled,
		"cache_policy":   route.CachePolicy,
		"cache_rules":    route.CacheRules,
		"custom_headers": route.CustomHeaders,
		"remark":         route.Remark,
	}).Error
}

func (route *ProxyRoute) Delete() error {
	return DB.Delete(route).Error
}
