// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/idgen"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

func listUsers(ctx context.Context, req listUsersRequest) (int64, []model.User, error) {
	return repository.ListAdminUsers(ctx, repository.AdminUserListFilter{
		UserID:   req.UserID,
		Username: strings.TrimSpace(req.Username),
		Email:    strings.TrimSpace(req.Email),
		Page:     req.Page,
		PageSize: req.PageSize,
	})
}

func getUserDetail(ctx context.Context, id uint64) (model.User, error) {
	return repository.GetAdminUserDetail(ctx, id)
}

func updateUserStatus(ctx context.Context, id uint64, active bool) error {
	flags, err := repository.GetUserAdminFlags(ctx, id)
	if err != nil {
		return err
	}
	if !active && flags.IsAdmin {
		return errors.New(cannotDisable)
	}

	var tokens []model.AccessToken
	if !active {
		tokens, _ = repository.ListAccessTokensByUserID(ctx, id)
	}

	err = repository.UpdateUserActive(ctx, id, active)
	if err == nil {
		oauth.InvalidateCachedUser(ctx, id)
		if !active {
			for _, token := range tokens {
				oauth.InvalidateCachedToken(ctx, token.TokenHash)
			}
		}
	}
	return err
}

func deleteUser(ctx context.Context, currentUserID, targetID uint64) error {
	if currentUserID == targetID {
		return errors.New(cannotDeleteSelf)
	}
	flags, err := repository.GetUserAdminFlags(ctx, targetID)
	if err != nil {
		return err
	}
	if flags.IsAdmin {
		return errors.New(cannotDelete)
	}

	tokens, _ := repository.ListAccessTokensByUserID(ctx, targetID)

	err = repository.DeleteUserWithRelations(ctx, targetID)
	if err == nil {
		oauth.InvalidateCachedUser(ctx, targetID)
		for _, token := range tokens {
			oauth.InvalidateCachedToken(ctx, token.TokenHash)
		}
	}
	return err
}

func createUser(ctx context.Context, req createUserRequest) (model.User, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Password = strings.TrimSpace(req.Password)
	req.Email = strings.TrimSpace(req.Email)

	if req.Username == "" {
		return model.User{}, errors.New(usernameRequired)
	}
	if req.Email == "" {
		return model.User{}, errors.New(emailRequired)
	}
	if len(req.Password) < minPasswordLength {
		return model.User{}, errors.New(passwordTooShort)
	}

	count, err := repository.CountUsersByUsername(ctx, req.Username)
	if err != nil {
		return model.User{}, err
	}
	if count > 0 {
		return model.User{}, errors.New(usernameExists)
	}

	emailCount, err := repository.CountUsersByEmail(ctx, req.Email)
	if err != nil {
		return model.User{}, err
	}
	if emailCount > 0 {
		return model.User{}, errors.New(emailExists)
	}

	newUser := model.User{
		ID:          idgen.NextUint64ID(),
		Username:    req.Username,
		Nickname:    req.Nickname,
		Email:       req.Email,
		IsActive:    req.IsActive,
		IsAdmin:     req.IsAdmin,
		LastLoginAt: time.Time{},
	}
	if newUser.Nickname == "" {
		newUser.Nickname = req.Username
	}
	if err := newUser.SetEncryptedPassword(req.Password); err != nil {
		return model.User{}, err
	}
	if err := repository.CreateUser(ctx, &newUser); err != nil {
		return model.User{}, err
	}
	return newUser, nil
}

type updateUserParam struct {
	ID       uint64
	Nickname string
	Email    string
	IsAdmin  bool
	Password string
}

func updateUser(ctx context.Context, currentUserID uint64, param updateUserParam) error {
	param.Nickname = strings.TrimSpace(param.Nickname)
	param.Email = strings.TrimSpace(param.Email)
	param.Password = strings.TrimSpace(param.Password)

	if param.Email == "" {
		return errors.New(emailRequired)
	}

	targetUser, err := repository.GetAdminUserDetail(ctx, param.ID)
	if err != nil {
		return err
	}

	// 不能撤销当前登录用户的管理员权限
	if currentUserID == param.ID && !param.IsAdmin && targetUser.IsAdmin {
		return errors.New(cannotRevokeSelfAdmin)
	}

	// 如果修改了邮箱，检查邮箱是否被其他用户占用
	if targetUser.Email != param.Email {
		count, err := repository.CountUsersByEmail(ctx, param.Email)
		if err != nil {
			return err
		}
		if count > 0 {
			return errors.New(emailExists)
		}
	}

	// 密码强度校验（如果输入了新密码）
	if param.Password != "" && len(param.Password) < minPasswordLength {
		return errors.New(passwordTooShort)
	}

	// 是否需要撤销 Token (重置密码或取消管理员)
	needRevokeTokens := (param.Password != "") || (targetUser.IsAdmin && !param.IsAdmin)
	var tokens []model.AccessToken
	if needRevokeTokens {
		tokens, _ = repository.ListAccessTokensByUserID(ctx, param.ID)
	}

	// 更新字段
	targetUser.Nickname = param.Nickname
	if targetUser.Nickname == "" {
		targetUser.Nickname = targetUser.Username
	}
	targetUser.Email = param.Email
	targetUser.IsAdmin = param.IsAdmin

	if param.Password != "" {
		if err := targetUser.SetEncryptedPassword(param.Password); err != nil {
			return err
		}
	}

	// 执行更新
	err = repository.UpdateUser(ctx, &targetUser)
	if err == nil {
		oauth.InvalidateCachedUser(ctx, param.ID)
		if needRevokeTokens {
			for _, token := range tokens {
				oauth.InvalidateCachedToken(ctx, token.TokenHash)
			}
		}
	}
	return err
}
