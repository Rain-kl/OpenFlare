// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/tls/acme"
	"github.com/Rain-kl/Wavelet/internal/model"
)

const acmeRenewLeadTime = 7 * 24 * time.Hour

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
	cert.ApplyStatus = "applying"
	if err := model.SaveTLSCertificate(ctx, cert); err != nil {
		return err
	}

	acmeAccount, err := model.GetAcmeAccountByID(ctx, cert.AcmeAccountID)
	if err != nil {
		acmeAccount, err = model.GetDefaultAcmeAccount(ctx)
		if err != nil {
			return updateCertError(ctx, cert, fmt.Sprintf("Failed to get ACME account: %v", err))
		}
		cert.AcmeAccountID = acmeAccount.ID
		if err := model.SaveTLSCertificate(ctx, cert); err != nil {
			return err
		}
	}

	dnsAccount, err := model.GetDNSAccountByID(ctx, cert.DnsAccountID)
	if err != nil {
		return updateCertError(ctx, cert, fmt.Sprintf("Failed to get DNS account: %v", err))
	}

	dnsAuth, err := openSensitive(dnsAccount.Authorization)
	if err != nil {
		return updateCertError(ctx, cert, fmt.Sprintf("Failed to decrypt DNS credentials: %v", err))
	}

	acmePrivateKey, err := openSensitive(acmeAccount.PrivateKey)
	if err != nil {
		return updateCertError(ctx, cert, fmt.Sprintf("Failed to decrypt ACME account key: %v", err))
	}

	domains := splitAcmeDomains(cert.PrimaryDomain, cert.OtherDomains)

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

	if (newPrivateKeyPEM != "" && acmePrivateKey != newPrivateKeyPEM) || (newAccountURL != "" && acmeAccount.URL != newAccountURL) {
		if newPrivateKeyPEM != "" {
			sealedKey, sealErr := sealSensitive(newPrivateKeyPEM)
			if sealErr != nil {
				return updateCertError(ctx, cert, fmt.Sprintf("Failed to seal ACME account key: %v", sealErr))
			}
			acmeAccount.PrivateKey = sealedKey
		}
		if newAccountURL != "" {
			acmeAccount.URL = newAccountURL
		}
		if acmeAccount.ID == 0 {
			if dbErr := model.CreateAcmeAccountRecord(ctx, acmeAccount); dbErr != nil {
				return updateCertError(ctx, cert, fmt.Sprintf("Failed to create ACME account: %v", dbErr))
			}
		} else if dbErr := model.SaveAcmeAccount(ctx, acmeAccount); dbErr != nil {
			return updateCertError(ctx, cert, fmt.Sprintf("Failed to save ACME account: %v", dbErr))
		}
		cert.AcmeAccountID = acmeAccount.ID
		if err := model.SaveTLSCertificate(ctx, cert); err != nil {
			return err
		}
	}

	if err != nil {
		return updateCertError(ctx, cert, err.Error())
	}

	sealedKey, err := sealSensitive(result.KeyPEM)
	if err != nil {
		return updateCertError(ctx, cert, fmt.Sprintf("Failed to seal certificate key: %v", err))
	}

	cert.CertPEM = result.CertPEM
	cert.KeyPEM = sealedKey
	cert.NotBefore = result.NotBefore
	cert.NotAfter = result.NotAfter
	cert.ApplyStatus = "ready"
	cert.ApplyMessage = ""

	return model.SaveTLSCertificate(ctx, cert)
}

func updateCertError(ctx context.Context, cert *model.TLSCertificate, message string) error {
	cert.ApplyStatus = "error"
	cert.ApplyMessage = message
	if err := model.SaveTLSCertificate(ctx, cert); err != nil {
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
		if !cert.AutoRenew || cert.Provider != "acme" || cert.ApplyStatus == "applying" {
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
