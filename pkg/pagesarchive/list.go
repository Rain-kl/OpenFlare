// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pagesarchive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"

	"github.com/bodgit/sevenzip"
	"github.com/ulikunitz/xz"
)

type archiveFile interface {
	Name() string
	IsDir() bool
	IsSymlink() bool
	Size() uint64
	Open() (io.ReadCloser, error)
}

type zipArchiveFile struct {
	file *zip.File
}

func (z zipArchiveFile) Name() string { return z.file.Name }
func (z zipArchiveFile) IsDir() bool  { return z.file.FileInfo().IsDir() }
func (z zipArchiveFile) IsSymlink() bool {
	return z.file.Mode()&os.ModeSymlink != 0
}
func (z zipArchiveFile) Size() uint64 { return z.file.UncompressedSize64 }
func (z zipArchiveFile) Open() (io.ReadCloser, error) {
	return z.file.Open()
}

type sevenZipArchiveFile struct {
	file *sevenzip.File
}

func (z sevenZipArchiveFile) Name() string { return z.file.Name }
func (z sevenZipArchiveFile) IsDir() bool  { return z.file.FileInfo().IsDir() }
func (z sevenZipArchiveFile) IsSymlink() bool {
	return z.file.Mode()&os.ModeSymlink != 0
}
func (z sevenZipArchiveFile) Size() uint64 { return z.file.UncompressedSize }
func (z sevenZipArchiveFile) Open() (io.ReadCloser, error) {
	return z.file.Open()
}

func listEntries(data []byte, format Format) ([]Entry, error) {
	switch format {
	case FormatZip:
		return listZipEntries(data)
	case FormatTar:
		return listTarEntries(bytes.NewReader(data))
	case FormatTarGz:
		gzReader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("open gzip pages package: %w", err)
		}
		defer func() { _ = gzReader.Close() }()
		return listTarEntries(gzReader)
	case FormatTarXz:
		xzReader, err := xz.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("open xz pages package: %w", err)
		}
		return listTarEntries(xzReader)
	case FormatTarBz2:
		return listTarEntries(bzip2.NewReader(bytes.NewReader(data)))
	case FormatSevenZip:
		return listSevenZipEntries(data)
	default:
		return nil, fmt.Errorf("unsupported pages package format: %s", format)
	}
}

func entriesFromArchiveFiles(files []archiveFile) []Entry {
	entries := make([]Entry, 0, len(files))
	for _, item := range files {
		file := item
		entries = append(entries, Entry{
			Name:      file.Name(),
			IsDir:     file.IsDir(),
			IsSymlink: file.IsSymlink(),
			Size:      file.Size(),
			Open:      file.Open,
		})
	}
	return entries
}

func listZipEntries(data []byte) ([]Entry, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open zip pages package: %w", err)
	}
	files := make([]archiveFile, 0, len(reader.File))
	for _, item := range reader.File {
		files = append(files, zipArchiveFile{file: item})
	}
	return entriesFromArchiveFiles(files), nil
}

func listSevenZipEntries(data []byte) ([]Entry, error) {
	reader, err := sevenzip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open 7z pages package: %w", err)
	}
	files := make([]archiveFile, 0, len(reader.File))
	for _, item := range reader.File {
		files = append(files, sevenZipArchiveFile{file: item})
	}
	return entriesFromArchiveFiles(files), nil
}

func listTarEntries(r io.Reader) ([]Entry, error) {
	tarReader := tar.NewReader(r)
	// Tar is sequential: materialize regular file bodies so entries can be opened later.
	type materialised struct {
		header *tar.Header
		body   []byte
	}
	items := make([]materialised, 0)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar pages package: %w", err)
		}
		item, skip, err := materialiseTarHeader(tarReader, header)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		items = append(items, item)
	}

	entries := make([]Entry, 0, len(items))
	for _, item := range items {
		entries = append(entries, tarEntryFromMaterialised(item.header, item.body))
	}
	return entries, nil
}

func materialiseTarHeader(tarReader *tar.Reader, header *tar.Header) (item struct {
	header *tar.Header
	body   []byte
}, skip bool, err error) {
	switch header.Typeflag {
	case tar.TypeDir, tar.TypeSymlink, tar.TypeLink:
		return struct {
			header *tar.Header
			body   []byte
		}{header: header}, false, nil
	case tar.TypeReg, tar.TypeRegA: //nolint:staticcheck // TypeRegA still appears in older archives
		body, readErr := readTarBody(tarReader, header)
		if readErr != nil {
			return item, false, readErr
		}
		return struct {
			header *tar.Header
			body   []byte
		}{header: header, body: body}, false, nil
	default:
		if header.Size > 0 {
			if _, copyErr := io.CopyN(io.Discard, tarReader, header.Size); copyErr != nil {
				return item, false, fmt.Errorf("skip tar entry %s: %w", header.Name, copyErr)
			}
		}
		return item, true, nil
	}
}

func readTarBody(tarReader *tar.Reader, header *tar.Header) ([]byte, error) {
	if header.Size > 0 {
		body := make([]byte, header.Size)
		if _, err := io.ReadFull(tarReader, body); err != nil {
			return nil, fmt.Errorf("read tar entry %s: %w", header.Name, err)
		}
		return body, nil
	}
	body, err := io.ReadAll(tarReader)
	if err != nil {
		return nil, fmt.Errorf("read tar entry %s: %w", header.Name, err)
	}
	return body, nil
}

func tarEntryFromMaterialised(header *tar.Header, body []byte) Entry {
	size := header.Size
	if int64(len(body)) > size {
		size = int64(len(body))
	}
	entry := Entry{
		Name:      header.Name,
		IsDir:     header.Typeflag == tar.TypeDir,
		IsSymlink: header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink,
		Size:      uint64(size), //nolint:gosec // non-negative sizes
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		},
	}
	if entry.IsDir || entry.IsSymlink {
		entry.Open = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(nil)), nil
		}
	}
	return entry
}
