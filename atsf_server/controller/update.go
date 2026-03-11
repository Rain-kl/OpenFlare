package controller

import (
	"atsflare/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetLatestRelease godoc
// @Summary Get latest GitHub release
// @Tags Update
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/update/latest-release [get]
func GetLatestRelease(c *gin.Context) {
	release, err := service.GetLatestServerRelease(c.Request.Context())
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
		"data":    release,
	})
}

// UpgradeServer godoc
// @Summary Upgrade server binary from latest GitHub release
// @Tags Update
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/update/upgrade [post]
func UpgradeServer(c *gin.Context) {
	release, err := service.ScheduleServerUpgrade()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "服务升级任务已启动，下载完成后将自动重启。",
		"data":    release,
	})
}
