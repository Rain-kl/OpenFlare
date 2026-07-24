// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"
)

const (
	// PagesOrphanUploadCandidateLimit bounds one delayed Pages upload cleanup pass.
	PagesOrphanUploadCandidateLimit = 100
	// PagesOrphanMarkerPredicatePostgres is the Postgres JSON marker match SQL fragment.
	PagesOrphanMarkerPredicatePostgres = "w_uploads.metadata #>> '{extra,pages_ingest_marker}' = ?"
	// PagesOrphanMarkerPredicateSQLite is the SQLite JSON marker match SQL fragment.
	PagesOrphanMarkerPredicateSQLite = "CASE WHEN json_valid(w_uploads.metadata) THEN json_extract(w_uploads.metadata, '$.extra.pages_ingest_marker') ELSE NULL END = ?"
)

// PagesOrphanUploadCandidateQuery describes the fail-closed SQL candidate set
// for delayed Pages upload compensation.
type PagesOrphanUploadCandidateQuery struct {
	SystemUserID  uint64
	UploadType    string
	Marker        string
	CreatedBefore time.Time
}
