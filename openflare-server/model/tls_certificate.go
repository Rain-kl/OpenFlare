package model

import "time"

type TLSCertificate struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	Name          string    `json:"name" gorm:"uniqueIndex;size:255;not null"`
	CertPEM       string    `json:"-" gorm:"type:text;not null"`
	KeyPEM        string    `json:"-" gorm:"type:text;not null"`
	NotBefore     time.Time `json:"not_before"`
	NotAfter      time.Time `json:"not_after"`
	Remark        string    `json:"remark" gorm:"size:255"`
	Provider      string    `json:"provider" gorm:"size:64;default:'upload'"` // upload, acme
	AcmeAccountID uint      `json:"acme_account_id"`
	DnsAccountID  uint      `json:"dns_account_id"`
	KeyAlgorithm  string    `json:"key_algorithm" gorm:"size:32"`
	AutoRenew     bool      `json:"auto_renew"`
	PrimaryDomain string    `json:"primary_domain" gorm:"size:255"`
	OtherDomains  string    `json:"other_domains" gorm:"type:text"`
	DisableCNAME  bool      `json:"disable_cname"`
	SkipDNS       bool      `json:"skip_dns"`
	DNS1          string    `json:"dns1" gorm:"size:128"`
	DNS2          string    `json:"dns2" gorm:"size:128"`
	ApplyStatus   string    `json:"apply_status" gorm:"size:64;default:'ready'"`
	ApplyMessage  string    `json:"apply_message" gorm:"type:text"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func ListTLSCertificates() (certificates []*TLSCertificate, err error) {
	err = DB.Order("id desc").Find(&certificates).Error
	return certificates, err
}

func GetTLSCertificateByID(id uint) (*TLSCertificate, error) {
	certificate := &TLSCertificate{}
	err := DB.First(certificate, id).Error
	return certificate, err
}

func (certificate *TLSCertificate) Insert() error {
	return DB.Create(certificate).Error
}

func (certificate *TLSCertificate) Update() error {
	return DB.Save(certificate).Error
}

func (certificate *TLSCertificate) Delete() error {
	return DB.Delete(certificate).Error
}
