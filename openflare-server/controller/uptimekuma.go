package controller

import (
	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/service"

	"github.com/gin-gonic/gin"
)

// SyncUptimeKuma godoc
// @Summary Manually trigger Uptime Kuma sync
// @Tags UptimeKuma
// @Accept json
// @Produce json
// @Security OpenFlareTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/uptimekuma/sync [post]
func SyncUptimeKuma(c *gin.Context) {
	err := service.SyncToUptimeKuma()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccessMessage(c, "同步成功")
}
