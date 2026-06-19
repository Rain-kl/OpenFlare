// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package db

const (
	errRedisHashSetFailed    = "failed to set redis hash: %w"
	errRedisHashDeleteFailed = "failed to delete redis hash field: %w"
	errUnmarshalDataFailed   = "failed to unmarshal data: %w"
	errMarshalDataFailed     = "failed to marshal data: %w"
	errRedisKeySetFailed     = "failed to set redis key: %w"
)
