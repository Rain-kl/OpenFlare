// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

type localBackend struct {
	root string
}

func newLocalBackend(cfg LocalConfig) (*localBackend, error) {
	root := filepath.Clean(cfg.Root)
	if root == "" {
		return nil, fmt.Errorf("local root is required")
	}
	return &localBackend{root: root}, nil
}

func (b *localBackend) Put(_ context.Context, key string, body io.Reader, _ int64, _ string) (PutResult, error) {
	path, err := b.path(key)
	if err != nil {
		return PutResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), storageDirPerm); err != nil {
		return PutResult{}, err
	}
	file, err := os.OpenFile( //nolint:gosec // path is constrained to the configured storage root.
		path,
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
		storageFilePerm,
	)
	if err != nil {
		return PutResult{}, err
	}
	if _, err := io.Copy(file, body); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return PutResult{}, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return PutResult{}, err
	}
	return PutResult{Key: filepath.ToSlash(key)}, nil
}

func (b *localBackend) Get(_ context.Context, key string) (*Object, error) {
	path, err := b.path(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path) //nolint:gosec // path is constrained to the configured storage root.
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = defaultContentType
	}
	return &Object{Body: file, ContentLength: info.Size(), ContentType: contentType}, nil
}

func (b *localBackend) Delete(_ context.Context, key string) error {
	path, err := b.path(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (b *localBackend) Test(_ context.Context) error {
	return os.MkdirAll(b.root, storageDirPerm)
}

func (b *localBackend) path(key string) (string, error) {
	if filepath.IsAbs(key) {
		cleanPath := filepath.Clean(key)
		absRoot, err := filepath.Abs(b.root)
		if err != nil {
			return "", err
		}
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(absRoot, absPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", fmt.Errorf("storage key escapes local root")
		}
		return cleanPath, nil
	}
	cleanKey := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(key, "/")))
	if cleanKey == "." || cleanKey == "" || strings.HasPrefix(cleanKey, "..") {
		return "", fmt.Errorf("invalid local storage key %q", key)
	}
	path := filepath.Join(b.root, cleanKey)
	rel, err := filepath.Rel(b.root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("storage key escapes local root")
	}
	return path, nil
}
