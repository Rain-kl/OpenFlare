// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"errors"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
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
	DnsAccountID  uint      `json:"dns_account_id"`
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
	return "of_proxy_routes"
}

// HasTLSProxyRoutesTable 判断代理规则表是否已迁移。
func HasTLSProxyRoutesTable(ctx context.Context) bool {
	return db.DB(ctx).Migrator().HasTable(&TLSProxyRouteRef{})
}

// ListTLSCertificates 列出全部证书（不含 PEM 敏感字段的 JSON 暴露由 struct tag 控制）。
func ListTLSCertificates(ctx context.Context) ([]TLSCertificate, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var certificates []TLSCertificate
	if err := conn.Order("id desc").Find(&certificates).Error; err != nil {
		return nil, err
	}
	return certificates, nil
}

// GetTLSCertificateByID 按 ID 查询证书。
func GetTLSCertificateByID(ctx context.Context, id uint) (*TLSCertificate, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var certificate TLSCertificate
	if err := conn.First(&certificate, id).Error; err != nil {
		return nil, err
	}
	return &certificate, nil
}

// CreateTLSCertificateRecord 创建证书记录。
func CreateTLSCertificateRecord(ctx context.Context, certificate *TLSCertificate) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Create(certificate).Error
}

// SaveTLSCertificate 保存证书记录。
func SaveTLSCertificate(ctx context.Context, certificate *TLSCertificate) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Save(certificate).Error
}

// DeleteTLSCertificateRecord 删除证书记录。
func DeleteTLSCertificateRecord(ctx context.Context, id uint) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Delete(&TLSCertificate{}, id).Error
}

// CountTLSCertificatesByDNSAccountID 统计引用指定 DNS 账号的证书数量。
func CountTLSCertificatesByDNSAccountID(ctx context.Context, dnsAccountID uint) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}
	var count int64
	if err := conn.Model(&TLSCertificate{}).Where("dns_account_id = ?", dnsAccountID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// ListTLSProxyRouteRefs 列出代理规则证书引用字段。
func ListTLSProxyRouteRefs(ctx context.Context) ([]TLSProxyRouteRef, error) {
	if !HasTLSProxyRoutesTable(ctx) {
		return nil, nil
	}
	var routes []TLSProxyRouteRef
	if err := db.DB(ctx).Order("id asc").Find(&routes).Error; err != nil {
		return nil, err
	}
	return routes, nil
}
