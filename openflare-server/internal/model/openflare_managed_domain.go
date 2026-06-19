// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"errors"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
)

// ManagedDomain OpenFlare 托管域名实体。
type ManagedDomain struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Domain    string    `json:"domain" gorm:"uniqueIndex;size:255;not null"`
	CertID    *uint     `json:"cert_id"`
	Enabled   bool      `json:"enabled" gorm:"not null;default:true"`
	Remark    string    `json:"remark" gorm:"size:255"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名。
func (ManagedDomain) TableName() string {
	return "of_managed_domains"
}

// ListManagedDomains 列出全部托管域名。
func ListManagedDomains(ctx context.Context) ([]ManagedDomain, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var domains []ManagedDomain
	if err := conn.Order("id desc").Find(&domains).Error; err != nil {
		return nil, err
	}
	return domains, nil
}

// ListEnabledManagedDomainsWithCertificate 列出已启用且绑定证书的托管域名。
func ListEnabledManagedDomainsWithCertificate(ctx context.Context) ([]ManagedDomain, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var domains []ManagedDomain
	if err := conn.Where("enabled = ? AND cert_id IS NOT NULL", true).Order("id desc").Find(&domains).Error; err != nil {
		return nil, err
	}
	return domains, nil
}

// GetManagedDomainByID 按 ID 查询托管域名。
func GetManagedDomainByID(ctx context.Context, id uint) (*ManagedDomain, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var domain ManagedDomain
	if err := conn.First(&domain, id).Error; err != nil {
		return nil, err
	}
	return &domain, nil
}

// CreateManagedDomainRecord 创建托管域名。
func CreateManagedDomainRecord(ctx context.Context, domain *ManagedDomain) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Create(domain).Error
}

// SaveManagedDomain 保存托管域名。
func SaveManagedDomain(ctx context.Context, domain *ManagedDomain) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Save(domain).Error
}

// DeleteManagedDomainRecord 删除托管域名。
func DeleteManagedDomainRecord(ctx context.Context, id uint) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Delete(&ManagedDomain{}, id).Error
}
