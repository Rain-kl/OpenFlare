package service

import (
	"crypto/tls"
	"errors"
	"fmt"
	"gin-template/model"
	"mime/multipart"
	"strings"
)

type TLSCertificateInput struct {
	Name    string `json:"name"`
	CertPEM string `json:"cert_pem"`
	KeyPEM  string `json:"key_pem"`
	Remark  string `json:"remark"`
}

func ListTLSCertificates() ([]*model.TLSCertificate, error) {
	return model.ListTLSCertificates()
}

func CreateTLSCertificate(input TLSCertificateInput) (*model.TLSCertificate, error) {
	certificate, err := buildTLSCertificate(nil, input)
	if err != nil {
		return nil, err
	}
	if err = certificate.Insert(); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("证书名称已存在")
		}
		return nil, err
	}
	return certificate, nil
}

func CreateTLSCertificateFromFiles(name string, certFile *multipart.FileHeader, keyFile *multipart.FileHeader, remark string) (*model.TLSCertificate, error) {
	if certFile == nil || keyFile == nil {
		return nil, errors.New("证书文件和私钥文件不能为空")
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

func DeleteTLSCertificate(id uint) error {
	var routeCount int64
	if err := model.DB.Model(&model.ProxyRoute{}).Where("cert_id = ?", id).Count(&routeCount).Error; err != nil {
		return err
	}
	if routeCount > 0 {
		return errors.New("证书仍被反代规则引用，无法删除")
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
		return nil, errors.New("证书名称不能为空")
	}
	if certPEM == "" || keyPEM == "" {
		return nil, errors.New("证书内容和私钥内容不能为空")
	}
	parsed, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return nil, fmt.Errorf("证书或私钥格式不合法: %w", err)
	}
	if len(parsed.Certificate) == 0 {
		return nil, errors.New("证书内容不合法")
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
