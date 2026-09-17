// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package ofupload wraps Wavelet upload ingest with Pages-specific helpers.
package ofupload

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"

	"Wavelet/core/contracts"
	"Wavelet/openflare/plugins/server/kernel/model"
)

// ReservedPagesDeploymentType is managed exclusively by the Pages domain.
const ReservedPagesDeploymentType = "openflare_pages_deployment"

const (
	// PolicyCreate always stores a new object and creates a new upload record.
	PolicyCreate = 1
	// PolicyDedupNewRecord reuses an existing object path on hash match but creates a new record.
	PolicyDedupNewRecord = 2
	// PolicyResolveExisting returns an existing upload on hash match.
	PolicyResolveExisting = 3
)

// IngestRequest is the programmatic upload ingest payload.
type IngestRequest struct {
	UserID             uint64
	Type               string
	FileName           string
	MimeType           string
	Extension          string
	Size               int64
	Policy             int
	Hash               string
	Reader             io.Reader
	AccessMode         *int
	SkipExtensionCheck bool
	Metadata           model.UploadMetadata
}

// IngestResult reports ingest side effects.
type IngestResult struct {
	Upload   contracts.UploadDTO
	Created  bool
	Stored   bool
	Resolved bool
}

// IngestPolicy controls hash-collision behavior during ingest.
type IngestPolicy = int

var (
	svcMu      sync.RWMutex
	storageSvc contracts.StorageService
	uploadSvc  contracts.UploadService
)

// SetStorage injects the platform StorageService used to open stored objects.
func SetStorage(s contracts.StorageService) {
	svcMu.Lock()
	defer svcMu.Unlock()
	storageSvc = s
}

// SetUploadService injects the platform UploadService.
func SetUploadService(s contracts.UploadService) {
	svcMu.Lock()
	defer svcMu.Unlock()
	uploadSvc = s
}

// CurrentStorage returns the currently registered storage service.
func CurrentStorage() contracts.StorageService {
	svcMu.RLock()
	defer svcMu.RUnlock()
	return storageSvc
}

func currentStorage() contracts.StorageService {
	return CurrentStorage()
}

func currentUpload() contracts.UploadService {
	svcMu.RLock()
	defer svcMu.RUnlock()
	return uploadSvc
}

// IngestFromLocalPath ingests a local regular file through Wavelet upload ingest.
func IngestFromLocalPath(ctx context.Context, localPath string, req IngestRequest) (IngestResult, error) {
	localPath = strings.TrimSpace(localPath)
	if localPath == "" {
		return IngestResult{}, errors.New("local path is required")
	}
	file, err := os.Open(localPath) //nolint:gosec // localPath is resolved from managed Pages artifacts
	if err != nil {
		return IngestResult{}, err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return IngestResult{}, err
	}
	if info.IsDir() {
		return IngestResult{}, errors.New("local path must be a regular file")
	}
	if req.Size <= 0 {
		req.Size = info.Size()
	}

	storage := currentStorage()
	if storage == nil {
		return IngestResult{}, errors.New("storage service not available")
	}
	res, err := storage.Ingest(ctx, file, contracts.IngestOptions{
		UserID:    req.UserID,
		Type:      req.Type,
		FileName:  req.FileName,
		MimeType:  req.MimeType,
		Extension: req.Extension,
		Size:      req.Size,
		Policy:    req.Policy,
		Metadata:  req.Metadata.Extra,
	})
	if err != nil {
		return IngestResult{}, err
	}

	uploadRecord, err := GetActiveUpload(ctx, res.ID)
	if err != nil {
		return IngestResult{}, err
	}

	return IngestResult{
		Upload:   uploadRecord,
		Created:  res.Created,
		Stored:   res.Stored,
		Resolved: res.Resolved,
	}, nil
}

// GetActiveUpload loads an active (non-deleted) upload by ID.
func GetActiveUpload(ctx context.Context, id uint64) (contracts.UploadDTO, error) {
	svc := currentUpload()
	if svc == nil {
		return contracts.UploadDTO{}, errors.New("upload service not available")
	}
	u, err := svc.GetByID(ctx, id)
	if err != nil {
		return contracts.UploadDTO{}, err
	}
	if u == nil {
		return contracts.UploadDTO{}, errors.New("upload not found")
	}
	return *u, nil
}

// OpenedUploadObject is a stored object stream plus the upload record.
type OpenedUploadObject struct {
	Upload        contracts.UploadDTO
	Body          io.ReadCloser
	ContentType   string
	ContentLength int64
}

// OpenStoredUpload opens the stored object for an active upload via UploadService.
func OpenStoredUpload(ctx context.Context, id uint64) (*OpenedUploadObject, error) {
	svc := currentUpload()
	if svc == nil {
		return nil, errors.New("upload service not available")
	}
	obj, err := svc.OpenStoredUpload(ctx, id)
	if err != nil {
		return nil, err
	}
	return &OpenedUploadObject{
		Upload:        obj.Upload,
		Body:          obj.Body,
		ContentType:   obj.ContentType,
		ContentLength: obj.ContentLength,
	}, nil
}

// Remove removes an upload by ID.
func Remove(ctx context.Context, id uint64) error {
	svc := currentUpload()
	if svc == nil {
		return errors.New("upload service not available")
	}
	return svc.Remove(ctx, id)
}

// ResolveLocalFile returns the first existing regular file among candidate paths.
func ResolveLocalFile(_ context.Context, req LocalFileCandidateRequest) (string, int64, error) {
	candidates := append([]string{req.StoredPath}, req.RelativePaths...)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		info, err := os.Stat(candidate) //nolint:gosec // candidate is resolved from managed Pages metadata
		if err != nil || info.IsDir() {
			continue
		}
		return candidate, info.Size(), nil
	}
	return "", 0, os.ErrNotExist
}

// LocalFileCandidateRequest describes filesystem locations that may host a legacy blob.
type LocalFileCandidateRequest struct {
	StoredPath    string
	RelativePaths []string
}

// RebuildUploadStats rebuilds aggregate upload stats.
func RebuildUploadStats(ctx context.Context) error {
	svc := currentUpload()
	if svc == nil {
		return errors.New("upload service not available")
	}
	return svc.RebuildStats(ctx)
}
