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
	"strconv"
	"strings"
	"time"

	"openflare-agent/internal/agent"
	"openflare-agent/internal/config"
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

func (s *Service) CheckAndUpdate(ctx context.Context, repo string, options agent.UpdateOptions) error {
	release, err := s.getRelease(ctx, repo, options)
	if err != nil {
		return fmt.Errorf("check latest release: %w", err)
	}
	if release == nil || release.TagName == "" {
		return nil
	}

	remoteVersion := normalizeVersion(release.TagName)
	localVersion := normalizeVersion(config.AgentVersion)
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

	slog.Info("agent update available", "from", localVersion, "to", remoteVersion)
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

func (s *Service) getRelease(ctx context.Context, repo string, options agent.UpdateOptions) (*githubRelease, error) {
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

	slog.Info("agent binary updated, restarting")
	return replaceAndRestartFunc(targetPath, tmpPath)
}

func assetNameForGOOSGOARCH(goos string, goarch string) string {
	name := fmt.Sprintf("openflare-agent-%s-%s", goos, goarch)
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

func buildReleaseCheckKey(options agent.UpdateOptions, remoteVersion string) string {
	channel := strings.TrimSpace(options.Channel)
	if channel == "" {
		channel = "stable"
	}
	if tagName := strings.TrimSpace(options.TagName); tagName != "" {
		return channel + ":" + tagName
	}
	return channel + ":" + remoteVersion
}

type versionInfo struct {
	valid      bool
	isDev      bool
	numbers    []int
	prerelease []string
}

func parseVersionInfo(version string) versionInfo {
	normalized := normalizeVersion(version)
	if normalized == "" || strings.EqualFold(normalized, "dev") {
		return versionInfo{isDev: strings.EqualFold(normalized, "dev")}
	}
	base := normalized
	prerelease := ""
	if index := strings.IndexRune(normalized, '-'); index >= 0 {
		base = normalized[:index]
		prerelease = normalized[index+1:]
	}
	segments := strings.Split(base, ".")
	parts := make([]int, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			parts = append(parts, 0)
			continue
		}
		numeric := strings.Builder{}
		for _, r := range segment {
			if r < '0' || r > '9' {
				break
			}
			numeric.WriteRune(r)
		}
		if numeric.Len() == 0 {
			return versionInfo{}
		}
		value, err := strconv.Atoi(numeric.String())
		if err != nil {
			return versionInfo{}
		}
		parts = append(parts, value)
	}
	info := versionInfo{valid: len(parts) > 0, numbers: parts}
	if prerelease != "" {
		info.prerelease = splitPrereleaseIdentifiers(prerelease)
	}
	return info
}

func splitPrereleaseIdentifiers(value string) []string {
	parts := strings.FieldsFunc(strings.TrimSpace(value), func(r rune) bool {
		return r == '.' || r == '-'
	})
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return filtered
}

func compareVersions(local string, remote string) int {
	left := parseVersionInfo(local)
	right := parseVersionInfo(remote)
	if left.isDev {
		if right.valid {
			return -1
		}
		return 0
	}
	if !left.valid || !right.valid {
		return 0
	}

	maxLen := len(left.numbers)
	if len(right.numbers) > maxLen {
		maxLen = len(right.numbers)
	}
	for index := 0; index < maxLen; index++ {
		leftValue := 0
		rightValue := 0
		if index < len(left.numbers) {
			leftValue = left.numbers[index]
		}
		if index < len(right.numbers) {
			rightValue = right.numbers[index]
		}
		if leftValue < rightValue {
			return -1
		}
		if leftValue > rightValue {
			return 1
		}
	}
	if len(left.prerelease) == 0 && len(right.prerelease) == 0 {
		return 0
	}
	if len(left.prerelease) == 0 {
		return 1
	}
	if len(right.prerelease) == 0 {
		return -1
	}
	maxLen = len(left.prerelease)
	if len(right.prerelease) > maxLen {
		maxLen = len(right.prerelease)
	}
	for index := 0; index < maxLen; index++ {
		if index >= len(left.prerelease) {
			return -1
		}
		if index >= len(right.prerelease) {
			return 1
		}
		leftPart := left.prerelease[index]
		rightPart := right.prerelease[index]
		leftNumber, leftErr := strconv.Atoi(leftPart)
		rightNumber, rightErr := strconv.Atoi(rightPart)
		switch {
		case leftErr == nil && rightErr == nil:
			if leftNumber < rightNumber {
				return -1
			}
			if leftNumber > rightNumber {
				return 1
			}
		case leftErr == nil:
			return -1
		case rightErr == nil:
			return 1
		default:
			if leftPart < rightPart {
				return -1
			}
			if leftPart > rightPart {
				return 1
			}
		}
	}
	return 0
}
