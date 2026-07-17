// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pagesarchive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ulikunitz/xz"
)

func TestDetectFormatFromName(t *testing.T) {
	cases := map[string]Format{
		"site.zip":     FormatZip,
		"site.TAR.GZ":  FormatTarGz,
		"site.tgz":     FormatTarGz,
		"site.tar.xz":  FormatTarXz,
		"site.txz":     FormatTarXz,
		"site.tar.bz2": FormatTarBz2,
		"site.tar":     FormatTar,
		"site.7z":      FormatSevenZip,
	}
	for name, want := range cases {
		got, ok := DetectFormatFromName(name)
		assert.True(t, ok, name)
		assert.Equal(t, want, got, name)
	}
	_, ok := DetectFormatFromName("site.rar")
	assert.False(t, ok)
}

func TestInspectAndExtractZip(t *testing.T) {
	data := testZip(t, map[string]string{
		"dist/index.html": "<html>ok</html>",
		"dist/app.js":     "console.log(1)",
	})
	manifest, err := InspectBytes(data, FormatZip, InspectOptions{
		EntryFile: "index.html",
		Limits:    Limits{MaxFiles: 100, MaxFileBytes: 1 << 20, MaxTotalBytes: 1 << 20},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, manifest.FileCount)
	paths := make(map[string]struct{}, len(manifest.Files))
	for _, file := range manifest.Files {
		paths[file.Path] = struct{}{}
		assert.Empty(t, file.Checksum, "per-file checksum should not be computed")
		assert.Positive(t, file.Size)
	}
	assert.Contains(t, paths, "index.html")
	assert.Contains(t, paths, "app.js")
	assert.Equal(t, int64(len("<html>ok</html>")+len("console.log(1)")), manifest.TotalSize)

	dest := t.TempDir()
	require.NoError(t, ExtractBytes(data, FormatZip, dest, ExtractOptions{
		StripCommonRoot: true,
		EnforceLimits:   true,
		Limits:          Limits{MaxFiles: 100, MaxFileBytes: 1 << 20, MaxTotalBytes: 1 << 20},
	}))
	body, err := os.ReadFile(filepath.Join(dest, "index.html")) //nolint:gosec
	require.NoError(t, err)
	assert.Equal(t, "<html>ok</html>", string(body))
}

func TestInspectFileUsesDeclaredSizesWithoutHash(t *testing.T) {
	data := testZip(t, map[string]string{
		"index.html": "<html>disk</html>",
		"asset.css":  "body{}",
	})
	path := filepath.Join(t.TempDir(), "site.zip")
	require.NoError(t, os.WriteFile(path, data, 0o600))

	manifest, err := InspectFile(path, FormatZip, InspectOptions{
		EntryFile: "index.html",
		Limits:    Limits{MaxFiles: 100, MaxFileBytes: 1 << 20, MaxTotalBytes: 1 << 20},
	})
	require.NoError(t, err)
	require.Equal(t, 2, manifest.FileCount)
	for _, file := range manifest.Files {
		assert.Empty(t, file.Checksum)
	}

	dest := t.TempDir()
	require.NoError(t, ExtractFile(path, FormatZip, dest, ExtractOptions{
		EnforceLimits: true,
		Limits:        Limits{MaxFiles: 100, MaxFileBytes: 1 << 20, MaxTotalBytes: 1 << 20},
	}))
	body, err := os.ReadFile(filepath.Join(dest, "index.html")) //nolint:gosec
	require.NoError(t, err)
	assert.Equal(t, "<html>disk</html>", string(body))
}

func TestInspectVerifySizesOptional(t *testing.T) {
	data := testZip(t, map[string]string{
		"index.html": "verify-me",
	})
	manifest, err := InspectBytes(data, FormatZip, InspectOptions{
		EntryFile:   "index.html",
		VerifySizes: true,
		Limits:      Limits{MaxFiles: 10, MaxFileBytes: 1 << 20, MaxTotalBytes: 1 << 20},
	})
	require.NoError(t, err)
	require.Len(t, manifest.Files, 1)
	assert.Equal(t, int64(len("verify-me")), manifest.Files[0].Size)
	assert.Empty(t, manifest.Files[0].Checksum)
}

func TestExtractTrustedSkipsSizeLimits(t *testing.T) {
	// Content larger than a tiny limit would fail if limits were enforced.
	large := strings.Repeat("x", 64)
	data := testZip(t, map[string]string{
		"index.html": large,
	})
	dest := t.TempDir()
	require.NoError(t, ExtractBytes(data, FormatZip, dest, ExtractOptions{
		// Agent trusts control-plane validation: no size/count re-check.
		EnforceLimits: false,
	}))
	body, err := os.ReadFile(filepath.Join(dest, "index.html")) //nolint:gosec
	require.NoError(t, err)
	assert.Equal(t, large, string(body))
}

func TestInspectAndExtractTarGz(t *testing.T) {
	data := testTarGz(t, map[string]string{
		"index.html": "<html>tar</html>",
		"style.css":  "body{}",
	})
	format, err := DetectFormat("site.tar.gz", data)
	require.NoError(t, err)
	assert.Equal(t, FormatTarGz, format)

	manifest, err := InspectBytes(data, format, InspectOptions{
		EntryFile: "index.html",
		Limits:    Limits{MaxFiles: 100, MaxFileBytes: 1 << 20, MaxTotalBytes: 1 << 20},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, manifest.FileCount)

	dest := t.TempDir()
	require.NoError(t, ExtractBytes(data, format, dest, ExtractOptions{
		EnforceLimits: true,
		Limits:        Limits{MaxFiles: 100, MaxFileBytes: 1 << 20, MaxTotalBytes: 1 << 20},
	}))
	body, err := os.ReadFile(filepath.Join(dest, "index.html")) //nolint:gosec
	require.NoError(t, err)
	assert.Equal(t, "<html>tar</html>", string(body))
}

func TestInspectTarXz(t *testing.T) {
	data := testTarXz(t, map[string]string{
		"index.html": "<html>xz</html>",
	})
	manifest, err := InspectBytes(data, FormatTarXz, InspectOptions{
		EntryFile: "index.html",
		Limits:    Limits{MaxFiles: 10, MaxFileBytes: 1 << 20, MaxTotalBytes: 1 << 20},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, manifest.FileCount)
}

func TestRejectZipSlip(t *testing.T) {
	data := testZip(t, map[string]string{
		"../evil.txt": "x",
		"index.html":  "ok",
	})
	_, err := InspectBytes(data, FormatZip, InspectOptions{
		EntryFile: "index.html",
		Limits:    Limits{MaxFiles: 10, MaxFileBytes: 1 << 20, MaxTotalBytes: 1 << 20},
	})
	require.Error(t, err)
}

func testZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		file, err := writer.Create(name)
		require.NoError(t, err)
		_, err = file.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func testTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzWriter)
	for name, content := range files {
		require.NoError(t, tarWriter.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}))
		_, err := tarWriter.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzWriter.Close())
	return buffer.Bytes()
}

func testTarXz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	xzWriter, err := xz.NewWriter(&buffer)
	require.NoError(t, err)
	tarWriter := tar.NewWriter(xzWriter)
	for name, content := range files {
		require.NoError(t, tarWriter.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}))
		_, err := tarWriter.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tarWriter.Close())
	require.NoError(t, xzWriter.Close())
	return buffer.Bytes()
}
