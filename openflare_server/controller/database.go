package controller

import (
	"openflare/common/response"
	"openflare/controller/bind"
	"openflare/service"

	"github.com/gin-gonic/gin"
)

// CleanupDatabaseObservability godoc
// @Summary Cleanup observability tables
// @Tags Options
// @Accept json
// @Produce json
// @Security OpenFlareTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/option/database/cleanup [post]
func CleanupDatabaseObservability(c *gin.Context) {
	var input service.DatabaseCleanupInput
	if err := bind.OptionalJSON(c.Request.Body, &input); err != nil {
		response.RespondBadRequest(c, "")
		return
	}
	result, err := service.CleanupDatabaseObservability(input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, result)
}
