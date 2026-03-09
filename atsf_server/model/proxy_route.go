package model

import "time"

type ProxyRoute struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Domain    string    `json:"domain" gorm:"uniqueIndex;size:255;not null"`
	OriginURL string    `json:"origin_url" gorm:"size:2048;not null"`
	Enabled   bool      `json:"enabled" gorm:"not null;default:true"`
	Remark    string    `json:"remark" gorm:"size:255"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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

func (route *ProxyRoute) Insert() error {
	return DB.Create(route).Error
}

func (route *ProxyRoute) Update() error {
	return DB.Model(route).Select("domain", "origin_url", "enabled", "remark").Updates(route).Error
}

func (route *ProxyRoute) Delete() error {
	return DB.Delete(route).Error
}
