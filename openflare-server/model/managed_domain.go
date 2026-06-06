package model

import "time"

type ManagedDomain struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Domain    string    `json:"domain" gorm:"uniqueIndex;size:255;not null"`
	CertID    *uint     `json:"cert_id"`
	Enabled   bool      `json:"enabled" gorm:"not null;default:true"`
	Remark    string    `json:"remark" gorm:"size:255"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func ListManagedDomains() (domains []*ManagedDomain, err error) {
	err = DB.Order("id desc").Find(&domains).Error
	return domains, err
}

func ListEnabledManagedDomainsWithCertificate() (domains []*ManagedDomain, err error) {
	err = DB.Where("enabled = ? AND cert_id IS NOT NULL", true).Order("id desc").Find(&domains).Error
	return domains, err
}

func GetManagedDomainByID(id uint) (*ManagedDomain, error) {
	domain := &ManagedDomain{}
	err := DB.First(domain, id).Error
	return domain, err
}

func (domain *ManagedDomain) Insert() error {
	return DB.Create(domain).Error
}

func (domain *ManagedDomain) Update() error {
	return DB.Save(domain).Error
}

func (domain *ManagedDomain) Delete() error {
	return DB.Delete(domain).Error
}
