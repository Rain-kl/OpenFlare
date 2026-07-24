// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"
)

// TLSCertificate OpenFlare TLS 证书实体。
type TLSCertificate struct {
	ID            uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name          string    `json:"name" gorm:"uniqueIndex;size:255;not null"`
	CertPEM       string    `json:"-" gorm:"type:text;not null"`
	KeyPEM        string    `json:"-" gorm:"type:text;not null"`
	NotBefore     time.Time `json:"not_before"`
	NotAfter      time.Time `json:"not_after"`
	Remark        string    `json:"remark" gorm:"size:255"`
	Provider      string    `json:"provider" gorm:"size:64;default:upload"`
	AcmeAccountID uint      `json:"acme_account_id"`
	DNSAccountID  uint      `json:"dns_account_id"`
	KeyAlgorithm  string    `json:"key_algorithm" gorm:"size:32"`
	AutoRenew     bool      `json:"auto_renew"`
	PrimaryDomain string    `json:"primary_domain" gorm:"size:255"`
	OtherDomains  string    `json:"other_domains" gorm:"type:text"`
	DisableCNAME  bool      `json:"disable_cname"`
	SkipDNS       bool      `json:"skip_dns"`
	DNS1          string    `json:"dns1" gorm:"size:128"`
	DNS2          string    `json:"dns2" gorm:"size:128"`
	ApplyStatus   string    `json:"apply_status" gorm:"size:64;default:ready"`
	ApplyMessage  string    `json:"apply_message" gorm:"type:text"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名。
func (TLSCertificate) TableName() string {
	return "of_tls_certificates"
}

// TLSProxyRouteRef 删除证书时检查代理规则引用的最小字段集。
type TLSProxyRouteRef struct {
	ID            uint   `gorm:"column:id;primaryKey"`
	CertID        *uint  `gorm:"column:cert_id"`
	CertIDs       string `gorm:"column:cert_ids"`
	DomainCertIDs string `gorm:"column:domain_cert_ids"`
}

// TableName 表名。
func (TLSProxyRouteRef) TableName() string {
	return tableOfProxyRoutes
}
