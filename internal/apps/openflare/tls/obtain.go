// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/tls/acme"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
)

const (
	acmeRenewLeadTime      = 7 * 24 * time.Hour
	tlsProviderACME        = "acme"
	tlsApplyStatusApplying = "applying"
	tlsApplyStatusReady    = "ready"
)

var obtainTLSCertificate = obtainCertificate

// SetObtainCertificateFuncForTest swaps the async obtain implementation for tests.
func SetObtainCertificateFuncForTest(fn func(context.Context, *model.TLSCertificate) error) func() {
	previous := obtainTLSCertificate
	obtainTLSCertificate = fn
	return func() {
		obtainTLSCertificate = previous
	}
}

func obtainCertificate(ctx context.Context, cert *model.TLSCertificate) error {
	task.AppendLog(ctx, "【续签任务】开始续签，设置申请状态为 applying...")
	cert.ApplyStatus = tlsApplyStatusApplying
	if err := repository.SaveTLSCertificate(ctx, cert); err != nil {
		return err
	}

	task.AppendLog(ctx, "【续签任务】正在解析 ACME 账户...")
	acmeAccount, err := resolveAcmeAccount(ctx, cert)
	if err != nil {
		return updateCertError(ctx, cert, fmt.Sprintf("Failed to get ACME account: %v", err))
	}

	task.AppendLog(ctx, "【续签任务】正在解析 DNS 账户信息 (ID=%d)...", cert.DNSAccountID)
	dnsAccount, err := repository.GetDNSAccountByID(ctx, cert.DNSAccountID)
	if err != nil {
		return updateCertError(ctx, cert, fmt.Sprintf("Failed to get DNS account: %v", err))
	}

	task.AppendLog(ctx, "【续签任务】正在解密 DNS 账号凭据及 ACME 账户私钥...")
	dnsAuth, err := openSensitive(dnsAccount.Authorization)
	if err != nil {
		return updateCertError(ctx, cert, fmt.Sprintf("Failed to decrypt DNS credentials: %v", err))
	}

	acmePrivateKey, err := openSensitive(acmeAccount.PrivateKey)
	if err != nil {
		return updateCertError(ctx, cert, fmt.Sprintf("Failed to decrypt ACME account key: %v", err))
	}

	domains := splitAcmeDomains(cert.PrimaryDomain, cert.OtherDomains)
	task.AppendLog(ctx, "【续签任务】待申请的域名列表: %v", domains)

	task.AppendLog(ctx, "【续签任务】正在调用 ACME 客户端（通过 DNS-01 挑战）发起 SSL 证书签发请求，请稍候...")
	newAccountURL, newPrivateKeyPEM, result, err := acme.ObtainSSL(
		acmeAccount.Email,
		acmePrivateKey,
		acmeAccount.URL,
		dnsAccount.Type,
		dnsAuth,
		cert.DNS1,
		cert.DNS2,
		cert.DisableCNAME,
		cert.SkipDNS,
		cert.KeyAlgorithm,
		domains,
	)

	task.AppendLog(ctx, "【续签任务】正在保存 ACME 账户可能的变更...")
	if err := persistAcmeAccountUpdates(ctx, cert, acmeAccount, newAccountURL, newPrivateKeyPEM, acmePrivateKey); err != nil {
		return updateCertError(ctx, cert, err.Error())
	}

	if err != nil {
		return updateCertError(ctx, cert, err.Error())
	}

	task.AppendLog(ctx, "【续签任务】证书签发成功，正在将证书内容与私钥安全写入数据库...")
	if err := saveObtainedCertificate(ctx, cert, result); err != nil {
		return updateCertError(ctx, cert, err.Error())
	}
	task.AppendLog(ctx, "【续签任务】证书数据存储完成！")
	return nil
}

func updateCertError(ctx context.Context, cert *model.TLSCertificate, message string) error {
	cert.ApplyStatus = "error"
	cert.ApplyMessage = message
	if err := repository.SaveTLSCertificate(ctx, cert); err != nil {
		return err
	}
	return fmt.Errorf("%s", message)
}

func splitAcmeDomains(primaryDomain, otherDomains string) []string {
	primaryDomain = strings.TrimSpace(primaryDomain)
	domains := []string{}
	if primaryDomain != "" {
		domains = append(domains, primaryDomain)
	}
	otherDomains = strings.TrimSpace(otherDomains)
	if otherDomains == "" {
		return domains
	}

	separator := "\n"
	if !strings.Contains(otherDomains, "\n") && strings.Contains(otherDomains, ",") {
		separator = ","
	}
	for _, domain := range strings.Split(otherDomains, separator) {
		domain = strings.TrimSpace(domain)
		if domain != "" {
			domains = append(domains, domain)
		}
	}
	return domains
}

// CertificatesDueForRenewal returns ACME certificates that should be renewed at the given time.
func CertificatesDueForRenewal(certificates []model.TLSCertificate, now time.Time) []model.TLSCertificate {
	due := make([]model.TLSCertificate, 0)
	for _, cert := range certificates {
		if !cert.AutoRenew || cert.Provider != tlsProviderACME || cert.ApplyStatus == tlsApplyStatusApplying {
			continue
		}
		if cert.NotAfter.IsZero() {
			continue
		}
		if cert.NotAfter.Sub(now) < acmeRenewLeadTime {
			due = append(due, cert)
		}
	}
	return due
}
