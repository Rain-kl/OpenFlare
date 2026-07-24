// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// HasTLSProxyRoutesTable 判断代理规则表是否已迁移。
func HasTLSProxyRoutesTable(ctx context.Context) bool {
	return db.DB(ctx).Migrator().HasTable(&model.TLSProxyRouteRef{})
}

// ListTLSCertificates 列出全部证书（不含 PEM 敏感字段的 JSON 暴露由 struct tag 控制）。
func ListTLSCertificates(ctx context.Context) ([]model.TLSCertificate, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var certificates []model.TLSCertificate
	if err := conn.Order("id desc").Find(&certificates).Error; err != nil {
		return nil, err
	}
	return certificates, nil
}

// GetTLSCertificateByID 按 ID 查询证书。
func GetTLSCertificateByID(ctx context.Context, id uint) (*model.TLSCertificate, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var certificate model.TLSCertificate
	if err := conn.First(&certificate, id).Error; err != nil {
		return nil, err
	}
	return &certificate, nil
}

// CreateTLSCertificateRecord 创建证书记录。
func CreateTLSCertificateRecord(ctx context.Context, certificate *model.TLSCertificate) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Create(certificate).Error
}

// SaveTLSCertificate 保存证书记录。
func SaveTLSCertificate(ctx context.Context, certificate *model.TLSCertificate) error {
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
	return conn.Delete(&model.TLSCertificate{}, id).Error
}

// CountTLSCertificatesByDNSAccountID 统计引用指定 DNS 账号的证书数量。
func CountTLSCertificatesByDNSAccountID(ctx context.Context, dnsAccountID uint) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}
	var count int64
	if err := conn.Model(&model.TLSCertificate{}).Where("dns_account_id = ?", dnsAccountID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// ListTLSProxyRouteRefs 列出代理规则证书引用字段。
func ListTLSProxyRouteRefs(ctx context.Context) ([]model.TLSProxyRouteRef, error) {
	if !HasTLSProxyRoutesTable(ctx) {
		return nil, nil
	}
	var routes []model.TLSProxyRouteRef
	if err := db.DB(ctx).Order("id asc").Find(&routes).Error; err != nil {
		return nil, err
	}
	return routes, nil
}
