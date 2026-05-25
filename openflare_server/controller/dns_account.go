package controller

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"openflare/model"
	"strconv"
)

type DnsAccountInput struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Authorization string `json:"authorization"`
}

// GetDnsAccounts godoc
// @Summary List DNS accounts
// @Tags DnsAccounts
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/dns-accounts/ [get]
func GetDnsAccounts(c *gin.Context) {
	accounts, err := model.ListDnsAccounts()
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
		"data":    accounts,
	})
}

// CreateDnsAccount godoc
// @Summary Create DNS account
// @Tags DnsAccounts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body DnsAccountInput true "DNS account payload"
// @Success 200 {object} map[string]interface{}
// @Router /api/dns-accounts/ [post]
func CreateDnsAccount(c *gin.Context) {
	var input DnsAccountInput
	if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}

	account := &model.DnsAccount{
		Name:          input.Name,
		Type:          input.Type,
		Authorization: input.Authorization,
	}

	if err := account.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    account,
	})
}

// UpdateDnsAccount godoc
// @Summary Update DNS account
// @Tags DnsAccounts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "DNS Account ID"
// @Param payload body DnsAccountInput true "DNS account payload"
// @Success 200 {object} map[string]interface{}
// @Router /api/dns-accounts/{id}/update [post]
func UpdateDnsAccount(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}

	var input DnsAccountInput
	if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}

	account, err := model.GetDnsAccountByID(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	account.Name = input.Name
	account.Type = input.Type
	account.Authorization = input.Authorization

	if err := account.Update(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    account,
	})
}

// DeleteDnsAccount godoc
// @Summary Delete DNS account
// @Tags DnsAccounts
// @Produce json
// @Security BearerAuth
// @Param id path int true "DNS Account ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/dns-accounts/{id}/delete [post]
func DeleteDnsAccount(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}

	account, err := model.GetDnsAccountByID(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Verify no cert uses this before deleting
	var count int64
	model.DB.Model(&model.TLSCertificate{}).Where("dns_account_id = ?", id).Count(&count)
	if count > 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "该 DNS 账号已被证书使用，无法删除",
		})
		return
	}

	if err := account.Delete(); err != nil {
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
