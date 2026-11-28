// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
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

// DeleteDNSAccountRecord 删除 DNS 账号。
func DeleteDNSAccountRecord(ctx context.Context, id uint) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("dns_account_id = ?", id).Delete(&model.CFConnection{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.DNSAccount{}, id).Error
	})
}

// SaveDNSAccountVerification publishes the latest probe for the same credentials.
func SaveDNSAccountVerification(ctx context.Context, account *model.DNSAccount, started time.Time, status string, scopes []model.DNSAccountScope) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	result := conn.Model(account).
		Where(`"authorization" = ? AND type = ?`, account.Authorization, account.Type).
		Where("last_checked_at IS NULL OR last_checked_at <= ?", started).
		Select("verification_status", "last_checked_at")
	update := model.DNSAccount{VerificationStatus: status, LastCheckedAt: &started}
	if status == model.DNSAccountStatusReady {
		update.Scopes = scopes
		result = result.Select("verification_status", "last_checked_at", "scopes")
	}
	return result.Updates(&update).Error
}

// ReplaceDNSAccountCredential updates a saved credential without recreating deleted rows.
func ReplaceDNSAccountCredential(ctx context.Context, account *model.DNSAccount) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Model(account).Select("name", "type", "authorization", "scopes", "verification_status", "last_checked_at").Updates(account).Error
}

// DNSAccountHasPointingRecords reports whether a credential owns managed DNS records.
func DNSAccountHasPointingRecords(ctx context.Context, id uint) (bool, error) {
	var count int64
	err := db.DB(ctx).Model(&model.CFPointingMember{}).Where("dns_account_id = ?", id).Limit(1).Count(&count).Error
	return count > 0, err
}
