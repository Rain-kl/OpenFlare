// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pagesarchive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testLimits = Limits{
	MaxFiles:      100,
	MaxFileBytes:  1 << 20,
	MaxTotalBytes: 1 << 20,
}

func TestNormalizeLogicalPathStrict(t *testing.T) {
	t.Parallel()

	valid := []struct {
		name       string
		raw        string
		allowEmpty bool
		want       string
	}{
		{name: "empty root", allowEmpty: true},
		{name: "single file", raw: "index.html", want: "index.html"},
		{name: "nested posix", raw: "public/assets/app.js", want: "public/assets/app.js"},
		{name: "unicode", raw: "静态/首页.html", want: "静态/首页.html"},
		{name: "repeated separator is normalized", raw: "public//app.js", want: "public/app.js"},
	}
	for _, tt := range valid {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeLogicalPath(tt.raw, tt.allowEmpty)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	invalidUTF8 := string([]byte{'a', '/', 0xff})
	invalid := []struct {
		name string
		raw  string
	}{
		{name: "empty entry"},
		{name: "absolute", raw: "/etc/passwd"},
		{name: "unc", raw: "//server/share"},
		{name: "windows drive", raw: "C:/site/index.html"},
		{name: "nested windows drive", raw: "site/C:/index.html"},
		{name: "windows separator", raw: `site\index.html`},
		{name: "windows unc", raw: `\\server\share`},
		{name: "parent segment", raw: "../index.html"},
		{name: "nested parent segment", raw: "site/../index.html"},
		{name: "current segment", raw: "site/./index.html"},
		{name: "nul", raw: "site/\x00index.html"},
		{name: "newline", raw: "site/\nindex.html"},
		{name: "delete control", raw: "site/\x7findex.html"},
		{name: "single quote", raw: "site/'index.html"},
		{name: "double quote", raw: `site/"index.html`},
		{name: "semicolon", raw: "site/;index.html"},
		{name: "invalid utf8", raw: invalidUTF8},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizeLogicalPath(tt.raw, false)
			require.Error(t, err)
		})
	}

	cleaned, skip, err := NormalizeEntryPath("")
	require.NoError(t, err)
	assert.Empty(t, cleaned)
	assert.True(t, skip)

	cleaned, skip, err = NormalizeEntryPath("assets/")
	require.NoError(t, err)
	assert.Equal(t, "assets", cleaned)
	assert.True(t, skip)
}

func TestSupportedFormatsInspectAndExtract(t *testing.T) {
	t.Parallel()

	sevenZipData := decodeFixture(t, "N3q8ryccAASgR6WICAAAAAAAAABmAAAAAAAAAN2R8/FiYXIKZm9vCgEEBgACCQQEAAcLAgABAQABAQAMBAQACAoB6bOiBKhlMn4AAAUCGQUAAAAAABERAGIAYQByAAAAZgBvAG8AAAAZAgAAFBIBAACFM3PyY9YBAFgCcvJj1gEVCgEAIICkgSCApIEAAA==")
	bzipTarData := decodeFixture(t, "QlpoOTFBWSZTWYp5f6EAAHV//P64A8RQAf/iOm/9cO/v/9AAAgBADlAABAADAAgwAU1RIZJpNCaammmnqbSGTI9Q0BoBpp6mjIaGmmRoaHGRpkxNBkyYTTIGQ0BoDTJoYATQGG1KCntExT01MhoAABoAAHqAAPU9QacVDtN45fA6MmuGVQlWowrijpZgwASITYSPUcpJpoQGMkKq69jMkUR6L86R5j0IySUaZEjazEqhQ9E8vuuxsmWZQLCA84jNsobYNEzuEB1eCPhw8nc2AOz+xrCY5hVxQW1IIokpfSRKi+McvXU+QoYuEg6BD4w8x3K0imi+bULpkLCylCZ4lzoGlTQgibvG67sQcrTCRBTbBCVL7zC0q0qULmK/WOneu94s9cs4s4K98SjY2YvpdZvl42kwtxvvPMheorYQ2pcxyF4sNQYvd4+bgqm5gKXElqnGF3jhxGTeXp9eCUxWVlbi9ikxAik4xxATl7cJrISVWnHwUFiLdhEnKWw0Lhm3ZyKlX7P5Wj7b9TLAmWBaAwH/F3JFOFCQinl/oQ==")

	cases := []struct {
		name      string
		format    Format
		data      []byte
		entryFile string
		wantPath  string
	}{
		{name: "zip", format: FormatZip, data: testZip(t, map[string]string{"bundle/index.html": "zip"}), entryFile: "index.html", wantPath: "index.html"},
		{name: "tar", format: FormatTar, data: testTar(t, map[string]string{"bundle/index.html": "tar"}), entryFile: "index.html", wantPath: "index.html"},
		{name: "tar gzip", format: FormatTarGz, data: testTarGz(t, map[string]string{"bundle/index.html": "gzip"}), entryFile: "index.html", wantPath: "index.html"},
		{name: "tar xz", format: FormatTarXz, data: testTarXz(t, map[string]string{"bundle/index.html": "xz"}), entryFile: "index.html", wantPath: "index.html"},
		{name: "tar bzip2", format: FormatTarBz2, data: bzipTarData, entryFile: "index.html", wantPath: "index.html"},
		{name: "7z", format: FormatSevenZip, data: sevenZipData, entryFile: "foo", wantPath: "foo"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			manifest, err := InspectBytes(tt.data, tt.format, InspectOptions{
				EntryFile: tt.entryFile,
				Limits:    testLimits,
			})
			require.NoError(t, err)
			assert.Positive(t, manifest.FileCount)
			assertManifestContains(t, manifest, tt.wantPath)

			destDir := t.TempDir()
			require.NoError(t, ExtractBytes(tt.data, tt.format, destDir, ExtractOptions{
				Limits:          testLimits,
				StripCommonRoot: true,
				EnforceLimits:   true,
			}))
			_, err = os.Stat(filepath.Join(destDir, filepath.FromSlash(tt.wantPath)))
			require.NoError(t, err)
		})
	}
}

func TestExtractFilePreservesCommonRootAndEnforcesLimits(t *testing.T) {
	t.Parallel()

	data := testTarGz(t, map[string]string{
		"repository/dist/index.html": "pages",
		"repository/dist/app.js":     "javascript",
	})
	archivePath := filepath.Join(t.TempDir(), "site.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, data, 0o600))

	manifest, err := InspectFile(archivePath, FormatTarGz, InspectOptions{
		RootDir:   "dist",
		EntryFile: "index.html",
		Limits:    testLimits,
	})
	require.NoError(t, err)
	assertManifestContains(t, manifest, "dist/index.html")

	destDir := t.TempDir()
	require.NoError(t, ExtractFile(archivePath, FormatTarGz, destDir, ExtractOptions{
		Limits:          testLimits,
		StripCommonRoot: true,
		EnforceLimits:   true,
	}))
	body, err := os.ReadFile(filepath.Join(destDir, "dist", "index.html")) //nolint:gosec
	require.NoError(t, err)
	assert.Equal(t, "pages", string(body))

	err = ExtractFile(archivePath, FormatTarGz, t.TempDir(), ExtractOptions{
		Limits: Limits{
			MaxFiles:      10,
			MaxFileBytes:  5,
			MaxTotalBytes: 1 << 20,
		},
		StripCommonRoot: true,
		EnforceLimits:   true,
	})
	require.ErrorContains(t, err, "file too large")

	err = ExtractFile(archivePath, FormatTarGz, t.TempDir(), ExtractOptions{
		Limits: Limits{
			MaxFiles:      10,
			MaxFileBytes:  1 << 20,
			MaxTotalBytes: int64(len("pages") + len("javascript") - 1),
		},
		StripCommonRoot: true,
		EnforceLimits:   true,
	})
	require.ErrorContains(t, err, "extracted size exceeds limit")
}

func TestRandomAccessMembersVerifyActualSize(t *testing.T) {
	t.Parallel()

	entry := Entry{
		Name: "index.html",
		Size: 1,
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("actual-body")), nil
		},
	}
	_, err := inspectRandomAccessEntries([]Entry{entry}, testLimits)
	require.ErrorContains(t, err, "declared size 1 does not match actual 11")

	destDir := t.TempDir()
	err = extractEntries([]Entry{entry}, destDir, ExtractOptions{
		Limits:        testLimits,
		EnforceLimits: true,
	})
	require.ErrorContains(t, err, "declared size 1 does not match actual 11")
	_, statErr := os.Stat(filepath.Join(destDir, "index.html"))
	assert.ErrorIs(t, statErr, os.ErrNotExist, "failed extraction must remove the partial file")
}

func TestActualByteLimitAbortsReaderEarly(t *testing.T) {
	t.Parallel()

	reader := &countingFillReader{remaining: 1 << 30}
	entry := Entry{
		Name: "index.html",
		Size: 0,
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(reader), nil
		},
	}
	_, err := inspectRandomAccessEntries([]Entry{entry}, Limits{
		MaxFiles:      1,
		MaxFileBytes:  32,
		MaxTotalBytes: 32,
	})
	require.ErrorContains(t, err, "size out of bounds")
	assert.LessOrEqual(t, reader.read, int64(33), "inspection must stop after limit+1 actual bytes")

	tarData := tarWithDeclaredBodyOnly(t, "index.html", 1<<30)
	_, err = InspectBytes(tarData, FormatTar, InspectOptions{
		EntryFile: "index.html",
		Limits: Limits{
			MaxFiles:      1,
			MaxFileBytes:  32,
			MaxTotalBytes: 32,
		},
	})
	require.ErrorContains(t, err, "file too large")
}

func TestArchiveLimitsUseFilesAndActualTotals(t *testing.T) {
	t.Parallel()

	data := testZip(t, map[string]string{
		"index.html": "1234",
		"app.js":     "5678",
	})
	_, err := InspectBytes(data, FormatZip, InspectOptions{
		EntryFile: "index.html",
		Limits: Limits{
			MaxFiles:      1,
			MaxFileBytes:  8,
			MaxTotalBytes: 16,
		},
	})
	require.ErrorContains(t, err, "file count exceeds")

	_, err = InspectBytes(data, FormatZip, InspectOptions{
		EntryFile: "index.html",
		Limits: Limits{
			MaxFiles:      2,
			MaxFileBytes:  8,
			MaxTotalBytes: 7,
		},
	})
	require.ErrorContains(t, err, "extracted size exceeds limit")

	sevenZipData := decodeFixture(t, "N3q8ryccAASgR6WICAAAAAAAAABmAAAAAAAAAN2R8/FiYXIKZm9vCgEEBgACCQQEAAcLAgABAQABAQAMBAQACAoB6bOiBKhlMn4AAAUCGQUAAAAAABERAGIAYQByAAAAZgBvAG8AAAAZAgAAFBIBAACFM3PyY9YBAFgCcvJj1gEVCgEAIICkgSCApIEAAA==")
	_, err = InspectBytes(sevenZipData, FormatSevenZip, InspectOptions{
		EntryFile: "foo",
		Limits: Limits{
			MaxFiles:      10,
			MaxFileBytes:  3,
			MaxTotalBytes: 32,
		},
	})
	require.ErrorContains(t, err, "file too large")
}

func TestRejectUnsupportedTarMemberTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		header  tar.Header
		wantErr string
	}{
		{name: "symlink", header: tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "index.html"}, wantErr: "unsupported symlink"},
		{name: "hardlink", header: tar.Header{Name: "hard", Typeflag: tar.TypeLink, Linkname: "index.html"}, wantErr: "unsupported hardlink"},
		{name: "fifo", header: tar.Header{Name: "pipe", Typeflag: tar.TypeFifo, Mode: 0o600}, wantErr: "unsupported special entry"},
		{name: "character device", header: tar.Header{Name: "tty", Typeflag: tar.TypeChar, Mode: 0o600, Devmajor: 1, Devminor: 3}, wantErr: "unsupported special entry"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			data := tarWithSpecialEntry(t, &tt.header)
			_, err := InspectBytes(data, FormatTar, InspectOptions{
				EntryFile: "index.html",
				Limits:    testLimits,
			})
			require.ErrorContains(t, err, tt.wantErr)

			destDir := t.TempDir()
			err = ExtractBytes(data, FormatTar, destDir, ExtractOptions{
				Limits:        testLimits,
				EnforceLimits: true,
			})
			require.ErrorContains(t, err, tt.wantErr)
			_, statErr := os.Stat(filepath.Join(destDir, "index.html"))
			assert.ErrorIs(t, statErr, os.ErrNotExist, "tar validation pass must reject before writing files")
		})
	}
}

func TestRejectUnsupportedZipMemberTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		mode    os.FileMode
		wantErr string
	}{
		{name: "symlink", mode: os.ModeSymlink | 0o777, wantErr: "unsupported symlink"},
		{name: "named pipe", mode: os.ModeNamedPipe | 0o600, wantErr: "unsupported special entry"},
		{name: "device", mode: os.ModeDevice | 0o600, wantErr: "unsupported special entry"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			data := zipWithSpecialEntry(t, tt.mode)
			_, err := InspectBytes(data, FormatZip, InspectOptions{
				EntryFile: "index.html",
				Limits:    testLimits,
			})
			require.ErrorContains(t, err, tt.wantErr)

			err = ExtractBytes(data, FormatZip, t.TempDir(), ExtractOptions{
				Limits:        testLimits,
				EnforceLimits: true,
			})
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestTarMetadataHeadersRemainTransparent(t *testing.T) {
	t.Parallel()

	for _, format := range []tar.Format{tar.FormatPAX, tar.FormatGNU} {
		format := format
		t.Run(format.String(), func(t *testing.T) {
			data := tarWithLongMetadata(t, format)
			manifest, err := InspectBytes(data, FormatTar, InspectOptions{
				EntryFile: "index.html",
				Limits:    testLimits,
			})
			require.NoError(t, err)
			assert.Equal(t, 2, manifest.FileCount)
			assertManifestContains(t, manifest, "index.html")

			destDir := t.TempDir()
			require.NoError(t, ExtractBytes(data, FormatTar, destDir, ExtractOptions{
				Limits:        testLimits,
				EnforceLimits: true,
			}))
			_, err = os.Stat(filepath.Join(destDir, "index.html"))
			require.NoError(t, err)
		})
	}
}

type countingFillReader struct {
	remaining int64
	read      int64
}

func (r *countingFillReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	for i := range p {
		p[i] = 'x'
	}
	r.remaining -= int64(len(p))
	r.read += int64(len(p))
	return len(p), nil
}

func decodeFixture(t *testing.T, encoded string) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	return data
}

func assertManifestContains(t *testing.T, manifest *Manifest, path string) {
	t.Helper()
	for _, file := range manifest.Files {
		if file.Path == path {
			return
		}
	}
	require.Failf(t, "manifest path missing", "path %q not found in %#v", path, manifest.Files)
}

func testTar(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for name, content := range files {
		require.NoError(t, writer.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}))
		_, err := writer.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func tarWithDeclaredBodyOnly(t *testing.T, name string, size int64) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	require.NoError(t, writer.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0o644,
		Size: size,
	}))
	// Deliberately omit the body and trailer. The limit must reject from the
	// header before archive/tar attempts to stream the declared body.
	return buffer.Bytes()
}

func tarWithSpecialEntry(t *testing.T, special *tar.Header) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	require.NoError(t, writer.WriteHeader(&tar.Header{
		Name: "index.html",
		Mode: 0o644,
		Size: 2,
	}))
	_, err := writer.Write([]byte("ok"))
	require.NoError(t, err)
	require.NoError(t, writer.WriteHeader(special))
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func zipWithSpecialEntry(t *testing.T, mode os.FileMode) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	index, err := writer.Create("index.html")
	require.NoError(t, err)
	_, err = index.Write([]byte("ok"))
	require.NoError(t, err)

	header := &zip.FileHeader{Name: "special"}
	header.SetMode(mode)
	special, err := writer.CreateHeader(header)
	require.NoError(t, err)
	if mode&os.ModeSymlink != 0 {
		_, err = special.Write([]byte("index.html"))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func tarWithLongMetadata(t *testing.T, format tar.Format) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	longName := strings.Repeat("long-segment-", 12) + "asset.js"
	header := &tar.Header{
		Name:   longName,
		Mode:   0o644,
		Size:   1,
		Format: format,
	}
	if format == tar.FormatPAX {
		header.PAXRecords = map[string]string{"comment": "metadata is not a deployable member"}
	}
	require.NoError(t, writer.WriteHeader(header))
	_, err := writer.Write([]byte("x"))
	require.NoError(t, err)
	require.NoError(t, writer.WriteHeader(&tar.Header{
		Name:   "index.html",
		Mode:   0o644,
		Size:   2,
		Format: format,
	}))
	_, err = writer.Write([]byte("ok"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}
