// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/Rain-kl/Wavelet/pkg/httppool"
	"github.com/studio-b12/gowebdav"
)

type contextTransport struct {
	ctx    context.Context
	parent http.RoundTripper
}

func (t *contextTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.parent.RoundTrip(req.WithContext(t.ctx))
}

type webDAVBackend struct {
	endpoint string
	username string
	password string
	basePath string
}

func newWebDAVBackend(cfg WebDAVConfig) (*webDAVBackend, error) {
	return &webDAVBackend{
		endpoint: strings.TrimRight(cfg.Endpoint, "/"),
		username: cfg.Username,
		password: cfg.Password,
		basePath: strings.Trim(cfg.BasePath, "/"),
	}, nil
}

func (b *webDAVBackend) newClient(ctx context.Context) *gowebdav.Client {
	client := gowebdav.NewClient(b.endpoint, b.username, b.password)
	client.SetTransport(&contextTransport{
		ctx:    ctx,
		parent: httppool.DefaultTransport(),
	})
	return client
}

func (b *webDAVBackend) Put(ctx context.Context, key string, body io.Reader, size int64, _ string) (PutResult, error) {
	key = b.key(key)
	client := b.newClient(ctx)
	if dir := path.Dir(key); dir != "." && dir != "/" {
		if err := client.MkdirAll(dir, storageDirPerm); err != nil {
			return PutResult{}, fmt.Errorf("create WebDAV directory: %w", err)
		}
	}
	if err := client.WriteStreamWithLength(key, body, size, storageFilePerm); err != nil {
		return PutResult{}, fmt.Errorf("put WebDAV object: %w", err)
	}
	return PutResult{Key: key}, nil
}

func (b *webDAVBackend) Get(ctx context.Context, key string) (*Object, error) {
	key = b.key(key)
	client := b.newClient(ctx)
	info, err := client.Stat(key)
	if err != nil {
		return nil, fmt.Errorf("stat WebDAV object: %w", err)
	}
	body, err := client.ReadStream(key)
	if err != nil {
		return nil, fmt.Errorf("get WebDAV object: %w", err)
	}
	contentType := defaultContentType
	if typed, ok := info.(interface{ ContentType() string }); ok && typed.ContentType() != "" {
		contentType = typed.ContentType()
	}
	return &Object{Body: body, ContentLength: info.Size(), ContentType: contentType}, nil
}

func (b *webDAVBackend) Delete(ctx context.Context, key string) error {
	client := b.newClient(ctx)
	if err := client.Remove(b.key(key)); err != nil {
		return fmt.Errorf("delete WebDAV object: %w", err)
	}
	return nil
}

func (b *webDAVBackend) Test(ctx context.Context) error {
	client := b.newClient(ctx)
	if err := client.Connect(); err != nil {
		return fmt.Errorf("connect WebDAV: %w", err)
	}
	return nil
}

func (b *webDAVBackend) key(key string) string {
	return "/" + path.Join(b.basePath, strings.TrimLeft(key, "/"))
}
