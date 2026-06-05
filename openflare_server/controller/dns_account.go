package controller

import (
	"openflare/common/response"
	"openflare/controller/bind"
	"openflare/model"

	"github.com/gin-gonic/gin"
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
// @Security OpenFlareTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/dns-accounts/ [get]
func GetDnsAccounts(c *gin.Context) {
	accounts, err := model.ListDnsAccounts()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, accounts)
}

// CreateDnsAccount godoc
// @Summary Create DNS account
// @Tags DnsAccounts
// @Accept json
// @Produce json
// @Security OpenFlareTokenAuth
// @Param payload body DnsAccountInput true "DNS account payload"
// @Success 200 {object} map[string]interface{}
// @Router /api/dns-accounts/ [post]
func CreateDnsAccount(c *gin.Context) {
	var input DnsAccountInput
	if !bind.JSON(c, &input) {
		return
	}

	account := &model.DnsAccount{
		Name:          input.Name,
		Type:          input.Type,
		Authorization: input.Authorization,
	}

	if err := account.Insert(); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}

	response.RespondSuccess(c, account)
}

// UpdateDnsAccount godoc
// @Summary Update DNS account
// @Tags DnsAccounts
// @Accept json
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "DNS Account ID"
// @Param payload body DnsAccountInput true "DNS account payload"
// @Success 200 {object} map[string]interface{}
// @Router /api/dns-accounts/{id}/update [post]
func UpdateDnsAccount(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}

	var input DnsAccountInput
	if !bind.JSON(c, &input) {
		return
	}

	account, err := model.GetDnsAccountByID(id)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}

	account.Name = input.Name
	account.Type = input.Type
	account.Authorization = input.Authorization

	if err := account.Update(); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}

	response.RespondSuccess(c, account)
}

// DeleteDnsAccount godoc
// @Summary Delete DNS account
// @Tags DnsAccounts
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "DNS Account ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/dns-accounts/{id}/delete [post]
func DeleteDnsAccount(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}

	account, err := model.GetDnsAccountByID(id)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}

	// Verify no cert uses this before deleting
	var count int64
	model.DB.Model(&model.TLSCertificate{}).Where("dns_account_id = ?", id).Count(&count)
	if count > 0 {
		response.RespondFailure(c, "该 DNS 账号已被证书使用，无法删除")
		return
	}

	if err := account.Delete(); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}

	response.RespondSuccess(c, nil)
}
