package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"atsflare-agent/internal/config"
)

type Service struct {
	httpClient   *http.Client
	lastCheckTag string
}

func New() *Service {
	return &Service{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func (s *Service) CheckAndUpdate(ctx context.Context, repo string) error {
	release, err := s.getLatestRelease(ctx, repo)
	if err != nil {
		return fmt.Errorf("check latest release: %w", err)
	}
	if release == nil || release.TagName == "" {
		return nil
	}

	remoteVersion := normalizeVersion(release.TagName)
	localVersion := normalizeVersion(config.AgentVersion)

	if remoteVersion == localVersion || remoteVersion == s.lastCheckTag {
		return nil
	}
	if !isNewer(localVersion, remoteVersion) {
		s.lastCheckTag = remoteVersion
		return nil
	}

	log.Printf("agent update available: %s -> %s", localVersion, remoteVersion)
	assetName := assetNameForGOOSGOARCH(runtime.GOOS, runtime.GOARCH)

	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		s.lastCheckTag = remoteVersion
		return fmt.Errorf("no matching asset %q in release %s", assetName, release.TagName)
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	if err = s.downloadAndRestart(ctx, downloadURL, execPath); err != nil {
		return fmt.Errorf("download and restart: %w", err)
	}
	return nil
}

func (s *Service) getLatestRelease(ctx context.Context, repo string) (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
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

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned %s", resp.Status)
	}

	var release githubRelease
	if err = json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func (s *Service) downloadAndRestart(ctx context.Context, url string, targetPath string) error {
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
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err = io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	tmpFile.Close()

	log.Printf("agent binary updated, restarting...")
	return replaceAndRestart(targetPath, tmpPath)
}

func assetNameForGOOSGOARCH(goos string, goarch string) string {
	name := fmt.Sprintf("atsflare-agent-%s-%s", goos, goarch)
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
	localParts := strings.Split(local, ".")
	remoteParts := strings.Split(remote, ".")
	maxLen := len(localParts)
	if len(remoteParts) > maxLen {
		maxLen = len(remoteParts)
	}
	for i := 0; i < maxLen; i++ {
		lp, rp := "0", "0"
		if i < len(localParts) {
			lp = localParts[i]
		}
		if i < len(remoteParts) {
			rp = remoteParts[i]
		}
		if rp > lp {
			return true
		}
		if rp < lp {
			return false
		}
	}
	return false
}
