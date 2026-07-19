// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func handleLogicError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	return apiutil.AbortNotFoundIfMissing(c, err, errPagesProjectNotFound)
}

func handleSourceLogicError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrRecordNotFound) || err.Error() == errPagesSourceNotFound {
		response.AbortNotFound(c, errPagesSourceNotFound)
		return true
	}
	switch err.Error() {
	case errPagesSourceActionBusy:
		response.AbortConflict(c, errPagesSourceActionBusy)
	case errPagesSourceTypeRequired,
		errPagesSourceTypeUnsupported,
		errPagesSourceRemoteFields,
		errPagesSourceRemoteURLRequired,
		errPagesSourceRemoteURLMode,
		errPagesSourceRemoteURLInvalid,
		errPagesSourceNetworkPolicy,
		errPagesSourceCheckUnsupported,
		errPagesSourceActionInvalid:
		response.AbortBadRequest(c, err.Error())
	case errPagesSourceTaskDispatchFailed:
		response.AbortInternal(c, errPagesSourceInternal)
	default:
		logger.ErrorF(c.Request.Context(), "[PagesSource] API operation failed: error=%v", err)
		response.AbortInternal(c, errPagesSourceInternal)
	}
	return true
}

func decodeStrictJSON(c *gin.Context, target any, allowEmpty bool) bool {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if allowEmpty && errors.Is(err, io.EOF) {
			return true
		}
		response.AbortBadRequest(c, errPagesSourceActionInvalid)
		return false
	}
	if err := ensureJSONEOF(decoder); err != nil {
		response.AbortBadRequest(c, errPagesSourceActionInvalid)
		return false
	}
	return true
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

func currentPagesActor(c *gin.Context) (string, bool) {
	user, ok := oauth.GetFromContext[*model.User](c, oauth.UserObjKey)
	if !ok || user == nil || user.ID == 0 {
		response.AbortUnauthorized(c, errPagesActorMissing)
		return "", false
	}
	return fmt.Sprintf("user:%d", user.ID), true
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

// GetSourceHandler 获取 Pages 项目的部署源。
// @Summary 获取 Pages 部署源
// @Description 返回脱敏后的项目部署源配置与运行状态，需要管理员权限
// @Tags openflare-pages
// @Produce json
// @Security SessionCookie
// @Param id path int true "项目 ID"
// @Success 200 {object} response.Any{data=pages.SourceView} "部署源"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "项目或部署源不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pages/{id}/source [get]
func GetSourceHandler(c *gin.Context) {
	projectID, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	source, err := GetSource(c.Request.Context(), projectID)
	if handleSourceLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(source))
}

// UpdateSourceHandler 创建或更新 Pages 项目部署源。
// @Summary 更新 Pages 部署源
// @Description Phase 1 支持 Remote URL 来源；完整地址仅写入，不会在响应中返回
// @Tags openflare-pages
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "项目 ID"
// @Param request body pages.SourceUpdateInput true "部署源配置"
// @Success 200 {object} response.Any{data=pages.SourceUpdateResult} "更新结果"
// @Failure 400 {object} response.Any "配置无效"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "项目不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pages/{id}/source/update [post]
func UpdateSourceHandler(c *gin.Context) {
	projectID, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input SourceUpdateInput
	if !decodeStrictJSON(c, &input, false) {
		return
	}
	result, err := UpdateSource(c.Request.Context(), projectID, input)
	if handleSourceLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

// DeleteSourceHandler 将 Pages 项目切换回手动部署模式。
// @Summary 删除 Pages 部署源
// @Description 幂等删除持久部署源；已有部署历史与当前生产部署保持不变
// @Tags openflare-pages
// @Produce json
// @Security SessionCookie
// @Param id path int true "项目 ID"
// @Success 200 {object} response.Any{data=pages.SourceView} "手动来源视图"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "项目不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pages/{id}/source/delete [post]
func DeleteSourceHandler(c *gin.Context) {
	projectID, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	source, err := DeleteSource(c.Request.Context(), projectID)
	if handleSourceLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(source))
}

// CheckSourceHandler 请求检查 Pages 部署源。
// @Summary 检查 Pages 部署源
// @Description Remote URL 来源不支持检查更新；该端点为后续 GitHub Release 来源保留统一动作契约
// @Tags openflare-pages
// @Produce json
// @Security SessionCookie
// @Param id path int true "项目 ID"
// @Success 200 {object} response.Any{data=pages.SourceActionReceipt} "任务回执"
// @Failure 400 {object} response.Any "当前来源不支持检查"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "部署源不存在"
// @Failure 409 {object} response.Any "来源任务正在执行"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pages/{id}/source/check [post]
func CheckSourceHandler(c *gin.Context) {
	projectID, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	actor, ok := currentPagesActor(c)
	if !ok {
		return
	}
	receipt, err := DispatchSourceAction(c.Request.Context(), projectID, sourceActionCheck, actor, "")
	if handleSourceLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(receipt))
}

// SourceSyncInput is the optional source sync action payload.
type SourceSyncInput struct {
	ConfirmedRevision string `json:"confirmed_revision"`
}

// SyncSourceHandler 请求同步并发布 Pages 部署源。
// @Summary 同步并发布 Pages 部署源
// @Description 异步下载、校验并原子激活来源部署包；空请求体与空 JSON 对象均有效
// @Tags openflare-pages
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "项目 ID"
// @Param request body pages.SourceSyncInput false "同步参数"
// @Success 200 {object} response.Any{data=pages.SourceActionReceipt} "任务回执"
// @Failure 400 {object} response.Any "参数或来源类型无效"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "部署源不存在"
// @Failure 409 {object} response.Any "来源任务正在执行"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pages/{id}/source/sync [post]
func SyncSourceHandler(c *gin.Context) {
	projectID, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input SourceSyncInput
	if !decodeStrictJSON(c, &input, true) {
		return
	}
	actor, ok := currentPagesActor(c)
	if !ok {
		return
	}
	receipt, err := DispatchSourceAction(
		c.Request.Context(),
		projectID,
		sourceActionSync,
		actor,
		input.ConfirmedRevision,
	)
	if handleSourceLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(receipt))
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
// @Description 为指定项目上传静态资源压缩包（zip/tar.gz/tar.xz/tar.bz2/tar/7z），需要管理员权限
// @Tags openflare-pages
// @Accept multipart/form-data
// @Produce json
// @Security SessionCookie
// @Param id path int true "项目 ID"
// @Param package formData file true "部署包文件"
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
	actor, ok := currentPagesActor(c)
	if !ok {
		return
	}
	deployment, err := UploadDeployment(c.Request.Context(), id, file, actor)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(deployment))
}

// UploadDeploymentFromURLHandler 从 URL 下载并创建 Pages 部署。
// @Summary 从 URL 导入 Pages 部署包
// @Description 已弃用的一次性 URL 导入；使用 trusted_internal 策略兼容内网与自签名证书，不创建持久部署源
// @Deprecated
// @Tags openflare-pages
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "项目 ID"
// @Param request body pages.UploadFromURLInput true "下载链接"
// @Success 200 {object} response.Any{data=pages.DeploymentView} "部署记录"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 404 {object} response.Any "项目不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pages/{id}/deployments/upload-from-url [post]
func UploadDeploymentFromURLHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var req UploadFromURLInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, errPagesPackageURLRequired)
		return
	}
	actor, ok := currentPagesActor(c)
	if !ok {
		return
	}
	deployment, err := UploadDeploymentFromURL(c.Request.Context(), id, req.URL, actor)
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
	actor, ok := currentPagesActor(c)
	if !ok {
		return
	}
	project, err := ActivateDeploymentAs(c.Request.Context(), projectID, deploymentID, actor)
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
