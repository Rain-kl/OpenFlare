package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/pkg/utils"

	"github.com/Rain-kl/Wavelet/internal/relay/config"
)

const maxChecksumAssetSize = 64 * 1024

var replaceAndRestartFunc = replaceAndRestart

type Service struct {
	httpClient   *http.Client
	lastCheckKey string
}

func New() *Service {
	return &Service{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Prerelease bool          `json:"prerelease"`
	Draft      bool          `json:"draft"`
	Assets     []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type UpdateOptions struct {
	Channel string
	TagName string
	Force   bool
}

func (s *Service) CheckAndUpdate(ctx context.Context, repo string, options UpdateOptions) error {
	release, err := s.getRelease(ctx, repo, options)
	if err != nil {
		return fmt.Errorf("check latest release: %w", err)
	}
	if release == nil || release.TagName == "" {
		return nil
	}

	remoteVersion := normalizeVersion(release.TagName)
	localVersion := normalizeVersion(config.Version)
	checkKey := buildReleaseCheckKey(options, remoteVersion)

	if remoteVersion == localVersion {
		return nil
	}
	if !options.Force && checkKey != "" && checkKey == s.lastCheckKey {
		return nil
	}
	if !isNewer(localVersion, remoteVersion) {
		s.lastCheckKey = checkKey
		return nil
	}

	slog.Info("relay update available", "from", localVersion, "to", remoteVersion)
	assetName := assetNameForGOOSGOARCH(runtime.GOOS, runtime.GOARCH)
	checksumAssetName := assetName + ".sha256"

	var downloadURL string
	var checksumURL string
	for _, asset := range release.Assets {
		switch asset.Name {
		case assetName:
			downloadURL = asset.BrowserDownloadURL
		case checksumAssetName:
			checksumURL = asset.BrowserDownloadURL
		}
	}
	if downloadURL == "" {
		s.lastCheckKey = checkKey
		return fmt.Errorf("no matching asset %q in release %s", assetName, release.TagName)
	}
	if checksumURL == "" {
		return fmt.Errorf("no matching checksum asset %q in release %s", checksumAssetName, release.TagName)
	}

	expectedChecksum, err := s.downloadChecksum(ctx, checksumURL, assetName)
	if err != nil {
		return fmt.Errorf("download checksum: %w", err)
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	if err = s.downloadAndRestart(ctx, downloadURL, expectedChecksum, execPath); err != nil {
		return fmt.Errorf("download and restart: %w", err)
	}
	s.lastCheckKey = checkKey
	return nil
}

func (s *Service) getRelease(ctx context.Context, repo string, options UpdateOptions) (*githubRelease, error) {
	tagName := strings.TrimSpace(options.TagName)
	if tagName != "" {
		return s.getReleaseByTag(ctx, repo, tagName)
	}
	if strings.EqualFold(strings.TrimSpace(options.Channel), "preview") {
		return s.getLatestPreviewRelease(ctx, repo)
	}
	return s.getLatestStableRelease(ctx, repo)
}

func (s *Service) getLatestStableRelease(ctx context.Context, repo string) (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	return s.fetchReleaseFromURL(ctx, url)
}

func (s *Service) getLatestPreviewRelease(ctx context.Context, repo string) (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=20", repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned %s", resp.Status)
	}

	var releases []githubRelease
	if err = json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}
	for _, release := range releases {
		if release.Draft || !release.Prerelease {
			continue
		}
		releaseCopy := release
		return &releaseCopy, nil
	}
	return nil, nil
}

func (s *Service) getReleaseByTag(ctx context.Context, repo string, tag string) (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repo, strings.TrimSpace(tag))
	return s.fetchReleaseFromURL(ctx, url)
}

func (s *Service) fetchReleaseFromURL(ctx context.Context, url string) (*githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned %s", resp.Status)
	}

	return decodeRelease(resp.Body)
}

func decodeRelease(reader io.Reader) (*githubRelease, error) {
	var release githubRelease
	if err := json.NewDecoder(reader).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func (s *Service) downloadChecksum(ctx context.Context, url string, assetName string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum download returned %s", resp.Status)
	}

	content, err := io.ReadAll(io.LimitReader(resp.Body, maxChecksumAssetSize+1))
	if err != nil {
		return "", err
	}
	if len(content) > maxChecksumAssetSize {
		return "", fmt.Errorf("checksum asset exceeds %d bytes", maxChecksumAssetSize)
	}
	checksum, err := parseSHA256Checksum(string(content), assetName)
	if err != nil {
		return "", err
	}
	return checksum, nil
}

func parseSHA256Checksum(content string, assetName string) (string, error) {
	assetName = strings.TrimSpace(assetName)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if checksum, ok := parseSHA256Line(line, assetName); ok {
			return checksum, nil
		}
	}
	if assetName == "" {
		return "", fmt.Errorf("checksum asset does not contain a valid sha256 digest")
	}
	return "", fmt.Errorf("checksum asset does not contain a sha256 digest for %q", assetName)
}

func parseSHA256Line(line string, assetName string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) == 1 && isSHA256Hex(fields[0]) {
		return strings.ToLower(fields[0]), true
	}
	if len(fields) >= 2 && isSHA256Hex(fields[0]) {
		fileName := strings.TrimPrefix(strings.TrimSpace(fields[1]), "*")
		if assetName == "" || fileName == assetName {
			return strings.ToLower(fields[0]), true
		}
	}

	prefix := "SHA256("
	if strings.HasPrefix(line, prefix) {
		closing := strings.Index(line, ")")
		if closing > len(prefix) && closing+1 < len(line) {
			fileName := strings.TrimSpace(line[len(prefix):closing])
			rest := strings.TrimSpace(line[closing+1:])
			rest = strings.TrimPrefix(rest, "=")
			rest = strings.TrimSpace(rest)
			if isSHA256Hex(rest) && (assetName == "" || fileName == assetName) {
				return strings.ToLower(rest), true
			}
		}
	}
	return "", false
}

func isSHA256Hex(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func (s *Service) downloadAndRestart(ctx context.Context, url string, expectedChecksum string, targetPath string) error {
	expectedChecksum = strings.ToLower(strings.TrimSpace(expectedChecksum))
	if !isSHA256Hex(expectedChecksum) {
		return fmt.Errorf("invalid expected sha256 checksum")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %s", resp.Status)
	}

	tmpPath := targetPath + ".update"
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(tmpPath), ".exe") {
		tmpPath += ".exe"
	}
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	if _, err = io.Copy(io.MultiWriter(tmpFile, hasher), resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	if err = tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		os.Remove(tmpPath)
		return fmt.Errorf("sha256 checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}
	if err = os.Chmod(tmpPath, 0o755); err != nil && runtime.GOOS != "windows" {
		os.Remove(tmpPath)
		return fmt.Errorf("set executable permission: %w", err)
	}

	slog.Info("relay binary updated, restarting")
	return replaceAndRestartFunc(targetPath, tmpPath)
}

func assetNameForGOOSGOARCH(goos string, goarch string) string {
	name := fmt.Sprintf("openflare-relay-%s-%s", goos, goarch)
	if goos == "windows" {
		return name + ".exe"
	}
	return name
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	return v
}

func isNewer(local, remote string) bool {
	return compareVersions(local, remote) < 0
}

func buildReleaseCheckKey(options UpdateOptions, remoteVersion string) string {
	channel := strings.TrimSpace(options.Channel)
	if channel == "" {
		channel = "stable"
	}
	if tagName := strings.TrimSpace(options.TagName); tagName != "" {
		return channel + ":" + tagName
	}
	return channel + ":" + remoteVersion
}

func compareVersions(local string, remote string) int {
	return utils.CompareVersions(local, remote)
}
