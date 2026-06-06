package model

import "time"

type Origin struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:255;not null"`
	Address   string    `json:"address" gorm:"uniqueIndex;size:255;not null"`
	Remark    string    `json:"remark" gorm:"size:255"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type OriginRouteCount struct {
	OriginID   uint  `json:"origin_id"`
	RouteCount int64 `json:"route_count"`
}

func ListOrigins() (origins []*Origin, err error) {
	err = DB.Order("id desc").Find(&origins).Error
	return origins, err
}

func GetOriginByID(id uint) (*Origin, error) {
	origin := &Origin{}
	err := DB.First(origin, id).Error
	return origin, err
}

func GetOriginByAddress(address string) (*Origin, error) {
	origin := &Origin{}
	err := DB.Where("address = ?", address).First(origin).Error
	return origin, err
}

func ListOriginRouteCounts() ([]OriginRouteCount, error) {
	result := make([]OriginRouteCount, 0)
	err := DB.Model(&ProxyRoute{}).
		Select("origin_id, COUNT(*) AS route_count").
		Where("origin_id IS NOT NULL").
		Group("origin_id").
		Scan(&result).Error
	return result, err
}

func (origin *Origin) Insert() error {
	return DB.Create(origin).Error
}

func (origin *Origin) Update() error {
	return DB.Save(origin).Error
}

func (origin *Origin) Delete() error {
	return DB.Delete(origin).Error
}
