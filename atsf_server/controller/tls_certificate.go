package controller

import (
	"atsflare/service"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

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
