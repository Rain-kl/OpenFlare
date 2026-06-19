// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"net/http"
	"strconv"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/gin-gonic/gin"
)


func handleLogicError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	return apiutil.AbortNotFoundIfMissing(c, err, errPagesProjectNotFound)
}

func deploymentIDParam(c *gin.Context) (uint, bool) {
	raw := c.Param("deployment_id")
	if raw == "" {
		response.AbortBadRequest(c, "无效的 ID")
		return 0, false
	}
	id64, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id64 == 0 {
		response.AbortBadRequest(c, "无效的 ID")
		return 0, false
	}
	return uint(id64), true
}

// ListProjectsHandler 列出全部 Pages 项目。
// @Summary 列出 Pages 项目
// @Description 返回全部 OpenFlare Pages 项目，需要管理员权限
// @Tags openflare-pages
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]pages.View} "Pages 项目列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pages [get]
func ListProjectsHandler(c *gin.Context) {
	projects, err := ListProjects(c.Request.Context())
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(projects))
}

// GetProjectHandler 获取 Pages 项目详情。
// @Summary 获取 Pages 项目详情
// @Description 按 ID 返回 Pages 项目详情，需要管理员权限
// @Tags openflare-pages
// @Produce json
// @Security SessionCookie
// @Param id path int true "项目 ID"
// @Success 200 {object} response.Any{data=pages.View} "Pages 项目详情"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "项目不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pages/{id} [get]
func GetProjectHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	project, err := GetProject(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(project))
}

// CreateProjectHandler 创建 Pages 项目。
// @Summary 创建 Pages 项目
// @Description 创建新的 OpenFlare Pages 项目，需要管理员权限
// @Tags openflare-pages
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body pages.Input true "项目参数"
// @Success 200 {object} response.Any{data=pages.View} "创建成功的项目"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pages [post]
func CreateProjectHandler(c *gin.Context) {
	var input Input
	if !apiutil.BindJSON(c, &input) {
		return
	}
	project, err := CreateProject(c.Request.Context(), input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(project))
}

// UpdateProjectHandler 更新 Pages 项目。
// @Summary 更新 Pages 项目
// @Description 按 ID 更新 OpenFlare Pages 项目，需要管理员权限
// @Tags openflare-pages
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "项目 ID"
// @Param request body pages.Input true "项目参数"
// @Success 200 {object} response.Any{data=pages.View} "更新后的项目"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "项目不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pages/{id}/update [post]
func UpdateProjectHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input Input
	if !apiutil.BindJSON(c, &input) {
		return
	}
	project, err := UpdateProject(c.Request.Context(), id, input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(project))
}

// DeleteProjectHandler 删除 Pages 项目。
// @Summary 删除 Pages 项目
// @Description 按 ID 删除 OpenFlare Pages 项目，需要管理员权限
// @Tags openflare-pages
// @Produce json
// @Security SessionCookie
// @Param id path int true "项目 ID"
// @Success 200 {object} response.Any "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "项目不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pages/{id}/delete [post]
func DeleteProjectHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	if err := DeleteProject(c.Request.Context(), id); handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// ListDeploymentsHandler 列出项目的全部部署。
// @Summary 列出 Pages 部署
// @Description 返回指定项目的全部部署记录，需要管理员权限
// @Tags openflare-pages
// @Produce json
// @Security SessionCookie
// @Param id path int true "项目 ID"
// @Success 200 {object} response.Any{data=[]pages.DeploymentView} "部署列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "项目不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pages/{id}/deployments [get]
func ListDeploymentsHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	deployments, err := ListProjectDeployments(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(deployments))
}

// UploadDeploymentHandler 上传 Pages 部署包。
// @Summary 上传 Pages 部署包
// @Description 为指定项目上传 ZIP 部署包，需要管理员权限
// @Tags openflare-pages
// @Accept multipart/form-data
// @Produce json
// @Security SessionCookie
// @Param id path int true "项目 ID"
// @Param package formData file true "部署包 ZIP 文件"
// @Success 200 {object} response.Any{data=pages.DeploymentView} "部署记录"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "项目不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pages/{id}/deployments/upload [post]
func UploadDeploymentHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	file, err := c.FormFile("package")
	if err != nil {
		response.AbortBadRequest(c, errPagesPackageMissing)
		return
	}
	deployment, err := UploadDeployment(c.Request.Context(), id, file, "")
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(deployment))
}

// ActivateDeploymentHandler 激活 Pages 部署。
// @Summary 激活 Pages 部署
// @Description 将指定部署设为项目当前生效版本，需要管理员权限
// @Tags openflare-pages
// @Produce json
// @Security SessionCookie
// @Param id path int true "项目 ID"
// @Param deployment_id path int true "部署 ID"
// @Success 200 {object} response.Any{data=pages.View} "激活后的项目"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "项目或部署不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pages/{id}/deployments/{deployment_id}/activate [post]
func ActivateDeploymentHandler(c *gin.Context) {
	projectID, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	deploymentID, ok := deploymentIDParam(c)
	if !ok {
		return
	}
	project, err := ActivateDeployment(c.Request.Context(), projectID, deploymentID)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(project))
}

// DeleteDeploymentHandler 删除 Pages 部署。
// @Summary 删除 Pages 部署
// @Description 删除指定项目的部署记录，需要管理员权限
// @Tags openflare-pages
// @Produce json
// @Security SessionCookie
// @Param id path int true "项目 ID"
// @Param deployment_id path int true "部署 ID"
// @Success 200 {object} response.Any "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "项目或部署不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pages/{id}/deployments/{deployment_id}/delete [post]
func DeleteDeploymentHandler(c *gin.Context) {
	projectID, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	deploymentID, ok := deploymentIDParam(c)
	if !ok {
		return
	}
	if err := DeleteDeployment(c.Request.Context(), projectID, deploymentID); handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// ListDeploymentFilesHandler 列出部署文件清单。
// @Summary 列出 Pages 部署文件
// @Description 返回指定部署包含的文件清单，需要管理员权限
// @Tags openflare-pages
// @Produce json
// @Security SessionCookie
// @Param deployment_id path int true "部署 ID"
// @Success 200 {object} response.Any{data=[]pages.DeploymentFileView} "部署文件列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "部署不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pages/deployments/{deployment_id}/files [get]
func ListDeploymentFilesHandler(c *gin.Context) {
	deploymentID, ok := deploymentIDParam(c)
	if !ok {
		return
	}
	files, err := ListDeploymentFiles(c.Request.Context(), deploymentID)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(files))
}