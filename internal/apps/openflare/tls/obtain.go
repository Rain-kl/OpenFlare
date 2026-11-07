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
	task.AppendLog(ctx, "[Renewal] Starting renewal; setting application status to applying...")
	cert.ApplyStatus = tlsApplyStatusApplying
	if err := repository.SaveTLSCertificate(ctx, cert); err != nil {
		return err
	}

	task.AppendLog(ctx, "[Renewal] Resolving ACME account...")
	acmeAccount, err := resolveAcmeAccount(ctx, cert)
	if err != nil {
		return updateCertError(ctx, cert, fmt.Sprintf("Failed to get ACME account: %v", err))
	}

	task.AppendLog(ctx, "[Renewal] Resolving DNS account information (ID=%d)...", cert.DNSAccountID)
	dnsAccount, err := repository.GetDNSAccountByID(ctx, cert.DNSAccountID)
	if err != nil {
		return updateCertError(ctx, cert, fmt.Sprintf("Failed to get DNS account: %v", err))
	}

	task.AppendLog(ctx, "[Renewal] Decrypting DNS account credentials and ACME account private key...")
	dnsAuth, err := openSensitive(dnsAccount.Authorization)
	if err != nil {
		return updateCertError(ctx, cert, fmt.Sprintf("Failed to decrypt DNS credentials: %v", err))
	}

	acmePrivateKey, err := openSensitive(acmeAccount.PrivateKey)
	if err != nil {
		return updateCertError(ctx, cert, fmt.Sprintf("Failed to decrypt ACME account key: %v", err))
	}

	domains := splitAcmeDomains(cert.PrimaryDomain, cert.OtherDomains)
	task.AppendLog(ctx, "[Renewal] Requested domains: %v", domains)

	task.AppendLog(ctx, "[Renewal] Requesting SSL certificate issuance through the ACME DNS-01 challenge; please wait...")
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

	task.AppendLog(ctx, "[Renewal] Saving ACME account changes...")
	if err := persistAcmeAccountUpdates(ctx, cert, acmeAccount, newAccountURL, newPrivateKeyPEM, acmePrivateKey); err != nil {
		return updateCertError(ctx, cert, err.Error())
	}

	if err != nil {
		return updateCertError(ctx, cert, err.Error())
	}

	task.AppendLog(ctx, "[Renewal] Certificate issued; securely storing certificate and private key in the database...")
	if err := saveObtainedCertificate(ctx, cert, result); err != nil {
		return updateCertError(ctx, cert, err.Error())
	}
	task.AppendLog(ctx, "[Renewal] Certificate data saved")
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
	for domain := range strings.SplitSeq(otherDomains, separator) {
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
