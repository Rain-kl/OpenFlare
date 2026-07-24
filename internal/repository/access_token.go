// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// ListAccessTokensByUserID returns all access tokens for a user ordered by created_at desc.
func ListAccessTokensByUserID(ctx context.Context, userID uint64) ([]model.AccessToken, error) {
	var tokens []model.AccessToken
	if err := db.DB(ctx).Where("user_id = ?", userID).Order("created_at desc").Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

// CountAccessTokensByUserID returns how many access tokens a user owns.
func CountAccessTokensByUserID(ctx context.Context, userID uint64) (int64, error) {
	var count int64
	if err := db.DB(ctx).Model(&model.AccessToken{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CreateAccessToken inserts a new access token record.
func CreateAccessToken(ctx context.Context, record *model.AccessToken) error {
	return db.DB(ctx).Create(record).Error
}

// GetAccessTokenByIDAndUserID loads a token owned by the given user.
func GetAccessTokenByIDAndUserID(ctx context.Context, id, userID uint64) (model.AccessToken, error) {
	var token model.AccessToken
	if err := db.DB(ctx).Where("id = ? AND user_id = ?", id, userID).First(&token).Error; err != nil {
		return model.AccessToken{}, err
	}
	return token, nil
}

// DeleteAccessTokenForUser deletes a token if it belongs to the user.
// Returns the number of rows affected.
func DeleteAccessTokenForUser(ctx context.Context, id, userID uint64) (int64, error) {
	tx := db.DB(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.AccessToken{})
	return tx.RowsAffected, tx.Error
}

// GetAccessTokenByHash loads an access token by its token hash.
func GetAccessTokenByHash(ctx context.Context, tokenHash string) (model.AccessToken, error) {
	var token model.AccessToken
	if err := db.DB(ctx).Where("token_hash = ?", tokenHash).First(&token).Error; err != nil {
		return model.AccessToken{}, err
	}
	return token, nil
}

// SaveAccessToken persists all fields of an existing access token.
func SaveAccessToken(ctx context.Context, record *model.AccessToken) error {
	return db.DB(ctx).Save(record).Error
}

// DeleteAccessTokensByUserID deletes all access tokens for a user.
func DeleteAccessTokensByUserID(ctx context.Context, userID uint64) error {
	return db.DB(ctx).Where("user_id = ?", userID).Delete(&model.AccessToken{}).Error
}
