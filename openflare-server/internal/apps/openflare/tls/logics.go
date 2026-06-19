// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
)

// CertificateInput TLS 证书创建/更新请求。
type CertificateInput struct {
	Name    string `json:"name"`
	CertPEM string `json:"cert_pem"`
	KeyPEM  string `json:"key_pem"`
	Remark  string `json:"remark"`
}

// CertificateContent TLS 证书 PEM 内容（仅 /content 端点返回）。
type CertificateContent struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	CertPEM       string `json:"cert_pem"`
	KeyPEM        string `json:"key_pem"`
	Remark        string `json:"remark"`
	Provider      string `json:"provider"`
	AcmeAccountID uint   `json:"acme_account_id"`
	DnsAccountID  uint   `json:"dns_account_id"`
	KeyAlgorithm  string `json:"key_algorithm"`
	AutoRenew     bool   `json:"auto_renew"`
	PrimaryDomain string `json:"primary_domain"`
	OtherDomains  string `json:"other_domains"`
	DisableCNAME  bool   `json:"disable_cname"`
	SkipDNS       bool   `json:"skip_dns"`
	DNS1          string `json:"dns1"`
	DNS2          string `json:"dns2"`
	ApplyStatus   string `json:"apply_status"`
	ApplyMessage  string `json:"apply_message"`
}

// ApplyInput ACME 证书申请/更新请求。
type ApplyInput struct {
	Name          string `json:"name"`
	Remark        string `json:"remark"`
	AcmeAccountID uint   `json:"acme_account_id"`
	DnsAccountID  uint   `json:"dns_account_id"`
	KeyAlgorithm  string `json:"key_algorithm"`
	AutoRenew     bool   `json:"auto_renew"`
	PrimaryDomain string `json:"primary_domain"`
	OtherDomains  string `json:"other_domains"`
	DisableCNAME  bool   `json:"disable_cname"`
	SkipDNS       bool   `json:"skip_dns"`
	DNS1          string `json:"dns1"`
	DNS2          string `json:"dns2"`
}

// DNSAccountInput DNS 账号创建/更新请求。
type DNSAccountInput struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Authorization string `json:"authorization"`
}

// ListCertificates 列出全部证书（不含 PEM）。
func ListCertificates(ctx context.Context) ([]model.TLSCertificate, error) {
	return model.ListTLSCertificates(ctx)
}

// GetCertificate 获取证书详情（不含 PEM）。
func GetCertificate(ctx context.Context, id uint) (*model.TLSCertificate, error) {
	return model.GetTLSCertificateByID(ctx, id)
}

// GetCertificateContent 获取证书 PEM 内容。
func GetCertificateContent(ctx context.Context, id uint) (*CertificateContent, error) {
	certificate, err := model.GetTLSCertificateByID(ctx, id)
	if err != nil {
		return nil, err
	}
	keyPEM, err := openSensitive(certificate.KeyPEM)
	if err != nil {
		return nil, err
	}
	return &CertificateContent{
		ID:            certificate.ID,
		Name:          certificate.Name,
		CertPEM:       certificate.CertPEM,
		KeyPEM:        keyPEM,
		Remark:        certificate.Remark,
		Provider:      certificate.Provider,
		AcmeAccountID: certificate.AcmeAccountID,
		DnsAccountID:  certificate.DnsAccountID,
		KeyAlgorithm:  certificate.KeyAlgorithm,
		AutoRenew:     certificate.AutoRenew,
		PrimaryDomain: certificate.PrimaryDomain,
		OtherDomains:  certificate.OtherDomains,
		DisableCNAME:  certificate.DisableCNAME,
		SkipDNS:       certificate.SkipDNS,
		DNS1:          certificate.DNS1,
		DNS2:          certificate.DNS2,
		ApplyStatus:   certificate.ApplyStatus,
		ApplyMessage:  certificate.ApplyMessage,
	}, nil
}

// CreateCertificate 从 PEM 创建证书。
func CreateCertificate(ctx context.Context, input CertificateInput) (*model.TLSCertificate, error) {
	certificate, err := buildCertificate(ctx, nil, input)
	if err != nil {
		return nil, err
	}
	if err = model.CreateTLSCertificateRecord(ctx, certificate); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errCertificateNameExists)
		}
		return nil, err
	}
	return sanitizeCertificateForResponse(certificate), nil
}

// CreateCertificateFromFiles 从上传文件创建证书。
func CreateCertificateFromFiles(ctx context.Context, name string, certFile *multipart.FileHeader, keyFile *multipart.FileHeader, remark string) (*model.TLSCertificate, error) {
	if certFile == nil || keyFile == nil {
		return nil, errors.New(errCertificateFilesRequired)
	}
	certContent, err := readMultipartFile(certFile)
	if err != nil {
		return nil, err
	}
	keyContent, err := readMultipartFile(keyFile)
	if err != nil {
		return nil, err
	}
	return CreateCertificate(ctx, CertificateInput{
		Name:    name,
		CertPEM: certContent,
		KeyPEM:  keyContent,
		Remark:  remark,
	})
}

// UpdateCertificate 更新上传证书。
func UpdateCertificate(ctx context.Context, id uint, input CertificateInput) (*model.TLSCertificate, error) {
	existing, err := model.GetTLSCertificateByID(ctx, id)
	if err != nil {
		return nil, err
	}
	certificate, err := buildCertificate(ctx, existing, input)
	if err != nil {
		return nil, err
	}
	if err = model.SaveTLSCertificate(ctx, certificate); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errCertificateNameExists)
		}
		return nil, err
	}
	return sanitizeCertificateForResponse(certificate), nil
}

// DeleteCertificate 删除证书。
func DeleteCertificate(ctx context.Context, id uint) error {
	if err := ensureCertificateNotReferenced(ctx, id); err != nil {
		return err
	}
	if _, err := model.GetTLSCertificateByID(ctx, id); err != nil {
		return err
	}
	return model.DeleteTLSCertificateRecord(ctx, id)
}

// ApplyCertificate 申请 ACME 证书。
func ApplyCertificate(ctx context.Context, input ApplyInput) (*model.TLSCertificate, error) {
	cert := &model.TLSCertificate{
		Provider: "acme",
		CertPEM:  " ",
		KeyPEM:   " ",
	}
	fillAcmeCertificateFields(cert, input)
	if cert.Name == "" {
		return nil, errors.New(errCertificateNameRequired)
	}
	if err := model.CreateTLSCertificateRecord(ctx, cert); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errCertificateNameExists)
		}
		return nil, err
	}

	go func(c *model.TLSCertificate) {
		_ = obtainTLSCertificate(context.Background(), c)
	}(cert)

	return sanitizeCertificateForResponse(cert), nil
}

// UpdateACMECertificate 更新 ACME 证书配置。
func UpdateACMECertificate(ctx context.Context, id uint, input ApplyInput) (*model.TLSCertificate, error) {
	cert, err := model.GetTLSCertificateByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cert.Provider != "acme" {
		return nil, errors.New(errCertificateOnlyACME)
	}
	fillAcmeCertificateFields(cert, input)
	if cert.Name == "" {
		return nil, errors.New(errCertificateNameRequired)
	}
	if err := model.SaveTLSCertificate(ctx, cert); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errCertificateNameExists)
		}
		return nil, err
	}

	go func(c *model.TLSCertificate) {
		_ = obtainTLSCertificate(context.Background(), c)
	}(cert)

	return sanitizeCertificateForResponse(cert), nil
}

// ConvertCertificateToACME 将上传证书转为 ACME 管理。
func ConvertCertificateToACME(ctx context.Context, id uint, input ApplyInput) (*model.TLSCertificate, error) {
	cert, err := model.GetTLSCertificateByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cert.Provider != "upload" {
		return nil, errors.New(errCertificateOnlyUploadConvert)
	}
	if cert.ApplyStatus == "applying" {
		return nil, errors.New(errCertificateAlreadyApplying)
	}
	fillAcmeCertificateFields(cert, input)
	if cert.Name == "" {
		return nil, errors.New(errCertificateNameRequired)
	}
	cert.ApplyMessage = ""
	if err := model.SaveTLSCertificate(ctx, cert); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errCertificateNameExists)
		}
		return nil, err
	}

	go func(c *model.TLSCertificate) {
		if err := obtainTLSCertificate(context.Background(), c); err != nil {
			return
		}
		latest, err := model.GetTLSCertificateByID(context.Background(), c.ID)
		if err != nil {
			return
		}
		latest.Provider = "acme"
		latest.ApplyStatus = "ready"
		latest.ApplyMessage = ""
		_ = model.SaveTLSCertificate(context.Background(), latest)
	}(cert)

	return sanitizeCertificateForResponse(cert), nil
}

// RenewCertificate 续期 ACME 证书。
func RenewCertificate(ctx context.Context, id uint) (*model.TLSCertificate, error) {
	cert, err := model.GetTLSCertificateByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cert.Provider != "acme" {
		return nil, errors.New(errCertificateOnlyACMERenew)
	}

	go func(c *model.TLSCertificate) {
		_ = obtainTLSCertificate(context.Background(), c)
	}(cert)

	cert.ApplyStatus = "applying"
	cert.ApplyMessage = ""
	if err := model.SaveTLSCertificate(ctx, cert); err != nil {
		return nil, err
	}
	return sanitizeCertificateForResponse(cert), nil
}

// ListDNSAccounts 列出 DNS 账号。
func ListDNSAccounts(ctx context.Context) ([]model.DNSAccount, error) {
	return model.ListDNSAccounts(ctx)
}

// CreateDNSAccount 创建 DNS 账号。
func CreateDNSAccount(ctx context.Context, input DNSAccountInput) (*model.DNSAccount, error) {
	authorization, err := sealSensitive(strings.TrimSpace(input.Authorization))
	if err != nil {
		return nil, err
	}
	account := &model.DNSAccount{
		Name:          strings.TrimSpace(input.Name),
		Type:          strings.TrimSpace(input.Type),
		Authorization: authorization,
	}
	if account.Name == "" || account.Type == "" || authorization == "" {
		return nil, errors.New("DNS 账号参数不完整")
	}
	if err := model.CreateDNSAccountRecord(ctx, account); err != nil {
		return nil, err
	}
	return sanitizeDNSAccountForResponse(account), nil
}

// UpdateDNSAccount 更新 DNS 账号。
func UpdateDNSAccount(ctx context.Context, id uint, input DNSAccountInput) (*model.DNSAccount, error) {
	account, err := model.GetDNSAccountByID(ctx, id)
	if err != nil {
		return nil, err
	}
	authorization, err := sealSensitive(strings.TrimSpace(input.Authorization))
	if err != nil {
		return nil, err
	}
	account.Name = strings.TrimSpace(input.Name)
	account.Type = strings.TrimSpace(input.Type)
	account.Authorization = authorization
	if account.Name == "" || account.Type == "" || authorization == "" {
		return nil, errors.New("DNS 账号参数不完整")
	}
	if err := model.SaveDNSAccount(ctx, account); err != nil {
		return nil, err
	}
	return sanitizeDNSAccountForResponse(account), nil
}

// DeleteDNSAccount 删除 DNS 账号。
func DeleteDNSAccount(ctx context.Context, id uint) error {
	if _, err := model.GetDNSAccountByID(ctx, id); err != nil {
		return err
	}
	count, err := model.CountTLSCertificatesByDNSAccountID(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New(errDNSAccountInUse)
	}
	return model.DeleteDNSAccountRecord(ctx, id)
}

// GetDefaultAcmeAccount 获取默认 ACME 账号。
func GetDefaultAcmeAccount(ctx context.Context) (*model.AcmeAccount, error) {
	account, err := model.GetDefaultAcmeAccount(ctx)
	if err != nil {
		return nil, err
	}
	return sanitizeAcmeAccountForResponse(account), nil
}

func buildCertificate(ctx context.Context, existing *model.TLSCertificate, input CertificateInput) (*model.TLSCertificate, error) {
	name := strings.TrimSpace(input.Name)
	certPEM := strings.TrimSpace(input.CertPEM)
	keyPEM := strings.TrimSpace(input.KeyPEM)
	remark := strings.TrimSpace(input.Remark)
	if name == "" {
		return nil, errors.New(errCertificateNameRequired)
	}
	if certPEM == "" || keyPEM == "" {
		return nil, errors.New(errCertificateContentRequired)
	}
	parsed, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errCertificateContentInvalid, err)
	}
	if len(parsed.Certificate) == 0 {
		return nil, errors.New(errCertificateContentInvalid)
	}
	leaf, err := parseLeafCertificate(certPEM)
	if err != nil {
		return nil, err
	}
	sealedKey, err := sealSensitive(keyPEM)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		existing = &model.TLSCertificate{
			Provider:    "upload",
			ApplyStatus: "ready",
		}
	}
	existing.Name = name
	existing.CertPEM = certPEM
	existing.KeyPEM = sealedKey
	existing.NotBefore = leaf.NotBefore
	existing.NotAfter = leaf.NotAfter
	existing.Remark = remark
	return existing, nil
}

func fillAcmeCertificateFields(cert *model.TLSCertificate, input ApplyInput) {
	cert.Name = strings.TrimSpace(input.Name)
	cert.Remark = strings.TrimSpace(input.Remark)
	cert.AcmeAccountID = input.AcmeAccountID
	cert.DnsAccountID = input.DnsAccountID
	cert.KeyAlgorithm = input.KeyAlgorithm
	cert.AutoRenew = input.AutoRenew
	cert.PrimaryDomain = strings.TrimSpace(input.PrimaryDomain)
	cert.OtherDomains = strings.TrimSpace(input.OtherDomains)
	cert.DisableCNAME = input.DisableCNAME
	cert.SkipDNS = input.SkipDNS
	cert.DNS1 = strings.TrimSpace(input.DNS1)
	cert.DNS2 = strings.TrimSpace(input.DNS2)
	cert.ApplyStatus = "applying"
}

func ensureCertificateNotReferenced(ctx context.Context, id uint) error {
	routes, err := model.ListTLSProxyRouteRefs(ctx)
	if err != nil {
		return err
	}
	for _, route := range routes {
		if route.CertID != nil && *route.CertID == id {
			return errors.New(errCertificateDeleteReferenced)
		}
		if strings.TrimSpace(route.CertIDs) == "" {
			continue
		}
		var certIDs []uint
		if err := json.Unmarshal([]byte(route.CertIDs), &certIDs); err != nil {
			return fmt.Errorf("proxy route %d cert_ids payload is invalid: %w", route.ID, err)
		}
		for _, certID := range certIDs {
			if certID == id {
				return errors.New(errCertificateDeleteReferenced)
			}
		}
		domainCertIDs, err := decodeStoredDomainCertIDs(route.DomainCertIDs, 0)
		if err != nil {
			return fmt.Errorf("proxy route %d domain_cert_ids payload is invalid: %w", route.ID, err)
		}
		for _, certID := range domainCertIDs {
			if certID == id {
				return errors.New(errCertificateDeleteReferenced)
			}
		}
	}
	return nil
}

func sanitizeCertificateForResponse(certificate *model.TLSCertificate) *model.TLSCertificate {
	if certificate == nil {
		return nil
	}
	copy := *certificate
	copy.CertPEM = ""
	copy.KeyPEM = ""
	return &copy
}

func sanitizeDNSAccountForResponse(account *model.DNSAccount) *model.DNSAccount {
	if account == nil {
		return nil
	}
	copy := *account
	copy.Authorization = ""
	return &copy
}

func sanitizeAcmeAccountForResponse(account *model.AcmeAccount) *model.AcmeAccount {
	if account == nil {
		return nil
	}
	copy := *account
	copy.PrivateKey = ""
	return &copy
}
