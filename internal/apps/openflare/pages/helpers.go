// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
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
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/Rain-kl/Wavelet/pkg/pagesarchive"
)

const (
	pagesMaxDeploymentFiles      = 1000
	defaultPagesMaxPackageSizeMB = 100
	maxPagesMaxPackageSizeMB     = 2048
	defaultPagesMaxHistoryCount  = 20
	defaultPagesEntryFile        = "index.html"
	defaultPagesFallbackPath     = "/index.html"
	pagesDeploymentUploadType    = "openflare_pages_deployment"
	pagesMaxPathLength           = 512
	bytesPerMiB                  = 1024 * 1024
	pagesExtractedSizeMultiplier = 4
	pagesMinExtractedSizeBytes   = 100 * bytesPerMiB
)

var pagesSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,126}[a-z0-9]$|^[a-z0-9]$`)

type deploymentManifest struct {
	Files     []model.PagesDeploymentFile
	FileCount int
	TotalSize int64
	EntryFile string
	Format    pagesarchive.Format
}

type pagesLimits struct {
	PackageBytes   int64
	ExtractedBytes int64
	MaxFiles       int
	HistoryCount   int
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}

func resolvePagesLimits(ctx context.Context) pagesLimits {
	packageMB := defaultPagesMaxPackageSizeMB
	if value, err := repository.GetIntByKey(ctx, model.ConfigKeyPagesMaxPackageSizeMB); err == nil && value > 0 {
		packageMB = value
	}
	if packageMB > maxPagesMaxPackageSizeMB {
		packageMB = maxPagesMaxPackageSizeMB
	}

	historyCount := defaultPagesMaxHistoryCount
	if value, err := repository.GetIntByKey(ctx, model.ConfigKeyPagesMaxHistoryCount); err == nil {
		if value < 0 {
			historyCount = 0
		} else {
			historyCount = value
		}
	}

	packageBytes := int64(packageMB) * bytesPerMiB
	extractedBytes := packageBytes * pagesExtractedSizeMultiplier
	if extractedBytes < pagesMinExtractedSizeBytes {
		extractedBytes = pagesMinExtractedSizeBytes
	}

	return pagesLimits{
		PackageBytes:   packageBytes,
		ExtractedBytes: extractedBytes,
		MaxFiles:       pagesMaxDeploymentFiles,
		HistoryCount:   historyCount,
	}
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
	if len(value) > pagesMaxPathLength {
		return "", errors.New("pages 根目录长度不能超过 512")
	}
	if strings.Contains(value, "\\") || strings.ContainsAny(value, "\"';") {
		return "", errors.New("pages 根目录包含不支持的字符")
	}
	for _, r := range value {
		if r <= 0x20 || r == 0x7f {
			return "", errors.New("pages 根目录不能包含空白或控制字符")
		}
	}
	cleaned := path.Clean(filepath.ToSlash(value))
	if cleaned == "." || cleaned == "/" {
		return "", nil
	}
	for _, segment := range strings.Split(cleaned, "/") {
		if segment == "." || segment == ".." {
			return "", errors.New("pages 根目录不能包含 . 或 .. 路径段")
		}
	}
	return strings.TrimPrefix(cleaned, "/"), nil
}

func normalizePagesFallbackPath(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = defaultPagesFallbackPath
	}
	if len(value) > pagesMaxPathLength {
		return "", errors.New("spa fallback 回退路径长度不能超过 512")
	}
	if !strings.HasPrefix(value, "/") {
		return "", errors.New("spa fallback 回退路径必须以 / 开头")
	}
	if value == "/" || strings.HasSuffix(value, "/") {
		return "", errors.New("spa fallback 回退路径必须指向具体文件")
	}
	if strings.Contains(value, "\\") || strings.ContainsAny(value, "\"';") {
		return "", errors.New("spa fallback 回退路径包含不支持的字符")
	}
	for _, r := range value {
		if r <= 0x20 || r == 0x7f {
			return "", errors.New("spa fallback 回退路径不能包含空白或控制字符")
		}
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." {
			return "", errors.New("spa fallback 回退路径不能包含 . 或 .. 路径段")
		}
	}
	cleaned := path.Clean(value)
	if cleaned == "." || !strings.HasPrefix(cleaned, "/") {
		return "", errors.New("spa fallback 回退路径不合法")
	}
	if cleaned == "/" || strings.HasSuffix(cleaned, "/") {
		return "", errors.New("spa fallback 回退路径必须指向具体文件")
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

func persistPagesUploadTemp(fileHeader *multipart.FileHeader, maxPackageBytes int64) (string, string, int64, pagesarchive.Format, error) {
	format, ok := pagesarchive.DetectFormatFromName(fileHeader.Filename)
	if !ok {
		return "", "", 0, "", errors.New(errPagesPackageUnsupported)
	}
	file, err := fileHeader.Open()
	if err != nil {
		return "", "", 0, "", err
	}
	defer func() { _ = file.Close() }()
	temp, err := os.CreateTemp("", "openflare-pages-*."+safeTempSuffix(format))
	if err != nil {
		return "", "", 0, "", err
	}
	defer func() { _ = temp.Close() }()
	hash := sha256.New()
	limited := io.LimitReader(file, maxPackageBytes+1)
	written, err := io.Copy(io.MultiWriter(temp, hash), limited)
	if err != nil {
		_ = os.Remove(temp.Name())
		return "", "", 0, "", err
	}
	if written > maxPackageBytes {
		_ = os.Remove(temp.Name())
		return "", "", 0, "", fmt.Errorf("pages 部署包不能超过 %d MiB", maxPackageBytes/bytesPerMiB)
	}
	return temp.Name(), hex.EncodeToString(hash.Sum(nil)), written, format, nil
}

func safeTempSuffix(format pagesarchive.Format) string {
	switch format {
	case pagesarchive.FormatTarGz:
		return "tar.gz"
	case pagesarchive.FormatTarXz:
		return "tar.xz"
	case pagesarchive.FormatTarBz2:
		return "tar.bz2"
	case pagesarchive.FormatSevenZip:
		return "7z"
	case pagesarchive.FormatTar:
		return "tar"
	default:
		return "zip"
	}
}

func pagesLegacyRelativeCandidates(project *model.PagesProject, deployment *model.PagesDeployment) []string {
	if project == nil || deployment == nil {
		return nil
	}
	slug := strings.TrimSpace(project.Slug)
	checksum := strings.TrimSpace(deployment.Checksum)
	if slug == "" || checksum == "" {
		return nil
	}
	// Legacy artifacts were always stored as .zip.
	fileName := checksum + ".zip"
	return []string{
		filepath.Join("artifacts", slug, fileName),
		filepath.Join("pages", "artifacts", slug, fileName),
		filepath.Join("data", "pages", "artifacts", slug, fileName),
	}
}

func ingestPagesDeploymentPackage(
	ctx context.Context,
	localPath string,
	checksum string,
	projectSlug string,
	fileName string,
	format pagesarchive.Format,
) (upload.IngestResult, error) {
	systemUser := repository.GetSystemUser(ctx)
	accessMode := 0
	extension := pagesarchive.NormalizeNameExtension(fileName, format)
	return upload.IngestFromLocalPath(ctx, localPath, upload.IngestRequest{
		UserID:             systemUser.ID,
		FileName:           fileName,
		MimeType:           pagesarchive.MIMEType(format),
		Extension:          extension,
		Hash:               checksum,
		Type:               pagesDeploymentUploadType,
		AccessMode:         &accessMode,
		SkipExtensionCheck: true,
		Policy:             upload.PolicyDedupNewRecord,
		Metadata: model.UploadMetadata{
			Extra: map[string]any{
				"project_slug": projectSlug,
				"format":       string(format),
			},
		},
	})
}

func removeDeploymentArtifact(ctx context.Context, deployment *model.PagesDeployment) {
	if deployment == nil {
		return
	}
	if deployment.UploadID == 0 {
		return
	}
	if _, err := upload.Remove(ctx, deployment.UploadID); err != nil {
		// Soft-delete / storage cleanup failure must not undo DB prune; log for ops.
		logger.WarnF(ctx,
			"[Pages] remove deployment artifact failed: deployment_id=%d upload_id=%d error=%v",
			deployment.ID, deployment.UploadID, err,
		)
	}
}

func inspectPagesPackage(packagePath string, format pagesarchive.Format, rootDir string, entryFile string, limits pagesLimits) (*deploymentManifest, error) {
	archiveManifest, err := pagesarchive.InspectFile(packagePath, format, pagesarchive.InspectOptions{
		RootDir:   rootDir,
		EntryFile: entryFile,
		Limits: pagesarchive.Limits{
			MaxFiles:      limits.MaxFiles,
			MaxFileBytes:  limits.ExtractedBytes,
			MaxTotalBytes: limits.ExtractedBytes,
		},
	})
	if err != nil {
		return nil, mapPagesArchiveError(err)
	}
	manifest := &deploymentManifest{
		Files:     make([]model.PagesDeploymentFile, 0, len(archiveManifest.Files)),
		FileCount: archiveManifest.FileCount,
		TotalSize: archiveManifest.TotalSize,
		EntryFile: entryFile,
		Format:    format,
	}
	for _, file := range archiveManifest.Files {
		manifest.Files = append(manifest.Files, model.PagesDeploymentFile{
			Path:     file.Path,
			Size:     file.Size,
			Checksum: file.Checksum,
		})
	}
	return manifest, nil
}

func mapPagesArchiveError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "unsupported pages package format"):
		return errors.New(errPagesPackageUnsupported)
	case strings.Contains(message, "open zip"), strings.Contains(message, "open gzip"),
		strings.Contains(message, "open xz"), strings.Contains(message, "open 7z"),
		strings.Contains(message, "read tar"):
		return errors.New(errPagesPackageInvalid)
	case strings.Contains(message, "empty"):
		return errors.New(errPagesPackageEmpty)
	case strings.Contains(message, "missing entry file"):
		return err
	case strings.Contains(message, "file count exceeds"):
		return fmt.Errorf("pages 部署文件数不能超过 %d", pagesMaxDeploymentFiles)
	case strings.Contains(message, "extracted size exceeds"):
		return errors.New(errPagesPackageExtractedTooLarge)
	case strings.Contains(message, "file too large"), strings.Contains(message, "size out of bounds"):
		return errors.New(errPagesPackageFileTooLarge)
	case strings.Contains(message, "symlink"):
		return err
	case strings.Contains(message, "absolute path"), strings.Contains(message, "escapes directory"):
		return err
	default:
		return err
	}
}

func packageDownloadName(deploymentID uint, fileName string, contentType string) string {
	if format, ok := pagesarchive.DetectFormatFromName(fileName); ok {
		return fmt.Sprintf("pages-deployment-%d.%s", deploymentID, pagesarchive.Extension(format))
	}
	// Fall back by content type.
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "application/gzip", "application/x-gzip":
		return fmt.Sprintf("pages-deployment-%d.tar.gz", deploymentID)
	case "application/x-xz":
		return fmt.Sprintf("pages-deployment-%d.tar.xz", deploymentID)
	case "application/x-bzip2":
		return fmt.Sprintf("pages-deployment-%d.tar.bz2", deploymentID)
	case "application/x-7z-compressed":
		return fmt.Sprintf("pages-deployment-%d.7z", deploymentID)
	case "application/x-tar":
		return fmt.Sprintf("pages-deployment-%d.tar", deploymentID)
	default:
		return fmt.Sprintf("pages-deployment-%d.zip", deploymentID)
	}
}
