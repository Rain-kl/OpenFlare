// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db/idgen"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

func listUsers(ctx context.Context, req listUsersRequest) (int64, []model.User, error) {
	return repository.ListAdminUsers(ctx, repository.AdminUserListFilter{
		UserID:   req.UserID,
		Username: strings.TrimSpace(req.Username),
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
	return repository.UpdateUserActive(ctx, id, active)
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
	return repository.DeleteUserWithRelations(ctx, targetID)
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
