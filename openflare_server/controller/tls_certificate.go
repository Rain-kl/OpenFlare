package controller

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"openflare/service"
	"strconv"
)

// GetTLSCertificates godoc
// @Summary List TLS certificates
// @Tags TLSCertificates
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/tls-certificates/ [get]
func GetTLSCertificates(c *gin.Context) {
	certificates, err := service.ListTLSCertificates()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    certificates,
	})
}

// GetTLSCertificate godoc
// @Summary Get TLS certificate detail
// @Tags TLSCertificates
// @Produce json
// @Security BearerAuth
// @Param id path int true "Certificate ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/{id} [get]
func GetTLSCertificate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid request",
		})
		return
	}

	certificate, err := service.GetTLSCertificate(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    certificate,
	})
}

// GetTLSCertificateContent godoc
// @Summary Get TLS certificate PEM content
// @Tags TLSCertificates
// @Produce json
// @Security BearerAuth
// @Param id path int true "Certificate ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/{id}/content [get]
func GetTLSCertificateContent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid request",
		})
		return
	}

	content, err := service.GetTLSCertificateContent(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    content,
	})
}

// CreateTLSCertificate godoc
// @Summary Create TLS certificate from PEM
// @Tags TLSCertificates
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body service.TLSCertificateInput true "TLS certificate payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/ [post]
func CreateTLSCertificate(c *gin.Context) {
	var input service.TLSCertificateInput
	if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	certificate, err := service.CreateTLSCertificate(input)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    certificate,
	})
}

// UpdateTLSCertificate godoc
// @Summary Update TLS certificate from PEM
// @Tags TLSCertificates
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Certificate ID"
// @Param payload body service.TLSCertificateInput true "TLS certificate payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/{id}/update [post]
func UpdateTLSCertificate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid request",
		})
		return
	}

	var input service.TLSCertificateInput
	if err = json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid request",
		})
		return
	}

	certificate, err := service.UpdateTLSCertificate(uint(id), input)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    certificate,
	})
}

// ImportTLSCertificateFile godoc
// @Summary Import TLS certificate from files
// @Tags TLSCertificates
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
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
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少证书文件",
		})
		return
	}
	keyFile, err := c.FormFile("key_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少私钥文件",
		})
		return
	}
	certificate, err := service.CreateTLSCertificateFromFiles(name, certFile, keyFile, remark)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    certificate,
	})
}

// DeleteTLSCertificate godoc
// @Summary Delete TLS certificate
// @Tags TLSCertificates
// @Produce json
// @Security BearerAuth
// @Param id path int true "Certificate ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/{id}/delete [post]
func DeleteTLSCertificate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	if err = service.DeleteTLSCertificate(uint(id)); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// ApplyTLSCertificate godoc
// @Summary Apply TLS certificate via ACME
// @Tags TLSCertificates
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body service.TLSApplyInput true "TLS apply payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/apply [post]
func ApplyTLSCertificate(c *gin.Context) {
	var input service.TLSApplyInput
	if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	certificate, err := service.ApplyTLSCertificate(input)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    certificate,
	})
}

// UpdateAcmeCertificate godoc
// @Summary Update ACME TLS certificate
// @Tags TLSCertificates
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Certificate ID"
// @Param payload body service.TLSApplyInput true "TLS apply payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/{id}/update-acme [post]
func UpdateAcmeCertificate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid request",
		})
		return
	}

	var input service.TLSApplyInput
	if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	certificate, err := service.UpdateAcmeCertificate(uint(id), input)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    certificate,
	})
}

// RenewTLSCertificate godoc
// @Summary Renew TLS certificate
// @Tags TLSCertificates
// @Produce json
// @Security BearerAuth
// @Param id path int true "Certificate ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/tls-certificates/{id}/renew [post]
func RenewTLSCertificate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	certificate, err := service.RenewTLSCertificate(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    certificate,
	})
}
