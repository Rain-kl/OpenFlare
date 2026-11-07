// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

// Persistence and repository-layer parameter messages live here (unexported).
// Domain field validation used by model.Validate stays in internal/model/errs.go;
// repository may call model.Validate and return those errors as-is.
// Keep wording aligned with model where the same user-facing phrase applies,
// but do not import or re-export model unexported consts (would require exporting).
const (
	errDatabaseNotInitialized               = "database not initialized"
	errConfigIntParseFailed                 = "cannot convert configuration %s value '%s' to an integer: %w"
	errConfigDecimalParseFailed             = "cannot convert configuration %s value '%s' to a decimal: %w"
	errConfigBoolParseFailed                = "cannot convert configuration %s value '%s' to a boolean: %w"
	errParseMenuDisplayConfigFailed         = "failed to parse directory display configuration: %w"
	errAuthSourceNameRequired               = "authentication source name must not be empty"
	errAuthSourceIDRequired                 = "authentication source ID must not be empty"
	errExternalAccountBindingIncomplete     = "external account binding information is incomplete"
	errExternalAccountAlreadyBoundToAnother = "this external account is bound to another user"
	errUserIDRequired                       = "user ID must not be empty"
	errExternalAccountBindingIDRequired     = "account binding ID must not be empty"
)

const colName = "name"

const colEnabled = "enabled"
