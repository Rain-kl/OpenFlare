package service

import (
	"fmt"
	"strings"

	"github.com/rain-kl/openflare/openflare-server/model"
	"github.com/rain-kl/openflare/openflare-server/utils/acme"
)

func ObtainSSL(cert *model.TLSCertificate) error {
	cert.ApplyStatus = "applying"
	model.DB.Save(cert)

	acmeAccount, err := model.GetAcmeAccountByID(cert.AcmeAccountID)
	if err != nil {
		// Fallback to default ACME account if the specified one is not found (e.g. ID 0 during testing)
		acmeAccount, err = model.GetDefaultAcmeAccount()
		if err != nil {
			updateCertError(cert, fmt.Sprintf("Failed to get ACME account: %v", err))
			return err
		}
		// Self-heal the certificate
		cert.AcmeAccountID = acmeAccount.ID
		model.DB.Save(cert)
	}

	dnsAccount, err := model.GetDnsAccountByID(cert.DnsAccountID)
	if err != nil {
		updateCertError(cert, fmt.Sprintf("Failed to get DNS account: %v", err))
		return err
	}

	domains := []string{cert.PrimaryDomain}
	if cert.OtherDomains != "" {
		for _, d := range strings.Split(cert.OtherDomains, "\n") {
			d = strings.TrimSpace(d)
			if d != "" {
				domains = append(domains, d)
			}
		}
	}

	newAccountURL, newPrivateKeyPEM, result, err := acme.ObtainSSL(
		acmeAccount.Email,
		acmeAccount.PrivateKey,
		acmeAccount.URL,
		dnsAccount.Type,
		dnsAccount.Authorization,
		cert.DNS1,
		cert.DNS2,
		cert.DisableCNAME,
		cert.SkipDNS,
		cert.KeyAlgorithm,
		domains,
	)

	// If new key or URL was generated, save them to the DB
	if (newPrivateKeyPEM != "" && acmeAccount.PrivateKey != newPrivateKeyPEM) || (newAccountURL != "" && acmeAccount.URL != newAccountURL) {
		if newPrivateKeyPEM != "" {
			acmeAccount.PrivateKey = newPrivateKeyPEM
		}
		if newAccountURL != "" {
			acmeAccount.URL = newAccountURL
		}
		if acmeAccount.ID == 0 {
			if dbErr := model.DB.Create(acmeAccount).Error; dbErr != nil {
				updateCertError(cert, fmt.Sprintf("Failed to create ACME account: %v", dbErr))
				return dbErr
			}
		} else {
			if dbErr := model.DB.Save(acmeAccount).Error; dbErr != nil {
				updateCertError(cert, fmt.Sprintf("Failed to save ACME account: %v", dbErr))
				return dbErr
			}
		}
		// Self-heal the cert
		cert.AcmeAccountID = acmeAccount.ID
		model.DB.Save(cert)
	}

	if err != nil {
		updateCertError(cert, err.Error())
		return err
	}

	cert.CertPEM = result.CertPEM
	cert.KeyPEM = result.KeyPEM
	cert.NotBefore = result.NotBefore
	cert.NotAfter = result.NotAfter
	cert.ApplyStatus = "ready"
	cert.ApplyMessage = ""

	return model.DB.Save(cert).Error
}

func updateCertError(cert *model.TLSCertificate, message string) {
	cert.ApplyStatus = "error"
	cert.ApplyMessage = message
	model.DB.Save(cert)
}
