// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/buildinfo"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"golang.org/x/mod/semver"
)

const (
	githubAPIBaseURL = "https://api.github.com"
	maxArchiveSize   = int64(1024 * 1024 * 1024)
	maxReleaseSize   = int64(4 * 1024 * 1024)
	repositoryParts  = 2
	windowsOS        = "windows"
	archiveFileMode  = 0o600
	stagedBinaryMode = 0o700
)

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	State              string `json:"state"`
}

type githubRelease struct {
	TagName    string         `json:"tag_name"`
	Name       string         `json:"name"`
	Body       string         `json:"body"`
	HTMLURL    string         `json:"html_url"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	Published  time.Time      `json:"published_at"`
	Assets     []releaseAsset `json:"assets"`
}

// Status describes the current build and the newest compatible upstream release.
type Status struct {
	CurrentVersion     string `json:"current_version"`
	BuildTime          string `json:"build_time"`
	LatestVersion      string `json:"latest_version"`
	UpdateAvailable    bool   `json:"update_available"`
	CanUpgrade         bool   `json:"can_upgrade"`
	Prerelease         bool   `json:"prerelease"`
	ReleaseName        string `json:"release_name"`
	ReleaseNotes       string `json:"release_notes"`
	ReleaseURL         string `json:"release_url"`
	PublishedAt        string `json:"published_at"`
	UpstreamRepository string `json:"upstream_repository"`
	AssetName          string `json:"asset_name"`
	Platform           string `json:"platform"`
}

type releaseClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type manager struct {
	client    releaseClient
	mu        sync.Mutex
	upgrading bool
}

var defaultManager = &manager{
	client: &http.Client{Timeout: 10 * time.Minute},
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "dev" {
		return ""
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	if !semver.IsValid(version) {
		return ""
	}
	return version
}

func parseRepository(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New(errInvalidRepository)
	}

	if !strings.Contains(raw, "://") {
		repo := strings.TrimSuffix(strings.Trim(raw, "/"), ".git")
		if len(strings.Split(repo, "/")) == repositoryParts {
			return repo, nil
		}
		return "", errors.New(errInvalidRepository)
	}

	parsed, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(parsed.Hostname(), "github.com") {
		return "", errors.New(errInvalidRepository)
	}
	repo := strings.TrimSuffix(strings.Trim(parsed.Path, "/"), ".git")
	if len(strings.Split(repo, "/")) != repositoryParts {
		return "", errors.New(errInvalidRepository)
	}
	return repo, nil
}

func expectedAssetName(tag string) string {
	extension := "tar.gz"
	if runtime.GOOS == windowsOS {
		extension = "zip"
	}
	return fmt.Sprintf("wavelet_%s_%s_%s.%s", tag, runtime.GOOS, runtime.GOARCH, extension)
}

func expectedAssetNames(repository, tag string) []string {
	names := []string{expectedAssetName(tag)}
	if parts := strings.Split(repository, "/"); len(parts) == repositoryParts {
		repoName := parts[1]
		if repoName != "wavelet" {
			extension := "tar.gz"
			if runtime.GOOS == windowsOS {
				extension = "zip"
			}
			names = append(names, fmt.Sprintf("%s_%s_%s_%s.%s", repoName, tag, runtime.GOOS, runtime.GOARCH, extension))
		}
	}
	return names
}

func selectLatestRelease(repository string, releases []githubRelease) (githubRelease, releaseAsset, error) {
	var selected githubRelease
	var selectedAsset releaseAsset
	selectedVersion := ""

	for _, release := range releases {
		version := normalizeVersion(release.TagName)
		if release.Draft || version == "" {
			continue
		}
		expectedNames := expectedAssetNames(repository, release.TagName)
		for _, asset := range release.Assets {
			matched := false
			for _, name := range expectedNames {
				if asset.Name == name {
					matched = true
					break
				}
			}
			if !matched || asset.BrowserDownloadURL == "" || asset.State != "uploaded" {
				continue
			}
			if selectedVersion == "" || semver.Compare(version, selectedVersion) > 0 {
				selected = release
				selectedAsset = asset
				selectedVersion = version
			}
		}
	}

	if selectedVersion == "" {
		return githubRelease{}, releaseAsset{}, errors.New(errNoCompatibleRelease)
	}
	return selected, selectedAsset, nil
}

func (m *manager) fetchRelease(ctx context.Context, repository string) (githubRelease, releaseAsset, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/repos/%s/releases?per_page=30", githubAPIBaseURL, repository),
		nil,
	)
	if err != nil {
		return githubRelease{}, releaseAsset{}, fmt.Errorf("%s: %w", errReleaseRequestFailed, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "OpenFlare-Updater")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := m.client.Do(req)
	if err != nil {
		return githubRelease{}, releaseAsset{}, fmt.Errorf("%s: %w", errReleaseRequestFailed, err)
	}
	defer func() {
		// The response body is read-only; close errors cannot affect the parsed result.
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return githubRelease{}, releaseAsset{}, fmt.Errorf("%s: HTTP %d", errReleaseRequestFailed, resp.StatusCode)
	}

	var releases []githubRelease
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxReleaseSize))
	if err := decoder.Decode(&releases); err != nil {
		return githubRelease{}, releaseAsset{}, fmt.Errorf("%s: %w", errReleaseResponseInvalid, err)
	}

	release, asset, err := selectLatestRelease(repository, releases)
	if err != nil {
		return githubRelease{}, releaseAsset{}, err
	}
	logger.InfoF(ctx, "[Updater] Selected latest compatible release: %s (Asset: %s)", release.TagName, asset.Name)
	return release, asset, nil
}

func loadRepository(ctx context.Context) (string, error) {
	config, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyUpdateUpstreamRepository)
	if err != nil {
		return "", fmt.Errorf("%s: %w", errInvalidRepository, err)
	}
	return parseRepository(config.Value)
}

func (m *manager) status(ctx context.Context) (Status, releaseAsset, error) {
	upstreamRepo, err := loadRepository(ctx)
	if err != nil {
		return Status{}, releaseAsset{}, err
	}
	release, asset, err := m.fetchRelease(ctx, upstreamRepo)
	if err != nil {
		return Status{}, releaseAsset{}, err
	}

	currentVersion := normalizeVersion(buildinfo.Version)
	latestVersion := normalizeVersion(release.TagName)
	updateAvailable := currentVersion != "" && semver.Compare(latestVersion, currentVersion) > 0

	logger.InfoF(ctx, "[Updater] Check update complete. current: %s, latest: %s, update_available: %t", buildinfo.Version, release.TagName, updateAvailable)

	return Status{
		CurrentVersion:     buildinfo.Version,
		BuildTime:          buildinfo.BuildTime,
		LatestVersion:      release.TagName,
		UpdateAvailable:    updateAvailable,
		CanUpgrade:         updateAvailable && runtime.GOOS != windowsOS,
		Prerelease:         release.Prerelease,
		ReleaseName:        release.Name,
		ReleaseNotes:       release.Body,
		ReleaseURL:         release.HTMLURL,
		PublishedAt:        release.Published.Format(time.RFC3339),
		UpstreamRepository: upstreamRepo,
		AssetName:          asset.Name,
		Platform:           runtime.GOOS + "/" + runtime.GOARCH,
	}, asset, nil
}

func downloadArchive(ctx context.Context, client releaseClient, asset releaseAsset, destination string) error {
	if asset.Size <= 0 || asset.Size > maxArchiveSize {
		return fmt.Errorf("release 资产大小无效: %d", asset.Size)
	}
	logger.InfoF(ctx, "[Updater] Downloading release asset: %s", asset.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return fmt.Errorf("创建升级下载请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "OpenFlare-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载升级资产失败: %w", err)
	}
	defer func() {
		// The downloaded body has already been validated by size before use.
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载升级资产失败: HTTP %d", resp.StatusCode)
	}

	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, archiveFileMode) //nolint:gosec // destination is created inside the verified executable directory.
	if err != nil {
		return fmt.Errorf("创建升级归档失败: %w", err)
	}

	written, err := io.Copy(file, io.LimitReader(resp.Body, maxArchiveSize+1))
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("写入升级归档失败: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("关闭升级归档失败: %w", err)
	}
	if written > maxArchiveSize || written != asset.Size {
		return fmt.Errorf("升级归档大小不匹配: got %d, want %d", written, asset.Size)
	}
	logger.InfoF(ctx, "[Updater] Successfully downloaded release asset to %s", destination)
	return nil
}

func safeArchivePath(destination, name string) (string, error) {
	cleanName := filepath.Clean(name)
	if filepath.IsAbs(cleanName) || cleanName == "." || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("归档包含非法路径: %s", name)
	}
	target := filepath.Join(destination, cleanName)
	relative, err := filepath.Rel(destination, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("归档路径越界: %s", name)
	}
	return target, nil
}

func matchBinaryName(name string, candidates []string) bool {
	for _, candidate := range candidates {
		if runtime.GOOS == windowsOS {
			if strings.EqualFold(name, candidate) {
				return true
			}
		} else {
			if name == candidate {
				return true
			}
		}
	}
	return false
}

func getCandidateBinaryNames(executable string, repository string) []string {
	execName := filepath.Base(executable)
	names := []string{execName}

	addName := func(base string) {
		name := base
		if runtime.GOOS == windowsOS && !strings.HasSuffix(strings.ToLower(name), ".exe") {
			name += ".exe"
		}
		for _, existing := range names {
			if existing == name {
				return
			}
		}
		names = append(names, name)
	}

	if parts := strings.Split(repository, "/"); len(parts) == repositoryParts {
		addName(parts[1])
	}
	addName("wavelet")

	return names
}

func isLikelyBinary(name string, isDir bool, mode os.FileMode) bool {
	if isDir {
		return false
	}
	base := strings.ToLower(filepath.Base(name))

	// Exclude typical non-binary metadata files
	exclusions := []string{
		"license", "licence", "copying", "notice", "readme", "changelog",
	}
	for _, excl := range exclusions {
		if strings.HasPrefix(base, excl) {
			return false
		}
	}

	if runtime.GOOS == windowsOS {
		return filepath.Ext(base) == ".exe"
	}

	// On Unix, it should either have the executable permission bit set, OR have no extension
	return (mode.Perm()&0111 != 0) || (filepath.Ext(base) == "")
}

func findBinaryInTarGz(archivePath string, candidates []string) (string, error) {
	file, err := os.Open(archivePath) //nolint:gosec // archivePath is created by prepareUpgrade in the executable directory.
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = gzipReader.Close()
	}()

	reader := tar.NewReader(gzipReader)
	var binaries []string
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if header.Typeflag == tar.TypeReg && isLikelyBinary(header.Name, false, header.FileInfo().Mode()) {
			binaries = append(binaries, header.Name)
		}
	}

	if len(binaries) == 1 {
		return binaries[0], nil
	}

	// Fallback to candidate match if multiple or zero likely binaries found
	for _, name := range binaries {
		if matchBinaryName(filepath.Base(name), candidates) {
			return name, nil
		}
	}

	return "", errors.New(errNoCompatibleAsset)
}

func findBinaryInZip(archivePath string, candidates []string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = reader.Close()
	}()

	var binaries []string
	for _, file := range reader.File {
		if !file.FileInfo().IsDir() && isLikelyBinary(file.Name, false, file.FileInfo().Mode()) {
			binaries = append(binaries, file.Name)
		}
	}

	if len(binaries) == 1 {
		return binaries[0], nil
	}

	// Fallback to candidate match if multiple or zero likely binaries found
	for _, name := range binaries {
		if matchBinaryName(filepath.Base(name), candidates) {
			return name, nil
		}
	}

	return "", errors.New(errNoCompatibleAsset)
}

func extractTarGz(ctx context.Context, archivePath, destination, targetName string, candidates []string) (string, error) {
	binaryPathInArchive, err := findBinaryInTarGz(archivePath, candidates)
	if err != nil {
		return "", err
	}

	logger.InfoF(ctx, "[Updater] Extracting tar.gz archive: %s (extracting: %s)", archivePath, binaryPathInArchive)
	file, err := os.Open(archivePath) //nolint:gosec // archivePath is created by prepareUpgrade in the executable directory.
	if err != nil {
		return "", err
	}
	defer func() {
		// Read-only archive close errors do not change extraction validity.
		_ = file.Close()
	}()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer func() {
		// The gzip checksum is verified while reading the selected file.
		_ = gzipReader.Close()
	}()

	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if header.Name != binaryPathInArchive {
			continue
		}
		target, err := safeArchivePath(destination, targetName)
		if err != nil {
			return "", err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, stagedBinaryMode) //nolint:gosec // target is constrained by safeArchivePath.
		if err != nil {
			return "", err
		}
		written, copyErr := io.Copy(output, io.LimitReader(reader, maxArchiveSize+1))
		closeErr := output.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if written > maxArchiveSize {
			return "", errors.New("解压后的程序文件超过大小限制")
		}
		logger.InfoF(ctx, "[Updater] Successfully extracted binary to %s", target)
		return target, nil
	}
	return "", errors.New(errNoCompatibleAsset)
}

func extractZip(ctx context.Context, archivePath, destination, targetName string, candidates []string) (string, error) {
	binaryPathInArchive, err := findBinaryInZip(archivePath, candidates)
	if err != nil {
		return "", err
	}

	logger.InfoF(ctx, "[Updater] Extracting zip archive: %s (extracting: %s)", archivePath, binaryPathInArchive)
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer func() {
		// Read-only archive close errors do not change extraction validity.
		_ = reader.Close()
	}()
	for _, file := range reader.File {
		if file.Name != binaryPathInArchive {
			continue
		}
		target, err := safeArchivePath(destination, targetName)
		if err != nil {
			return "", err
		}
		input, err := file.Open()
		if err != nil {
			return "", err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, stagedBinaryMode) //nolint:gosec // target is constrained by safeArchivePath.
		if err != nil {
			_ = input.Close()
			return "", err
		}
		written, copyErr := io.Copy(output, io.LimitReader(input, maxArchiveSize+1))
		inputCloseErr := input.Close()
		outputCloseErr := output.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if inputCloseErr != nil {
			return "", inputCloseErr
		}
		if outputCloseErr != nil {
			return "", outputCloseErr
		}
		if written > maxArchiveSize {
			return "", errors.New("解压后的程序文件超过大小限制")
		}
		logger.InfoF(ctx, "[Updater] Successfully extracted binary to %s", target)
		return target, nil
	}
	return "", errors.New(errNoCompatibleAsset)
}

func (m *manager) prepareUpgrade(ctx context.Context) (string, string, error) {
	if runtime.GOOS == windowsOS {
		return "", "", errors.New(errAutomaticUpgradeBlocked)
	}
	if normalizeVersion(buildinfo.Version) == "" {
		return "", "", errors.New(errDevelopmentBuild)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.upgrading {
		return "", "", errors.New(errUpgradeAlreadyRunning)
	}

	status, asset, err := m.status(ctx)
	if err != nil {
		return "", "", err
	}
	if !status.UpdateAvailable {
		return "", "", errors.New(errAlreadyUpToDate)
	}

	logger.InfoF(ctx, "[Updater] Preparing upgrade. current: %s, latest: %s", status.CurrentVersion, status.LatestVersion)

	executable, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("定位当前程序失败: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return "", "", fmt.Errorf("解析当前程序路径失败: %w", err)
	}

	tempDir, err := os.MkdirTemp(filepath.Dir(executable), ".wavelet-update-*")
	if err != nil {
		return "", "", fmt.Errorf("创建升级目录失败: %w", err)
	}

	archivePath := filepath.Join(tempDir, asset.Name)
	if err := downloadArchive(ctx, m.client, asset, archivePath); err != nil {
		// Cleanup is best effort because the download error is the actionable failure.
		_ = os.RemoveAll(tempDir)
		return "", "", err
	}

	targetName := filepath.Base(executable)
	candidates := getCandidateBinaryNames(executable, status.UpstreamRepository)

	var stagedBinary string
	if strings.HasSuffix(asset.Name, ".zip") {
		stagedBinary, err = extractZip(ctx, archivePath, tempDir, targetName, candidates)
	} else {
		stagedBinary, err = extractTarGz(ctx, archivePath, tempDir, targetName, candidates)
	}
	if err != nil {
		// Cleanup is best effort because the extraction error is the actionable failure.
		_ = os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("解压升级资产失败: %w", err)
	}
	logger.InfoF(ctx, "[Updater] Staged binary successfully prepared: %s", stagedBinary)
	m.upgrading = true
	return executable, stagedBinary, nil
}

func (m *manager) finishUpgrade() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upgrading = false
}
