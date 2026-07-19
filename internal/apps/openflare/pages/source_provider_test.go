// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

type remoteSourceResolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (function remoteSourceResolverFunc) LookupNetIP(
	ctx context.Context,
	network string,
	host string,
) ([]netip.Addr, error) {
	return function(ctx, network, host)
}

func TestFetchRemoteSourceTrustedInternalSelfSignedAndSafeLabel(t *testing.T) {
	packageBytes := makeRemoteSourceZIP(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("token") != "source-secret" {
			t.Error("signed query did not reach the artifact server")
		}
		if request.Header.Get("Accept-Encoding") != "identity" {
			t.Error("artifact request must disable automatic HTTP decompression")
		}
		writer.Header().Set("Content-Disposition", `attachment; filename="redirected.tar.gz"`)
		_, _ = writer.Write(packageBytes)
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	t.Cleanup(server.Close)

	candidate, err := FetchRemoteSource(t.Context(), RemoteSourceRequest{
		URL:             server.URL + "/original/site.zip?token=source-secret",
		NetworkPolicy:   RemoteNetworkPolicyTrustedInternal,
		MaxPackageBytes: int64(len(packageBytes) + 1),
	})
	if err != nil {
		t.Fatalf("FetchRemoteSource() error = %v", err)
	}
	if candidate.Format != "zip" {
		t.Fatalf("Format = %q, want zip", candidate.Format)
	}
	if candidate.SafeLabel != "site.zip" {
		t.Fatalf("SafeLabel = %q, want original path basename", candidate.SafeLabel)
	}
	if candidate.PackageSize != int64(len(packageBytes)) {
		t.Fatalf("PackageSize = %d, want %d", candidate.PackageSize, len(packageBytes))
	}
	wantChecksum := sha256.Sum256(packageBytes)
	if candidate.Checksum != hex.EncodeToString(wantChecksum[:]) {
		t.Fatalf("Checksum = %q, want SHA-256", candidate.Checksum)
	}
	downloaded, err := os.ReadFile(candidate.TempPath) //nolint:gosec // provider-owned test temp file
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Equal(downloaded, packageBytes) {
		t.Fatal("downloaded package differs from response body")
	}
	tempPath := candidate.TempPath
	if err := candidate.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if err := candidate.Cleanup(); err != nil {
		t.Fatalf("second Cleanup() error = %v", err)
	}
	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file still exists: %v", err)
	}
}

func TestFetchRemoteSourceKeepsOriginalLabelAcrossRedirect(t *testing.T) {
	packageBytes := makeRemoteSourceZIP(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/original/site.zip" {
			writer.Header().Set("Location", "/delivery/final.tar.gz?token=redirect-secret")
			writer.WriteHeader(http.StatusFound)
			return
		}
		if request.Header.Get("Referer") != "" {
			t.Error("redirect must not forward a signed source URL as Referer")
		}
		writer.Header().Set("Content-Disposition", `attachment; filename="response.7z"`)
		_, _ = writer.Write(packageBytes)
	}))
	t.Cleanup(server.Close)

	candidate, err := FetchRemoteSource(t.Context(), RemoteSourceRequest{
		URL:             server.URL + "/original/site.zip?token=initial-secret",
		NetworkPolicy:   RemoteNetworkPolicyTrustedInternal,
		MaxPackageBytes: int64(len(packageBytes) + 1),
	})
	if err != nil {
		t.Fatalf("FetchRemoteSource() error = %v", err)
	}
	defer func() { _ = candidate.Cleanup() }()
	if candidate.SafeLabel != "site.zip" || candidate.Format != "zip" {
		t.Fatalf("candidate = label %q format %q, want original site.zip", candidate.SafeLabel, candidate.Format)
	}
}

func TestFetchRemoteSourcePublicRejectsNonPublicAddresses(t *testing.T) {
	tests := []string{
		"http://127.0.0.1/site.zip?token=loopback-secret",
		"http://[::1]/site.zip?token=ipv6-secret",
		"http://100.64.0.1/site.zip?token=cgnat-secret",
		"http://192.0.2.1/site.zip?token=documentation-secret",
	}
	for _, rawURL := range tests {
		t.Run(rawURL, func(t *testing.T) {
			_, err := FetchRemoteSource(t.Context(), RemoteSourceRequest{
				URL:             rawURL,
				NetworkPolicy:   RemoteNetworkPolicyPublic,
				MaxPackageBytes: 1024,
			})
			if !errors.Is(err, errRemoteProviderBlockedAddress) {
				t.Fatalf("FetchRemoteSource() error = %v, want blocked address", err)
			}
			assertRemoteSourceErrorRedacted(t, err, rawURL, "secret", "token=")
		})
	}
}

func TestFetchRemoteSourcePublicDialsValidatedIP(t *testing.T) {
	packageBytes := makeRemoteSourceZIP(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(packageBytes)
	}))
	t.Cleanup(server.Close)

	var dialedAddress string
	dependencies := mappedRemoteSourceDependencies(server.Listener.Addr().String(), staticPublicRemoteSourceResolver())
	dependencies.dialContext = func(ctx context.Context, network string, address string) (net.Conn, error) {
		dialedAddress = address
		dialer := &net.Dialer{}
		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}
	candidate, err := fetchRemoteSource(t.Context(), RemoteSourceRequest{
		URL:             "http://artifact.example/site.zip",
		NetworkPolicy:   RemoteNetworkPolicyPublic,
		MaxPackageBytes: int64(len(packageBytes) + 1),
	}, dependencies)
	if err != nil {
		t.Fatalf("fetchRemoteSource() error = %v", err)
	}
	defer func() { _ = candidate.Cleanup() }()
	if dialedAddress != "93.184.216.34:80" {
		t.Fatalf("direct dial address = %q, want validated IP", dialedAddress)
	}
}

func TestFetchRemoteSourcePublicRejectsPrivateRedirect(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		writer.Header().Set("Location", "http://127.0.0.1/private.zip?token=redirect-secret")
		writer.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(server.Close)

	dependencies := mappedRemoteSourceDependencies(server.Listener.Addr().String(), staticPublicRemoteSourceResolver())
	rawURL := "http://artifact.example/start.zip?token=initial-secret"
	_, err := fetchRemoteSource(t.Context(), RemoteSourceRequest{
		URL:             rawURL,
		NetworkPolicy:   RemoteNetworkPolicyPublic,
		MaxPackageBytes: 1024,
	}, dependencies)
	if !errors.Is(err, errRemoteProviderBlockedAddress) {
		t.Fatalf("fetchRemoteSource() error = %v, want blocked redirect", err)
	}
	if requestCount.Load() != 1 {
		t.Fatalf("request count = %d, private redirect must not be requested", requestCount.Load())
	}
	assertRemoteSourceErrorRedacted(t, err, rawURL, "initial-secret", "redirect-secret", "token=")
}

func TestFetchRemoteSourcePublicRejectsDNSRebinding(t *testing.T) {
	var lookupCount atomic.Int32
	resolver := remoteSourceResolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		if lookupCount.Add(1) == 1 {
			return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
		}
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	})
	var dialCount atomic.Int32
	dependencies := remoteSourceDependencies{
		resolver: resolver,
		dialContext: func(context.Context, string, string) (net.Conn, error) {
			dialCount.Add(1)
			return nil, errors.New("unexpected dial")
		},
		createTemp: os.CreateTemp,
	}
	rawURL := "http://rebind.example/site.zip?signature=dns-secret"
	_, err := fetchRemoteSource(t.Context(), RemoteSourceRequest{
		URL:             rawURL,
		NetworkPolicy:   RemoteNetworkPolicyPublic,
		MaxPackageBytes: 1024,
	}, dependencies)
	if !errors.Is(err, errRemoteProviderBlockedAddress) {
		t.Fatalf("fetchRemoteSource() error = %v, want DNS rebinding rejection", err)
	}
	if lookupCount.Load() != 2 {
		t.Fatalf("DNS lookup count = %d, want preflight plus dial validation", lookupCount.Load())
	}
	if dialCount.Load() != 0 {
		t.Fatalf("direct dial count = %d, rebound address must not be dialed", dialCount.Load())
	}
	assertRemoteSourceErrorRedacted(t, err, rawURL, "dns-secret", "signature=")
}

func TestFetchRemoteSourcePublicRejectsSelfSignedTLS(t *testing.T) {
	packageBytes := makeRemoteSourceZIP(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(packageBytes)
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	t.Cleanup(server.Close)

	dependencies := mappedRemoteSourceDependencies(server.Listener.Addr().String(), staticPublicRemoteSourceResolver())
	rawURL := "https://artifact.example/site.zip?signature=tls-secret"
	_, err := fetchRemoteSource(t.Context(), RemoteSourceRequest{
		URL:             rawURL,
		NetworkPolicy:   RemoteNetworkPolicyPublic,
		MaxPackageBytes: int64(len(packageBytes) + 1),
	}, dependencies)
	if !errors.Is(err, errRemoteProviderDownloadFailed) {
		t.Fatalf("fetchRemoteSource() error = %v, want strict TLS failure", err)
	}
	assertRemoteSourceErrorRedacted(t, err, rawURL, "tls-secret", "signature=")
}

func TestFetchRemoteSourceRejectsChunkedBodyOverLimitAndCleansTemp(t *testing.T) {
	const maxPackageBytes = int64(64)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(bytes.Repeat([]byte{'x'}, int(maxPackageBytes)))
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = writer.Write([]byte("overflow"))
	}))
	t.Cleanup(server.Close)

	tempDir := t.TempDir()
	dependencies := defaultRemoteSourceDependenciesForTest()
	dependencies.createTemp = func(_ string, pattern string) (*os.File, error) {
		return os.CreateTemp(tempDir, pattern)
	}
	rawURL := server.URL + "/site.zip?token=chunk-secret"
	_, err := fetchRemoteSource(t.Context(), RemoteSourceRequest{
		URL:             rawURL,
		NetworkPolicy:   RemoteNetworkPolicyTrustedInternal,
		MaxPackageBytes: maxPackageBytes,
	}, dependencies)
	if !errors.Is(err, errRemoteProviderTooLarge) {
		t.Fatalf("fetchRemoteSource() error = %v, want actual stream limit", err)
	}
	assertRemoteSourceTempDirEmpty(t, tempDir)
	assertRemoteSourceErrorRedacted(t, err, rawURL, "chunk-secret", "token=")
}

func TestFetchRemoteSourceRejectsContentLengthBeforeCreatingTemp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Length", "4096")
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	var createCount atomic.Int32
	dependencies := defaultRemoteSourceDependenciesForTest()
	dependencies.createTemp = func(directory string, pattern string) (*os.File, error) {
		createCount.Add(1)
		return os.CreateTemp(directory, pattern)
	}
	_, err := fetchRemoteSource(t.Context(), RemoteSourceRequest{
		URL:             server.URL + "/site.zip",
		NetworkPolicy:   RemoteNetworkPolicyTrustedInternal,
		MaxPackageBytes: 1024,
	}, dependencies)
	if !errors.Is(err, errRemoteProviderTooLarge) {
		t.Fatalf("fetchRemoteSource() error = %v, want Content-Length rejection", err)
	}
	if createCount.Load() != 0 {
		t.Fatalf("CreateTemp called %d times before Content-Length rejection", createCount.Load())
	}
}

func TestFetchRemoteSourceSniffsAtLeast512BytesForTar(t *testing.T) {
	packageBytes := make([]byte, remoteSourceMagicSniffBytes)
	copy(packageBytes[257:], []byte("ustar"))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(packageBytes)
	}))
	t.Cleanup(server.Close)

	candidate, err := FetchRemoteSource(t.Context(), RemoteSourceRequest{
		URL:             server.URL + "/download",
		NetworkPolicy:   RemoteNetworkPolicyTrustedInternal,
		MaxPackageBytes: int64(len(packageBytes) + 1),
	})
	if err != nil {
		t.Fatalf("FetchRemoteSource() error = %v", err)
	}
	defer func() { _ = candidate.Cleanup() }()
	if candidate.Format != "tar" {
		t.Fatalf("Format = %q, want tar detected at byte 257", candidate.Format)
	}
	if candidate.SafeLabel != "download.tar" {
		t.Fatalf("SafeLabel = %q, want download.tar", candidate.SafeLabel)
	}
}

func TestFetchRemoteSourceRedactsURLHeadersAndBodyFromErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("X-Artifact-Secret", "header-secret")
		_, _ = writer.Write([]byte("response-body-secret"))
	}))
	t.Cleanup(server.Close)

	tempDir := t.TempDir()
	dependencies := defaultRemoteSourceDependenciesForTest()
	dependencies.createTemp = func(_ string, pattern string) (*os.File, error) {
		return os.CreateTemp(tempDir, pattern)
	}
	rawURL := server.URL + "/download?token=query-secret"
	_, err := fetchRemoteSource(t.Context(), RemoteSourceRequest{
		URL:             rawURL,
		NetworkPolicy:   RemoteNetworkPolicyTrustedInternal,
		MaxPackageBytes: 1024,
	}, dependencies)
	if !errors.Is(err, errRemoteProviderUnsupported) {
		t.Fatalf("fetchRemoteSource() error = %v, want unsupported archive", err)
	}
	assertRemoteSourceTempDirEmpty(t, tempDir)
	assertRemoteSourceErrorRedacted(
		t,
		err,
		rawURL,
		"query-secret",
		"header-secret",
		"response-body-secret",
		"token=",
	)
}

func TestFetchRemoteSourceAllowsFiveRedirectsOnly(t *testing.T) {
	packageBytes := makeRemoteSourceZIP(t)
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount.Add(1)
		redirectNumber, _ := strconv.Atoi(strings.TrimPrefix(request.URL.Path, "/"))
		if redirectNumber < remoteSourceMaxRedirects+1 {
			writer.Header().Set("Location", "/"+strconv.Itoa(redirectNumber+1))
			writer.WriteHeader(http.StatusFound)
			return
		}
		_, _ = writer.Write(packageBytes)
	}))
	t.Cleanup(server.Close)

	dependencies := mappedRemoteSourceDependencies(server.Listener.Addr().String(), staticPublicRemoteSourceResolver())
	_, err := fetchRemoteSource(t.Context(), RemoteSourceRequest{
		URL:             "http://artifact.example/0",
		NetworkPolicy:   RemoteNetworkPolicyPublic,
		MaxPackageBytes: int64(len(packageBytes) + 1),
	}, dependencies)
	if !errors.Is(err, errRemoteProviderRedirectLimit) {
		t.Fatalf("fetchRemoteSource() error = %v, want redirect limit", err)
	}
	if requestCount.Load() != remoteSourceMaxRedirects+1 {
		t.Fatalf("request count = %d, want initial plus five redirects", requestCount.Load())
	}
}

func staticPublicRemoteSourceResolver() remoteSourceResolver {
	return remoteSourceResolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	})
}

func defaultRemoteSourceDependenciesForTest() remoteSourceDependencies {
	dialer := &net.Dialer{}
	return remoteSourceDependencies{
		resolver:    net.DefaultResolver,
		dialContext: dialer.DialContext,
		createTemp:  os.CreateTemp,
	}
}

func mappedRemoteSourceDependencies(targetAddress string, resolver remoteSourceResolver) remoteSourceDependencies {
	dialer := &net.Dialer{}
	return remoteSourceDependencies{
		resolver: resolver,
		dialContext: func(ctx context.Context, network string, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, targetAddress)
		},
		createTemp: os.CreateTemp,
	}
}

func makeRemoteSourceZIP(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	file, err := archive.Create("index.html")
	if err != nil {
		t.Fatalf("zip.Create() error = %v", err)
	}
	if _, err := file.Write([]byte("<h1>OpenFlare</h1>")); err != nil {
		t.Fatalf("zip entry Write() error = %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("zip.Close() error = %v", err)
	}
	return buffer.Bytes()
}

func assertRemoteSourceTempDirEmpty(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary directory contains %d leaked files", len(entries))
	}
}

func assertRemoteSourceErrorRedacted(t *testing.T, err error, sensitiveValues ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	message := err.Error()
	for _, sensitiveValue := range sensitiveValues {
		if sensitiveValue != "" && strings.Contains(message, sensitiveValue) {
			t.Fatalf("error %q contains sensitive value %q", message, sensitiveValue)
		}
	}
}
