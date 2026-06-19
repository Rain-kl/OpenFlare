// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"net/http"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/gin-gonic/gin"
)


func handleLogicError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	return apiutil.AbortNotFoundIfMissing(c, err, "记录不存在")
}

// GetCertificates 列出 TLS 证书。
// @Summary 列出 TLS 证书
// @Description 返回全部 TLS 证书（不含 PEM），需要管理员权限
// @Tags openflare-tls
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]model.TLSCertificate} "证书列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/tls-certificates [get]
func GetCertificates(c *gin.Context) {
	certificates, err := ListCertificates(c.Request.Context())
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(certificates))
}

// GetCertificateDetail 获取 TLS 证书详情。
// @Summary 获取 TLS 证书详情
// @Description 按 ID 返回 TLS 证书详情（不含 PEM），需要管理员权限
// @Tags openflare-tls
// @Produce json
// @Security SessionCookie
// @Param id path int true "证书 ID"
// @Success 200 {object} response.Any{data=model.TLSCertificate} "证书详情"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/tls-certificates/{id} [get]
func GetCertificateDetail(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	certificate, err := GetCertificate(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(certificate))
}

// GetCertificateContentHandler 获取 TLS 证书 PEM 内容。
// @Summary 获取 TLS 证书 PEM 内容
// @Description 按 ID 返回证书与私钥 PEM 内容，需要管理员权限
// @Tags openflare-tls
// @Produce json
// @Security SessionCookie
// @Param id path int true "证书 ID"
// @Success 200 {object} response.Any{data=tls.CertificateContent} "证书 PEM 内容"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/tls-certificates/{id}/content [get]
func GetCertificateContentHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	content, err := GetCertificateContent(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(content))
}

// CreateCertificateHandler 从 PEM 创建证书。
// @Summary 创建 TLS 证书
// @Description 从 PEM 文本创建 TLS 证书，需要管理员权限
// @Tags openflare-tls
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body tls.CertificateInput true "证书参数"
// @Success 200 {object} response.Any{data=model.TLSCertificate} "创建成功的证书"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/tls-certificates [post]
func CreateCertificateHandler(c *gin.Context) {
	var input CertificateInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	certificate, err := CreateCertificate(c.Request.Context(), input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(certificate))
}

// UpdateCertificateHandler 更新证书。
// @Summary 更新 TLS 证书
// @Description 按 ID 更新 TLS 证书 PEM 信息，需要管理员权限
// @Tags openflare-tls
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "证书 ID"
// @Param request body tls.CertificateInput true "证书参数"
// @Success 200 {object} response.Any{data=model.TLSCertificate} "更新后的证书"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/tls-certificates/{id}/update [post]
func UpdateCertificateHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input CertificateInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	certificate, err := UpdateCertificate(c.Request.Context(), id, input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(certificate))
}

// ImportCertificateFile 从文件导入证书。
// @Summary 从文件导入 TLS 证书
// @Description 上传证书与私钥文件创建 TLS 证书，需要管理员权限
// @Tags openflare-tls
// @Accept multipart/form-data
// @Produce json
// @Security SessionCookie
// @Param name formData string false "证书名称"
// @Param remark formData string false "备注"
// @Param cert_file formData file true "证书文件"
// @Param key_file formData file true "私钥文件"
// @Success 200 {object} response.Any{data=model.TLSCertificate} "导入成功的证书"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/tls-certificates/import-file [post]
func ImportCertificateFile(c *gin.Context) {
	name := c.PostForm("name")
	remark := c.PostForm("remark")
	certFile, err := c.FormFile("cert_file")
	if err != nil {
		response.AbortBadRequest(c, "缺少证书文件")
		return
	}
	keyFile, err := c.FormFile("key_file")
	if err != nil {
		response.AbortBadRequest(c, "缺少私钥文件")
		return
	}
	certificate, err := CreateCertificateFromFiles(c.Request.Context(), name, certFile, keyFile, remark)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(certificate))
}

// DeleteCertificateHandler 删除证书。
// @Summary 删除 TLS 证书
// @Description 按 ID 删除 TLS 证书，需要管理员权限
// @Tags openflare-tls
// @Produce json
// @Security SessionCookie
// @Param id path int true "证书 ID"
// @Success 200 {object} response.Any "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/tls-certificates/{id}/delete [post]
func DeleteCertificateHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	if err := DeleteCertificate(c.Request.Context(), id); handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// ApplyCertificateHandler 申请 ACME 证书。
// @Summary 申请 ACME 证书
// @Description 通过 ACME 申请新的 TLS 证书，需要管理员权限
// @Tags openflare-tls
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body tls.ApplyInput true "ACME 申请参数"
// @Success 200 {object} response.Any{data=model.TLSCertificate} "申请中的证书"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/tls-certificates/apply [post]
func ApplyCertificateHandler(c *gin.Context) {
	var input ApplyInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	certificate, err := ApplyCertificate(c.Request.Context(), input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(certificate))
}

// UpdateACMECertificateHandler 更新 ACME 证书配置。
// @Summary 更新 ACME 证书配置
// @Description 按 ID 更新 ACME 证书申请配置，需要管理员权限
// @Tags openflare-tls
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "证书 ID"
// @Param request body tls.ApplyInput true "ACME 申请参数"
// @Success 200 {object} response.Any{data=model.TLSCertificate} "更新后的证书"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/tls-certificates/{id}/update-acme [post]
func UpdateACMECertificateHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input ApplyInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	certificate, err := UpdateACMECertificate(c.Request.Context(), id, input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(certificate))
}

// ConvertCertificateToACMEHandler 将上传证书转为 ACME。
// @Summary 将证书转为 ACME 管理
// @Description 将已上传证书转换为 ACME 自动续期模式，需要管理员权限
// @Tags openflare-tls
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "证书 ID"
// @Param request body tls.ApplyInput true "ACME 申请参数"
// @Success 200 {object} response.Any{data=model.TLSCertificate} "转换后的证书"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/tls-certificates/{id}/convert-acme [post]
func ConvertCertificateToACMEHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input ApplyInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	certificate, err := ConvertCertificateToACME(c.Request.Context(), id, input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(certificate))
}

// RenewCertificateHandler 续期 ACME 证书。
// @Summary 续期 ACME 证书
// @Description 手动触发 ACME 证书续期，需要管理员权限
// @Tags openflare-tls
// @Produce json
// @Security SessionCookie
// @Param id path int true "证书 ID"
// @Success 200 {object} response.Any{data=model.TLSCertificate} "续期后的证书"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/tls-certificates/{id}/renew [post]
func RenewCertificateHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	certificate, err := RenewCertificate(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(certificate))
}

// GetManagedDomains 列出托管域名。
// @Summary 列出托管域名
// @Description 返回全部托管域名及关联证书，需要管理员权限
// @Tags openflare-tls
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]model.ManagedDomain} "托管域名列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/managed-domains [get]
func GetManagedDomains(c *gin.Context) {
	domains, err := ListManagedDomains(c.Request.Context())
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(domains))
}

// CreateManagedDomainHandler 创建托管域名。
// @Summary 创建托管域名
// @Description 创建新的托管域名记录，需要管理员权限
// @Tags openflare-tls
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body tls.ManagedDomainInput true "托管域名参数"
// @Success 200 {object} response.Any{data=model.ManagedDomain} "创建成功的托管域名"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/managed-domains [post]
func CreateManagedDomainHandler(c *gin.Context) {
	var input ManagedDomainInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	domain, err := CreateManagedDomain(c.Request.Context(), input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(domain))
}

// UpdateManagedDomainHandler 更新托管域名。
// @Summary 更新托管域名
// @Description 按 ID 更新托管域名，需要管理员权限
// @Tags openflare-tls
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "托管域名 ID"
// @Param request body tls.ManagedDomainInput true "托管域名参数"
// @Success 200 {object} response.Any{data=model.ManagedDomain} "更新后的托管域名"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/managed-domains/{id}/update [post]
func UpdateManagedDomainHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input ManagedDomainInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	domain, err := UpdateManagedDomain(c.Request.Context(), id, input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(domain))
}

// DeleteManagedDomainHandler 删除托管域名。
// @Summary 删除托管域名
// @Description 按 ID 删除托管域名，需要管理员权限
// @Tags openflare-tls
// @Produce json
// @Security SessionCookie
// @Param id path int true "托管域名 ID"
// @Success 200 {object} response.Any "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/managed-domains/{id}/delete [post]
func DeleteManagedDomainHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	if err := DeleteManagedDomain(c.Request.Context(), id); handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// MatchManagedDomainCertificateHandler 匹配域名证书。
// @Summary 匹配托管域名证书
// @Description 按域名查询可用的证书匹配候选，需要管理员权限
// @Tags openflare-tls
// @Produce json
// @Security SessionCookie
// @Param domain query string true "域名"
// @Success 200 {object} response.Any{data=tls.ManagedDomainMatchResult} "证书匹配结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/managed-domains/match [get]
func MatchManagedDomainCertificateHandler(c *gin.Context) {
	domain := strings.TrimSpace(c.Query("domain"))
	result, err := MatchManagedDomainCertificate(c.Request.Context(), domain)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

// GetDNSAccounts 列出 DNS 账号。
// @Summary 列出 DNS 账号
// @Description 返回全部 DNS 提供商账号，需要管理员权限
// @Tags openflare-tls
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]model.DNSAccount} "DNS 账号列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/dns-accounts [get]
func GetDNSAccounts(c *gin.Context) {
	accounts, err := ListDNSAccounts(c.Request.Context())
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(accounts))
}

// CreateDNSAccountHandler 创建 DNS 账号。
// @Summary 创建 DNS 账号
// @Description 创建新的 DNS 提供商账号，需要管理员权限
// @Tags openflare-tls
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body tls.DNSAccountInput true "DNS 账号参数"
// @Success 200 {object} response.Any{data=model.DNSAccount} "创建成功的 DNS 账号"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/dns-accounts [post]
func CreateDNSAccountHandler(c *gin.Context) {
	var input DNSAccountInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	account, err := CreateDNSAccount(c.Request.Context(), input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(account))
}

// UpdateDNSAccountHandler 更新 DNS 账号。
// @Summary 更新 DNS 账号
// @Description 按 ID 更新 DNS 提供商账号，需要管理员权限
// @Tags openflare-tls
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "DNS 账号 ID"
// @Param request body tls.DNSAccountInput true "DNS 账号参数"
// @Success 200 {object} response.Any{data=model.DNSAccount} "更新后的 DNS 账号"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/dns-accounts/{id}/update [post]
func UpdateDNSAccountHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input DNSAccountInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	account, err := UpdateDNSAccount(c.Request.Context(), id, input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(account))
}

// DeleteDNSAccountHandler 删除 DNS 账号。
// @Summary 删除 DNS 账号
// @Description 按 ID 删除 DNS 提供商账号，需要管理员权限
// @Tags openflare-tls
// @Produce json
// @Security SessionCookie
// @Param id path int true "DNS 账号 ID"
// @Success 200 {object} response.Any "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/dns-accounts/{id}/delete [post]
func DeleteDNSAccountHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	if err := DeleteDNSAccount(c.Request.Context(), id); handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// GetDefaultAcmeAccountHandler 获取默认 ACME 账号。
// @Summary 获取默认 ACME 账号
// @Description 返回系统默认 ACME 账号配置，需要管理员权限
// @Tags openflare-tls
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=model.AcmeAccount} "默认 ACME 账号"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/acme-accounts/default [get]
func GetDefaultAcmeAccountHandler(c *gin.Context) {
	account, err := GetDefaultAcmeAccount(c.Request.Context())
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(account))
}