// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// ListDNSAccounts 列出全部 DNS 账号（授权信息不通过 JSON 暴露）。
func ListDNSAccounts(ctx context.Context) ([]model.DNSAccount, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var accounts []model.DNSAccount
	if err := conn.Order("id desc").Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

// GetDNSAccountByID 按 ID 查询 DNS 账号。
func GetDNSAccountByID(ctx context.Context, id uint) (*model.DNSAccount, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var account model.DNSAccount
	if err := conn.First(&account, id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// CreateDNSAccountRecord 创建 DNS 账号。
func CreateDNSAccountRecord(ctx context.Context, account *model.DNSAccount) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Create(account).Error
}

// SaveDNSAccount 保存 DNS 账号。
func SaveDNSAccount(ctx context.Context, account *model.DNSAccount) error {
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
	return conn.Delete(&model.DNSAccount{}, id).Error
}
