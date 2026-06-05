package controller

import (
	"openflare/common/response"
	"openflare/controller/bind"
	"openflare/service"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetManagedDomains godoc
// @Summary List managed domains
// @Tags ManagedDomains
// @Produce json
// @Security OpenFlareTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/managed-domains/ [get]
func GetManagedDomains(c *gin.Context) {
	domains, err := service.ListManagedDomains()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, domains)
}

// CreateManagedDomain godoc
// @Summary Create managed domain
// @Tags ManagedDomains
// @Accept json
// @Produce json
// @Security OpenFlareTokenAuth
// @Param payload body service.ManagedDomainInput true "Managed domain payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/managed-domains/ [post]
func CreateManagedDomain(c *gin.Context) {
	var input service.ManagedDomainInput
	if !bind.JSON(c, &input) {
		return
	}
	domain, err := service.CreateManagedDomain(input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, domain)
}

// UpdateManagedDomain godoc
// @Summary Update managed domain
// @Tags ManagedDomains
// @Accept json
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Managed domain ID"
// @Param payload body service.ManagedDomainInput true "Managed domain payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/managed-domains/{id}/update [post]
func UpdateManagedDomain(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}
	var input service.ManagedDomainInput
	if !bind.JSON(c, &input) {
		return
	}
	domain, err := service.UpdateManagedDomain(id, input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, domain)
}

// DeleteManagedDomain godoc
// @Summary Delete managed domain
// @Tags ManagedDomains
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Managed domain ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/managed-domains/{id}/delete [post]
func DeleteManagedDomain(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}
	if err := service.DeleteManagedDomain(id); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, nil)
}

// MatchManagedDomainCertificate godoc
// @Summary Match certificate for domain
// @Tags ManagedDomains
// @Produce json
// @Security OpenFlareTokenAuth
// @Param domain query string true "Domain"
// @Success 200 {object} map[string]interface{}
// @Router /api/managed-domains/match [get]
func MatchManagedDomainCertificate(c *gin.Context) {
	domain := strings.TrimSpace(c.Query("domain"))
	result, err := service.MatchManagedDomainCertificate(domain)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, result)
}
