// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package githubrelease

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type resolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (resolve resolverFunc) LookupNetIP(ctx context.Context, network string, host string) ([]netip.Addr, error) {
	return resolve(ctx, network, host)
}

func TestResolveLatestUsesGitHubContractAndSelectsUploadedAsset(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/acme/site/releases/latest" {
			t.Errorf("path = %q", request.URL.Path)
		}
		assertHeader(t, request, "Accept", metadataAccept)
		assertHeader(t, request, "User-Agent", defaultUserAgent)
		assertHeader(t, request, "X-GitHub-Api-Version", APIVersion)
		assertHeader(t, request, "If-None-Match", `W/"old"`)
		writer.Header().Set("ETag", `W/"new"`)
		writer.Header().Set("X-RateLimit-Remaining", "0")
		writer.Header().Set("X-RateLimit-Reset", "1800000000")
		_, _ = writer.Write([]byte(`{
            "id": 9007199254740991,
            "tag_name": "v1.2.3",
            "name": "Stable",
            "published_at": "2026-07-18T12:00:00Z",
            "assets": [
              {"id": 11, "name": "dist.zip", "state": "new", "size": 1},
              {"id": 9007199254740990, "name": "dist.zip", "state": "uploaded", "size": 42,
               "updated_at": "2026-07-18T12:10:00Z", "digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
            ]
          }`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil)
	result, err := client.Resolve(context.Background(), ResolveRequest{
		Repository: "acme/site",
		Selector:   SelectorLatest,
		AssetName:  "dist.zip",
		ETag:       `W/"old"`,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if result.Release.ID != "9007199254740991" || result.Asset.ID != "9007199254740990" {
		t.Fatalf("IDs lost precision: release=%q asset=%q", result.Release.ID, result.Asset.ID)
	}
	if result.ETag != `W/"new"` || result.Asset.Name != "dist.zip" || result.Asset.State != "uploaded" {
		t.Fatalf("Resolve() = %+v", result)
	}
	if result.RetryAt == nil || result.RetryAt.Unix() != 1800000000 {
		t.Fatalf("RetryAt = %v", result.RetryAt)
	}
}

func TestResolveTagEscapesPathAndHandlesNotModified(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.RequestURI != "/repos/acme/site/releases/tags/release%2Fcandidate" {
			t.Errorf("RequestURI = %q", request.RequestURI)
		}
		writer.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, nil)
	result, err := client.Resolve(context.Background(), ResolveRequest{
		Repository: "acme/site",
		Selector:   SelectorTag,
		Tag:        "release/candidate",
		AssetName:  "dist.zip",
		ETag:       `"cached"`,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !result.NotModified || result.ETag != `"cached"` {
		t.Fatalf("Resolve() = %+v", result)
	}
}

func TestResolveAssetMissingTruncatesSafeNamesAndNeverIncludesBody(t *testing.T) {
	t.Parallel()
	assets := make([]string, 0, 12)
	for index := 0; index < 12; index++ {
		assets = append(assets, fmt.Sprintf(`{"id":%d,"name":"asset-%02d.zip","state":"uploaded","size":1}`, index+1, index))
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"id":1,"tag_name":"v1","message":"body-token","assets":[` + strings.Join(assets, ",") + `]}`))
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, nil)
	_, err := client.Resolve(context.Background(), ResolveRequest{
		Repository: "acme/site", Selector: SelectorLatest, AssetName: "dist.zip",
	})
	if !errors.Is(err, errAssetMissing) {
		t.Fatalf("Resolve() error = %v", err)
	}
	message := err.Error()
	if !strings.Contains(message, "asset-00.zip") || !strings.Contains(message, "asset-09.zip") {
		t.Fatalf("error misses safe truncated names: %s", message)
	}
	if strings.Contains(message, "asset-10.zip") || strings.Contains(message, "asset-11.zip") || strings.Contains(message, "body-token") {
		t.Fatalf("error leaked/truncation failed: %s", message)
	}
}

func TestResolveHTTPErrorParsesRateLimitWithoutBodyLeak(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 19, 10, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Retry-After", "90")
		writer.Header().Set("X-GitHub-Request-Id", "request-123")
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = writer.Write([]byte(`{"message":"signed_url=https://secret.example/a?token=hidden"}`))
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, func(options *clientOptions) { options.now = func() time.Time { return now } })
	_, err := client.Resolve(context.Background(), ResolveRequest{
		Repository: "acme/site", Selector: SelectorLatest, AssetName: "dist.zip",
	})
	if err == nil || !strings.Contains(err.Error(), "status=429") || !strings.Contains(err.Error(), "request_id=request-123") {
		t.Fatalf("Resolve() error = %v", err)
	}
	if strings.Contains(err.Error(), "secret.example") || strings.Contains(err.Error(), "hidden") {
		t.Fatalf("error leaked body: %s", err)
	}
	retryAt, ok := RetryTime(err)
	if !ok || !retryAt.Equal(now.Add(90*time.Second)) {
		t.Fatalf("RetryTime() = %v, %v", retryAt, ok)
	}
}

func TestDownloadStreamsVerifiesDigestAndCleansUp(t *testing.T) {
	t.Parallel()
	payload := []byte("package bytes")
	digest := sha256.Sum256(payload)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/acme/site/releases/assets/42" {
			t.Errorf("path = %q", request.URL.Path)
		}
		assertHeader(t, request, "Accept", assetAccept)
		assertHeader(t, request, "Accept-Encoding", "identity")
		_, _ = writer.Write(payload)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, nil)
	result, err := client.Download(context.Background(), DownloadRequest{
		Repository: "acme/site",
		Asset: Asset{
			ID: "42", Name: "dist.zip", Digest: "sha256:" + hex.EncodeToString(digest[:]),
		},
		MaxBytes: 1024,
	})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if result.Size != int64(len(payload)) || result.SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("Download() = %+v", result)
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("temp file stat: %v", err)
	}
	if err := result.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if err := result.Cleanup(); err != nil {
		t.Fatalf("second Cleanup() error = %v", err)
	}
}

func TestDownloadFollows302AndStripsCrossHostSensitiveHeaders(t *testing.T) {
	t.Parallel()
	var targetHost string
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		for _, header := range []string{"Authorization", "Cookie", "Proxy-Authorization", "Referer", "If-None-Match", "If-Modified-Since", "X-GitHub-Api-Version"} {
			if value := request.Header.Get(header); value != "" {
				t.Errorf("redirect leaked %s=%q", header, value)
			}
		}
		_, _ = writer.Write([]byte("redirected package"))
	}))
	defer target.Close()
	targetURL, _ := url.Parse(target.URL)
	targetHost = "asset.example.test:" + targetURL.Port()

	api := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", "http://"+targetHost+"/signed/package.zip?token=must-not-leak")
		writer.WriteHeader(http.StatusFound)
	}))
	defer api.Close()
	apiURL, _ := url.Parse(api.URL)
	baseURL := "http://api.example.test:" + apiURL.Port()
	client := newMappedTestClient(t, baseURL, nil)
	result, err := client.Download(context.Background(), DownloadRequest{
		Repository: "acme/site", Asset: Asset{ID: "42", Name: "dist.zip"}, MaxBytes: 1024,
	})
	if err != nil {
		t.Fatalf("Download() redirect error = %v", err)
	}
	if cleanupErr := result.Cleanup(); cleanupErr != nil {
		t.Fatalf("Cleanup() error = %v", cleanupErr)
	}

	request, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/repos/acme/site/releases/assets/42", nil)
	applyAssetHeaders(request)
	request.Header.Set("Authorization", "Bearer secret")
	request.Header.Set("Cookie", "session=secret")
	request.Header.Set("Proxy-Authorization", "proxy-secret")
	request.Header.Set("Referer", "https://secret.example/path?token=x")
	request.Header.Set("If-None-Match", `"secret-etag"`)
	request.Header.Set("If-Modified-Since", time.Now().Format(http.TimeFormat))
	response, err := client.httpClient.Do(request)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	_ = response.Body.Close()
}

func TestRedirectSSRFAndDNSRebindingAreRejectedWithoutURLLeak(t *testing.T) {
	t.Parallel()
	t.Run("literal private redirect", func(t *testing.T) {
		api := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Location", "http://127.0.0.1/private?token=secret-query")
			writer.WriteHeader(http.StatusFound)
		}))
		defer api.Close()
		client := newTestClient(t, api.URL, nil)
		_, err := client.Download(context.Background(), DownloadRequest{
			Repository: "acme/site", Asset: Asset{ID: "1", Name: "dist.zip"}, MaxBytes: 100,
		})
		if err == nil || strings.Contains(err.Error(), "secret-query") || strings.Contains(err.Error(), "127.0.0.1") {
			t.Fatalf("Download() error = %v", err)
		}
	})

	t.Run("DNS rebind between redirect and dial", func(t *testing.T) {
		api := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Location", "http://rebind.example.test/package.zip")
			writer.WriteHeader(http.StatusFound)
		}))
		defer api.Close()
		var lock sync.Mutex
		calls := map[string]int{}
		resolve := resolverFunc(func(_ context.Context, _ string, host string) ([]netip.Addr, error) {
			lock.Lock()
			defer lock.Unlock()
			calls[host]++
			if host == "rebind.example.test" && calls[host] > 1 {
				return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
			}
			return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
		})
		client := newTestClient(t, api.URL, func(options *clientOptions) { options.resolver = resolve })
		_, err := client.Download(context.Background(), DownloadRequest{
			Repository: "acme/site", Asset: Asset{ID: "1", Name: "dist.zip"}, MaxBytes: 100,
		})
		if err == nil {
			t.Fatal("Download() error = nil")
		}
		lock.Lock()
		defer lock.Unlock()
		if calls["rebind.example.test"] != 2 {
			t.Fatalf("rebind lookup calls = %d", calls["rebind.example.test"])
		}
	})
}

func TestDownloadFailureRemovesTemporaryFile(t *testing.T) {
	t.Parallel()
	payload := []byte("package bytes")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(payload)
	}))
	defer server.Close()
	tempDir := t.TempDir()
	client := newTestClient(t, server.URL, func(options *clientOptions) {
		options.createTemp = func(_ string, pattern string) (*os.File, error) {
			return os.CreateTemp(tempDir, pattern)
		}
	})
	_, err := client.Download(context.Background(), DownloadRequest{
		Repository: "acme/site",
		Asset: Asset{
			ID: "42", Name: "dist.zip", Digest: "sha256:" + strings.Repeat("0", 64),
		},
		MaxBytes: 1024,
	})
	if !errors.Is(err, errDigest) {
		t.Fatalf("Download() error = %v", err)
	}
	files, readErr := filepath.Glob(filepath.Join(tempDir, "*"))
	if readErr != nil || len(files) != 0 {
		t.Fatalf("temporary files after failure = %v, err=%v", files, readErr)
	}
}

func TestResolveRejectsInvalidRepositoryAndAssetWithoutRequest(t *testing.T) {
	t.Parallel()
	client := NewClient()
	for _, request := range []ResolveRequest{
		{Repository: "https://github.com/acme/site", Selector: SelectorLatest, AssetName: "dist.zip"},
		{Repository: "acme/site/extra", Selector: SelectorLatest, AssetName: "dist.zip"},
		{Repository: "acme/site", Selector: SelectorLatest, AssetName: "../dist.zip"},
		{Repository: "acme/site", Selector: SelectorLatest, AssetName: `dir\dist.zip`},
		{Repository: "acme/site", Selector: SelectorLatest, AssetName: string([]byte{'d', 'i', 's', 't', 0xff})},
		{Repository: "acme/site", Selector: SelectorLatest, AssetName: "dist\nsecret.zip"},
		{Repository: "acme/site", Selector: SelectorLatest, AssetName: "dist\u2028secret.zip"},
		{Repository: "acme/site", Selector: SelectorLatest, AssetName: "dist\u202esecret.zip"},
		{Repository: "acme/site", Selector: SelectorTag, AssetName: "dist.zip"},
	} {
		_, err := client.Resolve(context.Background(), request)
		if !errors.Is(err, errInvalidRequest) {
			t.Errorf("Resolve(%+v) error = %v", request, err)
		}
	}
}

func TestResolveAndDownloadAssetNameWithDelimiters(t *testing.T) {
	t.Parallel()
	assetName := "dist?channel=stable#1&x.zip"
	payload := []byte("package with delimiter name")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/site/releases/latest":
			if request.Header.Get("If-None-Match") == "missing" {
				_, _ = writer.Write([]byte(`{"id":1,"tag_name":"v1","assets":[]}`))
				return
			}
			_, _ = fmt.Fprintf(writer, `{"id":1,"tag_name":"release/v1","assets":[{"id":42,"name":%q,"state":"uploaded","size":%d}]}`,
				assetName, len(payload))
		case "/repos/acme/site/releases/assets/42":
			_, _ = writer.Write(payload)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, nil)

	resolved, err := client.Resolve(context.Background(), ResolveRequest{
		Repository: "acme/site", Selector: SelectorLatest, AssetName: assetName,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Asset.Name != assetName || resolved.Release.Tag != "release/v1" {
		t.Fatalf("Resolve() = %+v", resolved)
	}
	download, err := client.Download(context.Background(), DownloadRequest{
		Repository: "acme/site", Asset: resolved.Asset, MaxBytes: 1024,
	})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if cleanupErr := download.Cleanup(); cleanupErr != nil {
		t.Fatalf("Cleanup() error = %v", cleanupErr)
	}

	_, err = client.Resolve(context.Background(), ResolveRequest{
		Repository: "acme/site", Selector: SelectorLatest, AssetName: assetName, ETag: "missing",
	})
	if !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("missing Resolve() error = %v", err)
	}
	if strings.Contains(err.Error(), assetName) || strings.Contains(err.Error(), "channel=stable") {
		t.Fatalf("missing error leaked delimiter-bearing name: %v", err)
	}
}

func TestFixedTagGitRefRulesAndEscaping(t *testing.T) {
	t.Parallel()
	valid := []string{"@", "release/v1#stable&channel=prod", "foo.LOCK", "中文/发布=稳定"}
	for _, tag := range valid {
		if !validTag(tag) {
			t.Errorf("validTag(%q) = false", tag)
		}
	}
	invalid := []string{
		"", "release v1", "release~v1", "release^v1", "release:v1", "release?v1", "release*v1",
		"release[v1", `release\v1`, "release..v1", "release@{v1", "release//v1", "/release", "release/",
		"release.", ".release", "release/.candidate", "release.lock", "release/v1.lock", "release\nsecret",
		"release\u2028secret", "release\u202esecret", string([]byte{'v', '1', 0xff}), strings.Repeat("a", 256),
	}
	for _, tag := range invalid {
		if validTag(tag) {
			t.Errorf("validTag(%q) = true", tag)
		}
	}

	tag := "release/v1#stable&channel=prod"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		wantURI := "/repos/acme/site/releases/tags/" + url.PathEscape(tag)
		if request.RequestURI != wantURI || request.URL.RawQuery != "" || request.URL.Fragment != "" {
			t.Errorf("tag request = %q query=%q fragment=%q, want %q", request.RequestURI, request.URL.RawQuery, request.URL.Fragment, wantURI)
		}
		_, _ = fmt.Fprintf(writer, `{"id":1,"tag_name":%q,"assets":[{"id":2,"name":"dist.zip","state":"uploaded","size":1}]}`, tag)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, nil)
	result, err := client.Resolve(context.Background(), ResolveRequest{
		Repository: "acme/site", Selector: SelectorTag, Tag: tag, AssetName: "dist.zip",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if result.Release.Tag != tag {
		t.Fatalf("Release.Tag = %q", result.Release.Tag)
	}
}

func TestResolveDoesNotMatchSanitizedRemoteAssetName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		remote    string
		requested string
	}{
		{name: "unicode line separator", remote: "dist\u2028.zip", requested: "dist?.zip"},
		{name: "overlong", remote: strings.Repeat("a", 256), requested: strings.Repeat("a", 255)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintf(writer, `{"id":1,"tag_name":"release/v1","assets":[{"id":2,"name":%q,"state":"uploaded","size":1}]}`, test.remote)
			}))
			defer server.Close()
			client := newTestClient(t, server.URL, nil)
			_, err := client.Resolve(context.Background(), ResolveRequest{
				Repository: "acme/site", Selector: SelectorLatest, AssetName: test.requested,
			})
			if !errors.Is(err, ErrAssetNotFound) {
				t.Fatalf("Resolve() error = %v", err)
			}
		})
	}
}

func TestReleaseDisplayTagValidation(t *testing.T) {
	t.Parallel()
	for _, tag := range []string{"release/v1", "release v1", "release/v1#stable&channel=prod"} {
		if !validReleaseDisplayTag(tag) {
			t.Errorf("validReleaseDisplayTag(%q) = false", tag)
		}
	}
	for _, tag := range []string{"", strings.Repeat("a", 256), "release\nsecret", "release\u2028secret", "release\u202esecret"} {
		if validReleaseDisplayTag(tag) {
			t.Errorf("validReleaseDisplayTag(%q) = true", tag)
		}
	}
}

func TestResolveRejectsInvalidUTF8Metadata(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(append([]byte(`{"id":1,"tag_name":"v1","assets":[{"id":2,"name":"dist`),
			append([]byte{0xff}, []byte(`.zip","state":"uploaded","size":1}]}`)...)...))
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, nil)
	_, err := client.Resolve(context.Background(), ResolveRequest{
		Repository: "acme/site", Selector: SelectorLatest, AssetName: "dist�.zip",
	})
	if !errors.Is(err, ErrMetadata) {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestDownloadRejectsImpossibleMetadataBeforeNetwork(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, nil)
	tests := []struct {
		name  string
		size  int64
		kind  error
		limit int64
	}{
		{name: "negative", size: -1, kind: ErrInvalidRequest, limit: 100},
		{name: "declared too large", size: 101, kind: ErrAssetTooLarge, limit: 100},
	}
	for _, test := range tests {
		_, err := client.Download(context.Background(), DownloadRequest{
			Repository: "acme/site",
			Asset:      Asset{ID: "1", Name: "dist?token=hidden#asset.zip", Size: test.size},
			MaxBytes:   test.limit,
		})
		if !errors.Is(err, test.kind) {
			t.Errorf("%s Download() error = %v", test.name, err)
		}
		if strings.Contains(err.Error(), "token=hidden") {
			t.Errorf("%s error leaked asset name: %v", test.name, err)
		}
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("HTTP requests = %d, want 0", got)
	}
}

func TestLogControlCharactersNeverEnterSafeErrors(t *testing.T) {
	t.Parallel()
	controls := []string{"\u2028", "\u2029", "\u061c", "\u200e", "\u200f", "\u202e", "\u2066", "\u2069"}
	for _, control := range controls {
		secret := "before" + control + "after"
		err := safeError(errInvalidRequest, 0, secret, secret, secret, secret, nil, nil)
		message := err.Error()
		if strings.Contains(message, secret) || strings.Contains(message, control) || strings.Contains(message, "before") {
			t.Errorf("safe error retained control %U: %q", []rune(control)[0], message)
		}
	}
}

func TestResolveRejectsMetadataOverHardLimit(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"id":1,"tag_name":"v1","assets":[]}` + strings.Repeat(" ", maxMetadataBytes)))
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, nil)
	_, err := client.Resolve(context.Background(), ResolveRequest{
		Repository: "acme/site", Selector: SelectorLatest, AssetName: "dist.zip",
	})
	if !errors.Is(err, ErrMetadata) {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestProductionTransportRejectsSelfSignedTLS(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("package"))
	}))
	defer server.Close()
	parsed, _ := url.Parse(server.URL)
	dialer := &net.Dialer{Timeout: time.Second}
	client := newClient(clientOptions{
		baseURL: "https://api.example.test:" + parsed.Port(),
		resolver: resolverFunc(func(_ context.Context, _ string, _ string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
		}),
		dialContext: func(ctx context.Context, network string, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, server.Listener.Addr().String())
		},
		tlsConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
		createTemp:    os.CreateTemp,
		now:           time.Now,
		clientTimeout: 5 * time.Second,
	})
	_, err := client.Download(context.Background(), DownloadRequest{
		Repository: "acme/site", Asset: Asset{ID: "1", Name: "dist.zip"}, MaxBytes: 100,
	})
	if !errors.Is(err, ErrDownload) {
		t.Fatalf("Download() error = %v", err)
	}
	if strings.Contains(err.Error(), "api.example.test") || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("TLS error leaked URL: %v", err)
	}
}

func TestStableErrorClassification(t *testing.T) {
	t.Parallel()
	now := time.Now()
	assetMissing := safeError(errAssetMissing, http.StatusOK, "", "acme/site", "v1", "dist.zip", nil, nil)
	if !IsNotFound(assetMissing) || IsRetryable(assetMissing) {
		t.Fatalf("asset missing classification failed: %v", assetMissing)
	}
	metadata404 := safeError(errMetadata, http.StatusNotFound, "", "acme/site", "v1", "dist.zip", nil, nil)
	if !IsNotFound(metadata404) || IsRetryable(metadata404) {
		t.Fatalf("metadata 404 classification failed: %v", metadata404)
	}
	digest := safeError(errDigest, http.StatusOK, "", "acme/site", "", "dist.zip", nil, nil)
	if !IsDigestError(digest) || IsRetryable(digest) {
		t.Fatalf("digest classification failed: %v", digest)
	}
	for _, retryable := range []error{
		safeError(errMetadata, 0, "", "acme/site", "", "dist.zip", nil, nil),
		safeError(errMetadata, http.StatusOK, "", "acme/site", "", "dist.zip", nil, nil),
		safeError(errDownload, http.StatusOK, "", "acme/site", "", "dist.zip", nil, nil),
		safeError(errMetadata, http.StatusInternalServerError, "", "acme/site", "", "dist.zip", nil, nil),
		safeError(errMetadata, http.StatusForbidden, "", "acme/site", "", "dist.zip", nil, &now),
	} {
		if !IsRetryable(retryable) {
			t.Errorf("IsRetryable(%v) = false", retryable)
		}
	}
}

func TestDownloadRedirectLimitIsSafe(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		step, _ := strconv.Atoi(request.URL.Query().Get("step"))
		writer.Header().Set("Location", fmt.Sprintf("/repos/acme/site/releases/assets/1?step=%d&token=redirect-secret", step+1))
		writer.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, nil)
	_, err := client.Download(context.Background(), DownloadRequest{
		Repository: "acme/site", Asset: Asset{ID: "1", Name: "dist.zip"}, MaxBytes: 100,
	})
	if err == nil {
		t.Fatal("Download() error = nil")
	}
	if strings.Contains(err.Error(), "redirect-secret") || strings.Contains(err.Error(), "step=") {
		t.Fatalf("redirect error leaked Location: %v", err)
	}
}

func TestInvalidRequestDoesNotEchoURLQueryTagOrAsset(t *testing.T) {
	t.Parallel()
	client := NewClient()
	requests := []ResolveRequest{
		{Repository: "https://github.com/acme/site?token=repo-secret", Selector: SelectorLatest, AssetName: "dist.zip"},
		{Repository: "acme/site", Selector: SelectorTag, Tag: "?token=tag-secret", AssetName: "dist.zip"},
		{Repository: "acme/site", Selector: SelectorLatest, AssetName: "../dist.zip?token=asset-secret"},
	}
	for _, request := range requests {
		_, err := client.Resolve(context.Background(), request)
		if err == nil {
			t.Fatalf("Resolve(%+v) error = nil", request)
		}
		for _, secret := range []string{"repo-secret", "tag-secret", "asset-secret", "https://github.com"} {
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("Resolve(%+v) leaked %q: %v", request, secret, err)
			}
		}
	}
}

func newTestClient(t *testing.T, rawBaseURL string, customize func(*clientOptions)) *Client {
	t.Helper()
	parsed, err := url.Parse(rawBaseURL)
	if err != nil {
		t.Fatal(err)
	}
	baseURL := "http://api.example.test:" + parsed.Port()
	return newMappedTestClient(t, baseURL, customize)
}

func newMappedTestClient(t *testing.T, baseURL string, customize func(*clientOptions)) *Client {
	t.Helper()
	resolve := resolverFunc(func(_ context.Context, _ string, _ string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	})
	dialer := &net.Dialer{Timeout: time.Second}
	options := clientOptions{
		baseURL:   baseURL,
		resolver:  resolve,
		allowHTTP: true,
		tlsConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		dialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			_, port, splitErr := net.SplitHostPort(address)
			if splitErr != nil {
				return nil, splitErr
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort("127.0.0.1", port))
		},
		createTemp:    os.CreateTemp,
		now:           time.Now,
		clientTimeout: 5 * time.Second,
	}
	if customize != nil {
		customize(&options)
	}
	return newClient(options)
}

func assertHeader(t *testing.T, request *http.Request, name string, expected string) {
	t.Helper()
	if actual := request.Header.Get(name); actual != expected {
		t.Errorf("%s = %q, want %q", name, actual, expected)
	}
}

func TestResponseRetryAtHTTPDate(t *testing.T) {
	t.Parallel()
	want := time.Date(2026, time.July, 19, 12, 30, 0, 0, time.UTC)
	response := &http.Response{Header: make(http.Header)}
	response.Header.Set("Retry-After", want.Format(http.TimeFormat))
	if got := responseRetryAt(response, time.Time{}); got == nil || !got.Equal(want) {
		t.Fatalf("responseRetryAt() = %v", got)
	}
}

func TestResponseRetryAtRejectsDurationOverflow(t *testing.T) {
	t.Parallel()
	response := &http.Response{Header: make(http.Header)}
	response.Header.Set("Retry-After", strconv.FormatInt(maxRetryAfterSeconds+1, 10))
	if got := responseRetryAt(response, time.Now()); got != nil {
		t.Fatalf("responseRetryAt(overflow) = %v", got)
	}
}

func TestSafeETagDropsOversizedOrControlValue(t *testing.T) {
	t.Parallel()
	if got := safeETag(strings.Repeat("x", 513)); got != "" {
		t.Fatalf("safeETag(overlong) = %q", got)
	}
	if got := safeETag("ok\nsecret"); got != "" {
		t.Fatalf("safeETag(control) = %q", got)
	}
}

func TestDownloadSizeLimit(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Length", strconv.Itoa(20))
		_, _ = writer.Write([]byte(strings.Repeat("x", 20)))
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, nil)
	_, err := client.Download(context.Background(), DownloadRequest{
		Repository: "acme/site", Asset: Asset{ID: "1", Name: "dist.zip"}, MaxBytes: 10,
	})
	if !errors.Is(err, errTooLarge) {
		t.Fatalf("Download() error = %v", err)
	}
}
