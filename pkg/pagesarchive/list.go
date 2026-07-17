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

// listEntriesAt lists archive members from a random-access source.
// When materializeBodies is true, tar-family streams buffer regular-file bodies so Entry.Open works.
// When false (inspect path), tar bodies are discarded after reading headers; zip/7z only use central directory metadata.
func listEntriesAt(ra io.ReaderAt, size int64, format Format, materializeBodies bool) ([]Entry, error) {
	if size < 0 {
		return nil, fmt.Errorf("invalid pages package size")
	}
	switch format {
	case FormatZip:
		return listZipEntriesAt(ra, size)
	case FormatTar:
		return listTarFamily(io.NewSectionReader(ra, 0, size), FormatTar, materializeBodies)
	case FormatTarGz:
		return listTarFamily(io.NewSectionReader(ra, 0, size), FormatTarGz, materializeBodies)
	case FormatTarXz:
		return listTarFamily(io.NewSectionReader(ra, 0, size), FormatTarXz, materializeBodies)
	case FormatTarBz2:
		return listTarFamily(io.NewSectionReader(ra, 0, size), FormatTarBz2, materializeBodies)
	case FormatSevenZip:
		return listSevenZipEntriesAt(ra, size)
	default:
		return nil, fmt.Errorf("unsupported pages package format: %s", format)
	}
}

func listTarFamily(r io.Reader, format Format, materializeBodies bool) ([]Entry, error) {
	switch format {
	case FormatTar:
		if materializeBodies {
			return listTarEntries(r, true)
		}
		return listTarEntries(r, false)
	case FormatTarGz:
		gzReader, err := gzip.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("open gzip pages package: %w", err)
		}
		defer func() { _ = gzReader.Close() }()
		return listTarEntries(gzReader, materializeBodies)
	case FormatTarXz:
		xzReader, err := xz.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("open xz pages package: %w", err)
		}
		return listTarEntries(xzReader, materializeBodies)
	case FormatTarBz2:
		return listTarEntries(bzip2.NewReader(r), materializeBodies)
	default:
		return nil, fmt.Errorf("unsupported tar family format: %s", format)
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

func listZipEntriesAt(ra io.ReaderAt, size int64) ([]Entry, error) {
	reader, err := zip.NewReader(ra, size)
	if err != nil {
		return nil, fmt.Errorf("open zip pages package: %w", err)
	}
	files := make([]archiveFile, 0, len(reader.File))
	for _, item := range reader.File {
		files = append(files, zipArchiveFile{file: item})
	}
	return entriesFromArchiveFiles(files), nil
}

func listSevenZipEntriesAt(ra io.ReaderAt, size int64) ([]Entry, error) {
	reader, err := sevenzip.NewReader(ra, size)
	if err != nil {
		return nil, fmt.Errorf("open 7z pages package: %w", err)
	}
	files := make([]archiveFile, 0, len(reader.File))
	for _, item := range reader.File {
		files = append(files, sevenZipArchiveFile{file: item})
	}
	return entriesFromArchiveFiles(files), nil
}

func listTarEntries(r io.Reader, materializeBodies bool) ([]Entry, error) {
	tarReader := tar.NewReader(r)
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
		item, skip, err := readTarHeader(tarReader, header, materializeBodies)
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
		entries = append(entries, tarEntryFromHeader(item.header, item.body, materializeBodies))
	}
	return entries, nil
}

func readTarHeader(tarReader *tar.Reader, header *tar.Header, materializeBodies bool) (item struct {
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
		if !materializeBodies {
			if err := discardTarBody(tarReader, header); err != nil {
				return item, false, err
			}
			return struct {
				header *tar.Header
				body   []byte
			}{header: header}, false, nil
		}
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

func discardTarBody(tarReader *tar.Reader, header *tar.Header) error {
	if header.Size <= 0 {
		_, err := io.Copy(io.Discard, tarReader)
		if err != nil {
			return fmt.Errorf("discard tar entry %s: %w", header.Name, err)
		}
		return nil
	}
	if _, err := io.CopyN(io.Discard, tarReader, header.Size); err != nil {
		return fmt.Errorf("discard tar entry %s: %w", header.Name, err)
	}
	return nil
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

func tarEntryFromHeader(header *tar.Header, body []byte, materializeBodies bool) Entry {
	size := header.Size
	if materializeBodies && int64(len(body)) > size {
		size = int64(len(body))
	}
	entry := Entry{
		Name:      header.Name,
		IsDir:     header.Typeflag == tar.TypeDir,
		IsSymlink: header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink,
		Size:      uint64(size), //nolint:gosec // non-negative sizes
	}
	if entry.IsDir || entry.IsSymlink {
		entry.Open = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(nil)), nil
		}
		return entry
	}
	if materializeBodies {
		bodyCopy := body
		entry.Open = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyCopy)), nil
		}
		return entry
	}
	// Inspect path: body not retained; Open is unavailable.
	entry.Open = func() (io.ReadCloser, error) {
		return nil, fmt.Errorf("tar entry body not materialized: %s", header.Name)
	}
	return entry
}
