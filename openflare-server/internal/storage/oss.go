// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

type ossBackend struct {
	client    *oss.Client
	bucket    string
	keyPrefix string
	cdnURL    string
}

func newOSSBackend(cfg ObjectConfig) (*ossBackend, error) {
	options := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey)).
		WithRegion(cfg.Region)
	if cfg.Endpoint != "" {
		options.WithEndpoint(cfg.Endpoint)
	}
	return &ossBackend{
		client:    oss.NewClient(options),
		bucket:    cfg.Bucket,
		keyPrefix: strings.Trim(cfg.KeyPrefix, "/"),
		cdnURL:    strings.TrimRight(cfg.CDNURL, "/"),
	}, nil
}

func (b *ossBackend) Put(ctx context.Context, key string, body io.Reader, _ int64, _ string) (PutResult, error) {
	key = b.key(key)
	_, err := b.client.PutObject(ctx, &oss.PutObjectRequest{
		Bucket: oss.Ptr(b.bucket),
		Key:    oss.Ptr(key),
		Body:   body,
	})
	if err != nil {
		return PutResult{}, fmt.Errorf("put OSS object: %w", err)
	}
	return PutResult{Key: key, Bucket: b.bucket}, nil
}

func (b *ossBackend) Get(ctx context.Context, key string) (*Object, error) {
	key = b.key(key)
	if b.cdnURL != "" {
		return getHTTPObject(ctx, b.cdnURL, key)
	}
	output, err := b.client.GetObject(ctx, &oss.GetObjectRequest{
		Bucket: oss.Ptr(b.bucket),
		Key:    oss.Ptr(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get OSS object: %w", err)
	}
	contentType := defaultContentType
	if output.ContentType != nil {
		contentType = *output.ContentType
	}
	return &Object{Body: output.Body, ContentLength: output.ContentLength, ContentType: contentType}, nil
}

func (b *ossBackend) Delete(ctx context.Context, key string) error {
	_, err := b.client.DeleteObject(ctx, &oss.DeleteObjectRequest{
		Bucket: oss.Ptr(b.bucket),
		Key:    oss.Ptr(b.key(key)),
	})
	if err != nil {
		return fmt.Errorf("delete OSS object: %w", err)
	}
	return nil
}

func (b *ossBackend) Test(ctx context.Context) error {
	ok, err := b.client.IsBucketExist(ctx, b.bucket)
	if err != nil {
		return fmt.Errorf("access OSS bucket: %w", err)
	}
	if !ok {
		return fmt.Errorf("OSS bucket %q does not exist", b.bucket)
	}
	return nil
}

func (b *ossBackend) key(key string) string {
	key = strings.TrimLeft(key, "/")
	if b.keyPrefix == "" || strings.HasPrefix(key, b.keyPrefix+"/") {
		return key
	}
	return b.keyPrefix + "/" + key
}
