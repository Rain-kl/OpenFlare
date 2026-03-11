package service

import (
	"atsflare/common"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const latestReleaseURL = "https://api.github.com/repos/Rain-kl/ATSFlare/releases/latest"

var updateHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

var serverUpgradeState struct {
	sync.Mutex
	inProgress bool
}

type LatestServerRelease struct {
	TagName          string `json:"tag_name"`
	Body             string `json:"body"`
	HTMLURL          string `json:"html_url"`
	PublishedAt      string `json:"published_at"`
	CurrentVersion   string `json:"current_version"`
	HasUpdate        bool   `json:"has_update"`
	UpgradeSupported bool   `json:"upgrade_supported"`
	InProgress       bool   `json:"in_progress"`
}

type githubReleaseResponse struct {
	TagName     string        `json:"tag_name"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	PublishedAt string        `json:"published_at"`
	Assets      []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type preparedServerUpgrade struct {
	release     *LatestServerRelease
	downloadURL string
	execPath    string
}

func GetLatestServerRelease(ctx context.Context) (*LatestServerRelease, error) {
	release, err := fetchLatestRelease(ctx)
	if err != nil {
		return nil, err
	}
	return buildLatestServerReleaseView(release), nil
}

func ScheduleServerUpgrade() (*LatestServerRelease, error) {
	serverUpgradeState.Lock()
	if serverUpgradeState.inProgress {
		serverUpgradeState.Unlock()
		return nil, fmt.Errorf("服务升级已在执行中，请稍后再试")
	}

	prepared, err := prepareServerUpgrade(context.Background())
	if err != nil {
		serverUpgradeState.Unlock()
		return nil, err
	}

	serverUpgradeState.inProgress = true
	serverUpgradeState.Unlock()

	prepared.release.InProgress = true

	go func(task *preparedServerUpgrade) {
		time.Sleep(500 * time.Millisecond)
		if err := executeServerUpgrade(task); err != nil {
			log.Printf("server self-update failed: %v", err)
			serverUpgradeState.Lock()
			serverUpgradeState.inProgress = false
			serverUpgradeState.Unlock()
		}
	}(prepared)

	return prepared.release, nil
}

func fetchLatestRelease(ctx context.Context) (*githubReleaseResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建更新请求失败")
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ATSFlare-Server")

	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取最新版本失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 返回异常状态: %s", resp.Status)
	}

	var release githubReleaseResponse
	if err = json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("解析最新版本信息失败")
	}
	return &release, nil
}

func buildLatestServerReleaseView(release *githubReleaseResponse) *LatestServerRelease {
	currentVersion := strings.TrimSpace(common.Version)
	hasUpdate := false
	if release != nil {
		hasUpdate = isVersionNewer(currentVersion, release.TagName)
	}

	serverUpgradeState.Lock()
	inProgress := serverUpgradeState.inProgress
	serverUpgradeState.Unlock()

	view := &LatestServerRelease{
		CurrentVersion:   currentVersion,
		HasUpdate:        hasUpdate,
		UpgradeSupported: runtime.GOOS != "windows",
		InProgress:       inProgress,
	}
	if release != nil {
		view.TagName = release.TagName
		view.Body = release.Body
		view.HTMLURL = release.HTMLURL
		view.PublishedAt = release.PublishedAt
	}
	return view
}

func prepareServerUpgrade(ctx context.Context) (*preparedServerUpgrade, error) {
	release, err := fetchLatestRelease(ctx)
	if err != nil {
		return nil, err
	}

	view := buildLatestServerReleaseView(release)
	if !view.HasUpdate {
		return nil, fmt.Errorf("当前已是最新版本")
	}
	if !view.UpgradeSupported {
		return nil, fmt.Errorf("当前平台暂不支持自动升级")
	}

	assetName := serverAssetName(runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return nil, fmt.Errorf("最新版本缺少当前平台的服务端二进制: %s", assetName)
	}

	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("获取当前服务程序路径失败: %v", err)
	}
	if err = verifyExecutableDirectoryWritable(execPath); err != nil {
		return nil, err
	}

	return &preparedServerUpgrade{
		release:     view,
		downloadURL: downloadURL,
		execPath:    execPath,
	}, nil
}

func verifyExecutableDirectoryWritable(execPath string) error {
	dir := filepath.Dir(execPath)
	tempFile, err := os.CreateTemp(dir, "atsflare-server-upgrade-check-*")
	if err != nil {
		return fmt.Errorf("当前服务二进制目录不可写，无法升级: %v", err)
	}
	tempPath := tempFile.Name()
	if closeErr := tempFile.Close(); closeErr != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("校验服务升级目录失败: %v", closeErr)
	}
	if err = os.Remove(tempPath); err != nil {
		return fmt.Errorf("清理升级校验文件失败: %v", err)
	}
	return nil
}

func executeServerUpgrade(task *preparedServerUpgrade) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, task.downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "ATSFlare-Server")

	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载服务端升级包失败: %s", resp.Status)
	}

	tmpPath := task.execPath + ".update"
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err = io.Copy(tmpFile, resp.Body); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err = tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	common.SysLog("server self-update starting: from=" + strings.TrimSpace(common.Version) + " to=" + task.release.TagName)
	return replaceAndRestartServer(task.execPath, tmpPath)
}

func serverAssetName(goos string, goarch string) string {
	name := fmt.Sprintf("atsflare-server-%s-%s", goos, goarch)
	if goos == "windows" {
		return name + ".exe"
	}
	return name
}

func isVersionNewer(current string, latest string) bool {
	currentParts := parseVersionParts(current)
	latestParts := parseVersionParts(latest)
	maxLen := len(currentParts)
	if len(latestParts) > maxLen {
		maxLen = len(latestParts)
	}

	for i := 0; i < maxLen; i++ {
		currentPart := 0
		latestPart := 0
		if i < len(currentParts) {
			currentPart = currentParts[i]
		}
		if i < len(latestParts) {
			latestPart = latestParts[i]
		}
		if latestPart > currentPart {
			return true
		}
		if latestPart < currentPart {
			return false
		}
	}
	return false
}

func parseVersionParts(version string) []int {
	normalized := strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if normalized == "" || normalized == "dev" {
		return nil
	}

	segments := strings.Split(normalized, ".")
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
			parts = append(parts, 0)
			continue
		}
		value, err := strconv.Atoi(numeric.String())
		if err != nil {
			parts = append(parts, 0)
			continue
		}
		parts = append(parts, value)
	}
	return parts
}

func UpdateHTTPClientForTest() *http.Client {
	return updateHTTPClient
}

func SetUpdateHTTPClientForTest(client *http.Client) {
	updateHTTPClient = client
}
