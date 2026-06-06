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

	"github.com/rain-kl/openflare/openflare-agent/internal/agent"
	"github.com/rain-kl/openflare/openflare-agent/internal/config"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGetLatestPreviewRelease(t *testing.T) {
	service := &Service{
		httpClient: &http.Client{
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
		},
	}

	release, err := service.getRelease(context.Background(), "Rain-kl/OpenFlare", agent.UpdateOptions{Channel: "preview"})
	if err != nil {
		t.Fatalf("expected preview release query to succeed: %v", err)
	}
	if release == nil || release.TagName != "v1.1.0-rc.1" {
		t.Fatalf("unexpected preview release: %#v", release)
	}
}

func TestGetReleaseByTag(t *testing.T) {
	service := &Service{
		httpClient: &http.Client{
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
		},
	}

	release, err := service.getRelease(context.Background(), "Rain-kl/OpenFlare", agent.UpdateOptions{Channel: "preview", TagName: "v1.1.0-rc.1", Force: true})
	if err != nil {
		t.Fatalf("expected tag release query to succeed: %v", err)
	}
	if release == nil || release.TagName != "v1.1.0-rc.1" {
		t.Fatalf("unexpected tag release: %#v", release)
	}
}

func TestCheckAndUpdateRequiresChecksumAsset(t *testing.T) {
	originalVersion := config.Version
	config.Version = "v1.0.0"
	t.Cleanup(func() {
		config.Version = originalVersion
	})

	assetName := assetNameForGOOSGOARCH(runtime.GOOS, runtime.GOARCH)
	service := &Service{
		httpClient: &http.Client{
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
		},
	}

	err := service.CheckAndUpdate(context.Background(), "Rain-kl/OpenFlare", agent.UpdateOptions{})
	if err == nil || !strings.Contains(err.Error(), "no matching checksum asset") {
		t.Fatalf("expected missing checksum asset error, got %v", err)
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

	service := &Service{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(string(payload))),
				}, nil
			}),
		},
	}

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

	service := &Service{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("tampered")),
				}, nil
			}),
		},
	}

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
