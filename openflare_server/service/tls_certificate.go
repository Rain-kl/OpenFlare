package service

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"openflare/model"
	"strings"
)

type TLSCertificateInput struct {
	Name    string `json:"name"`
	CertPEM string `json:"cert_pem"`
	KeyPEM  string `json:"key_pem"`
	Remark  string `json:"remark"`
}

type TLSCertificateContent struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	CertPEM string `json:"cert_pem"`
	KeyPEM  string `json:"key_pem"`
	Remark  string `json:"remark"`
}

func ListTLSCertificates() ([]*model.TLSCertificate, error) {
	return model.ListTLSCertificates()
}

func GetTLSCertificate(id uint) (*model.TLSCertificate, error) {
	return model.GetTLSCertificateByID(id)
}

func GetTLSCertificateContent(id uint) (*TLSCertificateContent, error) {
	certificate, err := model.GetTLSCertificateByID(id)
	if err != nil {
		return nil, err
	}

	return &TLSCertificateContent{
		ID:      certificate.ID,
		Name:    certificate.Name,
		CertPEM: certificate.CertPEM,
		KeyPEM:  certificate.KeyPEM,
		Remark:  certificate.Remark,
	}, nil
}

func CreateTLSCertificate(input TLSCertificateInput) (*model.TLSCertificate, error) {
	certificate, err := buildTLSCertificate(nil, input)
	if err != nil {
		return nil, err
	}
	if err = certificate.Insert(); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("certificate name already exists")
		}
		return nil, err
	}
	return certificate, nil
}

func CreateTLSCertificateFromFiles(name string, certFile *multipart.FileHeader, keyFile *multipart.FileHeader, remark string) (*model.TLSCertificate, error) {
	if certFile == nil || keyFile == nil {
		return nil, errors.New("certificate file and key file cannot be empty")
	}
	certContent, err := readMultipartFile(certFile)
	if err != nil {
		return nil, err
	}
	keyContent, err := readMultipartFile(keyFile)
	if err != nil {
		return nil, err
	}
	return CreateTLSCertificate(TLSCertificateInput{
		Name:    name,
		CertPEM: certContent,
		KeyPEM:  keyContent,
		Remark:  remark,
	})
}

func UpdateTLSCertificate(id uint, input TLSCertificateInput) (*model.TLSCertificate, error) {
	existing, err := model.GetTLSCertificateByID(id)
	if err != nil {
		return nil, err
	}

	certificate, err := buildTLSCertificate(existing, input)
	if err != nil {
		return nil, err
	}
	if err = certificate.Update(); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("certificate name already exists")
		}
		return nil, err
	}
	return certificate, nil
}

func DeleteTLSCertificate(id uint) error {
	routes, err := model.ListProxyRoutes()
	if err != nil {
		return err
	}
	for _, route := range routes {
		if route == nil {
			continue
		}
		if route.CertID != nil && *route.CertID == id {
			return errors.New("certificate is still referenced by proxy routes")
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
				return errors.New("certificate is still referenced by proxy routes")
			}
		}
		domainCertIDs, err := decodeStoredDomainCertIDs(route.DomainCertIDs, 0)
		if err != nil {
			return fmt.Errorf("proxy route %d domain_cert_ids payload is invalid: %w", route.ID, err)
		}
		for _, certID := range domainCertIDs {
			if certID == id {
				return errors.New("certificate is still referenced by proxy routes")
			}
		}
	}

	certificate, err := model.GetTLSCertificateByID(id)
	if err != nil {
		return err
	}
	return certificate.Delete()
}

func buildTLSCertificate(existing *model.TLSCertificate, input TLSCertificateInput) (*model.TLSCertificate, error) {
	name := strings.TrimSpace(input.Name)
	certPEM := strings.TrimSpace(input.CertPEM)
	keyPEM := strings.TrimSpace(input.KeyPEM)
	remark := strings.TrimSpace(input.Remark)
	if name == "" {
		return nil, errors.New("certificate name cannot be empty")
	}
	if certPEM == "" || keyPEM == "" {
		return nil, errors.New("certificate content and key content cannot be empty")
	}
	parsed, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return nil, fmt.Errorf("certificate or key format is invalid: %w", err)
	}
	if len(parsed.Certificate) == 0 {
		return nil, errors.New("certificate content is invalid")
	}
	leaf, err := parseLeafCertificate(certPEM)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		existing = &model.TLSCertificate{}
	}
	existing.Name = name
	existing.CertPEM = certPEM
	existing.KeyPEM = keyPEM
	existing.NotBefore = leaf.NotBefore
	existing.NotAfter = leaf.NotAfter
	existing.Remark = remark
	return existing, nil
}
