package controller

import (
	"net/http"
	"openflare/service"

	"github.com/gin-gonic/gin"
)

// CleanupDatabaseObservability godoc
// @Summary Cleanup observability tables
// @Tags Options
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/option/database/cleanup [post]
func CleanupDatabaseObservability(c *gin.Context) {
	var input service.DatabaseCleanupInput
	if err := decodeOptionalJSONBody(c.Request.Body, &input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}
	result, err := service.CleanupDatabaseObservability(input)
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, result)
}
