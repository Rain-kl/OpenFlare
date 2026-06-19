// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package acme

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/go-acme/lego/v4/acme"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/registration"
)

// AcmeUser implements lego's user interface.
type AcmeUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *AcmeUser) GetEmail() string {
	return u.Email
}

func (u *AcmeUser) GetRegistration() *registration.Resource {
	return u.Registration
}

func (u *AcmeUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// CertificateResult holds obtained certificate material.
type CertificateResult struct {
	CertPEM   string
	KeyPEM    string
	NotBefore time.Time
	NotAfter  time.Time
}

func parsePrivateKey(pemData string) (crypto.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the key")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, errors.New("failed to parse private key")
}

func encodePrivateKey(key crypto.PrivateKey) (string, error) {
	var pemBlock *pem.Block
	switch k := key.(type) {
	case *rsa.PrivateKey:
		pemBlock = &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			return "", err
		}
		pemBlock = &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return "", errors.New("unsupported key type")
	}
	return string(pem.EncodeToMemory(pemBlock)), nil
}

// GetOrCreateLegoClient returns a configured lego client and optional new account credentials.
func GetOrCreateLegoClient(acmeEmail, privateKeyPEM, accountURL string, keyAlgorithm string) (*lego.Client, *AcmeUser, string, string, error) {
	var privateKey crypto.PrivateKey
	var err error
	var newPrivateKeyPEM string
	var newAccountURL string

	if privateKeyPEM == "" {
		privateKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, nil, "", "", err
		}
		pemStr, err := encodePrivateKey(privateKey)
		if err != nil {
			return nil, nil, "", "", err
		}
		newPrivateKeyPEM = pemStr
	} else {
		privateKey, err = parsePrivateKey(privateKeyPEM)
		if err != nil {
			return nil, nil, "", "", err
		}
	}

	user := &AcmeUser{
		Email: acmeEmail,
		key:   privateKey,
	}

	if accountURL != "" {
		user.Registration = &registration.Resource{
			Body: acme.Account{
				Status:  "valid",
				Contact: []string{"mailto:" + acmeEmail},
			},
			URI: accountURL,
		}
	}

	config := lego.NewConfig(user)
	config.CADirURL = lego.LEDirectoryProduction

	switch keyAlgorithm {
	case "RSA2048":
		config.Certificate.KeyType = certcrypto.RSA2048
	case "RSA4096":
		config.Certificate.KeyType = certcrypto.RSA4096
	case "EC256":
		config.Certificate.KeyType = certcrypto.EC256
	case "EC384":
		config.Certificate.KeyType = certcrypto.EC384
	default:
		config.Certificate.KeyType = certcrypto.RSA2048
	}

	client, err := lego.NewClient(config)
	if err != nil {
		return nil, nil, "", "", err
	}

	if accountURL == "" {
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return nil, nil, "", "", err
		}
		user.Registration = reg
		newAccountURL = reg.URI
	}

	return client, user, newPrivateKeyPEM, newAccountURL, nil
}

// SetupDNSProvider configures DNS-01 challenge for the lego client.
func SetupDNSProvider(client *lego.Client, dnsType, dnsAuth string, dns1, dns2 string, disableCNAME, skipDNS bool) error {
	var provider challengeProvider

	switch dnsType {
	case "cloudflare":
		var creds map[string]string
		if err := json.Unmarshal([]byte(dnsAuth), &creds); err != nil {
			return fmt.Errorf("failed to parse cloudflare credentials: %v", err)
		}

		config := cloudflare.NewDefaultConfig()
		config.AuthToken = creds["api_token"]

		p, err := cloudflare.NewDNSProviderConfig(config)
		if err != nil {
			return err
		}
		provider = p
	default:
		return fmt.Errorf("unsupported DNS provider: %s", dnsType)
	}

	var resolvers []string
	if dns1 != "" {
		resolvers = append(resolvers, dns1+":53")
	}
	if dns2 != "" {
		resolvers = append(resolvers, dns2+":53")
	}

	var opts []dns01.ChallengeOption

	if len(resolvers) > 0 {
		opts = append(opts, dns01.AddRecursiveNameservers(resolvers))
	}

	if disableCNAME {
		opts = append(opts, dns01.DisableCompletePropagationRequirement())
	}

	if skipDNS {
		opts = append(opts, dns01.WrapPreCheck(func(domain, fqdn, value string, check dns01.PreCheckFunc) (bool, error) {
			time.Sleep(20 * time.Second)
			return true, nil
		}))
	}

	return client.Challenge.SetDNS01Provider(provider, opts...)
}

type challengeProvider interface {
	Present(domain, token, keyAuth string) error
	CleanUp(domain, token, keyAuth string) error
}

// ObtainSSL obtains a certificate via ACME DNS-01 challenge.
func ObtainSSL(
	acmeEmail, acmePrivateKeyPEM, acmeURL string,
	dnsType, dnsAuth string,
	dns1, dns2 string,
	disableCNAME, skipDNS bool,
	keyAlgorithm string,
	domains []string,
) (string, string, *CertificateResult, error) {
	client, _, newPrivateKeyPEM, newAccountURL, err := GetOrCreateLegoClient(acmeEmail, acmePrivateKeyPEM, acmeURL, keyAlgorithm)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create ACME client: %w", err)
	}

	err = SetupDNSProvider(client, dnsType, dnsAuth, dns1, dns2, disableCNAME, skipDNS)
	if err != nil {
		return newAccountURL, newPrivateKeyPEM, nil, fmt.Errorf("failed to setup DNS provider: %w", err)
	}

	request := certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	}

	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return newAccountURL, newPrivateKeyPEM, nil, fmt.Errorf("failed to obtain certificate: %w", err)
	}

	result := &CertificateResult{
		CertPEM: string(certificates.Certificate),
		KeyPEM:  string(certificates.PrivateKey),
	}

	certBlock, _ := pem.Decode(certificates.Certificate)
	if certBlock != nil {
		parsedCert, err := x509.ParseCertificate(certBlock.Bytes)
		if err == nil {
			result.NotBefore = parsedCert.NotBefore
			result.NotAfter = parsedCert.NotAfter
		}
	}

	return newAccountURL, newPrivateKeyPEM, result, nil
}
