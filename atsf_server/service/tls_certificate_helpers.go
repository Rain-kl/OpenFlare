package service

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"mime/multipart"
)

func parseLeafCertificate(certPEM string) (*x509.Certificate, error) {
	certPEMBlock, _ := pem.Decode([]byte(certPEM))
	if certPEMBlock == nil {
		return nil, errors.New("证书 PEM 内容不合法")
	}
	leaf, err := x509.ParseCertificate(certPEMBlock.Bytes)
	if err != nil {
		return nil, err
	}
	return leaf, nil
}

func readMultipartFile(fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
