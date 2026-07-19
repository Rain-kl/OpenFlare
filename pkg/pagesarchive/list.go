// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pagesarchive

import (
	"archive/tar"
	"archive/zip"
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
	Mode() os.FileMode
	IsDir() bool
	Size() uint64
	Open() (io.ReadCloser, error)
}

type zipArchiveFile struct {
	file *zip.File
}

func (z zipArchiveFile) Name() string      { return z.file.Name }
func (z zipArchiveFile) Mode() os.FileMode { return z.file.Mode() }
func (z zipArchiveFile) IsDir() bool       { return z.file.FileInfo().IsDir() }
func (z zipArchiveFile) Size() uint64      { return z.file.UncompressedSize64 }
func (z zipArchiveFile) Open() (io.ReadCloser, error) {
	return z.file.Open()
}

type sevenZipArchiveFile struct {
	file *sevenzip.File
}

func (z sevenZipArchiveFile) Name() string      { return z.file.Name }
func (z sevenZipArchiveFile) Mode() os.FileMode { return z.file.Mode() }
func (z sevenZipArchiveFile) IsDir() bool       { return z.file.FileInfo().IsDir() }
func (z sevenZipArchiveFile) Size() uint64      { return z.file.UncompressedSize }
func (z sevenZipArchiveFile) Open() (io.ReadCloser, error) {
	return z.file.Open()
}

// listRandomAccessEntriesAt lists zip/7z members without reading their bodies.
// Tar-family archives use the sequential streaming paths in inspect.go/extract.go.
func listRandomAccessEntriesAt(ra io.ReaderAt, size int64, format Format) ([]Entry, error) {
	if size < 0 {
		return nil, fmt.Errorf("invalid pages package size")
	}
	switch format {
	case FormatZip:
		return listZipEntriesAt(ra, size)
	case FormatSevenZip:
		return listSevenZipEntriesAt(ra, size)
	default:
		return nil, fmt.Errorf("unsupported random-access pages package format: %s", format)
	}
}

func entriesFromArchiveFiles(files []archiveFile) []Entry {
	entries := make([]Entry, 0, len(files))
	for _, item := range files {
		file := item
		mode := file.Mode()
		isDir := file.IsDir()
		isSymlink := mode&os.ModeSymlink != 0
		isSpecial := !isDir && !isSymlink && !mode.IsRegular()
		entries = append(entries, Entry{
			Name:      file.Name(),
			IsDir:     isDir,
			IsSymlink: isSymlink,
			IsSpecial: isSpecial,
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

func openTarFamilyReader(r io.Reader, format Format) (*tar.Reader, func() error, error) {
	switch format {
	case FormatTar:
		return tar.NewReader(r), func() error { return nil }, nil
	case FormatTarGz:
		gzReader, err := gzip.NewReader(r)
		if err != nil {
			return nil, nil, fmt.Errorf("open gzip pages package: %w", err)
		}
		return tar.NewReader(gzReader), gzReader.Close, nil
	case FormatTarXz:
		xzReader, err := xz.NewReader(r)
		if err != nil {
			return nil, nil, fmt.Errorf("open xz pages package: %w", err)
		}
		return tar.NewReader(xzReader), func() error { return nil }, nil
	case FormatTarBz2:
		return tar.NewReader(bzip2.NewReader(r)), func() error { return nil }, nil
	default:
		return nil, nil, fmt.Errorf("unsupported tar family format: %s", format)
	}
}

func entryFromTarHeader(header *tar.Header) Entry {
	entry := Entry{Name: header.Name}
	if header.Size > 0 {
		entry.Size = uint64(header.Size) //nolint:gosec // archive/tar rejects negative sizes
	}
	switch header.Typeflag {
	case tar.TypeReg, tar.TypeRegA: //nolint:staticcheck // TypeRegA appears in older archives
		// Regular file.
	case tar.TypeDir:
		entry.IsDir = true
	case tar.TypeSymlink:
		entry.IsSymlink = true
	case tar.TypeLink:
		entry.IsHardlink = true
	default:
		entry.IsSpecial = true
	}
	return entry
}
