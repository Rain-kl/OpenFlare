// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

type automationTransactionKey struct{}

func automationDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(automationTransactionKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return db.DB(db.WithPrimaryReads(ctx))
}

// BeginAutomationTransaction starts a primary transaction shared by an OAuth exchange.
func BeginAutomationTransaction(ctx context.Context) (context.Context, error) {
	if _, ok := ctx.Value(automationTransactionKey{}).(*gorm.DB); ok {
		return nil, errors.New("automation transaction already started")
	}
	tx := automationDB(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	return context.WithValue(ctx, automationTransactionKey{}, tx), nil
}

// CommitAutomationTransaction commits the active OAuth transaction.
func CommitAutomationTransaction(ctx context.Context) error {
	return automationDB(ctx).Commit().Error
}

// RollbackAutomationTransaction discards uncommitted changes from the exchange.
func RollbackAutomationTransaction(ctx context.Context) error {
	return automationDB(ctx).Rollback().Error
}

// CreateAutomationSession stores a hashed credential and its state.
func CreateAutomationSession(ctx context.Context, session model.AutomationSession) error {
	return automationDB(ctx).Create(&session).Error
}

// GetAutomationSession loads a credential that has not expired.
func GetAutomationSession(ctx context.Context, kind, signature string, now time.Time) (model.AutomationSession, error) {
	var session model.AutomationSession
	err := automationDB(ctx).First(&session, "kind = ? AND signature = ? AND expires_at > ?", kind, signature, now).Error
	return session, err
}

// ConsumeAutomationSession atomically marks an active credential as used.
func ConsumeAutomationSession(ctx context.Context, kind, signature string, now time.Time) error {
	result := automationDB(ctx).Model(&model.AutomationSession{}).
		Where("kind = ? AND signature = ? AND active = ? AND expires_at > ?", kind, signature, true, now).
		Update("active", false)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteAutomationSession removes one credential.
func DeleteAutomationSession(ctx context.Context, kind, signature string) error {
	return automationDB(ctx).Where("kind = ? AND signature = ?", kind, signature).Delete(&model.AutomationSession{}).Error
}

// RevokeAutomationSessions removes all credentials of the given kind for a grant.
func RevokeAutomationSessions(ctx context.Context, requestID, kind string) error {
	return automationDB(ctx).Where("request_id = ? AND kind = ?", requestID, kind).
		Delete(&model.AutomationSession{}).Error
}

// PruneAutomationSessions removes expired credential records.
func PruneAutomationSessions(ctx context.Context, now time.Time) error {
	return automationDB(ctx).Where("expires_at <= ?", now).Delete(&model.AutomationSession{}).Error
}

// RevokeUserAutomationGrants removes the user's sessions and direct access tokens.
func RevokeUserAutomationGrants(ctx context.Context, userID uint64) error {
	return automationDB(ctx).Where("user_id = ?", userID).Delete(&model.AutomationSession{}).Error
}

// GetAutomationUser reads the current account from the primary or active transaction.
func GetAutomationUser(ctx context.Context, id uint64) (model.User, error) {
	return getActiveUserByID(automationDB(ctx), id)
}
