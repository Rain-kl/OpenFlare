package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rain-kl/openflare/openflare-server/common"
	"github.com/rain-kl/openflare/openflare-server/utils"
)

const (
	serverReleaseRepo     = "Rain-kl/OpenFlare"
	githubReleasesAPIBase = "https://api.github.com/repos/%s/releases"
)

type ReleaseChannel string

const (
	ReleaseChannelStable  ReleaseChannel = "stable"
	ReleaseChannelPreview ReleaseChannel = "preview"
)

var updateHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

var serverUpgradeState struct {
	sync.Mutex
	inProgress bool
	status     string
	logs       []ServerUpgradeLogRecord
}

var serverUpgradeSubscribers struct {
	sync.Mutex
	nextID    int
	listeners map[int]chan ServerUpgradeStreamSnapshot
}

var manualServerBinaryState struct {
	sync.Mutex
	candidate *manualServerBinaryCandidate
}

var serverBinaryUpgradeExecutor = replaceAndRestartServer

var serverUpgradeDispatchDelay = 500 * time.Millisecond

type LatestServerRelease struct {
	TagName          string                   `json:"tag_name"`
	Body             string                   `json:"body"`
	HTMLURL          string                   `json:"html_url"`
	PublishedAt      string                   `json:"published_at"`
	Channel          string                   `json:"channel"`
	Prerelease       bool                     `json:"prerelease"`
	CurrentVersion   string                   `json:"current_version"`
	HasUpdate        bool                     `json:"has_update"`
	UpgradeSupported bool                     `json:"upgrade_supported"`
	InProgress       bool                     `json:"in_progress"`
	UpgradeStatus    string                   `json:"upgrade_status"`
	UpgradeLogs      []ServerUpgradeLogRecord `json:"upgrade_logs"`
}

type ServerUpgradeLogRecord struct {
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type ServerUpgradeStreamSnapshot struct {
	InProgress    bool                     `json:"in_progress"`
	UpgradeStatus string                   `json:"upgrade_status"`
	UpgradeLogs   []ServerUpgradeLogRecord `json:"upgrade_logs"`
}

type githubReleaseResponse struct {
	TagName     string        `json:"tag_name"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	PublishedAt string        `json:"published_at"`
	Prerelease  bool          `json:"prerelease"`
	Draft       bool          `json:"draft"`
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

type UploadedServerBinary struct {
	UploadToken       string    `json:"upload_token"`
	FileName          string    `json:"file_name"`
	DetectedVersion   string    `json:"detected_version"`
	CurrentVersion    string    `json:"current_version"`
	HasUpdate         bool      `json:"has_update"`
	UpgradeSupported  bool      `json:"upgrade_supported"`
	ReadyToUpgrade    bool      `json:"ready_to_upgrade"`
	ComparisonMessage string    `json:"comparison_message"`
	UploadedAt        time.Time `json:"uploaded_at"`
}

type manualServerBinaryCandidate struct {
	UploadToken     string
	FileName        string
	DetectedVersion string
	CurrentVersion  string
	TempPath        string
	ExecPath        string
	UploadedAt      time.Time
}

func GetLatestServerRelease(ctx context.Context, channel string) (*LatestServerRelease, error) {
	normalizedChannel := normalizeReleaseChannel(channel)
	release, err := fetchLatestRelease(ctx, normalizedChannel)
	if err != nil {
		return nil, err
	}
	return buildLatestServerReleaseView(release, normalizedChannel), nil
}

func ScheduleServerUpgrade(channel string) (*LatestServerRelease, error) {
	normalizedChannel := normalizeReleaseChannel(channel)
	serverUpgradeState.Lock()
	if serverUpgradeState.inProgress {
		serverUpgradeState.Unlock()
		return nil, fmt.Errorf("服务升级正在执行中，请稍后再试")
	}

	resetServerUpgradeLogsLocked()
	serverUpgradeState.status = "running"
	appendServerUpgradeLogLocked("info", fmt.Sprintf("Automatic upgrade scheduled for channel: %s.", normalizedChannel.String()))
	serverUpgradeState.Unlock()
	broadcastServerUpgradeSnapshot()

	prepared, err := prepareServerUpgrade(context.Background(), normalizedChannel)
	if err != nil {
		serverUpgradeState.Lock()
		serverUpgradeState.status = "failed"
		appendServerUpgradeLogLocked("error", err.Error())
		serverUpgradeState.Unlock()
		broadcastServerUpgradeSnapshot()
		return nil, err
	}

	serverUpgradeState.Lock()
	serverUpgradeState.inProgress = true
	serverUpgradeState.Unlock()
	broadcastServerUpgradeSnapshot()

	prepared.release.InProgress = true

	go func(task *preparedServerUpgrade) {
		time.Sleep(serverUpgradeDispatchDelay)
		if err := executeServerUpgrade(task); err != nil {
			recordServerUpgradeFailure(err)
			slog.Error("server self-update failed", "error", err)
		}
	}(prepared)

	return prepared.release, nil
}

func UploadManualServerBinary(ctx context.Context, fileName string, reader io.Reader) (*UploadedServerBinary, error) {
	inProgress, _, _ := snapshotServerUpgradeState()
	if inProgress {
		return nil, fmt.Errorf("服务升级正在执行中，请稍后再试")
	}
	if strings.TrimSpace(fileName) == "" {
		return nil, fmt.Errorf("缺少上传文件名")
	}
	if reader == nil {
		return nil, fmt.Errorf("缺少上传文件内容")
	}

	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("获取当前服务程序路径失败: %v", err)
	}
	if err = verifyExecutableDirectoryWritable(execPath); err != nil {
		return nil, err
	}
	tempPath, err := persistUploadedServerBinary(filepath.Dir(execPath), fileName, reader)
	if err != nil {
		return nil, err
	}

	detectedVersion, err := detectUploadedServerBinaryVersion(ctx, tempPath)
	if err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}

	currentVersion := strings.TrimSpace(common.Version)
	uploadedAt := time.Now()
	info := buildUploadedServerBinaryView(fileName, currentVersion, detectedVersion, uploadedAt)
	if !info.ReadyToUpgrade {
		_ = os.Remove(tempPath)
		return info, nil
	}

	uploadToken, err := newUpgradeToken()
	if err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("生成升级令牌失败: %v", err)
	}

	manualServerBinaryState.Lock()
	cleanupManualServerBinaryCandidateLocked()
	manualServerBinaryState.candidate = &manualServerBinaryCandidate{
		UploadToken:     uploadToken,
		FileName:        fileName,
		DetectedVersion: detectedVersion,
		CurrentVersion:  currentVersion,
		TempPath:        tempPath,
		ExecPath:        execPath,
		UploadedAt:      uploadedAt,
	}
	manualServerBinaryState.Unlock()

	info.UploadToken = uploadToken
	return info, nil
}

func ConfirmManualServerUpgrade(uploadToken string) (*UploadedServerBinary, error) {
	uploadToken = strings.TrimSpace(uploadToken)
	if uploadToken == "" {
		return nil, fmt.Errorf("缺少升级令牌")
	}

	serverUpgradeState.Lock()
	if serverUpgradeState.inProgress {
		serverUpgradeState.Unlock()
		return nil, fmt.Errorf("服务升级正在执行中，请稍后再试")
	}
	serverUpgradeState.Unlock()

	manualServerBinaryState.Lock()
	candidate := manualServerBinaryState.candidate
	if candidate == nil {
		manualServerBinaryState.Unlock()
		return nil, fmt.Errorf("未找到待确认的上传升级包，请重新上传")
	}
	if candidate.UploadToken != uploadToken {
		manualServerBinaryState.Unlock()
		return nil, fmt.Errorf("升级令牌无效或已过期，请重新上传")
	}
	manualServerBinaryState.candidate = nil
	manualServerBinaryState.Unlock()

	info := buildUploadedServerBinaryView(candidate.FileName, candidate.CurrentVersion, candidate.DetectedVersion, candidate.UploadedAt)
	info.UploadToken = candidate.UploadToken
	if !info.ReadyToUpgrade {
		_ = os.Remove(candidate.TempPath)
		return nil, fmt.Errorf("当前上传的二进制不满足升级条件")
	}

	serverUpgradeState.Lock()
	serverUpgradeState.inProgress = true
	resetServerUpgradeLogsLocked()
	serverUpgradeState.status = "running"
	appendServerUpgradeLogLocked("info", fmt.Sprintf("Manual upgrade confirmed for version: %s.", strings.TrimSpace(candidate.DetectedVersion)))
	serverUpgradeState.Unlock()
	broadcastServerUpgradeSnapshot()

	go func(task *manualServerBinaryCandidate) {
		time.Sleep(serverUpgradeDispatchDelay)
		if err := executeServerBinaryCandidateUpgrade(task, "manual"); err != nil {
			recordServerUpgradeFailure(err)
			slog.Error("server manual upgrade failed", "error", err)
			_ = os.Remove(task.TempPath)
		}
	}(candidate)

	return info, nil
}

func fetchLatestRelease(ctx context.Context, channel ReleaseChannel) (*githubReleaseResponse, error) {
	return fetchLatestGitHubRelease(ctx, serverReleaseRepo, channel)
}

func fetchLatestGitHubRelease(ctx context.Context, repo string, channel ReleaseChannel) (*githubReleaseResponse, error) {
	switch normalizeReleaseChannel(string(channel)) {
	case ReleaseChannelPreview:
		return fetchLatestPreviewGitHubRelease(ctx, repo)
	default:
		return fetchLatestStableGitHubRelease(ctx, repo)
	}
}

func fetchLatestStableGitHubRelease(ctx context.Context, repo string) (*githubReleaseResponse, error) {
	url := fmt.Sprintf(githubReleasesAPIBase+"/latest", strings.TrimSpace(repo))
	req, err := newGitHubReleaseRequest(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("创建更新请求失败")
	}

	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取最新版本失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 返回异常状态: %s", resp.Status)
	}

	return decodeGitHubRelease(resp.Body)
}

func fetchLatestPreviewGitHubRelease(ctx context.Context, repo string) (*githubReleaseResponse, error) {
	url := fmt.Sprintf(githubReleasesAPIBase+"?per_page=20", strings.TrimSpace(repo))
	req, err := newGitHubReleaseRequest(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("创建更新请求失败")
	}

	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取 preview 版本失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 返回异常状态: %s", resp.Status)
	}

	var releases []githubReleaseResponse
	if err = json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("解析 preview 版本信息失败")
	}
	for _, release := range releases {
		if release.Draft || !release.Prerelease {
			continue
		}
		releaseCopy := release
		return &releaseCopy, nil
	}
	return nil, fmt.Errorf("当前没有可用的 preview 发布")
}

func fetchGitHubReleaseByTag(ctx context.Context, repo string, tag string) (*githubReleaseResponse, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, fmt.Errorf("缺少发布版本号")
	}
	url := fmt.Sprintf(githubReleasesAPIBase+"/tags/%s", strings.TrimSpace(repo), tag)
	req, err := newGitHubReleaseRequest(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("创建更新请求失败")
	}

	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取指定版本失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("未找到指定版本: %s", tag)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 返回异常状态: %s", resp.Status)
	}

	return decodeGitHubRelease(resp.Body)
}

func newGitHubReleaseRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "OpenFlare-Server")
	return req, nil
}

func decodeGitHubRelease(reader io.Reader) (*githubReleaseResponse, error) {
	var release githubReleaseResponse
	if err := json.NewDecoder(reader).Decode(&release); err != nil {
		return nil, fmt.Errorf("解析版本信息失败")
	}
	return &release, nil
}

func buildLatestServerReleaseView(release *githubReleaseResponse, channel ReleaseChannel) *LatestServerRelease {
	currentVersion := strings.TrimSpace(common.Version)
	isDevBuild := currentVersion == "" || strings.EqualFold(currentVersion, "dev")
	hasUpdate := false
	if release != nil && !isDevBuild {
		if channel == ReleaseChannelPreview {
			// Preview releases use a "major.minor.patch-git-<commit>" scheme that cannot
			// be meaningfully compared against the running stable version, so we skip the
			// version check and always allow upgrading when the user explicitly selects
			// the preview channel.
			hasUpdate = true
		} else {
			hasUpdate = isVersionNewer(currentVersion, release.TagName)
		}
	}

	inProgress, upgradeStatus, upgradeLogs := snapshotServerUpgradeState()

	view := &LatestServerRelease{
		Channel:          channel.String(),
		CurrentVersion:   currentVersion,
		HasUpdate:        hasUpdate,
		UpgradeSupported: !isDevBuild && runtime.GOOS != "windows",
		InProgress:       inProgress,
		UpgradeStatus:    upgradeStatus,
		UpgradeLogs:      upgradeLogs,
	}
	if release != nil {
		view.TagName = release.TagName
		view.Body = release.Body
		view.HTMLURL = release.HTMLURL
		view.PublishedAt = release.PublishedAt
		view.Prerelease = release.Prerelease
	}
	return view
}

func prepareServerUpgrade(ctx context.Context, channel ReleaseChannel) (*preparedServerUpgrade, error) {
	release, err := fetchLatestRelease(ctx, channel)
	if err != nil {
		return nil, err
	}

	view := buildLatestServerReleaseView(release, channel)
	if !view.HasUpdate {
		return nil, fmt.Errorf("当前已经是最新版本")
	}
	if !view.UpgradeSupported {
		return nil, fmt.Errorf("当前平台暂不支持自动升级")
	}

	assetName := serverAssetName(runtime.GOOS, runtime.GOARCH)
	recordServerUpgradeLog("info", fmt.Sprintf("Matching release asset: %s.", assetName))

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
	recordServerUpgradeLog("info", "Verified current executable directory is writable.")

	return &preparedServerUpgrade{
		release:     view,
		downloadURL: downloadURL,
		execPath:    execPath,
	}, nil
}

func verifyExecutableDirectoryWritable(execPath string) error {
	dir := filepath.Dir(execPath)
	tempFile, err := os.CreateTemp(dir, "openflare-server-upgrade-check-*")
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
	recordServerUpgradeLog("info", fmt.Sprintf("Downloading automatic upgrade package for version: %s.", strings.TrimSpace(task.release.TagName)))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, task.downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "OpenFlare-Server")

	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载服务端升级包失败: %s", resp.Status)
	}

	recordServerUpgradeLog("info", "Download finished, validating binary version.")
	candidate, err := persistDownloadedServerBinary(ctx, task.execPath, task.release.TagName, resp.Body)
	if err != nil {
		return err
	}
	return executeServerBinaryCandidateUpgrade(candidate, "auto")
}

func executeServerBinaryCandidateUpgrade(task *manualServerBinaryCandidate, source string) error {
	recordServerUpgradeLog("info", fmt.Sprintf("Validated binary version: %s -> %s.", strings.TrimSpace(task.CurrentVersion), strings.TrimSpace(task.DetectedVersion)))
	recordServerUpgradeLog("info", "Replacing executable and preparing restart.")
	if source == "manual" {
		slog.Info("server manual self-update starting", "from", strings.TrimSpace(task.CurrentVersion), "to", strings.TrimSpace(task.DetectedVersion))
	} else {
		slog.Info("server self-update starting", "from", strings.TrimSpace(task.CurrentVersion), "to", strings.TrimSpace(task.DetectedVersion))
	}
	markServerUpgradeSucceeded()
	return serverBinaryUpgradeExecutor(task.ExecPath, task.TempPath)
}

func serverAssetName(goos string, goarch string) string {
	name := fmt.Sprintf("openflare-server-%s-%s", goos, goarch)
	if goos == "windows" {
		return name + ".exe"
	}
	return name
}

func normalizeReleaseChannel(channel string) ReleaseChannel {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case string(ReleaseChannelPreview):
		return ReleaseChannelPreview
	default:
		return ReleaseChannelStable
	}
}

func (channel ReleaseChannel) String() string {
	if channel == ReleaseChannelPreview {
		return string(ReleaseChannelPreview)
	}
	return string(ReleaseChannelStable)
}

func isVersionNewer(current string, latest string) bool {
	return utils.CompareVersions(current, latest) < 0
}

func buildUploadedServerBinaryView(fileName string, currentVersion string, detectedVersion string, uploadedAt time.Time) *UploadedServerBinary {
	upgradeSupported := isManualServerUpgradeSupported(currentVersion)
	hasUpdate := false
	comparisonMessage := ""

	switch {
	case !upgradeSupported:
		comparisonMessage = "当前服务版本不支持手动升级确认流程"
	case normalizeVersion(currentVersion) == normalizeVersion(detectedVersion):
		comparisonMessage = "上传二进制与当前服务版本一致，无需升级"
	case isVersionNewer(currentVersion, detectedVersion):
		hasUpdate = true
		comparisonMessage = fmt.Sprintf("检测到可升级版本：%s -> %s", strings.TrimSpace(currentVersion), strings.TrimSpace(detectedVersion))
	default:
		comparisonMessage = "上传二进制版本不高于当前服务版本，已拒绝升级"
	}

	return &UploadedServerBinary{
		FileName:          strings.TrimSpace(fileName),
		DetectedVersion:   strings.TrimSpace(detectedVersion),
		CurrentVersion:    strings.TrimSpace(currentVersion),
		HasUpdate:         hasUpdate,
		UpgradeSupported:  upgradeSupported,
		ReadyToUpgrade:    upgradeSupported && hasUpdate,
		ComparisonMessage: comparisonMessage,
		UploadedAt:        uploadedAt,
	}
}

func isManualServerUpgradeSupported(currentVersion string) bool {
	normalized := strings.TrimSpace(strings.TrimPrefix(currentVersion, "v"))
	return normalized != "" && !strings.EqualFold(normalized, "dev")
}

func persistUploadedServerBinary(tempDir string, fileName string, reader io.Reader) (string, error) {
	suffix := filepath.Ext(strings.TrimSpace(fileName))
	if runtime.GOOS == "windows" && suffix == "" {
		suffix = ".exe"
	}
	tempDir = strings.TrimSpace(tempDir)
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	tempFile, err := os.CreateTemp(tempDir, "openflare-server-manual-upgrade-*"+suffix)
	if err != nil {
		return "", fmt.Errorf("创建临时升级文件失败: %v", err)
	}
	tempPath := tempFile.Name()
	if _, err = io.Copy(tempFile, reader); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("写入上传二进制失败: %v", err)
	}
	if err = tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("关闭临时升级文件失败: %v", err)
	}
	if err = os.Chmod(tempPath, 0o755); err != nil && runtime.GOOS != "windows" {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("设置临时升级文件权限失败: %v", err)
	}
	return tempPath, nil
}

func detectUploadedServerBinaryVersion(ctx context.Context, filePath string) (string, error) {
	commandCtx := ctx
	if commandCtx == nil {
		commandCtx = context.Background()
	}
	cmd := exec.CommandContext(commandCtx, filePath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("检查上传二进制版本失败: %w: %s", err, strings.TrimSpace(string(output)))
	}
	version := strings.TrimSpace(string(output))
	if version == "" {
		return "", fmt.Errorf("上传二进制未返回有效版本号")
	}
	for _, line := range strings.Split(version, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed, nil
		}
	}
	return "", fmt.Errorf("上传二进制未返回有效版本号")
}

func persistDownloadedServerBinary(ctx context.Context, execPath string, releaseTag string, reader io.Reader) (*manualServerBinaryCandidate, error) {
	fileName := serverAssetName(runtime.GOOS, runtime.GOARCH)
	tempPath, err := persistUploadedServerBinary(filepath.Dir(execPath), fileName, reader)
	if err != nil {
		return nil, err
	}

	detectedVersion, err := detectUploadedServerBinaryVersion(ctx, tempPath)
	if err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}
	recordServerUpgradeLog("info", fmt.Sprintf("Detected downloaded binary version: %s.", strings.TrimSpace(detectedVersion)))

	if normalizeVersion(detectedVersion) != normalizeVersion(releaseTag) {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("下载包版本校验失败：release=%s，binary=%s", strings.TrimSpace(releaseTag), strings.TrimSpace(detectedVersion))
	}

	info := buildUploadedServerBinaryView(fileName, common.Version, detectedVersion, time.Now())
	if !info.ReadyToUpgrade {
		_ = os.Remove(tempPath)
		return nil, errors.New(info.ComparisonMessage)
	}

	return &manualServerBinaryCandidate{
		FileName:        fileName,
		DetectedVersion: detectedVersion,
		CurrentVersion:  strings.TrimSpace(common.Version),
		TempPath:        tempPath,
		ExecPath:        execPath,
		UploadedAt:      time.Now(),
	}, nil
}

func cleanupManualServerBinaryCandidateLocked() {
	if manualServerBinaryState.candidate == nil {
		return
	}
	_ = os.Remove(manualServerBinaryState.candidate.TempPath)
	manualServerBinaryState.candidate = nil
}

func newUpgradeToken() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func normalizeVersion(version string) string {
	return strings.TrimSpace(strings.TrimPrefix(version, "v"))
}

func snapshotServerUpgradeState() (bool, string, []ServerUpgradeLogRecord) {
	serverUpgradeState.Lock()
	defer serverUpgradeState.Unlock()

	status := strings.TrimSpace(serverUpgradeState.status)
	if status == "" {
		status = "idle"
	}
	logs := make([]ServerUpgradeLogRecord, len(serverUpgradeState.logs))
	copy(logs, serverUpgradeState.logs)
	return serverUpgradeState.inProgress, status, logs
}

func snapshotServerUpgradeStream() ServerUpgradeStreamSnapshot {
	inProgress, status, logs := snapshotServerUpgradeState()
	return ServerUpgradeStreamSnapshot{
		InProgress:    inProgress,
		UpgradeStatus: status,
		UpgradeLogs:   logs,
	}
}

func resetServerUpgradeLogsLocked() {
	serverUpgradeState.logs = nil
}

func appendServerUpgradeLogLocked(level string, message string) {
	serverUpgradeState.logs = append(serverUpgradeState.logs, ServerUpgradeLogRecord{
		Level:     strings.TrimSpace(level),
		Message:   strings.TrimSpace(message),
		CreatedAt: time.Now(),
	})
	if len(serverUpgradeState.logs) > 100 {
		serverUpgradeState.logs = append([]ServerUpgradeLogRecord(nil), serverUpgradeState.logs[len(serverUpgradeState.logs)-100:]...)
	}
}

func recordServerUpgradeLog(level string, message string) {
	serverUpgradeState.Lock()
	appendServerUpgradeLogLocked(level, message)
	serverUpgradeState.Unlock()
	broadcastServerUpgradeSnapshot()
}

func markServerUpgradeSucceeded() {
	serverUpgradeState.Lock()
	serverUpgradeState.inProgress = false
	serverUpgradeState.status = "succeeded"
	appendServerUpgradeLogLocked("info", "Upgrade binary is ready; server restart will begin.")
	serverUpgradeState.Unlock()
	broadcastServerUpgradeSnapshot()
}

func recordServerUpgradeFailure(err error) {
	serverUpgradeState.Lock()
	serverUpgradeState.inProgress = false
	serverUpgradeState.status = "failed"
	if err != nil {
		appendServerUpgradeLogLocked("error", err.Error())
	}
	serverUpgradeState.Unlock()
	broadcastServerUpgradeSnapshot()
}

func SubscribeServerUpgradeStream() (<-chan ServerUpgradeStreamSnapshot, func()) {
	serverUpgradeSubscribers.Lock()
	if serverUpgradeSubscribers.listeners == nil {
		serverUpgradeSubscribers.listeners = make(map[int]chan ServerUpgradeStreamSnapshot)
	}
	serverUpgradeSubscribers.nextID++
	listenerID := serverUpgradeSubscribers.nextID
	listener := make(chan ServerUpgradeStreamSnapshot, 8)
	serverUpgradeSubscribers.listeners[listenerID] = listener
	serverUpgradeSubscribers.Unlock()

	listener <- snapshotServerUpgradeStream()

	unsubscribe := func() {
		serverUpgradeSubscribers.Lock()
		ch, ok := serverUpgradeSubscribers.listeners[listenerID]
		if ok {
			delete(serverUpgradeSubscribers.listeners, listenerID)
		}
		serverUpgradeSubscribers.Unlock()
		if ok {
			close(ch)
		}
	}

	return listener, unsubscribe
}

func broadcastServerUpgradeSnapshot() {
	snapshot := snapshotServerUpgradeStream()

	serverUpgradeSubscribers.Lock()
	if len(serverUpgradeSubscribers.listeners) == 0 {
		serverUpgradeSubscribers.Unlock()
		return
	}
	listeners := make([]chan ServerUpgradeStreamSnapshot, 0, len(serverUpgradeSubscribers.listeners))
	for _, listener := range serverUpgradeSubscribers.listeners {
		listeners = append(listeners, listener)
	}
	serverUpgradeSubscribers.Unlock()

	for _, listener := range listeners {
		select {
		case listener <- snapshot:
		default:
			select {
			case <-listener:
			default:
			}
			select {
			case listener <- snapshot:
			default:
			}
		}
	}
}

func UpdateHTTPClientForTest() *http.Client {
	return updateHTTPClient
}

func SetUpdateHTTPClientForTest(client *http.Client) {
	updateHTTPClient = client
}

func ServerBinaryUpgradeExecutorForTest() func(string, string) error {
	return serverBinaryUpgradeExecutor
}

func SetServerBinaryUpgradeExecutorForTest(executor func(string, string) error) {
	if executor == nil {
		serverBinaryUpgradeExecutor = replaceAndRestartServer
		return
	}
	serverBinaryUpgradeExecutor = executor
}

func ServerUpgradeDispatchDelayForTest() time.Duration {
	return serverUpgradeDispatchDelay
}

func SetServerUpgradeDispatchDelayForTest(delay time.Duration) {
	if delay < 0 {
		delay = 0
	}
	serverUpgradeDispatchDelay = delay
}
