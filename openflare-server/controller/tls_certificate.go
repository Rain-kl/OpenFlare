package controller

import (
	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/controller/bind"
	"github.com/rain-kl/openflare/openflare-server/service"

	"github.com/gin-gonic/gin"
)

// GetTLSCertificates godoc
// @Summary List TLS certificates
// @Tags TLSCertificates
// @Produce json
// @Security OpenFlareTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/tls-certificates/ [get]
func GetTLSCertificates(c *gin.Context) {
	certificates, err := service.ListTLSCertificates()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, certificates)
}

// GetTLSCertificate godoc
// @Summary Get TLS certificate detail
// @Tags TLSCertificates
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Certificate ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/{id} [get]
func GetTLSCertificate(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}

	certificate, err := service.GetTLSCertificate(id)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, certificate)
}

// GetTLSCertificateContent godoc
// @Summary Get TLS certificate PEM content
// @Tags TLSCertificates
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Certificate ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/{id}/content [get]
func GetTLSCertificateContent(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}

	content, err := service.GetTLSCertificateContent(id)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, content)
}

// CreateTLSCertificate godoc
// @Summary Create TLS certificate from PEM
// @Tags TLSCertificates
// @Accept json
// @Produce json
// @Security OpenFlareTokenAuth
// @Param payload body service.TLSCertificateInput true "TLS certificate payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/ [post]
func CreateTLSCertificate(c *gin.Context) {
	var input service.TLSCertificateInput
	if !bind.JSON(c, &input) {
		return
	}
	certificate, err := service.CreateTLSCertificate(input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, certificate)
}

// UpdateTLSCertificate godoc
// @Summary Update TLS certificate from PEM
// @Tags TLSCertificates
// @Accept json
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Certificate ID"
// @Param payload body service.TLSCertificateInput true "TLS certificate payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/{id}/update [post]
func UpdateTLSCertificate(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}

	var input service.TLSCertificateInput
	if !bind.JSON(c, &input) {
		return
	}

	certificate, err := service.UpdateTLSCertificate(id, input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, certificate)
}

// ImportTLSCertificateFile godoc
// @Summary Import TLS certificate from files
// @Tags TLSCertificates
// @Accept multipart/form-data
// @Produce json
// @Security OpenFlareTokenAuth
// @Param name formData string true "Certificate name"
// @Param remark formData string false "Remark"
// @Param cert_file formData file true "Certificate file"
// @Param key_file formData file true "Private key file"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/import-file [post]
func ImportTLSCertificateFile(c *gin.Context) {
	name := c.PostForm("name")
	remark := c.PostForm("remark")
	certFile, err := c.FormFile("cert_file")
	if err != nil {
		response.RespondBadRequest(c, "缺少证书文件")
		return
	}
	keyFile, err := c.FormFile("key_file")
	if err != nil {
		response.RespondBadRequest(c, "缺少私钥文件")
		return
	}
	certificate, err := service.CreateTLSCertificateFromFiles(name, certFile, keyFile, remark)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, certificate)
}

// DeleteTLSCertificate godoc
// @Summary Delete TLS certificate
// @Tags TLSCertificates
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Certificate ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/{id}/delete [post]
func DeleteTLSCertificate(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}
	if err := service.DeleteTLSCertificate(id); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, nil)
}

// ApplyTLSCertificate godoc
// @Summary Apply TLS certificate via ACME
// @Tags TLSCertificates
// @Accept json
// @Produce json
// @Security OpenFlareTokenAuth
// @Param payload body service.TLSApplyInput true "TLS apply payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/apply [post]
func ApplyTLSCertificate(c *gin.Context) {
	var input service.TLSApplyInput
	if !bind.JSON(c, &input) {
		return
	}
	certificate, err := service.ApplyTLSCertificate(input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, certificate)
}

// UpdateAcmeCertificate godoc
// @Summary Update ACME TLS certificate
// @Tags TLSCertificates
// @Accept json
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Certificate ID"
// @Param payload body service.TLSApplyInput true "TLS apply payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/{id}/update-acme [post]
func UpdateAcmeCertificate(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}

	var input service.TLSApplyInput
	if !bind.JSON(c, &input) {
		return
	}
	certificate, err := service.UpdateAcmeCertificate(id, input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, certificate)
}

// ConvertTLSCertificateToAcme godoc
// @Summary Convert uploaded TLS certificate to ACME managed certificate
// @Tags TLSCertificates
// @Accept json
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Certificate ID"
// @Param payload body service.TLSApplyInput true "TLS apply payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/{id}/convert-acme [post]
func ConvertTLSCertificateToAcme(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}

	var input service.TLSApplyInput
	if !bind.JSON(c, &input) {
		return
	}
	certificate, err := service.ConvertTLSCertificateToAcme(id, input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, certificate)
}

// RenewTLSCertificate godoc
// @Summary Renew TLS certificate
// @Tags TLSCertificates
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Certificate ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/{id}/renew [post]
func RenewTLSCertificate(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}
	certificate, err := service.RenewTLSCertificate(id)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, certificate)
}
