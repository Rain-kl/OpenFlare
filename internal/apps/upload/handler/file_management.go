// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/ingest"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/shared"
	uploadstorage "github.com/Rain-kl/Wavelet/internal/apps/upload/storage"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type listFilesRequest struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	Keyword   string `form:"keyword"`
	Type      string `form:"type"`
	Extension string `form:"extension"`
	UserID    uint64 `form:"user_id"`
}

type listFilesResponse struct {
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Items    []model.Upload `json:"items"`
}

// ListFiles 获取系统上传的文件列表
// @Summary 获取文件列表
// @Description 分页获取系统上传的文件列表，支持文件名关键词、业务类型、扩展名、上传用户ID过滤
// @Tags admin
// @Produce json
// @Param page query int false "页码（默认 1）"
// @Param page_size query int false "每页数量（默认 20，最大 100）"
// @Param keyword query string false "文件名关键词（模糊匹配）"
// @Param type query string false "业务分类过滤"
// @Param extension query string false "扩展名过滤"
// @Param user_id query uint64 false "上传用户 ID"
// @Security SessionCookie
// @Success 200 {object} response.Any{data=listFilesResponse} "查询成功"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/uploads [get]
func ListFiles(c *gin.Context) {
	ctx := c.Request.Context()

	var req listFilesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.AbortBadRequest(c, shared.ErrInvalidParams)
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 20
	}

	total, items, err := listUploadFiles(ctx, repository.UploadListFilter{
		UserID:    req.UserID,
		Keyword:   req.Keyword,
		Type:      req.Type,
		Extension: req.Extension,
		Page:      req.Page,
		PageSize:  req.PageSize,
	})
	if err != nil {
		response.AbortBadRequest(c, shared.ErrQueryFileListFailed)
		return
	}

	c.JSON(http.StatusOK, response.OK(listFilesResponse{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}))
}

// DeleteFile 软删除文件记录
// @Summary 删除文件
// @Description 将文件状态置为 deleted（软删除），不会立即清理底层存储对象
// @Tags admin
// @Produce json
// @Param id path string true "文件 ID"
// @Security SessionCookie
// @Success 200 {object} response.Any "删除成功"
// @Failure 403 {object} response.Any "无权操作"
// @Failure 404 {object} response.Any "文件不存在"
// @Failure 409 {object} response.Any "系统保留类型或存储只读"
// @Router /api/v1/admin/uploads/{id} [delete]
func DeleteFile(c *gin.Context) {
	ctx := c.Request.Context()
	if uploadstorage.ReadOnly(ctx) {
		response.AbortConflict(c, shared.ErrStorageReadOnly)
		return
	}

	uploadID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, shared.ErrInvalidFileID)
		return
	}

	if _, err := softDeleteUpload(ctx, uploadID); err != nil {
		if errors.Is(err, ingest.ErrReservedUploadType) {
			response.AbortConflict(c, shared.ErrReservedUploadType)
			return
		}
		if isRecordNotFound(err) {
			response.AbortNotFound(c, "文件记录未找到")
			return
		}
		response.AbortBadRequest(c, shared.ErrDeleteFileFailed)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// GetDistinctUploadTypes 获取数据库中所有已存在的文件业务类型
// @Summary 获取文件业务类型列表
// @Description 返回数据库中所有已上传文件实际拥有的业务类型列表
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]string} "业务类型列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/uploads/types [get]
func GetDistinctUploadTypes(c *gin.Context) {
	types, err := listDistinctUploadTypes(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(types))
}

type listMyFilesRequest struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	Keyword   string `form:"keyword"`
	Type      string `form:"type"`
	Extension string `form:"extension"`
}

type listMyFilesResponse struct {
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Items    []model.Upload `json:"items"`
}

// ListMyFiles 获取当前用户上传的文件列表
// @Summary 获取我的文件列表
// @Description 分页获取当前登录用户上传的文件，支持文件名关键词、业务类型、扩展名过滤
// @Tags upload
// @Produce json
// @Param page query int false "页码（默认 1）"
// @Param page_size query int false "每页数量（默认 20，最大 100）"
// @Param keyword query string false "文件名关键词（模糊匹配）"
// @Param type query string false "业务分类过滤"
// @Param extension query string false "扩展名过滤"
// @Security SessionCookie
// @Success 200 {object} response.Any{data=listMyFilesResponse} "查询成功"
// @Failure 401 {object} response.Any "未登录"
// @Router /api/v1/upload/my [get]
func ListMyFiles(c *gin.Context) {
	currUser, _ := oauth.GetFromContext[*model.User](c, oauth.UserObjKey)
	ctx := c.Request.Context()

	var req listMyFilesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.AbortBadRequest(c, shared.ErrInvalidParams)
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 20
	}

	total, items, err := listMyUploadFiles(ctx, currUser.ID, repository.UploadListFilter{
		Keyword:   req.Keyword,
		Type:      req.Type,
		Extension: req.Extension,
		Page:      req.Page,
		PageSize:  req.PageSize,
	})
	if err != nil {
		response.AbortBadRequest(c, shared.ErrQueryFileListFailed)
		return
	}

	c.JSON(http.StatusOK, response.OK(listMyFilesResponse{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}))
}

// DeleteMyFile 软删除当前用户本人的文件
// @Summary 删除我的文件
// @Description 将当前用户本人的文件状态置为 deleted（软删除）
// @Tags upload
// @Produce json
// @Param id path string true "文件 ID"
// @Security SessionCookie
// @Success 200 {object} response.Any "删除成功"
// @Failure 403 {object} response.Any "无权操作"
// @Failure 404 {object} response.Any "文件不存在"
// @Failure 409 {object} response.Any "系统保留类型或存储只读"
// @Router /api/v1/upload/{id} [delete]
func DeleteMyFile(c *gin.Context) {
	currUser, _ := oauth.GetFromContext[*model.User](c, oauth.UserObjKey)
	ctx := c.Request.Context()
	if uploadstorage.ReadOnly(ctx) {
		response.AbortConflict(c, shared.ErrStorageReadOnly)
		return
	}

	uploadID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, shared.ErrInvalidFileID)
		return
	}

	if _, err := softDeleteOwnedUpload(ctx, currUser.ID, uploadID); err != nil {
		if errors.Is(err, ingest.ErrReservedUploadType) {
			response.AbortConflict(c, shared.ErrReservedUploadType)
			return
		}
		if isRecordNotFound(err) {
			response.AbortNotFound(c, "文件记录未找到")
			return
		}
		if errors.Is(err, ingest.ErrForbidden) {
			response.AbortForbidden(c, "无权操作")
			return
		}
		response.AbortBadRequest(c, shared.ErrDeleteFileFailed)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

type updateMyFileRequest struct {
	FileName   string `json:"file_name" binding:"max=255"`
	AccessMode *int   `json:"access_mode" binding:"omitempty,oneof=0 1"`
}

// UpdateMyFile 更新当前用户本人的文件信息
// @Summary 更新我的文件信息
// @Description 更新当前用户本人的文件名或访问权限模式 (AccessMode)
// @Tags upload
// @Accept json
// @Produce json
// @Param id path string true "文件 ID"
// @Param request body updateMyFileRequest true "更新字段"
// @Security SessionCookie
// @Success 200 {object} response.Any{data=model.Upload} "更新成功"
// @Failure 403 {object} response.Any "无权操作"
// @Failure 404 {object} response.Any "文件不存在"
// @Router /api/v1/upload/{id} [put]
func UpdateMyFile(c *gin.Context) {
	currUser, _ := oauth.GetFromContext[*model.User](c, oauth.UserObjKey)
	ctx := c.Request.Context()
	if uploadstorage.ReadOnly(ctx) {
		response.AbortConflict(c, shared.ErrStorageReadOnly)
		return
	}

	uploadID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, shared.ErrInvalidFileID)
		return
	}

	var req updateMyFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, shared.ErrInvalidParams)
		return
	}

	upload, err := updateOwnedUpload(ctx, currUser.ID, uploadID, updateMyUploadInput(req))
	if err != nil {
		if isRecordNotFound(err) {
			response.AbortNotFound(c, "文件记录未找到")
			return
		}
		if err == ingest.ErrForbidden {
			response.AbortForbidden(c, "无权操作")
			return
		}
		response.AbortBadRequest(c, "更新文件记录失败")
		return
	}

	c.JSON(http.StatusOK, response.OK(upload))
}
