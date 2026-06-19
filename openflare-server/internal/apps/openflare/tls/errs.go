// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

const (
	errCertificateNameRequired      = "certificate name cannot be empty"
	errCertificateNameExists        = "certificate name already exists"
	errCertificateContentRequired   = "certificate content and key content cannot be empty"
	errCertificateContentInvalid    = "certificate or key format is invalid"
	errCertificateDeleteReferenced  = "certificate is still referenced by proxy routes"
	errCertificateOnlyACME          = "only acme certificates can be updated via this endpoint"
	errCertificateOnlyUploadConvert = "only uploaded certificates can be converted to acme"
	errCertificateAlreadyApplying   = "certificate is already applying"
	errCertificateOnlyACMERenew     = "only acme certificates can be renewed"
	errCertificateFilesRequired     = "certificate file and key file cannot be empty"
	errCertificatePEMInvalid        = "证书 PEM 内容不合法"

	errManagedDomainRequired        = "域名不能为空"
	errManagedDomainInvalid         = "域名格式不合法"
	errManagedDomainWildcardInvalid = "通配符域名仅支持 *.example.com 格式"
	errManagedDomainExists          = "域名已存在"
	errManagedDomainCertNotFound    = "所选证书不存在"

	errDNSAccountInUse = "该 DNS 账号已被证书使用，无法删除"
)
