// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

func uniqueUsername(ctx context.Context, base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "user"
	}

	var existingUsernames []string
	if err := db.DB(ctx).Model(&model.User{}).
		Where("username = ? OR username LIKE ?", base, base+"-%").
		Pluck("username", &existingUsernames).Error; err != nil {
		return "", err
	}

	// 将现有的用户名放入 map 中，以便 O(1) 查找
	exists := make(map[string]bool, len(existingUsernames))
	for _, u := range existingUsernames {
		exists[strings.ToLower(u)] = true
	}

	// 检查 base 是否被占用
	if !exists[strings.ToLower(base)] {
		return base, nil
	}

	// 顺序查找第一个可用的带后缀用户名
	for i := 1; i <= 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !exists[strings.ToLower(candidate)] {
			return candidate, nil
		}
	}

	return "", errors.New(errUsernameGenerateFailed)
}

func buildOAuthUserInfo(ctx context.Context, source *model.AuthSource, code string, nonce string, redirectURL string) (*model.OAuthUserInfo, error) {
	authConfig, verifier, err := buildOAuthConfig(ctx, source, redirectURL)
	if err != nil {
		return nil, err
	}

	token, err := authConfig.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}

	userInfo := &model.OAuthUserInfo{Active: true}
	if verifier != nil {
		if verifyErr := verifyIDToken(ctx, verifier, token, nonce, userInfo); verifyErr != nil {
			return nil, verifyErr
		}
	}

	if userInfo.Username == "" && userInfo.PreferredUsername != "" {
		userInfo.Username = userInfo.PreferredUsername
	}
	if userInfo.Username == "" && userInfo.Email != "" {
		userInfo.Username = strings.Split(userInfo.Email, "@")[0]
	}
	if userInfo.Username == "" && userInfo.Sub != "" {
		userInfo.Username = userInfo.Sub
	}
	if userInfo.Name == "" {
		userInfo.Name = userInfo.Username
	}

	return userInfo, nil
}

// verifyIDToken 验证 OIDC ID Token 并将 Claims 解析到 userInfo
func verifyIDToken(ctx context.Context, verifier *oidc.IDTokenVerifier, token *oauth2.Token, nonce string, userInfo *model.OAuthUserInfo) error {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil
	}
	idToken, verifyErr := verifier.Verify(ctx, rawIDToken)
	if verifyErr != nil {
		return fmt.Errorf(errIDTokenVerifyFailedFormat, errIDTokenVerifyFailed, verifyErr)
	}
	if nonce != "" && idToken.Nonce != nonce {
		return errors.New(errNonceMismatch)
	}
	if claimsErr := idToken.Claims(userInfo); claimsErr != nil {
		return claimsErr
	}
	return nil
}

func normalizeOAuthUserInfo(userInfo *model.OAuthUserInfo) error {
	userInfo.Username = strings.TrimSpace(userInfo.Username)
	userInfo.PreferredUsername = strings.TrimSpace(userInfo.PreferredUsername)
	userInfo.Email = strings.TrimSpace(userInfo.Email)
	userInfo.Name = strings.TrimSpace(userInfo.Name)
	userInfo.AvatarURL = strings.TrimSpace(userInfo.AvatarURL)

	if userInfo.Username == "" && userInfo.PreferredUsername != "" {
		userInfo.Username = userInfo.PreferredUsername
	}
	if userInfo.Username == "" && userInfo.Email != "" {
		userInfo.Username = strings.Split(userInfo.Email, "@")[0]
	}
	if userInfo.Username == "" && userInfo.Sub != "" {
		userInfo.Username = userInfo.Sub
	}
	if userInfo.Username == "" {
		return errors.New(errUsernameFromSourceFailed)
	}
	if userInfo.Name == "" {
		userInfo.Name = userInfo.Username
	}
	if !userInfo.Active {
		userInfo.Active = true
	}
	return nil
}

func buildCallbackResult(user *model.User, status string) OAuthCallbackResult {
	result := OAuthCallbackResult{Status: status}
	if user != nil {
		info := BuildBasicUserInfo(user, false)
		result.User = &info
	}
	return result
}
