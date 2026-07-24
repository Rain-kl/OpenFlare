// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/apps/upload"
)

// ProjectLatestPackageMetadata describes the active package limits published to Agents.
type ProjectLatestPackageMetadata struct {
	DeploymentID uint
	Hash         string
	PackageSize  int64
	FileCount    int
	TotalSize    int64
}

// GetProjectLatestPackageMetadata returns one coherent metadata snapshot for a
// project's currently active deployment.
func GetProjectLatestPackageMetadata(ctx context.Context, projectID uint) (*ProjectLatestPackageMetadata, error) {
	deployment, err := resolveProjectActiveDeploymentForAgent(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if deployment.UploadID == 0 {
		if err := ensureDeploymentUploadRecord(ctx, deployment); err != nil {
			return nil, err
		}
		deployment, err = repository.GetPagesDeploymentByID(ctx, deployment.ID)
		if err != nil {
			return nil, err
		}
	}
	if deployment.UploadID == 0 {
		return nil, errors.New(errPagesDeploymentNotFound)
	}

	uploadRecord, err := upload.GetActiveUpload(ctx, deployment.UploadID)
	if err != nil {
		return nil, fmt.Errorf("pages 部署包不存在: %w", err)
	}
	hash := strings.TrimSpace(uploadRecord.Hash)
	if hash == "" {
		hash = strings.TrimSpace(deployment.Checksum)
	}
	if hash == "" {
		return nil, errors.New(errPagesDeploymentHashMissing)
	}

	return &ProjectLatestPackageMetadata{
		DeploymentID: deployment.ID,
		Hash:         hash,
		PackageSize:  uploadRecord.FileSize,
		FileCount:    deployment.FileCount,
		TotalSize:    deployment.TotalSize,
	}, nil
}
