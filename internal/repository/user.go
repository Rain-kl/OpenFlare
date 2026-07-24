// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/idgen"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

// GetUserByID loads an active user by ID.
func GetUserByID(ctx context.Context, id uint64) (model.User, error) {
	var user model.User
	if err := db.DB(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return model.User{}, err
	}
	return user, nil
}

// GetUserByUsername loads a user by username.
func GetUserByUsername(ctx context.Context, username string) (model.User, error) {
	var user model.User
	if err := db.DB(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return model.User{}, err
	}
	return user, nil
}

// GetSystemUser loads the built-in system user, or returns a synthetic fallback.
func GetSystemUser(ctx context.Context) model.User {
	var user model.User
	if err := db.DB(ctx).Where("username = ?", configTypeSystem).First(&user).Error; err == nil {
		return user
	}
	return model.User{
		ID:       999,
		Username: configTypeSystem,
		Nickname: "系统",
	}
}

// GetFirstAdminUser loads the earliest admin user.
func GetFirstAdminUser(ctx context.Context) (model.User, error) {
	var user model.User
	if err := db.DB(ctx).Where("is_admin = ?", true).Order("id asc").First(&user).Error; err != nil {
		return model.User{}, err
	}
	return user, nil
}

// AdminUserListFilter filters admin user list queries.
type AdminUserListFilter struct {
	UserID   *uint64
	Username string
	Email    string
	Page     int
	PageSize int
}

// ListAdminUsers returns paginated users for the admin console.
func ListAdminUsers(ctx context.Context, filter AdminUserListFilter) (int64, []model.User, error) {
	query := db.DB(ctx).Model(&model.User{})
	if filter.UserID != nil {
		query = query.Where("id = ?", *filter.UserID)
	}
	if filter.Username != "" {
		query = query.Where("username LIKE ?", filter.Username+"%")
	}
	if filter.Email != "" {
		query = query.Where("email LIKE ?", filter.Email+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	var users []model.User
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.
		Select("id, username, nickname, email, avatar_url, is_active, is_admin, last_login_at, created_at, updated_at").
		Order("id ASC").
		Offset(offset).
		Limit(filter.PageSize).
		Find(&users).Error; err != nil {
		return 0, nil, err
	}
	return total, users, nil
}

// GetAdminUserDetail loads full user profile fields for admin detail view.
func GetAdminUserDetail(ctx context.Context, id uint64) (model.User, error) {
	var user model.User
	if err := db.DB(ctx).
		Select("id, username, nickname, email, avatar_url, is_active, is_admin, bio, phone, gender, website, location, last_login_at, created_at, updated_at").
		Where("id = ?", id).
		First(&user).Error; err != nil {
		return model.User{}, err
	}
	return user, nil
}

// UserAdminFlags stores minimal user authorization flags.
type UserAdminFlags struct {
	ID      uint64
	IsAdmin bool
}

// GetUserAdminFlags loads id and is_admin for authorization checks.
func GetUserAdminFlags(ctx context.Context, id uint64) (UserAdminFlags, error) {
	var flags UserAdminFlags
	if err := db.DB(ctx).
		Model(&model.User{}).
		Select("id, is_admin").
		Where("id = ?", id).
		First(&flags).Error; err != nil {
		return UserAdminFlags{}, err
	}
	return flags, nil
}

// UpdateUserActive updates the is_active flag for a user.
func UpdateUserActive(ctx context.Context, id uint64, active bool) error {
	return db.DB(ctx).Model(&model.User{}).Where("id = ?", id).Update("is_active", active).Error
}

// DeleteUserWithRelations removes a user and related access tokens / external accounts.
func DeleteUserWithRelations(ctx context.Context, id uint64) error {
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", id).Delete(&model.AccessToken{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&model.ExternalAccount{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&model.User{}).Error
	})
}

// CountUsersByUsername returns how many users share the username.
func CountUsersByUsername(ctx context.Context, username string) (int64, error) {
	var count int64
	if err := db.DB(ctx).Model(&model.User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountUsersByEmail returns how many users share the email.
func CountUsersByEmail(ctx context.Context, email string) (int64, error) {
	var count int64
	if err := db.DB(ctx).Model(&model.User{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CreateUser persists a new user record.
func CreateUser(ctx context.Context, user *model.User) error {
	return db.DB(ctx).Create(user).Error
}

// ListUsersByIDs loads users matching the given IDs.
func ListUsersByIDs(ctx context.Context, ids []uint64) ([]model.User, error) {
	if len(ids) == 0 {
		return []model.User{}, nil
	}
	var users []model.User
	if err := db.DB(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// ListUserIDsByUsernameContains returns user IDs whose username contains the given fragment.
func ListUserIDsByUsernameContains(ctx context.Context, username string) ([]uint64, error) {
	if username == "" {
		return []uint64{}, nil
	}
	var userIDs []uint64
	if err := db.DB(ctx).Model(&model.User{}).
		Where("username LIKE ?", "%"+username+"%").
		Pluck("id", &userIDs).Error; err != nil {
		return nil, err
	}
	return userIDs, nil
}

// UpdateUser updates all fields of an existing user.
func UpdateUser(ctx context.Context, user *model.User) error {
	return db.DB(ctx).Save(user).Error
}

// CreateUserFromOAuth creates a user from OAuth profile data and fills userOut.
func CreateUserFromOAuth(ctx context.Context, userOut *model.User, oauthInfo *model.OAuthUserInfo) error {
	now := time.Now()
	userID := oauthInfo.GetID()
	newUser := model.User{
		ID:          userID,
		Username:    oauthInfo.Username,
		Nickname:    oauthInfo.Name,
		Email:       oauthInfo.Email,
		AvatarURL:   oauthInfo.AvatarURL,
		IsActive:    oauthInfo.Active,
		LastLoginAt: now,
		IsAdmin:     false,
	}
	if newUser.ID == 0 {
		newUser.ID = idgen.NextUint64ID()
	}
	if err := db.DB(ctx).Create(&newUser).Error; err != nil {
		return err
	}
	*userOut = newUser
	return nil
}

// ListUsernamesMatchingBase returns usernames equal to base or prefixed with base+"-".
func ListUsernamesMatchingBase(ctx context.Context, base string) ([]string, error) {
	var names []string
	if err := db.DB(ctx).Model(&model.User{}).
		Where("username = ? OR username LIKE ?", base, base+"-%").
		Pluck("username", &names).Error; err != nil {
		return nil, err
	}
	return names, nil
}

// GetActiveUserByID loads a user by ID who is active.
func GetActiveUserByID(ctx context.Context, id uint64) (model.User, error) {
	var user model.User
	if err := db.DB(ctx).Where("id = ? AND is_active = ?", id, true).First(&user).Error; err != nil {
		return model.User{}, err
	}
	return user, nil
}

// GetUserByUsernameOrEmail loads a user by username or email.
func GetUserByUsernameOrEmail(ctx context.Context, input string) (model.User, error) {
	var user model.User
	if err := db.DB(ctx).Where("username = ? OR email = ?", input, input).First(&user).Error; err != nil {
		return model.User{}, err
	}
	return user, nil
}

// CountUsersByEmailExceptID counts users with the email excluding a given user id.
func CountUsersByEmailExceptID(ctx context.Context, email string, exceptID uint64) (int64, error) {
	var count int64
	if err := db.DB(ctx).Model(&model.User{}).Where("email = ? AND id != ?", email, exceptID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// UpdateUserLastLoginAt updates only last_login_at for a user.
func UpdateUserLastLoginAt(ctx context.Context, userID uint64, at time.Time) error {
	return db.DB(ctx).Model(&model.User{}).Where("id = ?", userID).Update("last_login_at", at).Error
}

// UpdateUserPassword updates only the password hash for a user.
func UpdateUserPassword(ctx context.Context, userID uint64, passwordHash string) error {
	return db.DB(ctx).Model(&model.User{}).Where("id = ?", userID).Update("password", passwordHash).Error
}

// RegisterUserWithChecks validates username/email uniqueness then creates the user.
func RegisterUserWithChecks(ctx context.Context, user *model.User) error {
	var count int64
	if err := db.DB(ctx).Model(&model.User{}).Where("username = ?", user.Username).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("用户名已存在")
	}
	if user.Email != "" {
		var emailCount int64
		if err := db.DB(ctx).Model(&model.User{}).Where("email = ?", user.Email).Count(&emailCount).Error; err != nil {
			return err
		}
		if emailCount > 0 {
			return errors.New("该邮箱已被其他账号绑定")
		}
	}
	if user.ID == 0 {
		user.ID = idgen.NextUint64ID()
	}
	return db.DB(ctx).Create(user).Error
}
