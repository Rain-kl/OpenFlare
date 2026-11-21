// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import "time"

// AutomationSession stores an OAuth session by hash.
type AutomationSession struct {
	Kind      string `gorm:"primaryKey"`
	Signature string `gorm:"primaryKey"`
	RequestID string
	UserID    uint64
	Data      string
	Active    bool
	ExpiresAt time.Time
}

// TableName returns the automation session table.
func (AutomationSession) TableName() string { return "w_automation_sessions" }
