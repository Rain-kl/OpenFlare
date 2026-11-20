package repository

import (
	"context"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

// CreateAutomationCode stores the code and prunes expired credentials.
func CreateAutomationCode(ctx context.Context, code model.AutomationCode, now time.Time) error {
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at <= ?", now).Delete(&model.AutomationCode{}).Error; err != nil {
			return err
		}
		if err := tx.Where("refresh_expires_at <= ?", now).Delete(&model.AutomationGrant{}).Error; err != nil {
			return err
		}
		return tx.Create(&code).Error
	})
}

// GetAutomationCode retrieves a code from the primary database.
func GetAutomationCode(ctx context.Context, hash string) (model.AutomationCode, error) {
	var code model.AutomationCode
	err := db.DB(db.WithPrimaryReads(ctx)).First(&code, "hash = ?", hash).Error
	return code, err
}

// ExchangeAutomationCode atomically consumes one code and creates its grant.
func ExchangeAutomationCode(ctx context.Context, hash string, grant model.AutomationGrant, now time.Time) error {
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.AutomationCode{}).
			Where("hash = ? AND used = ? AND expires_at > ?", hash, false, now).
			Update("used", true)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return tx.Create(&grant).Error
	})
}

// GetAutomationAccessGrant checks the access credential and audience on the primary.
func GetAutomationAccessGrant(ctx context.Context, hash, resource string, now time.Time) (model.AutomationGrant, error) {
	var grant model.AutomationGrant
	err := db.DB(db.WithPrimaryReads(ctx)).
		First(&grant, "access_hash = ? AND resource = ? AND access_expires_at > ?", hash, resource, now).Error
	return grant, err
}

// GetAutomationRefreshGrant checks the refresh credential, client, and audience.
func GetAutomationRefreshGrant(ctx context.Context, hash, clientID, resource string, now time.Time) (model.AutomationGrant, error) {
	var grant model.AutomationGrant
	err := db.DB(db.WithPrimaryReads(ctx)).
		First(&grant, "refresh_hash = ? AND client_id = ? AND resource = ? AND refresh_expires_at > ?", hash, clientID, resource, now).Error
	return grant, err
}

// RotateAutomationGrant replaces credentials only while the original token is valid.
func RotateAutomationGrant(ctx context.Context, previousHash string, grant model.AutomationGrant, now time.Time) error {
	result := db.DB(ctx).Model(&model.AutomationGrant{}).
		Where("id = ? AND refresh_hash = ? AND refresh_expires_at > ?", grant.ID, previousHash, now).
		Updates(map[string]any{
			"access_hash":       grant.AccessHash,
			"refresh_hash":      grant.RefreshHash,
			"access_expires_at": grant.AccessExpiresAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// RevokeAutomationToken removes the grant associated with a credential.
func RevokeAutomationToken(ctx context.Context, clientID, hash string) error {
	return db.DB(ctx).Where("client_id = ? AND (access_hash = ? OR refresh_hash = ?)", clientID, hash, hash).
		Delete(&model.AutomationGrant{}).Error
}

// RevokeUserAutomationGrants removes all grants and pending codes for one account.
func RevokeUserAutomationGrants(ctx context.Context, userID uint64) error {
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.AutomationCode{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", userID).Delete(&model.AutomationGrant{}).Error
	})
}
