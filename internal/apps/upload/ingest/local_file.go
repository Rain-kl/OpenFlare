// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/storage"
)

// LocalFileCandidateRequest describes filesystem locations that may host a legacy blob.
type LocalFileCandidateRequest struct {
	StoredPath    string
	RelativePaths []string
}

// ResolveLocalFile returns the first existing regular file among managed candidate paths.
func ResolveLocalFile(ctx context.Context, req LocalFileCandidateRequest) (string, int64, error) {
	for _, candidate := range buildLocalFileCandidates(ctx, req) {
		info, err := os.Stat(candidate) //nolint:gosec // candidate is resolved from managed legacy metadata
		if err != nil || info.IsDir() {
			continue
		}
		return candidate, info.Size(), nil
	}
	return "", 0, os.ErrNotExist
}

// IngestFromLocalPath ingests a local regular file through the standard upload ingest path.
func IngestFromLocalPath(ctx context.Context, localPath string, req Request) (Result, error) {
	localPath = strings.TrimSpace(localPath)
	if localPath == "" {
		return Result{}, errors.New("local path is required")
	}
	file, err := os.Open(localPath) //nolint:gosec // localPath is resolved from managed legacy metadata
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return Result{}, err
	}
	if info.IsDir() {
		return Result{}, errors.New("local path must be a regular file")
	}
	if req.Size <= 0 {
		req.Size = info.Size()
	}
	req.Reader = file
	return Ingest(ctx, req)
}

func buildLocalFileCandidates(ctx context.Context, req LocalFileCandidateRequest) []string {
	seen := make(map[string]struct{})
	candidates := make([]string, 0, 8+len(req.RelativePaths)*4)
	add := func(raw string) {
		value := strings.TrimSpace(raw)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		candidates = append(candidates, value)
	}

	storedPath := strings.TrimSpace(req.StoredPath)
	add(storedPath)
	if storedPath != "" {
		add(filepath.Clean(storedPath))
		add(strings.ReplaceAll(storedPath, "/data/data/", "/data/"))
		add(strings.ReplaceAll(
			filepath.Clean(storedPath),
			string(filepath.Separator)+string(filepath.Separator),
			string(filepath.Separator),
		))
	}
	for _, relativePath := range req.RelativePaths {
		add(relativePath)
	}

	for _, root := range localStorageRoots(ctx) {
		if storedPath != "" && !filepath.IsAbs(storedPath) {
			add(filepath.Join(root, storedPath))
		}
		for _, relativePath := range req.RelativePaths {
			add(filepath.Join(root, relativePath))
		}
	}
	return candidates
}

func localStorageRoots(ctx context.Context) []string {
	cfg, err := storage.LoadConfig(ctx)
	if err != nil {
		return nil
	}
	root := strings.TrimSpace(cfg.Local.Root)
	if root == "" {
		return nil
	}
	return []string{root}
}
