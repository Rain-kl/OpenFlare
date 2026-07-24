// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// ListPagesOrphanUploadCandidates returns at most 100 unreferenced, isolated
// Pages V2 upload records. Callers must still lock and recheck every condition
// before deleting a candidate.
func ListPagesOrphanUploadCandidates(
	ctx context.Context,
	input model.PagesOrphanUploadCandidateQuery,
) ([]model.Upload, error) {
	if input.SystemUserID == 0 || input.UploadType == "" || input.Marker == "" || input.CreatedBefore.IsZero() {
		return nil, errors.New("invalid pages orphan upload candidate query")
	}
	markerPredicate, err := pagesOrphanMarkerPredicate(db.DB(ctx).Name())
	if err != nil {
		return nil, err
	}

	deploymentTable := (model.PagesDeployment{}).TableName()
	uploadTable := (model.Upload{}).TableName()
	var candidates []model.Upload
	err = db.DB(ctx).
		Model(&model.Upload{}).
		Where(uploadTable+".status = ?", model.UploadStatusUsed).
		Where(uploadTable+".user_id = ?", input.SystemUserID).
		Where(uploadTable+".type = ?", input.UploadType).
		Where(uploadTable+".created_at < ?", input.CreatedBefore).
		Where(markerPredicate, input.Marker).
		Where("NOT EXISTS (SELECT 1 FROM " + deploymentTable + " WHERE " + deploymentTable + ".upload_id = " + uploadTable + ".id)").
		Order(uploadTable + ".id ASC").
		Limit(model.PagesOrphanUploadCandidateLimit).
		Find(&candidates).Error
	if err != nil {
		return nil, err
	}
	return candidates, nil
}

func pagesOrphanMarkerPredicate(dialect string) (string, error) {
	switch dialect {
	case "postgres":
		return model.PagesOrphanMarkerPredicatePostgres, nil
	case "sqlite":
		return model.PagesOrphanMarkerPredicateSQLite, nil
	default:
		return "", errors.New("unsupported database dialect for Pages orphan cleanup")
	}
}
