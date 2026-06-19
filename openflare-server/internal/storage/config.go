// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package storage provides dynamically configured file storage backends.
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

// Driver identifies a supported storage backend.
type Driver string

const (
	// DriverLocal stores files on the local filesystem.
	DriverLocal Driver = "local"
	// DriverS3 stores files in an S3-compatible object store.
	DriverS3 Driver = "s3"
	// DriverR2 stores files in Cloudflare R2.
	DriverR2 Driver = "r2"
	// DriverMinIO stores files in MinIO.
	DriverMinIO Driver = "minio"
	// DriverOSS stores files in Aliyun OSS.
	DriverOSS Driver = "oss"
	// DriverWebDAV stores files through WebDAV.
	DriverWebDAV Driver = "webdav"

	// ConfigMask replaces secrets returned to the frontend.
	ConfigMask = "******"
)

// LocalConfig configures local filesystem storage.
type LocalConfig struct {
	Root string `json:"root"`
}

// ObjectConfig configures S3-compatible or OSS object storage.
type ObjectConfig struct {
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	AccountID       string `json:"account_id,omitempty"`
	PathStyle       bool   `json:"path_style"`
	KeyPrefix       string `json:"key_prefix"`
	CDNURL          string `json:"cdn_url"`
}

// WebDAVConfig configures WebDAV storage.
type WebDAVConfig struct {
	Endpoint string `json:"endpoint"`
	Username string `json:"username"`
	Password string `json:"password"`
	BasePath string `json:"base_path"`
}

// Config contains all storage backends and the currently active driver.
type Config struct {
	Driver Driver       `json:"driver"`
	Local  LocalConfig  `json:"local"`
	S3     ObjectConfig `json:"s3"`
	R2     ObjectConfig `json:"r2"`
	MinIO  ObjectConfig `json:"minio"`
	OSS    ObjectConfig `json:"oss"`
	WebDAV WebDAVConfig `json:"webdav"`
}

// DefaultConfig returns the local-storage default configuration.
func DefaultConfig() Config {
	return Config{
		Driver: DriverLocal,
		Local:  LocalConfig{Root: "."},
		S3:     ObjectConfig{Region: "us-east-1"},
		R2:     ObjectConfig{Region: "auto"},
		MinIO:  ObjectConfig{Region: "us-east-1", PathStyle: true},
	}
}

// LoadConfig loads the active storage configuration.
func LoadConfig(ctx context.Context) (Config, error) {
	pubSubOnce.Do(startPubSubListener)

	cacheMutex.RLock()
	isCacheValid := time.Since(lastChecked) < 5*time.Second && activeConfigJSON != ""
	configJSON := activeConfigJSON
	cacheMutex.RUnlock()

	if isCacheValid {
		cfg := DefaultConfig()
		if strings.TrimSpace(configJSON) != "" {
			if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
				return Config{}, fmt.Errorf("parse storage config from cache: %w", err)
			}
		}
		return cfg, nil
	}

	return loadConfigByKey(ctx, model.ConfigKeyStorageConfig, DefaultConfig())
}

func loadConfigByKey(ctx context.Context, key string, fallback Config) (Config, error) {
	sc, err := repository.GetSystemConfigByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fallback, nil
		}
		return Config{}, err
	}
	if strings.TrimSpace(sc.Value) == "" {
		return fallback, nil
	}
	if err := json.Unmarshal([]byte(sc.Value), &fallback); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", key, err)
	}
	return fallback, nil
}

// ValidateConfig validates the selected backend configuration.
func ValidateConfig(cfg Config) error {
	switch cfg.Driver {
	case DriverLocal:
		if strings.TrimSpace(cfg.Local.Root) == "" {
			return errors.New("local root is required")
		}
	case DriverS3:
		return validateObjectConfig(cfg.S3, false)
	case DriverR2:
		if strings.TrimSpace(cfg.R2.AccountID) == "" && strings.TrimSpace(cfg.R2.Endpoint) == "" {
			return errors.New("R2 account ID or endpoint is required")
		}
		return validateObjectConfig(cfg.R2, false)
	case DriverMinIO:
		if strings.TrimSpace(cfg.MinIO.Endpoint) == "" {
			return errors.New("MinIO endpoint is required")
		}
		return validateObjectConfig(cfg.MinIO, true)
	case DriverOSS:
		if strings.TrimSpace(cfg.OSS.Endpoint) == "" {
			return errors.New("OSS endpoint is required")
		}
		return validateObjectConfig(cfg.OSS, true)
	case DriverWebDAV:
		if strings.TrimSpace(cfg.WebDAV.Endpoint) == "" {
			return errors.New("WebDAV endpoint is required")
		}
	default:
		return fmt.Errorf("unsupported storage driver %q", cfg.Driver)
	}
	return nil
}

func validateObjectConfig(cfg ObjectConfig, endpointRequired bool) error {
	if endpointRequired && strings.TrimSpace(cfg.Endpoint) == "" {
		return errors.New("endpoint is required")
	}
	if strings.TrimSpace(cfg.Region) == "" {
		return errors.New("region is required")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return errors.New("bucket is required")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return errors.New("access key ID and secret access key are required")
	}
	return nil
}

// SaveActiveConfig persists the active storage configuration.
func SaveActiveConfig(ctx context.Context, cfg Config) error {
	return saveSystemConfig(ctx, model.ConfigKeyStorageConfig, cfg, "文件存储驱动及连接配置（JSON）")
}

func saveSystemConfig(ctx context.Context, key string, value any, description string) error {
	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		return upsertSystemConfig(ctx, tx, key, value, description)
	})
	if err == nil && key == model.ConfigKeyStorageConfig {
		ResetCache()
		PublishCacheInvalidation(ctx)
	}
	return err
}

func upsertSystemConfig(ctx context.Context, tx *gorm.DB, key string, value any, description string) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", key, err)
	}
	sc := model.SystemConfig{
		Key:         key,
		Value:       string(data),
		Type:        "system",
		Visibility:  model.ConfigVisibilityHidden,
		Description: description,
	}
	if err := tx.Where("key = ?", key).
		Assign(map[string]any{"value": sc.Value, "description": description, "visibility": model.ConfigVisibilityHidden}).
		FirstOrCreate(&sc).Error; err != nil {
		return err
	}
	return repository.InvalidateSystemConfigCache(ctx, key)
}

// MergeMaskedSecrets restores unchanged secrets from the current configuration.
func MergeMaskedSecrets(next, current Config) Config {
	mergeObjectSecret := func(dst *ObjectConfig, src ObjectConfig) {
		if dst.AccessKeyID == ConfigMask {
			dst.AccessKeyID = src.AccessKeyID
		}
		if dst.SecretAccessKey == ConfigMask {
			dst.SecretAccessKey = src.SecretAccessKey
		}
	}
	mergeObjectSecret(&next.S3, current.S3)
	mergeObjectSecret(&next.R2, current.R2)
	mergeObjectSecret(&next.MinIO, current.MinIO)
	mergeObjectSecret(&next.OSS, current.OSS)
	if next.WebDAV.Password == ConfigMask {
		next.WebDAV.Password = current.WebDAV.Password
	}
	return next
}

// MaskSecrets replaces stored credentials with placeholders for API responses.
func MaskSecrets(cfg Config) Config {
	maskObject := func(value *ObjectConfig) {
		if value.AccessKeyID != "" {
			value.AccessKeyID = ConfigMask
		}
		if value.SecretAccessKey != "" {
			value.SecretAccessKey = ConfigMask
		}
	}
	maskObject(&cfg.S3)
	maskObject(&cfg.R2)
	maskObject(&cfg.MinIO)
	maskObject(&cfg.OSS)
	if cfg.WebDAV.Password != "" {
		cfg.WebDAV.Password = ConfigMask
	}
	return cfg
}
