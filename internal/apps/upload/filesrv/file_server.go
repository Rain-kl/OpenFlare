// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package filesrv serves uploaded files with access control and image compression.
package filesrv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/cache"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/shared"
	uploadstorage "github.com/Rain-kl/Wavelet/internal/apps/upload/storage"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/util"
	"github.com/Rain-kl/Wavelet/internal/infra/diskcache"
	"github.com/Rain-kl/Wavelet/internal/model"
	appshared "github.com/Rain-kl/Wavelet/internal/shared"
	"github.com/Rain-kl/Wavelet/internal/shared/response"

	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

var compressedImageFlight singleflight.Group

type compressedImageCacheResult struct {
	bytes  []byte
	cached bool
	err    error
}

type fileTypeCategory string

const (
	fileTypeImage fileTypeCategory = "image"
	fileTypeVideo fileTypeCategory = "video"
	fileTypeAudio fileTypeCategory = "audio"
	fileTypeOther fileTypeCategory = "other"
)

// ServeFileByID 根据 ID 获取并提供已上传的文件
// @Summary 获取已上传文件
// @Description 根据文件 ID 获取并提供已上传的临时或正式文件，若配置了缓存则优先走本地缓存，否则从 S3 等后端存储读取并流式返回
// @Tags upload
// @Produce octet-stream
// @Param id path string true "文件 ID"
// @Param quality query string false "图片质量 (low, medium, high, origin)，默认为 origin"
// @Success 200 {file} file "成功获取文件内容"
// @Failure 400 {object} response.Any "文件 ID 格式错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "文件未找到"
// @Failure 500 {object} response.Any "服务内部错误"
// @Router /f/{id} [get]
func ServeFileByID(c *gin.Context) {
	upload, err := GetUploadRecordByID(c)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.AbortNotFound(c, "文件记录未找到")
			return
		}
		if _, ok := err.(*strconv.NumError); ok {
			response.AbortBadRequest(c, "无效的上传ID")
			return
		}
		response.AbortInternal(c, "服务器内部错误")
		return
	}

	if err := CheckFileAccessPermission(c, upload); err != nil {
		response.AbortUnauthorized(c, appshared.UnAuthorized)
		return
	}

	ServeUpload(c, upload)
}

// GetUploadRecordByID 从请求路径参数中解析文件 ID 并从数据库中检索处于 Pending 或 Used 状态的上传记录。
func GetUploadRecordByID(c *gin.Context) (*model.Upload, error) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox")

	idStr := c.Param("id")
	uploadID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return nil, err
	}

	upload, err := cache.GetUploadByID(c.Request.Context(), uploadID)
	if err != nil {
		return nil, err
	}

	return &upload, nil
}

func getFileTypeCategory(upload *model.Upload) fileTypeCategory {
	mime := strings.ToLower(upload.MimeType)
	ext := strings.ToLower(upload.Extension)

	if strings.HasPrefix(mime, "image/") || util.IsImageExtension(ext) {
		return fileTypeImage
	}
	if strings.HasPrefix(mime, "video/") {
		return fileTypeVideo
	}
	if strings.HasPrefix(mime, "audio/") {
		return fileTypeAudio
	}
	return fileTypeOther
}

// ServeUpload 将已存在的文件内容读取并流式响应给客户端。
func ServeUpload(c *gin.Context, upload *model.Upload) {
	setCacheHeaders(c, upload)

	category := getFileTypeCategory(upload)
	quality := util.NormalizeImageQuality(c.Query("quality"))

	switch category {
	case fileTypeImage:
		if quality != shared.ImageQualityOrigin {
			serveCompressedImage(c, upload, quality)
			return
		}
		fallthrough
	default:
		serveOriginalWithConditionalCheck(c, upload)
	}
}

func setCacheHeaders(c *gin.Context, upload *model.Upload) {
	if cache.IsFilePublic(c.Request.Context(), upload.Type) {
		c.Header("Cache-Control", "public, max-age=31536000")
	} else {
		c.Header("Cache-Control", "private, no-cache")
	}
}

func serveOriginalWithConditionalCheck(c *gin.Context, upload *model.Upload) {
	etag := fmt.Sprintf(`W/"%s"`, upload.Hash)
	c.Header("ETag", etag)

	if c.GetHeader("If-None-Match") == etag {
		c.AbortWithStatus(http.StatusNotModified)
		return
	}

	serveOriginal(c, upload)
}

func serveCompressedImage(c *gin.Context, upload *model.Upload, quality string) {
	etag := fmt.Sprintf(`W/"%s-%s"`, upload.Hash, quality)
	c.Header("ETag", etag)

	if c.GetHeader("If-None-Match") == etag {
		c.AbortWithStatus(http.StatusNotModified)
		return
	}

	webpBytes, _, err := EnsureCompressedImageCache(c.Request.Context(), upload, quality)
	if err != nil {
		if len(webpBytes) > 0 {
			logger.WarnF(c.Request.Context(), "failed to cache compressed image: %v", err)
			c.Data(http.StatusOK, "image/webp", webpBytes)
			return
		}
		logger.ErrorF(c.Request.Context(), "failed to prepare compressed image cache: %v", err)
		serveOriginal(c, upload)
		return
	}

	c.Data(http.StatusOK, "image/webp", webpBytes)
}

// EnsureCompressedImageCache returns cached or freshly generated WebP bytes for an upload.
func EnsureCompressedImageCache(
	ctx context.Context,
	upload *model.Upload,
	quality string,
) ([]byte, bool, error) {
	cacheStore := diskcache.GetGlobalCache()
	cacheKey := ImageCompressionCacheKey(upload, quality)
	webpBytes, err := cacheStore.Get(cacheKey)
	if err == nil {
		return webpBytes, true, nil
	}
	if !errors.Is(err, diskcache.ErrCacheMiss) {
		return nil, false, fmt.Errorf("read compressed image cache: %w", err)
	}

	result, err, _ := compressedImageFlight.Do(cacheKey, func() (any, error) {
		return generateCompressedImageCache(ctx, upload, quality, cacheKey)
	})
	if err != nil {
		return nil, false, err
	}

	res := result.(compressedImageCacheResult)
	return res.bytes, res.cached, res.err
}

func generateCompressedImageCache(
	ctx context.Context,
	upload *model.Upload,
	quality string,
	cacheKey string,
) (compressedImageCacheResult, error) {
	cacheStore := diskcache.GetGlobalCache()

	webpBytes, err := cacheStore.Get(cacheKey)
	if err == nil {
		return compressedImageCacheResult{bytes: webpBytes, cached: true}, nil
	}
	if !errors.Is(err, diskcache.ErrCacheMiss) {
		return compressedImageCacheResult{}, fmt.Errorf("read compressed image cache: %w", err)
	}

	origBytes, err := getOriginalFileBytes(ctx, upload)
	if err != nil {
		return compressedImageCacheResult{}, fmt.Errorf("read original image: %w", err)
	}

	webpBytes, err = util.CompressImageToWebP(bytes.NewReader(origBytes), quality)
	if err != nil {
		return compressedImageCacheResult{}, fmt.Errorf("compress image to WebP: %w", err)
	}

	if err := cacheStore.Set(cacheKey, webpBytes, diskcache.NoExpiration); err != nil {
		return compressedImageCacheResult{
			bytes: webpBytes,
			err:   fmt.Errorf("write compressed image cache: %w", err),
		}, nil
	}

	return compressedImageCacheResult{bytes: webpBytes}, nil
}

// ImageCompressionCacheKey returns the disk cache key for a compressed upload image.
func ImageCompressionCacheKey(upload *model.Upload, quality string) string {
	return fmt.Sprintf(
		"upload_webp_v1_%d_%d_%d_%s_%s",
		upload.ID,
		upload.UpdatedAt.UnixNano(),
		upload.FileSize,
		upload.Hash,
		quality,
	)
}

func serveOriginal(c *gin.Context, upload *model.Upload) {
	obj, err := uploadstorage.OpenStoredObject(c.Request.Context(), upload)
	if err != nil {
		response.AbortNotFound(c, "文件未找到")
		return
	}
	defer func() { _ = obj.Body.Close() }()
	c.DataFromReader(http.StatusOK, obj.ContentLength, obj.ContentType, obj.Body, nil)
}

func getOriginalFileBytes(ctx context.Context, upload *model.Upload) ([]byte, error) {
	obj, err := uploadstorage.OpenStoredObject(ctx, upload)
	if err != nil {
		return nil, err
	}
	defer func() { _ = obj.Body.Close() }()
	return io.ReadAll(obj.Body)
}

func checkPrivateFileOwner(c *gin.Context, ownerID uint64) error {
	var currUser *model.User
	var err error
	if u, ok := oauth.GetFromContext[*model.User](c, oauth.UserObjKey); ok && u != nil {
		currUser = u
	} else {
		currUser, err = oauth.GetUserFromRequest(c)
		if err != nil {
			return err
		}
	}
	if currUser.IsAdmin {
		return nil
	}
	if currUser.ID != ownerID {
		return errors.New("forbidden: cross-user access denied")
	}
	return nil
}

// CheckFileAccessPermission 校验文件是否可以被当前请求访问
func CheckFileAccessPermission(c *gin.Context, upload *model.Upload) error {
	if upload.AccessMode == 0 {
		return checkPrivateFileOwner(c, upload.UserID)
	}

	if !cache.IsFilePublic(c.Request.Context(), upload.Type) {
		if _, ok := oauth.GetFromContext[*model.User](c, oauth.UserObjKey); !ok {
			if _, err := oauth.GetUserFromRequest(c); err != nil {
				return err
			}
		}
	}
	return nil
}
