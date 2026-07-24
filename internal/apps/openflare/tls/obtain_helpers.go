// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"context"
	"fmt"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/tls/acme"
	"github.com/Rain-kl/Wavelet/internal/model"
)

func resolveAcmeAccount(ctx context.Context, cert *model.TLSCertificate) (*model.AcmeAccount, error) {
	acmeAccount, err := repository.GetAcmeAccountByID(ctx, cert.AcmeAccountID)
	if err == nil {
		return acmeAccount, nil
	}
	acmeAccount, err = repository.GetDefaultAcmeAccount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get ACME account: %w", err)
	}
	cert.AcmeAccountID = acmeAccount.ID
	if err := repository.SaveTLSCertificate(ctx, cert); err != nil {
		return nil, err
	}
	return acmeAccount, nil
}

func persistAcmeAccountUpdates(
	ctx context.Context,
	cert *model.TLSCertificate,
	acmeAccount *model.AcmeAccount,
	newAccountURL, newPrivateKeyPEM, acmePrivateKey string,
) error {
	accountChanged := (newPrivateKeyPEM != "" && acmePrivateKey != newPrivateKeyPEM) ||
		(newAccountURL != "" && acmeAccount.URL != newAccountURL)
	if !accountChanged {
		return nil
	}
	if newPrivateKeyPEM != "" && acmePrivateKey != newPrivateKeyPEM {
		sealedKey, sealErr := sealSensitive(newPrivateKeyPEM)
		if sealErr != nil {
			return fmt.Errorf("failed to seal ACME account key: %w", sealErr)
		}
		acmeAccount.PrivateKey = sealedKey
	}
	if newAccountURL != "" {
		acmeAccount.URL = newAccountURL
	}
	if acmeAccount.ID == 0 {
		if dbErr := repository.CreateAcmeAccountRecord(ctx, acmeAccount); dbErr != nil {
			return fmt.Errorf("failed to create ACME account: %w", dbErr)
		}
	} else if dbErr := repository.SaveAcmeAccount(ctx, acmeAccount); dbErr != nil {
		return fmt.Errorf("failed to save ACME account: %w", dbErr)
	}
	cert.AcmeAccountID = acmeAccount.ID
	return repository.SaveTLSCertificate(ctx, cert)
}

func saveObtainedCertificate(ctx context.Context, cert *model.TLSCertificate, result *acme.CertificateResult) error {
	sealedKey, err := sealSensitive(result.KeyPEM)
	if err != nil {
		return fmt.Errorf("failed to seal certificate key: %w", err)
	}
	cert.CertPEM = result.CertPEM
	cert.KeyPEM = sealedKey
	cert.NotBefore = result.NotBefore
	cert.NotAfter = result.NotAfter
	cert.ApplyStatus = tlsApplyStatusReady
	cert.ApplyMessage = ""
	return repository.SaveTLSCertificate(ctx, cert)
}
