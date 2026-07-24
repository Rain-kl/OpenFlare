// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// GetAcmeAccountByID 按 ID 查询 ACME 账号。
func GetAcmeAccountByID(ctx context.Context, id uint) (*model.AcmeAccount, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var account model.AcmeAccount
	if err := conn.First(&account, id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// CreateAcmeAccountRecord 创建 ACME 账号。
func CreateAcmeAccountRecord(ctx context.Context, account *model.AcmeAccount) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Create(account).Error
}

// SaveAcmeAccount 保存 ACME 账号。
func SaveAcmeAccount(ctx context.Context, account *model.AcmeAccount) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Save(account).Error
}

// GetDefaultAcmeAccount 获取默认 ACME 账号，不存在时创建占位记录。
func GetDefaultAcmeAccount(ctx context.Context) (*model.AcmeAccount, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var account model.AcmeAccount
	err := conn.Order("id asc").First(&account).Error
	if err == nil {
		return &account, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	account = model.AcmeAccount{
		Email: "admin@openflare.dev",
	}
	if err = conn.Create(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}
