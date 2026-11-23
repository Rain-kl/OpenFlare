// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/httppool"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"golang.org/x/mod/semver"
)

const (
	defaultTunnelUpdateRepo        = "Rain-kl/OpenFlare"
	tunnelReleasePrefix            = "tunnel-"
	tunnelReleasePageSize          = 100
	maxTunnelReleaseMetadataBytes  = 16 << 20
	tunnelReleaseHTTPTimeout       = 15 * time.Second
	channelStable                  = "stable"
	channelPreview                 = "preview"
	defaultTunnelHeartbeatMS       = 5000
	defaultTunnelLeaseTTLSeconds   = 30
	defaultTunnelHARelays          = 2
	githubReleasesEndpointTemplate = "https://api.github.com/repos/%s/releases?per_page=%d"
)

var (
	githubRepoPattern   = regexp.MustCompile(`^[A-Za-z0-9_-]+/[A-Za-z0-9_.-]+$`)
	releaseHTTPClientFn = func() *http.Client { return httppool.NewClient(tunnelReleaseHTTPTimeout) }
)

// TunnelReleaseInfo describes the latest available tunnel binary release (tunnel-v*).
type TunnelReleaseInfo struct {
	Channel     string `json:"channel"`
	Repository  string `json:"repository"`
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Prerelease  bool   `json:"prerelease"`
}

// BinaryUpdateInput requests an immediate binary update on a relay or tunnel connector group.
type BinaryUpdateInput struct {
	Channel string `json:"channel"`
	Tag     string `json:"tag"`
}

type githubReleaseEntry struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Prerelease  bool   `json:"prerelease"`
	Draft       bool   `json:"draft"`
}

func resolveTunnelUpdateRepo(ctx context.Context) string {
	sc, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyTunnelUpdateRepo)
	if err == nil {
		repo := strings.TrimSpace(sc.Value)
		if githubRepoPattern.MatchString(repo) {
			return repo
		}
	}
	return defaultTunnelUpdateRepo
}

func resolveTunnelHeartbeatMS(ctx context.Context) int {
	val, err := repository.GetIntByKey(ctx, model.ConfigKeyTunnelHeartbeatInterval)
	if err == nil && val > 0 {
		return val
	}
	return defaultTunnelHeartbeatMS
}

func resolveTunnelLeaseTTLSeconds(ctx context.Context) int {
	val, err := repository.GetIntByKey(ctx, model.ConfigKeyTunnelLeaseTTLSeconds)
	if err == nil && val > 0 {
		return val
	}
	return defaultTunnelLeaseTTLSeconds
}

func resolveDefaultHARelays(ctx context.Context) int {
	val, err := repository.GetIntByKey(ctx, model.ConfigKeyTunnelDefaultHARelays)
	if err == nil && val > 0 {
		return val
	}
	return defaultTunnelHARelays
}

func normalizeUpdateChannel(channel string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "", channelStable:
		return channelStable, nil
	case channelPreview:
		return channelPreview, nil
	default:
		return "", errors.New(errInvalidUpdateChannel)
	}
}

func parseTunnelReleaseVersion(tag string) (version string, isPrerelease bool, ok bool) {
	tag = strings.TrimSpace(tag)
	if len(tag) > 64 || !strings.HasPrefix(tag, tunnelReleasePrefix+"v") || strings.Contains(tag, "+") {
		return "", false, false
	}
	version = strings.TrimPrefix(tag, tunnelReleasePrefix)
	if !semver.IsValid(version) || semver.Canonical(version) != version {
		return "", false, false
	}
	return version, semver.Prerelease(version) != "", true
}

// GetLatestTunnelRelease queries GitHub Releases for the latest matching tunnel-v* release.
func GetLatestTunnelRelease(ctx context.Context, rawChannel string) (*TunnelReleaseInfo, error) {
	channel, err := normalizeUpdateChannel(rawChannel)
	if err != nil {
		return nil, err
	}
	repo := resolveTunnelUpdateRepo(ctx)
	url := fmt.Sprintf(githubReleasesEndpointTemplate, repo, tunnelReleasePageSize)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "OpenFlare-ControlPlane")

	resp, err := releaseHTTPClientFn().Do(req)
	if err != nil {
		logger.WarnF(ctx, "fetch tunnel releases failed: repo=%s err=%v", repo, err)
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases API returned status %d", resp.StatusCode)
	}

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, maxTunnelReleaseMetadataBytes))
	if err != nil {
		return nil, err
	}
	var releases []githubReleaseEntry
	if err := json.Unmarshal(rawBody, &releases); err != nil {
		return nil, err
	}

	var best *githubReleaseEntry
	var bestVersion string
	for i := range releases {
		entry := &releases[i]
		if entry.Draft {
			continue
		}
		ver, pre, ok := parseTunnelReleaseVersion(entry.TagName)
		if !ok {
			continue
		}
		if channel == channelStable && (entry.Prerelease || pre) {
			continue
		}
		if best == nil || semver.Compare(ver, bestVersion) > 0 {
			best = entry
			bestVersion = ver
		}
	}
	if best == nil {
		return nil, errors.New(errRelayReleaseNotFound)
	}
	return &TunnelReleaseInfo{
		Channel:     channel,
		Repository:  repo,
		TagName:     best.TagName,
		Name:        best.Name,
		Body:        best.Body,
		HTMLURL:     best.HTMLURL,
		PublishedAt: best.PublishedAt,
		Prerelease:  best.Prerelease || semver.Prerelease(bestVersion) != "",
	}, nil
}

func buildTunnelUpdateSettings(ctx context.Context, autoUpdate, requested bool, channel, tag string) *protocol.TunnelUpdateSettings {
	repo := resolveTunnelUpdateRepo(ctx)
	normalizedChannel, err := normalizeUpdateChannel(channel)
	if err != nil {
		normalizedChannel = channelStable
	}
	trimmedTag := strings.TrimSpace(tag)
	if !autoUpdate && !requested && trimmedTag == "" {
		return nil
	}
	return &protocol.TunnelUpdateSettings{
		Repo:       repo,
		AutoUpdate: autoUpdate,
		Channel:    normalizedChannel,
		Tag:        trimmedTag,
	}
}
