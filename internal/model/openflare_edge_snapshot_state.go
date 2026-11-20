// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

// EdgeSnapshotState tracks committed changes to the source and their compilation.
type EdgeSnapshotState struct {
	ID               int   `gorm:"primaryKey;autoIncrement:false"`
	SourceRevision   int64 `gorm:"not null;default:0"`
	CompiledRevision int64 `gorm:"not null;default:0"`
}

// TableName returns the singleton state table.
func (EdgeSnapshotState) TableName() string { return "of_edge_snapshot_state" }
