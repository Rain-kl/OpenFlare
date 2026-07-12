// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"mime/multipart"
	"strings"
)

func parseLeafCertificate(certPEM string) (*x509.Certificate, error) {
	certPEMBlock, _ := pem.Decode([]byte(certPEM))
	if certPEMBlock == nil {
		return nil, errors.New(errCertificatePEMInvalid)
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
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}
