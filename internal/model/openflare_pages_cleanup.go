// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"errors"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
)

const (
	// PagesOrphanUploadCandidateLimit bounds one delayed Pages upload cleanup pass.
	PagesOrphanUploadCandidateLimit = 100

	pagesOrphanMarkerPredicatePostgres = "w_uploads.metadata #>> '{extra,pages_ingest_marker}' = ?"
	pagesOrphanMarkerPredicateSQLite   = "CASE WHEN json_valid(w_uploads.metadata) THEN json_extract(w_uploads.metadata, '$.extra.pages_ingest_marker') ELSE NULL END = ?"
)

// PagesOrphanUploadCandidateQuery describes the fail-closed SQL candidate set
// for delayed Pages upload compensation.
type PagesOrphanUploadCandidateQuery struct {
	SystemUserID  uint64
	UploadType    string
	Marker        string
	CreatedBefore time.Time
}

// ListPagesOrphanUploadCandidates returns at most 100 unreferenced, isolated
// Pages V2 upload records. Callers must still lock and recheck every condition
// before deleting a candidate.
func ListPagesOrphanUploadCandidates(
	ctx context.Context,
	input PagesOrphanUploadCandidateQuery,
) ([]Upload, error) {
	if input.SystemUserID == 0 || input.UploadType == "" || input.Marker == "" || input.CreatedBefore.IsZero() {
		return nil, errors.New("invalid pages orphan upload candidate query")
	}
	markerPredicate, err := pagesOrphanMarkerPredicate(db.DB(ctx).Name())
	if err != nil {
		return nil, err
	}

	deploymentTable := (PagesDeployment{}).TableName()
	uploadTable := (Upload{}).TableName()
	var candidates []Upload
	err = db.DB(ctx).
		Model(&Upload{}).
		Where(uploadTable+".status = ?", UploadStatusUsed).
		Where(uploadTable+".user_id = ?", input.SystemUserID).
		Where(uploadTable+".type = ?", input.UploadType).
		Where(uploadTable+".created_at < ?", input.CreatedBefore).
		Where(markerPredicate, input.Marker).
		Where("NOT EXISTS (SELECT 1 FROM " + deploymentTable + " WHERE " + deploymentTable + ".upload_id = " + uploadTable + ".id)").
		Order(uploadTable + ".id ASC").
		Limit(PagesOrphanUploadCandidateLimit).
		Find(&candidates).Error
	if err != nil {
		return nil, err
	}
	return candidates, nil
}

func pagesOrphanMarkerPredicate(dialect string) (string, error) {
	switch dialect {
	case "postgres":
		return pagesOrphanMarkerPredicatePostgres, nil
	case "sqlite":
		return pagesOrphanMarkerPredicateSQLite, nil
	default:
		return "", errors.New("unsupported database dialect for Pages orphan cleanup")
	}
}
