// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"errors"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
)

// DNSAccount OpenFlare DNS 账号实体。
type DNSAccount struct {
	ID            uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name          string    `json:"name" gorm:"size:255;not null"`
	Type          string    `json:"type" gorm:"size:64;not null"`
	Authorization string    `json:"-" gorm:"type:text;not null"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名。
func (DNSAccount) TableName() string {
	return "of_dns_accounts"
}

// ListDNSAccounts 列出全部 DNS 账号（授权信息不通过 JSON 暴露）。
func ListDNSAccounts(ctx context.Context) ([]DNSAccount, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var accounts []DNSAccount
	if err := conn.Order("id desc").Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

// GetDNSAccountByID 按 ID 查询 DNS 账号。
func GetDNSAccountByID(ctx context.Context, id uint) (*DNSAccount, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var account DNSAccount
	if err := conn.First(&account, id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// CreateDNSAccountRecord 创建 DNS 账号。
func CreateDNSAccountRecord(ctx context.Context, account *DNSAccount) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Create(account).Error
}

// SaveDNSAccount 保存 DNS 账号。
func SaveDNSAccount(ctx context.Context, account *DNSAccount) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Save(account).Error
}

// DeleteDNSAccountRecord 删除 DNS 账号。
func DeleteDNSAccountRecord(ctx context.Context, id uint) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Delete(&DNSAccount{}, id).Error
}
