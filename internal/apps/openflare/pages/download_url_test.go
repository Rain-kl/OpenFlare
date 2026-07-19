// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAndValidatePagesDownloadURL(t *testing.T) {
	_, err := parseAndValidatePagesDownloadURL("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "填写")

	_, err = parseAndValidatePagesDownloadURL("ftp://example.com/a.zip")
	require.Error(t, err)

	// Private / local hosts are allowed (operator-controlled artifact hosts).
	for _, raw := range []string{
		"http://127.0.0.1/a.zip",
		"https://localhost/a.zip",
		"http://192.168.1.10:8080/site.tar.gz",
		"https://example.com/dist/site.tar.gz?x=1",
	} {
		parsed, parseErr := parseAndValidatePagesDownloadURL(raw)
		require.NoError(t, parseErr, raw)
		assert.NotEmpty(t, parsed.Hostname(), raw)
	}
}

func TestDownloadPagesPackageFromURLAllowsPrivateHost(t *testing.T) {
	var body bytes.Buffer
	zw := zip.NewWriter(&body)
	w, err := zw.Create("index.html")
	require.NoError(t, err)
	_, err = w.Write([]byte("ok"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	zipBytes := body.Bytes()

	var sawProviderUA bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == remoteSourceUserAgent {
			sawProviderUA = true
		}
		w.Header().Set("Content-Disposition", `attachment; filename="remote-site.zip"`)
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(zipBytes)
	}))
	t.Cleanup(server.Close)

	tempPath, checksum, size, format, fileName, err := downloadPagesPackageFromURL(
		context.Background(),
		server.URL+"/pkg.zip",
		10*1024*1024,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(tempPath) })
	assert.True(t, sawProviderUA)
	assert.NotEmpty(t, checksum)
	assert.Positive(t, size)
	assert.Equal(t, "zip", string(format))
	assert.Equal(t, "pkg.zip", fileName)
}

func TestUploadDeploymentFromURLPrivateHost(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	_, disableStorage := setupPagesStorageMock(t)
	defer disableStorage()
	ctx := context.Background()

	var body bytes.Buffer
	zw := zip.NewWriter(&body)
	w, err := zw.Create("index.html")
	require.NoError(t, err)
	_, err = w.Write([]byte("from-url"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="from-url.zip"`)
		_, _ = w.Write(body.Bytes())
	}))
	t.Cleanup(server.Close)

	project, err := CreateProject(ctx, Input{Name: "URL Site", Slug: "url-site", Enabled: true})
	require.NoError(t, err)

	deployment, err := UploadDeploymentFromURL(ctx, project.ID, server.URL+"/from-url.zip", "root")
	require.NoError(t, err)
	assert.NotZero(t, deployment.ID)
	assert.Equal(t, 1, deployment.FileCount)
}
