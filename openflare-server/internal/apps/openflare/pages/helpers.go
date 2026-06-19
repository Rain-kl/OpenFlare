// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/upload"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const (
	pagesMaxDeploymentFiles  = 1000
	pagesMaxDeploymentBytes  = 100 * 1024 * 1024
	defaultPagesEntryFile    = "index.html"
	defaultPagesFallbackPath = "/index.html"
	pagesDeploymentUploadType = "openflare_pages_deployment"
)

var pagesSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,126}[a-z0-9]$|^[a-z0-9]$`)

type deploymentManifest struct {
	Files     []model.PagesDeploymentFile
	FileCount int
	TotalSize int64
	EntryFile string
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}

func normalizePagesSlug(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if valid {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func validateAndNormalizePagesRootDir(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if len(value) > 512 {
		return "", errors.New("Pages 根目录长度不能超过 512")
	}
	if strings.Contains(value, "\\") || strings.ContainsAny(value, "\"';") {
		return "", errors.New("Pages 根目录包含不支持的字符")
	}
	for _, r := range value {
		if r <= 0x20 || r == 0x7f {
			return "", errors.New("Pages 根目录不能包含空白或控制字符")
		}
	}
	cleaned := path.Clean(filepath.ToSlash(value))
	if cleaned == "." || cleaned == "/" {
		return "", nil
	}
	for _, segment := range strings.Split(cleaned, "/") {
		if segment == "." || segment == ".." {
			return "", errors.New("Pages 根目录不能包含 . 或 .. 路径段")
		}
	}
	return strings.TrimPrefix(cleaned, "/"), nil
}

func normalizePagesFallbackPath(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = defaultPagesFallbackPath
	}
	if len(value) > 512 {
		return "", errors.New("SPA fallback 回退路径长度不能超过 512")
	}
	if !strings.HasPrefix(value, "/") {
		return "", errors.New("SPA fallback 回退路径必须以 / 开头")
	}
	if value == "/" || strings.HasSuffix(value, "/") {
		return "", errors.New("SPA fallback 回退路径必须指向具体文件")
	}
	if strings.Contains(value, "\\") || strings.ContainsAny(value, "\"';") {
		return "", errors.New("SPA fallback 回退路径包含不支持的字符")
	}
	for _, r := range value {
		if r <= 0x20 || r == 0x7f {
			return "", errors.New("SPA fallback 回退路径不能包含空白或控制字符")
		}
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." {
			return "", errors.New("SPA fallback 回退路径不能包含 . 或 .. 路径段")
		}
	}
	cleaned := path.Clean(value)
	if cleaned == "." || !strings.HasPrefix(cleaned, "/") {
		return "", errors.New("SPA fallback 回退路径不合法")
	}
	if cleaned == "/" || strings.HasSuffix(cleaned, "/") {
		return "", errors.New("SPA fallback 回退路径必须指向具体文件")
	}
	return cleaned, nil
}

func normalizeStoredPagesFallbackPath(value string) string {
	normalized, err := normalizePagesFallbackPath(value)
	if err != nil {
		return defaultPagesFallbackPath
	}
	return normalized
}

func normalizePagesEntryFile(raw string) string {
	value := path.Clean(strings.TrimSpace(filepath.ToSlash(raw)))
	if value == "." || value == "/" {
		return defaultPagesEntryFile
	}
	return strings.TrimPrefix(value, "/")
}

func persistPagesUploadTemp(fileHeader *multipart.FileHeader) (string, string, int64, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", "", 0, err
	}
	defer file.Close()
	temp, err := os.CreateTemp("", "openflare-pages-*.zip")
	if err != nil {
		return "", "", 0, err
	}
	defer temp.Close()
	hash := sha256.New()
	limited := io.LimitReader(file, pagesMaxDeploymentBytes+1)
	written, err := io.Copy(io.MultiWriter(temp, hash), limited)
	if err != nil {
		_ = os.Remove(temp.Name())
		return "", "", 0, err
	}
	if written > pagesMaxDeploymentBytes {
		_ = os.Remove(temp.Name())
		return "", "", 0, fmt.Errorf("Pages 部署包不能超过 %d MiB", pagesMaxDeploymentBytes/1024/1024)
	}
	return temp.Name(), hex.EncodeToString(hash.Sum(nil)), written, nil
}

func ingestPagesDeploymentPackage(
	ctx context.Context,
	tempPath string,
	checksum string,
	size int64,
	projectSlug string,
	fileName string,
) (upload.IngestResult, error) {
	file, err := os.Open(tempPath)
	if err != nil {
		return upload.IngestResult{}, err
	}
	defer file.Close()

	systemUser := repository.GetSystemUser(ctx)
	accessMode := 0
	return upload.Ingest(ctx, upload.IngestRequest{
		UserID:             systemUser.ID,
		Reader:             file,
		Size:               size,
		FileName:           fileName,
		MimeType:           "application/zip",
		Extension:          "zip",
		Hash:               checksum,
		Type:               pagesDeploymentUploadType,
		AccessMode:         &accessMode,
		SkipExtensionCheck: true,
		Policy:             upload.PolicyDedupNewRecord,
		Metadata: model.UploadMetadata{
			Extra: map[string]any{
				"project_slug": projectSlug,
			},
		},
	})
}

func removeDeploymentArtifact(ctx context.Context, deployment *model.PagesDeployment) {
	if deployment == nil {
		return
	}
	if deployment.UploadID > 0 {
		if _, err := upload.Remove(ctx, deployment.UploadID); err != nil {
			return
		}
		return
	}
	if strings.TrimSpace(deployment.ArtifactPath) != "" {
		_ = os.Remove(deployment.ArtifactPath)
	}
}

func findCommonRootPrefix(files []*zip.File) (string, error) {
	var firstFilePath string
	hasMultipleFiles := false
	for _, item := range files {
		normalizedPath, skip, err := normalizePagesZipPath(item.Name)
		if err != nil {
			return "", err
		}
		if skip {
			continue
		}
		if firstFilePath == "" {
			firstFilePath = normalizedPath
		} else {
			hasMultipleFiles = true
		}
	}
	if firstFilePath == "" {
		return "", nil
	}
	parts := strings.Split(firstFilePath, "/")
	if len(parts) <= 1 {
		return "", nil
	}
	commonPrefix := parts[0] + "/"
	if hasMultipleFiles {
		for _, item := range files {
			normalizedPath, skip, err := normalizePagesZipPath(item.Name)
			if err != nil {
				return "", err
			}
			if skip {
				continue
			}
			if !strings.HasPrefix(normalizedPath, commonPrefix) {
				return "", nil
			}
		}
	}
	return commonPrefix, nil
}

func inspectPagesZip(zipPath string, rootDir string, entryFile string) (*deploymentManifest, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, errors.New(errPagesPackageInvalidZip)
	}
	defer reader.Close()

	commonPrefix, err := findCommonRootPrefix(reader.File)
	if err != nil {
		return nil, err
	}

	manifest := &deploymentManifest{
		Files:     []model.PagesDeploymentFile{},
		EntryFile: entryFile,
	}
	targetEntryPath := entryFile
	if rootDir != "" {
		targetEntryPath = path.Join(rootDir, entryFile)
	}
	entrySeen := false
	for _, item := range reader.File {
		normalizedPath, skip, err := normalizePagesZipPath(item.Name)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		if commonPrefix != "" {
			normalizedPath = strings.TrimPrefix(normalizedPath, commonPrefix)
		}
		if item.FileInfo().Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("Pages 部署包不支持符号链接: %s", normalizedPath)
		}
		if item.UncompressedSize64 > pagesMaxDeploymentBytes {
			return nil, fmt.Errorf("Pages 文件过大: %s", normalizedPath)
		}
		manifest.FileCount++
		if manifest.FileCount > pagesMaxDeploymentFiles {
			return nil, fmt.Errorf("Pages 部署文件数不能超过 %d", pagesMaxDeploymentFiles)
		}
		manifest.TotalSize += int64(item.UncompressedSize64)
		if manifest.TotalSize > pagesMaxDeploymentBytes {
			return nil, fmt.Errorf("Pages 部署展开后不能超过 %d MiB", pagesMaxDeploymentBytes/1024/1024)
		}
		checksum, err := checksumZipFile(item)
		if err != nil {
			return nil, err
		}
		if normalizedPath == targetEntryPath {
			entrySeen = true
		}
		manifest.Files = append(manifest.Files, model.PagesDeploymentFile{
			Path:     normalizedPath,
			Size:     int64(item.UncompressedSize64),
			Checksum: checksum,
		})
	}
	if manifest.FileCount == 0 {
		return nil, errors.New(errPagesPackageEmpty)
	}
	if !entrySeen {
		return nil, fmt.Errorf("Pages 部署包缺少入口文件 %s", targetEntryPath)
	}
	return manifest, nil
}

func normalizePagesZipPath(raw string) (string, bool, error) {
	name := strings.TrimSpace(filepath.ToSlash(raw))
	if name == "" {
		return "", true, nil
	}
	if strings.HasSuffix(name, "/") {
		return "", true, nil
	}
	if strings.HasPrefix(name, "/") || path.IsAbs(name) {
		return "", false, fmt.Errorf("Pages 部署包不能包含绝对路径: %s", raw)
	}
	cleaned := path.Clean(name)
	if cleaned == "." {
		return "", true, nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return "", false, fmt.Errorf("Pages 部署包路径不能逃逸目录: %s", raw)
	}
	return cleaned, false, nil
}

func checksumZipFile(item *zip.File) (string, error) {
	file, err := item.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}


