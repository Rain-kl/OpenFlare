// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/upload"
	"github.com/Rain-kl/Wavelet/internal/model"
)

func syncRemoteSource(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	actor string,
) (*sourceSyncOutcome, error) {
	return syncRemoteSourceWithTrigger(ctx, snapshot, actor, pagesSourceTriggerManualSync)
}

func syncGitHubSource(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	actor string,
	targetRevision string,
	confirmedRevision string,
) (*sourceSyncOutcome, error) {
	return syncGitHubSourceWithTrigger(
		ctx, snapshot, actor, targetRevision, confirmedRevision, pagesSourceTriggerManualSync,
	)
}

func commitSourceDeployment(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	revision string,
	packageChecksum string,
	detail sourceDetail,
	detailJSON string,
	actor string,
	manifest *deploymentManifest,
	ingestResult upload.IngestResult,
	hasIngest bool,
	nextCheckNotBefore *time.Time,
) (*model.PagesDeployment, bool, bool, error) {
	return commitSourceDeploymentWithTrigger(
		ctx, snapshot, revision, packageChecksum, detail, detailJSON, actor,
		pagesSourceTriggerManualSync, manifest, ingestResult, hasIngest, nextCheckNotBefore,
	)
}
