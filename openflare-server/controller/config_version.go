package controller

import (
	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/controller/bind"
	"github.com/rain-kl/openflare/openflare-server/service"

	"github.com/gin-gonic/gin"
)

// GetConfigVersions godoc
// @Summary List config versions
// @Tags ConfigVersions
// @Produce json
// @Security OpenFlareTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/config-versions/ [get]
func GetConfigVersions(c *gin.Context) {
	versions, err := service.ListConfigVersions()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, versions)
}

// GetConfigVersion godoc
// @Summary Get config version detail
// @Tags ConfigVersions
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Version ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/config-versions/{id} [get]
func GetConfigVersion(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}
	version, err := service.GetConfigVersionDetail(id)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, version)
}

// GetActiveConfigVersion godoc
// @Summary Get active config version
// @Tags ConfigVersions
// @Produce json
// @Security OpenFlareTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/config-versions/active [get]
func GetActiveConfigVersion(c *gin.Context) {
	version, err := service.GetActiveConfigVersion()
	if err != nil {
		response.RespondFailure(c, "当前没有激活版本")
		return
	}
	response.RespondSuccess(c, version)
}

// PreviewConfigVersion godoc
// @Summary Preview config rendering
// @Tags ConfigVersions
// @Produce json
// @Security OpenFlareTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/config-versions/preview [get]
func PreviewConfigVersion(c *gin.Context) {
	preview, err := service.PreviewConfigVersion()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, preview)
}

// DiffConfigVersion godoc
// @Summary Diff current draft against active version
// @Tags ConfigVersions
// @Produce json
// @Security OpenFlareTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/config-versions/diff [get]
func DiffConfigVersion(c *gin.Context) {
	diff, err := service.DiffConfigVersion()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, diff)
}

// PublishConfigVersion godoc
// @Summary Publish a new config version
// @Tags ConfigVersions
// @Produce json
// @Security OpenFlareTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/config-versions/publish [post]
func PublishConfigVersion(c *gin.Context) {
	username := c.GetString("username")
	force := c.Query("force") == "true"
	result, err := service.PublishConfigVersion(username, force)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, result.Version)
}

// ActivateConfigVersion godoc
// @Summary Activate an existing config version
// @Tags ConfigVersions
// @Produce json
// @Security OpenFlareTokenAuth
// @Param id path int true "Version ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/config-versions/{id}/activate [post]
func ActivateConfigVersion(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}
	version, err := service.ActivateConfigVersion(id)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, version)
}

type CleanupConfigVersionRequest struct {
	KeepCount int `json:"keep_count" binding:"required,min=3"`
}

// CleanupConfigVersions godoc
// @Summary Cleanup old config versions
// @Tags ConfigVersions
// @Produce json
// @Security OpenFlareTokenAuth
// @Param request body CleanupConfigVersionRequest true "Cleanup request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/config-versions/cleanup [post]
func CleanupConfigVersions(c *gin.Context) {
	var req CleanupConfigVersionRequest
	if !bind.JSON(c, &req) {
		return
	}

	deletedCount, err := service.CleanupConfigVersions(req.KeepCount)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}

	response.RespondSuccessWithExtras(c, map[string]interface{}{"deleted_count": deletedCount}, gin.H{
		"message": "清理成功",
	})
}
