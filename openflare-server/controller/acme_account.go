package controller

import (
	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/model"

	"github.com/gin-gonic/gin"
)

// GetDefaultAcmeAccount godoc
// @Summary Get default ACME account
// @Tags AcmeAccounts
// @Produce json
// @Security OpenFlareTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/acme-accounts/default [get]
func GetDefaultAcmeAccount(c *gin.Context) {
	account, err := model.GetDefaultAcmeAccount()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, account)
}
