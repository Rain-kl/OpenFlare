// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetUserIDFromSession 从 Session 中提取用户 ID
func GetUserIDFromSession(s sessions.Session) uint64 {
	userID, ok := s.Get(UserIDKey).(uint64)
	if !ok {
		return 0
	}
	return userID
}

// GetUserIDFromContext 从 Gin Context 的 Session 中提取用户 ID
func GetUserIDFromContext(c *gin.Context) uint64 {
	session := sessions.Default(c)
	return GetUserIDFromSession(session)
}

func ensureSessionToken(s sessions.Session) (string, bool) {
	token, ok := s.Get(SessionTokenKey).(string)
	if !ok || token == "" {
		token = uuid.NewString()
		s.Set(SessionTokenKey, token)
		return token, true
	}
	return token, false
}

func hashSessionToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

func setLoginSession(ctx context.Context, c *gin.Context, user *model.User) error {
	session := sessions.Default(c)
	session.Set(UserIDKey, user.ID)
	session.Set(UserNameKey, user.Username)
	session.Set(PasswordHashKey, user.Password)

	// 根据系统配置动态设置 Session 过期时间
	maxAge := config.Config.App.SessionAge
	isSessionCookie := false

	ttlHours, err := repository.GetIntByKey(ctx, model.ConfigKeyLoginSessionTTLHours)
	if err == nil {
		switch {
		case ttlHours == -1:
			// 永不过期，设置为 10 年
			maxAge = 10 * 365 * 24 * 3600
		case ttlHours > 0:
			maxAge = ttlHours * 3600
		case ttlHours == 0:
			isSessionCookie = true
		}
	}
	session.Options(GetSessionOptions(maxAge))

	if err := session.Save(); err != nil {
		return err
	}

	if isSessionCookie {
		StripCookieMaxAgeAndExpires(c.Writer.Header(), config.Config.App.SessionCookieName)
	}

	return nil
}
