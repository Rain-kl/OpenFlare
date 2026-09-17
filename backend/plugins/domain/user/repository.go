// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/idgen"
	"Wavelet/pkg/util"
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

var (
	dbMu       sync.RWMutex
	dbSvc      contracts.DBService
	limiterMu  sync.RWMutex
	limiterSvc contracts.LimiterService
)

// SetDBService sets the active DBService contract for the user domain plugin.
func SetDBService(s contracts.DBService) {
	dbMu.Lock()
	defer dbMu.Unlock()
	dbSvc = s
}

// SetLimiterService sets the active LimiterService contract for the user domain plugin.
func SetLimiterService(s contracts.LimiterService) {
	limiterMu.Lock()
	defer limiterMu.Unlock()
	limiterSvc = s
}

func getDB(ctx context.Context) *gorm.DB {
	if s, err := core.InjectFrom[contracts.DBService](ctx); err == nil && s != nil {
		return s.DB(ctx)
	}

	dbMu.RLock()
	s := dbSvc
	dbMu.RUnlock()
	if s != nil {
		return s.DB(ctx)
	}

	return nil
}

func getLimiter(ctx context.Context) contracts.LimiterService {
	if s, err := core.InjectFrom[contracts.LimiterService](ctx); err == nil && s != nil {
		return s
	}

	limiterMu.RLock()
	s := limiterSvc
	limiterMu.RUnlock()
	return s
}

// GetUserByID 通过 ID 获取用户
func GetUserByID(ctx context.Context, id uint64) (*User, error) {
	var u User
	if err := getDB(ctx).First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUsersByIDs 一次性批量获取多个用户，避免调用方按 ID 逐条查询。
func GetUsersByIDs(ctx context.Context, ids []uint64) ([]User, error) {
	if len(ids) == 0 {
		return []User{}, nil
	}
	var users []User
	if err := getDB(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// GetUserByUsername 通过用户名获取用户
func GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	if err := getDB(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserByEmail 通过邮箱获取用户
func GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	if err := getDB(ctx).Where("email = ?", email).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// CreateUser 创建用户。ID 为空时用雪花算法分配，避免 SQLite/GORM 把 0 当成自增主键。
func CreateUser(ctx context.Context, u *User) error {
	if u != nil && u.ID == 0 {
		u.ID = idgen.NextUint64ID()
	}
	return getDB(ctx).Create(u).Error
}

// UpdateUser 更新用户
func UpdateUser(ctx context.Context, u *User) error {
	return getDB(ctx).Save(u).Error
}

// GetAccessTokenByHash 通过 Hash 查询访问令牌
func GetAccessTokenByHash(ctx context.Context, tokenHash string) (*AccessToken, error) {
	var token AccessToken
	if err := getDB(ctx).Where("token_hash = ?", tokenHash).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

// AdminUserListFilter 包含后台用户列表过滤条件
type AdminUserListFilter struct {
	Username string
	Keyword  string
	Page     int
	PageSize int
}

// ListAdminUsers 获取后台管理用户列表
func ListAdminUsers(ctx context.Context, filter AdminUserListFilter) (int64, []User, error) {
	query := getDB(ctx).Model(&User{})
	if filter.Username != "" {
		escaped := util.EscapeLike(strings.ToLower(filter.Username))
		query = query.Where("LOWER(username) LIKE ? ESCAPE '\\'", "%"+escaped+"%")
	}
	if filter.Keyword != "" {
		escaped := util.EscapeLike(strings.ToLower(filter.Keyword))
		query = query.Where("LOWER(username) LIKE ? ESCAPE '\\' OR LOWER(nickname) LIKE ? ESCAPE '\\' OR LOWER(email) LIKE ? ESCAPE '\\'",
			"%"+escaped+"%", "%"+escaped+"%", "%"+escaped+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	var users []User
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(filter.PageSize).Find(&users).Error; err != nil {
		return 0, nil, err
	}
	return total, users, nil
}

// UpdateUserActive 更新用户激活状态
func UpdateUserActive(ctx context.Context, id uint64, active bool) error {
	return getDB(ctx).Model(&User{}).Where("id = ?", id).Update("is_active", active).Error
}

// GetActiveUserByID 获取处于激活状态的用户
func GetActiveUserByID(ctx context.Context, id uint64) (*User, error) {
	var u User
	if err := getDB(ctx).Where("id = ? AND is_active = ?", id, true).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// DeleteUserWithRelations 删除用户及其级联关系
func DeleteUserWithRelations(ctx context.Context, id uint64) error {
	return getDB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", id).Delete(&AccessToken{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&User{}).Error
	})
}

// ListInactiveNeverLoggedInUserIDs returns non-admin users created before cutoff
// who have never logged in. Seeded admin/system accounts are excluded.
func ListInactiveNeverLoggedInUserIDs(ctx context.Context, cutoff time.Time) ([]uint64, error) {
	db := getDB(ctx)
	if db == nil {
		return nil, errors.New("database not available")
	}
	var ids []uint64
	unixEpoch := time.Unix(0, 0).UTC()
	err := db.Model(&User{}).
		Where("is_admin = ? AND username NOT IN ?", false, []string{"admin", "system"}).
		Where("created_at < ?", cutoff).
		Where("last_login_at IS NULL OR last_login_at < ?", unixEpoch).
		Pluck("id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// GetFirstAdminUser 获取第一个管理员用户
func GetFirstAdminUser(ctx context.Context) (*User, error) {
	var u User
	if err := getDB(ctx).Where("is_admin = ?", true).Order("id ASC").First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// ListUsernamesMatchingBase 列出匹配基础用户名的所有用户名
func ListUsernamesMatchingBase(ctx context.Context, base string) ([]string, error) {
	var usernames []string
	escaped := util.EscapeLike(strings.ToLower(base))
	if err := getDB(ctx).Model(&User{}).
		Where("LOWER(username) LIKE ? ESCAPE '\\'", escaped+"%").
		Pluck("username", &usernames).Error; err != nil {
		return nil, err
	}
	return usernames, nil
}

// adminUserColumns 后台用户查询显式列清单
const adminUserColumns = "id, username, nickname, email, avatar_url, is_active, is_admin, bio, phone, gender, website, location, last_login_at, created_at, updated_at"

// updateUserColumns 按列局部更新指定用户行
func updateUserColumns(ctx context.Context, id uint64, updates map[string]any) error {
	return getDB(ctx).Model(&User{}).Where("id = ?", id).Updates(updates).Error
}

// setUserAdminFlag 更新指定用户的管理员标记列
func setUserAdminFlag(ctx context.Context, id uint64, admin bool) error {
	return getDB(ctx).Model(&User{}).Where("id = ?", id).Update("is_admin", admin).Error
}

// setUserActiveColumn 后台启用/禁用用户，沿用无 schema 的按表直写语义
// （与导出函数 UpdateUserActive 不同，后者经由 Model 会额外刷新 updated_at）
func setUserActiveColumn(ctx context.Context, id uint64, active bool) error {
	return getDB(ctx).Table("w_users").Where("id = ?", id).Update("is_active", active).Error
}

// countAllUsers 统计用户总数
func countAllUsers(ctx context.Context) (int64, error) {
	var count int64
	err := getDB(ctx).Model(&User{}).Count(&count).Error
	return count, err
}

// countActiveUsers 统计激活状态用户数
func countActiveUsers(ctx context.Context) (int64, error) {
	var count int64
	err := getDB(ctx).Model(&User{}).Where("is_active = ?", true).Count(&count).Error
	return count, err
}

// countUsersByUsername 统计同名用户数量
func countUsersByUsername(ctx context.Context, username string) (int64, error) {
	var count int64
	err := getDB(ctx).Table("w_users").Where("username = ?", username).Count(&count).Error
	return count, err
}

// countUsersByEmail 统计同邮箱用户数量
func countUsersByEmail(ctx context.Context, email string) (int64, error) {
	var count int64
	err := getDB(ctx).Table("w_users").Where("email = ?", email).Count(&count).Error
	return count, err
}

// countOtherUsersByEmail 统计除指定用户外的同邮箱数量
func countOtherUsersByEmail(ctx context.Context, email string, id uint64) (int64, error) {
	var count int64
	err := getDB(ctx).Table("w_users").Where("email = ? AND id != ?", email, id).Count(&count).Error
	return count, err
}

// adminListUserRows 后台条件分页查询用户
func adminListUserRows(ctx context.Context, filter contracts.AdminListUsersFilter) (int64, []*contracts.UserDTO, error) {
	query := getDB(ctx).Table("w_users")
	if filter.UserID != nil {
		query = query.Where("id = ?", *filter.UserID)
	}
	if filter.Username != "" {
		query = query.Where("username LIKE ? ESCAPE '\\'", util.EscapeLike(filter.Username)+"%")
	}
	if filter.Email != "" {
		query = query.Where("email LIKE ? ESCAPE '\\'", util.EscapeLike(filter.Email)+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	var users []*contracts.UserDTO
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.
		Select(adminUserColumns).
		Order("id ASC").
		Offset(offset).
		Limit(filter.PageSize).
		Find(&users).Error; err != nil {
		return 0, nil, err
	}
	return total, users, nil
}

// adminGetUserRow 后台按 ID 读取用户视图
func adminGetUserRow(ctx context.Context, id uint64) (*contracts.UserDTO, error) {
	var user contracts.UserDTO
	if err := getDB(ctx).Table("w_users").
		Select(adminUserColumns).
		Where("id = ?", id).
		First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// getUserRow 按 ID 整行读取用户视图
func getUserRow(ctx context.Context, id uint64) (*contracts.UserDTO, error) {
	var user contracts.UserDTO
	if err := getDB(ctx).Table("w_users").Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// insertUserRow 以显式列映射写入用户行
func insertUserRow(ctx context.Context, row map[string]any) error {
	return getDB(ctx).Table("w_users").Create(row).Error
}

// getUserAdminFlags 读取指定用户的管理员标记
func getUserAdminFlags(ctx context.Context, id uint64) (userAdminFlags, error) {
	var flags userAdminFlags
	if err := getDB(ctx).Table("w_users").Select("id, is_admin").Where("id = ?", id).First(&flags).Error; err != nil {
		return userAdminFlags{}, err
	}
	return flags, nil
}

// deleteUserCascadeAdmin 在事务中删除用户的访问令牌、外部账号绑定与用户行
func deleteUserCascadeAdmin(ctx context.Context, id uint64) error {
	return getDB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("w_access_tokens").Where("user_id = ?", id).Delete(map[string]any{}).Error; err != nil {
			return err
		}
		if err := tx.Table("w_external_accounts").Where("user_id = ?", id).Delete(map[string]any{}).Error; err != nil {
			return err
		}
		return tx.Table("w_users").Where("id = ?", id).Delete(map[string]any{}).Error
	})
}

// listAccessTokensByUser 列出指定用户的访问令牌
func listAccessTokensByUser(ctx context.Context, userID uint64) ([]AccessToken, error) {
	var tokens []AccessToken
	err := getDB(ctx).Where("user_id = ?", userID).Find(&tokens).Error
	return tokens, err
}

// getAccessTokenOfUser 按 ID 与所属用户读取访问令牌
func getAccessTokenOfUser(ctx context.Context, id, userID uint64) (*AccessToken, error) {
	var token AccessToken
	if err := getDB(ctx).Where("id = ? AND user_id = ?", id, userID).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

// createAccessTokenRow 写入访问令牌记录
func createAccessTokenRow(ctx context.Context, token *AccessToken) error {
	return getDB(ctx).Create(token).Error
}

// saveAccessTokenRow 全量保存访问令牌记录
func saveAccessTokenRow(ctx context.Context, token *AccessToken) error {
	return getDB(ctx).Save(token).Error
}

// deleteAccessTokenRow 删除访问令牌记录
func deleteAccessTokenRow(ctx context.Context, token *AccessToken) error {
	return getDB(ctx).Delete(token).Error
}
