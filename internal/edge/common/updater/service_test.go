// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testService(httpClient *http.Client) *Service {
	return New(Config{
		LocalVersion: "v1.0.0",
		AssetPrefix:  "openflare-agent",
		LogLabel:     "agent",
		HTTPClient:   httpClient,
	})
}

func TestGetLatestPreviewRelease(t *testing.T) {
	service := testService(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://api.github.com/repos/Rain-kl/OpenFlare/releases?per_page=20" {
				t.Fatalf("unexpected request url: %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`[
						{"tag_name":"v1.0.0","prerelease":false},
						{"tag_name":"v1.1.0-rc.1","prerelease":true}
					]`)),
			}, nil
		}),
	})

	release, err := service.getRelease(context.Background(), "Rain-kl/OpenFlare", UpdateOptions{Channel: "preview"})
	if err != nil {
		t.Fatalf("expected preview release query to succeed: %v", err)
	}
	if release == nil || release.TagName != "v1.1.0-rc.1" {
		t.Fatalf("unexpected preview release: %#v", release)
	}
}

func TestGetReleaseByTag(t *testing.T) {
	service := testService(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://api.github.com/repos/Rain-kl/OpenFlare/releases/tags/v1.1.0-rc.1" {
				t.Fatalf("unexpected request url: %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v1.1.0-rc.1","prerelease":true}`)),
			}, nil
		}),
	})

	release, err := service.getRelease(context.Background(), "Rain-kl/OpenFlare", UpdateOptions{Channel: "preview", TagName: "v1.1.0-rc.1", Force: true})
	if err != nil {
		t.Fatalf("expected tag release query to succeed: %v", err)
	}
	if release == nil || release.TagName != "v1.1.0-rc.1" {
		t.Fatalf("unexpected tag release: %#v", release)
	}
}

func TestCheckAndUpdateRequiresChecksumSource(t *testing.T) {
	assetName := testService(nil).assetNameForGOOSGOARCH(runtime.GOOS, runtime.GOARCH)
	service := testService(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://api.github.com/repos/Rain-kl/OpenFlare/releases/latest" {
				t.Fatalf("unexpected request url: %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
						"tag_name":"v1.0.1",
						"assets":[
							{"name":"` + assetName + `","browser_download_url":"https://example.test/agent"}
						]
					}`)),
			}, nil
		}),
	})

	err := service.CheckAndUpdate(context.Background(), "Rain-kl/OpenFlare", UpdateOptions{})
	if err == nil || !strings.Contains(err.Error(), "no sha256 digest or checksum asset") {
		t.Fatalf("expected missing checksum source error, got %v", err)
	}
}

func TestCheckAndUpdateSchedulesNextCheckByFailureKind(t *testing.T) {
	assetName := testService(nil).assetNameForGOOSGOARCH(runtime.GOOS, runtime.GOARCH)
	digest := "sha256:" + strings.Repeat("a", sha256.Size*2)
	testCases := []struct {
		name             string
		releaseAssetName string
		wantRetry        bool
	}{
		{name: "asset download failure retries with backoff", releaseAssetName: assetName, wantRetry: true},
		{name: "missing platform asset waits for regular interval", releaseAssetName: "openflare-agent-other", wantRetry: false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			currentTime := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
			service := New(Config{
				LocalVersion: "v1.0.0",
				AssetPrefix:  "openflare-agent",
				LogLabel:     "agent",
				NowFunc:      func() time.Time { return currentTime },
				HTTPClient: &http.Client{
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						if req.URL.Host != "api.github.com" {
							return &http.Response{
								StatusCode: http.StatusInternalServerError,
								Status:     "500 Internal Server Error",
								Header:     make(http.Header),
								Body:       io.NopCloser(strings.NewReader("error")),
							}, nil
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     make(http.Header),
							Body: io.NopCloser(strings.NewReader(`{"tag_name":"v1.0.1","assets":[` +
								`{"name":"` + tc.releaseAssetName + `","browser_download_url":"https://example.test/agent","digest":"` + digest + `"}]}`)),
						}, nil
					}),
				},
			})

			if err := service.CheckAndUpdate(context.Background(), "Rain-kl/OpenFlare", UpdateOptions{}); err == nil {
				t.Fatalf("expected update error")
			}
			currentTime = currentTime.Add(DefaultInitialBackoff)
			if got := service.CanCheck(UpdateOptions{}); got != tc.wantRetry {
				t.Fatalf("CanCheck after initial backoff = %v, want %v", got, tc.wantRetry)
			}
		})
	}
}

func TestNormalizeGitHubDigest(t *testing.T) {
	checksum := strings.Repeat("a", sha256.Size*2)
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "prefixed digest", input: "sha256:" + checksum, want: checksum},
		{name: "bare hex", input: checksum, want: checksum},
		{name: "empty", input: "", want: ""},
		{name: "invalid", input: "sha256:not-a-digest", want: ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := normalizeGitHubDigest(testCase.input); got != testCase.want {
				t.Fatalf("unexpected digest: got %q want %q", got, testCase.want)
			}
		})
	}
}

func TestResolveReleaseAssetPrefersDigest(t *testing.T) {
	assetName := testService(nil).assetNameForGOOSGOARCH(runtime.GOOS, runtime.GOARCH)
	checksum := strings.Repeat("b", sha256.Size*2)
	service := testService(nil)

	downloadURL, expectedChecksum, err := service.resolveReleaseAsset(context.Background(), &githubRelease{
		TagName: "v1.0.1",
		Assets: []githubAsset{
			{
				Name:               assetName,
				BrowserDownloadURL: "https://example.test/agent",
				Digest:             "sha256:" + checksum,
			},
			{
				Name:               assetName + ".sha256",
				BrowserDownloadURL: "https://example.test/agent.sha256",
			},
		},
	}, assetName)
	if err != nil {
		t.Fatalf("expected digest resolution to succeed: %v", err)
	}
	if downloadURL != "https://example.test/agent" {
		t.Fatalf("unexpected download url: %s", downloadURL)
	}
	if expectedChecksum != checksum {
		t.Fatalf("unexpected checksum: got %s want %s", expectedChecksum, checksum)
	}
}

func TestResolveReleaseAssetFallsBackToChecksumAsset(t *testing.T) {
	assetName := testService(nil).assetNameForGOOSGOARCH(runtime.GOOS, runtime.GOARCH)
	checksum := strings.Repeat("c", sha256.Size*2)
	service := testService(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://example.test/agent.sha256" {
				t.Fatalf("unexpected request url: %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(checksum + "\n")),
			}, nil
		}),
	})

	downloadURL, expectedChecksum, err := service.resolveReleaseAsset(context.Background(), &githubRelease{
		TagName: "v1.0.1",
		Assets: []githubAsset{
			{
				Name:               assetName,
				BrowserDownloadURL: "https://example.test/agent",
			},
			{
				Name:               assetName + ".sha256",
				BrowserDownloadURL: "https://example.test/agent.sha256",
			},
		},
	}, assetName)
	if err != nil {
		t.Fatalf("expected checksum fallback to succeed: %v", err)
	}
	if downloadURL != "https://example.test/agent" {
		t.Fatalf("unexpected download url: %s", downloadURL)
	}
	if expectedChecksum != checksum {
		t.Fatalf("unexpected checksum: got %s want %s", expectedChecksum, checksum)
	}
}

func TestParseSHA256Checksum(t *testing.T) {
	checksum := strings.Repeat("a", sha256.Size*2)
	testCases := []struct {
		name    string
		content string
		asset   string
		want    string
	}{
		{name: "single digest", content: checksum + "\n", asset: "openflare-agent-linux-amd64", want: checksum},
		{name: "sha256sum format", content: checksum + "  openflare-agent-linux-amd64\n", asset: "openflare-agent-linux-amd64", want: checksum},
		{name: "bsd format", content: "SHA256(openflare-agent-linux-amd64)= " + checksum + "\n", asset: "openflare-agent-linux-amd64", want: checksum},
		{name: "selects matching file", content: strings.Repeat("b", sha256.Size*2) + "  other\n" + checksum + "  openflare-agent-linux-amd64\n", asset: "openflare-agent-linux-amd64", want: checksum},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := parseSHA256Checksum(testCase.content, testCase.asset)
			if err != nil {
				t.Fatalf("expected checksum parse to succeed: %v", err)
			}
			if got != testCase.want {
				t.Fatalf("unexpected checksum: got %s want %s", got, testCase.want)
			}
		})
	}
}

func TestDownloadAndRestartVerifiesChecksum(t *testing.T) {
	payload := []byte("new-agent-binary")
	sum := sha256.Sum256(payload)
	expectedChecksum := hex.EncodeToString(sum[:])
	targetPath := filepath.Join(t.TempDir(), "openflare-agent")
	if err := os.WriteFile(targetPath, []byte("old-agent-binary"), 0o755); err != nil {
		t.Fatalf("write target: %v", err)
	}

	var replacedTarget string
	var replacedTemp string
	originalReplace := replaceAndRestartFunc
	replaceAndRestartFunc = func(execPath string, tmpPath string) error {
		replacedTarget = execPath
		replacedTemp = tmpPath
		return nil
	}
	t.Cleanup(func() {
		replaceAndRestartFunc = originalReplace
	})

	service := testService(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(string(payload))),
			}, nil
		}),
	})

	if err := service.downloadAndRestart(context.Background(), "https://example.test/agent", expectedChecksum, targetPath); err != nil {
		t.Fatalf("expected verified download to succeed: %v", err)
	}
	if replacedTarget != targetPath {
		t.Fatalf("unexpected replace target: %s", replacedTarget)
	}
	if replacedTemp == "" {
		t.Fatal("expected replacement temp path to be recorded")
	}
	if _, err := os.Stat(replacedTemp); err != nil {
		t.Fatalf("expected verified temp binary to remain for replacement: %v", err)
	}
}

func TestDownloadAndRestartRejectsChecksumMismatch(t *testing.T) {
	targetPath := filepath.Join(t.TempDir(), "openflare-agent")
	if err := os.WriteFile(targetPath, []byte("old-agent-binary"), 0o755); err != nil {
		t.Fatalf("write target: %v", err)
	}

	originalReplace := replaceAndRestartFunc
	replaceAndRestartFunc = func(execPath string, tmpPath string) error {
		t.Fatal("replace should not run on checksum mismatch")
		return nil
	}
	t.Cleanup(func() {
		replaceAndRestartFunc = originalReplace
	})

	service := testService(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("tampered")),
			}, nil
		}),
	})

	err := service.downloadAndRestart(context.Background(), "https://example.test/agent", strings.Repeat("0", sha256.Size*2), targetPath)
	if err == nil || !strings.Contains(err.Error(), "sha256 checksum mismatch") {
		t.Fatalf("expected checksum mismatch error, got %v", err)
	}
	if _, err = os.Stat(targetPath + ".update"); !os.IsNotExist(err) {
		t.Fatalf("expected temp update file to be removed, stat err=%v", err)
	}
}

func TestIsNewerSupportsPrerelease(t *testing.T) {
	testCases := []struct {
		name     string
		local    string
		remote   string
		expected bool
	}{
		{name: "stable newer than prerelease", local: "1.2.3-rc.1", remote: "1.2.3", expected: true},
		{name: "same stable not newer", local: "1.2.3", remote: "1.2.3-rc.1", expected: false},
		{name: "higher prerelease sequence", local: "1.2.3-rc.1", remote: "1.2.3-rc.2", expected: true},
		{name: "higher minor", local: "1.2.3", remote: "1.3.0-rc.1", expected: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if actual := isNewer(testCase.local, testCase.remote); actual != testCase.expected {
				t.Fatalf("unexpected compare result: local=%s remote=%s actual=%v expected=%v", testCase.local, testCase.remote, actual, testCase.expected)
			}
		})
	}
}

func TestCheckAndUpdateIntervalAndBackoff(t *testing.T) {
	currentTime := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	nowFunc := func() time.Time { return currentTime }

	var (
		statusCode   = http.StatusOK
		responseBody = `{"tag_name":"v1.0.0"}`
		requestCount = 0
	)

	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestCount++
			if statusCode != http.StatusOK {
				return &http.Response{
					StatusCode: statusCode,
					Status:     http.StatusText(statusCode),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("error")),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(responseBody)),
			}, nil
		}),
	}

	service := New(Config{
		LocalVersion:   "v1.0.0",
		AssetPrefix:    "openflare-agent",
		LogLabel:       "agent",
		CheckInterval:  6 * time.Hour,
		InitialBackoff: 1 * time.Minute,
		MaxBackoff:     3 * time.Hour,
		NowFunc:        nowFunc,
		HTTPClient:     httpClient,
	})

	// 1. Initial state: CanCheck should be true
	if !service.CanCheck(UpdateOptions{}) {
		t.Fatalf("expected CanCheck to be true initially")
	}

	// 2. First check succeeds (version v1.0.0 is same as local, no update needed)
	err := service.CheckAndUpdate(context.Background(), "Rain-kl/OpenFlare", UpdateOptions{})
	if err != nil {
		t.Fatalf("unexpected error on first check: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("expected 1 request, got %d", requestCount)
	}

	// Immediately after success: CanCheck should be false, and CheckAndUpdate should be throttled (no HTTP call)
	if service.CanCheck(UpdateOptions{}) {
		t.Fatalf("expected CanCheck to be false immediately after success")
	}
	err = service.CheckAndUpdate(context.Background(), "Rain-kl/OpenFlare", UpdateOptions{})
	if err != nil {
		t.Fatalf("unexpected error on throttled check: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("expected request count to remain 1, got %d", requestCount)
	}

	// 3. Fast-forward 5 hours: still throttled (< 6h)
	currentTime = currentTime.Add(5 * time.Hour)
	if service.CanCheck(UpdateOptions{}) {
		t.Fatalf("expected CanCheck to be false after 5 hours")
	}

	// Force check should bypass throttle even within 6 hours
	if !service.CanCheck(UpdateOptions{Force: true}) {
		t.Fatalf("expected CanCheck(Force: true) to be true within 6 hours")
	}
	err = service.CheckAndUpdate(context.Background(), "Rain-kl/OpenFlare", UpdateOptions{Force: true})
	if err != nil {
		t.Fatalf("unexpected error on force check: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("expected request count 2 after force check, got %d", requestCount)
	}

	// 4. Fast-forward to 6 hours after the force check: should be allowed again
	currentTime = currentTime.Add(6 * time.Hour)
	if !service.CanCheck(UpdateOptions{}) {
		t.Fatalf("expected CanCheck to be true after 6 hours")
	}

	// 5. Simulate Failure: GitHub returns 403 Forbidden
	statusCode = http.StatusForbidden
	err = service.CheckAndUpdate(context.Background(), "Rain-kl/OpenFlare", UpdateOptions{})
	if err == nil {
		t.Fatalf("expected error on 403 Forbidden check")
	}
	if requestCount != 3 {
		t.Fatalf("expected request count 3, got %d", requestCount)
	}

	// Immediately after 1st failure: CanCheck should be false (1 min initial backoff)
	if service.CanCheck(UpdateOptions{}) {
		t.Fatalf("expected CanCheck to be false after 1st failure")
	}
	// At +30s: still false
	currentTime = currentTime.Add(30 * time.Second)
	if service.CanCheck(UpdateOptions{}) {
		t.Fatalf("expected CanCheck to be false at +30s after 1st failure")
	}
	// At +1m (total +60s): should be true
	currentTime = currentTime.Add(30 * time.Second)
	if !service.CanCheck(UpdateOptions{}) {
		t.Fatalf("expected CanCheck to be true at +1m after 1st failure")
	}

	// 6. Second failure: backoff doubles to 2 minutes
	err = service.CheckAndUpdate(context.Background(), "Rain-kl/OpenFlare", UpdateOptions{})
	if err == nil {
		t.Fatalf("expected error on 2nd failure")
	}
	if requestCount != 4 {
		t.Fatalf("expected request count 4, got %d", requestCount)
	}
	// At +1m: should still be false
	currentTime = currentTime.Add(1 * time.Minute)
	if service.CanCheck(UpdateOptions{}) {
		t.Fatalf("expected CanCheck to be false at +1m after 2nd failure (backoff is 2m)")
	}
	// At +2m: should be true
	currentTime = currentTime.Add(1 * time.Minute)
	if !service.CanCheck(UpdateOptions{}) {
		t.Fatalf("expected CanCheck to be true at +2m after 2nd failure")
	}

	// 7. Simulate multiple consecutive failures to verify exponential doubling and capping at 3 hours
	expectedBackoffs := []time.Duration{
		4 * time.Minute,
		8 * time.Minute,
		16 * time.Minute,
		32 * time.Minute,
		64 * time.Minute,
		128 * time.Minute,
		3 * time.Hour, // 256m capped at 180m (3h)
		3 * time.Hour, // capped at 3h
	}
	for i, expectedBackoff := range expectedBackoffs {
		err = service.CheckAndUpdate(context.Background(), "Rain-kl/OpenFlare", UpdateOptions{})
		if err == nil {
			t.Fatalf("expected error on consecutive failure iteration %d", i)
		}
		// Halfway through backoff: should be false
		currentTime = currentTime.Add(expectedBackoff / 2)
		if service.CanCheck(UpdateOptions{}) {
			t.Fatalf("expected CanCheck to be false halfway through backoff %v (iter %d)", expectedBackoff, i)
		}
		// At full backoff: should be true
		currentTime = currentTime.Add(expectedBackoff / 2)
		if !service.CanCheck(UpdateOptions{}) {
			t.Fatalf("expected CanCheck to be true at full backoff %v (iter %d)", expectedBackoff, i)
		}
	}

	// 8. Recovery: next check succeeds, backoff should be reset and next check scheduled in 6 hours
	statusCode = http.StatusOK
	err = service.CheckAndUpdate(context.Background(), "Rain-kl/OpenFlare", UpdateOptions{})
	if err != nil {
		t.Fatalf("expected success on recovery, got: %v", err)
	}
	// Immediately: CanCheck is false
	if service.CanCheck(UpdateOptions{}) {
		t.Fatalf("expected CanCheck to be false immediately after recovery")
	}
	// At +3 hours: still false (because next check is 6 hours, not 3 hours)
	currentTime = currentTime.Add(3 * time.Hour)
	if service.CanCheck(UpdateOptions{}) {
		t.Fatalf("expected CanCheck to be false at +3 hours after recovery")
	}
	// At +6 hours: should be true
	currentTime = currentTime.Add(3 * time.Hour)
	if !service.CanCheck(UpdateOptions{}) {
		t.Fatalf("expected CanCheck to be true at +6 hours after recovery")
	}
}
