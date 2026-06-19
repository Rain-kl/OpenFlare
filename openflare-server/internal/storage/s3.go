// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3Backend struct {
	client    *s3.Client
	bucket    string
	keyPrefix string
	cdnURL    string
}

func newS3Backend(ctx context.Context, cfg ObjectConfig) (*s3Backend, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load S3 config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		if cfg.Endpoint != "" {
			options.BaseEndpoint = aws.String(strings.TrimRight(cfg.Endpoint, "/"))
		}
		options.UsePathStyle = cfg.PathStyle
	})
	return &s3Backend{
		client:    client,
		bucket:    cfg.Bucket,
		keyPrefix: strings.Trim(cfg.KeyPrefix, "/"),
		cdnURL:    strings.TrimRight(cfg.CDNURL, "/"),
	}, nil
}

func newR2Backend(ctx context.Context, cfg ObjectConfig) (*s3Backend, error) {
	if cfg.Endpoint == "" {
		cfg.Endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
	}
	cfg.Region = "auto"
	return newS3Backend(ctx, cfg)
}

func (b *s3Backend) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) (PutResult, error) {
	key = b.key(key)
	_, err := b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(b.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return PutResult{}, fmt.Errorf("put S3 object: %w", err)
	}
	return PutResult{Key: key, Bucket: b.bucket}, nil
}

func (b *s3Backend) Get(ctx context.Context, key string) (*Object, error) {
	key = b.key(key)
	if b.cdnURL != "" {
		return getHTTPObject(ctx, b.cdnURL, key)
	}
	output, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get S3 object: %w", err)
	}
	contentType := defaultContentType
	if output.ContentType != nil {
		contentType = *output.ContentType
	}
	var size int64
	if output.ContentLength != nil {
		size = *output.ContentLength
	}
	return &Object{Body: output.Body, ContentLength: size, ContentType: contentType}, nil
}

func (b *s3Backend) Delete(ctx context.Context, key string) error {
	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.key(key)),
	})
	if err != nil {
		return fmt.Errorf("delete S3 object: %w", err)
	}
	return nil
}

func (b *s3Backend) Test(ctx context.Context) error {
	_, err := b.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(b.bucket)})
	if err != nil {
		return fmt.Errorf("access S3 bucket: %w", err)
	}
	return nil
}

func (b *s3Backend) key(key string) string {
	key = strings.TrimLeft(key, "/")
	if b.keyPrefix == "" || strings.HasPrefix(key, b.keyPrefix+"/") {
		return key
	}
	return b.keyPrefix + "/" + key
}
