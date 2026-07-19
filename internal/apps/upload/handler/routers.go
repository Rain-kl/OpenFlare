// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package handler provides upload HTTP API handlers.
package handler

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/filesrv"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/ingest"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/shared"
	uploadstorage "github.com/Rain-kl/Wavelet/internal/apps/upload/storage"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/util"
	"github.com/Rain-kl/Wavelet/internal/common"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type batchDownloadRequest struct {
	IDs []string `json:"ids" binding:"required,min=1"`
}

// UploadFile 通用上传文件接口
// @Summary 上传文件
// @Description 支持各种类型的通用文件上传，支持自动文件类型检测、哈希计算与“秒传”去重
// @Tags upload
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "要上传的文件"
// @Param type formData string false "业务分类 (例如: avatar, attachment, doc，默认为 generic)"
// @Param metadata formData string false "额外的 JSON 格式元数据"
// @Security SessionCookie
// @Success 200 {object} response.Any{data=model.Upload} "上传成功"
// @Failure 400 {object} response.Any "请求参数错误或文件受限"
// @Failure 401 {object} response.Any "未登录"
// @Failure 409 {object} response.Any "系统保留类型或存储只读"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/upload [post]
//
//nolint:revive
func UploadFile(c *gin.Context) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox")

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, shared.MaxUploadSize)

	currUser, _ := oauth.GetFromContext[*model.User](c, oauth.UserObjKey)
	ctx := c.Request.Context()

	header, err := c.FormFile("file")
	if err != nil {
		response.AbortBadRequest(c, shared.ErrNoFileSelected)
		return
	}

	file, err := header.Open()
	if err != nil {
		response.AbortBadRequest(c, shared.ErrOpenFileFailed)
		return
	}
	defer func() { _ = file.Close() }()

	if header.Size > shared.MaxUploadSize {
		response.AbortBadRequest(c, shared.ErrGenericFileTooLarge)
		return
	}

	origName := header.Filename
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(origName), "."))
	if ext == "" {
		ext = "bin"
	}

	hashWriter := sha256.New()
	var buf bytes.Buffer
	size, err := io.Copy(&buf, io.TeeReader(file, hashWriter))
	if err != nil {
		response.AbortBadRequest(c, shared.ErrProcessFileFailed)
		return
	}

	fileHash := hex.EncodeToString(hashWriter.Sum(nil))
	mimeType := detectMimeType(&buf, header, size)

	if util.IsImageExtension(ext) && !strings.HasPrefix(mimeType, "image/") {
		response.AbortBadRequest(c, shared.ErrFileContentExtensionMismatch)
		return
	}

	uploadType := c.DefaultPostForm("type", "generic")
	if uploadType == shared.ReservedPagesDeploymentType {
		response.AbortConflict(c, shared.ErrReservedUploadType)
		return
	}

	accessMode, errMsg := resolveUploadAccessMode(c, uploadType)
	if errMsg != "" {
		response.AbortBadRequest(c, errMsg)
		return
	}

	meta, errMsg := parseUploadMetadata(c, mimeType)
	if errMsg != "" {
		response.AbortBadRequest(c, errMsg)
		return
	}

	result, err := ingest.Ingest(ctx, ingest.Request{
		UserID:     currUser.ID,
		Reader:     bytes.NewReader(buf.Bytes()),
		Size:       size,
		FileName:   origName,
		MimeType:   mimeType,
		Extension:  ext,
		Hash:       fileHash,
		Type:       uploadType,
		AccessMode: &accessMode,
		Metadata:   meta,
		Policy:     ingest.PolicyDedupNewRecord,
	})
	if err != nil {
		if errors.Is(err, ingest.ErrStorageReadOnly) {
			response.AbortConflict(c, shared.ErrStorageReadOnly)
			return
		}
		if err.Error() == shared.ErrUnsupportedFormat {
			response.AbortBadRequest(c, shared.ErrUnsupportedFormat)
			return
		}
		if err.Error() == shared.ErrSaveFileFailed {
			response.AbortBadRequest(c, shared.ErrSaveFileFailed)
			return
		}
		response.AbortBadRequest(c, shared.ErrSaveUploadRecordFailed)
		return
	}

	c.JSON(http.StatusOK, response.OK(result.Upload))
}

// DownloadFile 通用单文件下载接口
// @Summary 下载单文件
// @Description 根据文件 ID 获取文件，以附件形式 (Attachment) 强制开启客户端浏览器下载
// @Tags admin
// @Produce octet-stream
// @Param id path string true "文件 ID"
// @Param quality query string false "图片质量 (low, medium, high, origin)，默认为 origin"
// @Security SessionCookie
// @Success 200 {file} file "成功下载文件"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 404 {object} response.Any "文件不存在"
// @Failure 500 {object} response.Any "服务内部错误"
// @Router /api/v1/admin/uploads/download/{id} [get]
func DownloadFile(c *gin.Context) {
	upload, err := filesrv.GetUploadRecordByID(c)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.AbortNotFound(c, "文件记录未找到")
			return
		}
		if _, ok := err.(*strconv.NumError); ok {
			response.AbortBadRequest(c, shared.ErrInvalidFileID)
			return
		}
		response.AbortBadRequest(c, shared.ErrQueryUploadRecordFailed)
		return
	}

	if err := filesrv.CheckFileAccessPermission(c, upload); err != nil {
		response.AbortUnauthorized(c, common.UnAuthorized)
		return
	}

	fileName := upload.FileName
	quality := util.NormalizeImageQuality(c.Query("quality"))
	isImage := strings.HasPrefix(strings.ToLower(upload.MimeType), "image/") || util.IsImageExtension(strings.ToLower(upload.Extension))

	if quality != shared.ImageQualityOrigin && isImage {
		ext := filepath.Ext(fileName)
		if ext != "" {
			fileName = strings.TrimSuffix(fileName, ext) + ".webp"
		} else {
			fileName += ".webp"
		}
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(fileName)))
	filesrv.ServeUpload(c, upload)
}

// BatchDownloadFiles 批量打包 ZIP 下载接口
// @Summary 批量打包下载
// @Description 传入多个文件 ID，后台实时将其打包压缩为 ZIP 流并输出，自动处理文件名重复冲突
// @Tags admin
// @Accept json
// @Produce octet-stream
// @Param request body handler.batchDownloadRequest true "包含文件 ID 数组 of string 的请求体"
// @Security SessionCookie
// @Success 200 {file} file "成功下载打包后的 ZIP"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 500 {object} response.Any "打包失败"
// @Router /api/v1/admin/uploads/download/batch [post]
func BatchDownloadFiles(c *gin.Context) {
	ctx := c.Request.Context()

	var req batchDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, shared.ErrInvalidBatchDownloadRequest)
		return
	}

	var ids []uint64
	for _, idStr := range req.IDs {
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			response.AbortBadRequest(c, fmt.Sprintf(shared.ErrInvalidIDValueFormat, idStr))
			return
		}
		ids = append(ids, id)
	}

	uploads, err := listUploadsForBatchDownload(ctx, ids)
	if err != nil {
		response.AbortBadRequest(c, shared.ErrRetrieveUploadRecordsFailed)
		return
	}

	if len(uploads) == 0 {
		response.AbortBadRequest(c, shared.ErrNoValidFilesForArchive)
		return
	}

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", "attachment; filename=\"batch_download.zip\"")

	bufferedWriter := bufio.NewWriter(c.Writer)
	zipWriter := zip.NewWriter(bufferedWriter)
	defer func() {
		_ = zipWriter.Close()
		_ = bufferedWriter.Flush()
	}()

	usedNames := make(map[string]int)

	for _, upload := range uploads {
		if err := filesrv.CheckFileAccessPermission(c, &upload); err != nil {
			logger.WarnF(ctx, "Batch download: skip file %d due to permission denied: %v", upload.ID, err)
			continue
		}

		fileName := upload.FileName
		if count, exists := usedNames[fileName]; exists {
			usedNames[fileName] = count + 1
			ext := filepath.Ext(fileName)
			base := strings.TrimSuffix(fileName, ext)
			fileName = fmt.Sprintf("%s_%d%s", base, count, ext)
		} else {
			usedNames[fileName] = 1
		}

		zipFileEntry, err := zipWriter.Create(fileName)
		if err != nil {
			logger.ErrorF(ctx, "ZIP 添加条目失败 [%s]: %v", fileName, err)
			continue
		}

		obj, err := uploadstorage.OpenStoredObject(ctx, &upload)
		if err != nil {
			logger.ErrorF(ctx, "打包时读取文件失败: %v", err)
			continue
		}
		rc := obj.Body

		_, err = io.Copy(zipFileEntry, rc)
		_ = rc.Close()
		if err != nil {
			logger.ErrorF(ctx, "写入 ZIP 流失败: %v", err)
		}
	}
}

func resolveUploadAccessMode(c *gin.Context, uploadType string) (int, string) {
	accessModeStr := c.PostForm("access_mode")
	if accessModeStr == "" {
		if uploadType == shared.DefaultPublicUploadType {
			return 1, ""
		}
		return 0, ""
	}

	accessMode, err := strconv.Atoi(accessModeStr)
	if err != nil || (accessMode != 0 && accessMode != 1) {
		return 0, "无效的 access_mode 参数"
	}
	return accessMode, ""
}

func parseUploadMetadata(c *gin.Context, mimeType string) (model.UploadMetadata, string) {
	var meta model.UploadMetadata
	metadataStr := c.DefaultPostForm("metadata", "")
	if metadataStr != "" {
		if err := json.Unmarshal([]byte(metadataStr), &meta); err != nil {
			return meta, shared.ErrInvalidMetadataJSON
		}
	}
	meta.OriginalMime = mimeType
	meta.UserAgent = c.Request.UserAgent()
	meta.ClientIP = c.ClientIP()
	return meta, ""
}

func detectMimeType(buf *bytes.Buffer, header *multipart.FileHeader, size int64) string {
	mimeType := http.DetectContentType(buf.Bytes()[:min(shared.DetectContentBytes, int(size))])
	if mimeType == "application/octet-stream" && header.Header.Get("Content-Type") != "" {
		mimeType = header.Header.Get("Content-Type")
	}
	return mimeType
}
