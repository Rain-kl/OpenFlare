package model

import "time"

type TLSCertificate struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex;size:255;not null"`
	CertPEM   string    `json:"cert_pem" gorm:"type:text;not null"`
	KeyPEM    string    `json:"key_pem" gorm:"type:text;not null"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
	Remark    string    `json:"remark" gorm:"size:255"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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

func (certificate *TLSCertificate) Delete() error {
	return DB.Delete(certificate).Error
}
