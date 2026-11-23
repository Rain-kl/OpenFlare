// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"golang.org/x/mod/semver"
)

const (
	edgeReleasePrefix      = "edge-"
	edgeReleasePageSize    = 100
	maxReleaseMetadataSize = 16 << 20
)

var githubRepositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+/[A-Za-z0-9_.-]+$`)

func isPingoraNode(node *model.OpenFlareNode) bool {
	return node.Engine == enginePingora
}

func edgeReleaseVersion(tag string) string {
	if len(tag) > 64 || !strings.HasPrefix(tag, "edge-v") || strings.Contains(tag, "+") {
		return ""
	}
	version := strings.TrimPrefix(tag, edgeReleasePrefix)
	if !semver.IsValid(version) || semver.Canonical(version) != version {
		return ""
	}
	return version
}

func isNewerEdgeRelease(current, tag string) bool {
	version := edgeReleaseVersion(tag)
	if version == "" {
		return false
	}
	current = strings.TrimPrefix(current, edgeReleasePrefix)
	if !strings.HasPrefix(current, "v") {
		current = "v" + current
	}
	return semver.Compare(version, current) > 0
}

func matchesEdgeRelease(release *githubReleaseResponse, channel releaseChannel) bool {
	version := edgeReleaseVersion(release.TagName)
	if version == "" || release.Draft || release.Prerelease != (channel == releaseChannelPreview) {
		return false
	}
	if channel == releaseChannelStable && semver.Prerelease(version) != "" {
		return false
	}
	pkg := "openflare-edge_" + release.TagName + "_linux_"
	for _, asset := range release.Assets {
		if asset.Name == pkg+"amd64.tar.gz" || asset.Name == pkg+"arm64.tar.gz" {
			return true
		}
	}
	return false
}

func fetchEdgeRelease(ctx context.Context, repo string, channel releaseChannel, tag string) (*githubReleaseResponse, error) {
	if !githubRepositoryPattern.MatchString(repo) {
		return nil, errors.New(errEdgeReleaseLookup)
	}
	if tag == "" {
		release, err := fetchLatestEdgeRelease(ctx, repo, channel)
		if err != nil {
			logger.WarnF(ctx, "failed to check Edge releases: %v", err)
			return nil, errors.New(errEdgeReleaseLookup)
		}
		return release, nil
	}
	if edgeReleaseVersion(tag) == "" {
		return nil, errors.New(errEdgeUpdateUnavailable)
	}
	release, err := fetchGitHubReleaseByTag(ctx, repo, tag)
	if err != nil {
		logger.WarnF(ctx, "failed to check Edge release %s: %v", tag, err)
		return nil, errors.New(errEdgeReleaseLookup)
	}
	if release.TagName != tag || !matchesEdgeRelease(release, channel) {
		return nil, errors.New(errEdgeUpdateUnavailable)
	}
	return release, nil
}

func fetchLatestEdgeRelease(ctx context.Context, repo string, channel releaseChannel) (*githubReleaseResponse, error) {
	base := fmt.Sprintf(githubReleasesAPIBase+"?per_page=%d", repo, edgeReleasePageSize)
	url := base
	for page := 1; ; page++ {
		releases, err := fetchEdgeReleasePage(ctx, url)
		if err != nil {
			return nil, err
		}
		var latest *githubReleaseResponse
		for i := range releases {
			release := &releases[i]
			if matchesEdgeRelease(release, channel) && (latest == nil || isNewerEdgeRelease(latest.TagName, release.TagName)) {
				latest = release
			}
		}
		if latest != nil || len(releases) < edgeReleasePageSize {
			return latest, nil
		}
		url = fmt.Sprintf("%s&page=%d", base, page+1)
	}
}

func fetchEdgeReleasePage(ctx context.Context, url string) ([]githubReleaseResponse, error) {
	req, err := newGitHubReleaseRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	resp, err := releaseHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub returned an unexpected status: %s", resp.Status)
	}
	var releases []githubReleaseResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxReleaseMetadataSize)).Decode(&releases); err != nil {
		return nil, err
	}
	return releases, nil
}
