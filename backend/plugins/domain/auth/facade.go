// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"Wavelet/core/contracts"
	"Wavelet/plugins/domain/auth/consts"
	"Wavelet/plugins/domain/auth/controller"
	"Wavelet/plugins/domain/auth/dao"
	"Wavelet/plugins/domain/auth/model/do"
	"Wavelet/plugins/domain/auth/model/dto"
	"Wavelet/plugins/domain/auth/model/entity"
	"Wavelet/plugins/domain/auth/service"
	"context"
	"net/http"
	"sync"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Exported Type Aliases for Backward Compatibility
//
//nolint:revive // backward compatibility type aliases with legacy naming
type (
	AuthSource             = entity.AuthSource
	ExternalAccount        = entity.ExternalAccount
	CachedToken            = do.CachedToken
	CapRuntimeSettings     = do.CapRuntimeSettings
	AuthSourceView         = dto.AuthSourceView
	BasicUserInfo          = dto.BasicUserInfo
	OAuthAuthorizeResponse = dto.OAuthAuthorizeResponse
	OAuthCallbackResult    = dto.OAuthCallbackResult
	CallbackRequest        = dto.CallbackRequest
	ChallengeResponse      = dto.ChallengeResponse
	RedeemResponse         = dto.RedeemResponse
	CaptchaManager         = service.CaptchaManager
)

// Exported Constant Aliases for Backward Compatibility
const (
	UserNameKey     = consts.UserNameKey
	UserIDKey       = consts.UserIDKey
	UserObjKey      = consts.UserObjKey
	TokenAuthKey    = consts.TokenAuthKey
	TokenAdminKey   = consts.TokenAdminKey
	SessionTokenKey = consts.SessionTokenKey
	PasswordHashKey = consts.PasswordHashKey
	SystemUsername  = consts.SystemUsername

	OAuthStateCacheKeyFormat     = consts.OAuthStateCacheKeyFormat
	OAuthStateCacheKeyExpiration = consts.OAuthStateCacheKeyExpiration
	OAuthPurposeLogin            = consts.OAuthPurposeLogin
	OAuthPurposeBind             = consts.OAuthPurposeBind
	AuthSourceTypeOIDC           = consts.AuthSourceTypeOIDC

	ErrTokenAuthNotAllowed = consts.ErrTokenAuthNotAllowed
)

var (
	defaultMu      sync.RWMutex
	defaultDAO     = dao.New(nil, nil, nil)
	defaultService = service.New(defaultDAO, SessionConfig{SessionCookieName: "wavelet_session", SessionAge: 86400, SessionHTTPOnly: true}, nil)
	defaultCtrl    = controller.New(defaultService)
)

func setDefaultRuntime(d *dao.DAO, s *service.Service, c *controller.Controller) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultDAO = d
	defaultService = s
	defaultCtrl = c
}

func getDefaultRuntime() (*dao.DAO, *service.Service, *controller.Controller) {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultDAO, defaultService, defaultCtrl
}

// ParseUserID parses a string, int, or float64 user ID representation.
func ParseUserID(v any) uint64 {
	return dto.ParseUserID(v)
}

// BuildBasicUserInfo converts UserDTO to BasicUserInfo.
func BuildBasicUserInfo(user *contracts.UserDTO, needChange bool) BasicUserInfo {
	return dto.BuildBasicUserInfo(user, needChange)
}

// SetSessionConfig updates the active session configuration.
func SetSessionConfig(cfg SessionConfig) {
	_, s, _ := getDefaultRuntime()
	s.Session.SetConfig(cfg)
}

// GetSessionConfig returns the active session configuration.
func GetSessionConfig() SessionConfig {
	_, s, _ := getDefaultRuntime()
	return s.Session.Config()
}

// GetSessionOptions builds session cookie options based on config and maxAge.
func GetSessionOptions(maxAge int) sessions.Options {
	_, s, _ := getDefaultRuntime()
	return s.Session.GetSessionOptions(maxAge)
}

// StripCookieMaxAgeAndExpires removes max-age and expires from cookie header.
func StripCookieMaxAgeAndExpires(header http.Header, cookieName string) {
	_, s, _ := getDefaultRuntime()
	s.Session.StripCookieMaxAgeAndExpires(header, cookieName)
}

// GetUserIDFromSession extracts user ID from session.
func GetUserIDFromSession(s sessions.Session) uint64 {
	return controller.GetUserIDFromSession(s)
}

// GetUserIDFromContext extracts user ID from Gin context.
func GetUserIDFromContext(c *gin.Context) uint64 {
	return controller.GetUserIDFromContext(c)
}

// SetLoginSession sets the login session for the authenticated user.
func SetLoginSession(ctx context.Context, c *gin.Context, user *contracts.UserDTO, extras ...map[string]any) error {
	_, s, _ := getDefaultRuntime()
	session := sessions.Default(c)
	isSessionCookie, err := s.Session.ApplyLoginSession(ctx, session, user, extras...)
	if err != nil {
		return err
	}
	if isSessionCookie {
		s.Session.StripCookieMaxAgeAndExpires(c.Writer.Header(), s.Session.Config().SessionCookieName)
	}
	return nil
}

// RegisterWhitelist adds whitelist path patterns.
func RegisterWhitelist(patterns ...string) {
	_, _, c := getDefaultRuntime()
	c.RegisterWhitelist(patterns...)
}

// IsWhitelisted checks if the path matches the auth whitelist.
func IsWhitelisted(path string) bool {
	_, _, c := getDefaultRuntime()
	if wl := c.Whitelist(); wl != nil {
		return wl.Match(path)
	}
	return false
}

// GetUserFromRequest extracts user from Request (Token or Session).
func GetUserFromRequest(c *gin.Context) (*contracts.UserDTO, error) {
	d, _, _ := getDefaultRuntime()
	return controller.GetUserFromRequest(c, d)
}

// LoginRequired returns authentication required middleware.
func LoginRequired() gin.HandlerFunc {
	_, _, c := getDefaultRuntime()
	return c.LoginRequired()
}

// AdminRequired returns admin authorization middleware.
func AdminRequired() gin.HandlerFunc {
	_, _, c := getDefaultRuntime()
	return c.AdminRequired()
}

// LoginAdminRequired alias for AdminRequired.
func LoginAdminRequired() gin.HandlerFunc {
	return AdminRequired()
}

// DisallowTokenAuth returns middleware rejecting access token requests.
func DisallowTokenAuth() gin.HandlerFunc {
	return controller.DisallowTokenAuth()
}

// GetCachedToken reads cached access token.
func GetCachedToken(ctx context.Context, tokenHash string) (*CachedToken, error) {
	d, _, _ := getDefaultRuntime()
	return d.GetCachedToken(ctx, tokenHash)
}

// SetCachedToken stores access token into cache.
func SetCachedToken(ctx context.Context, tokenHash string, token *CachedToken) {
	d, _, _ := getDefaultRuntime()
	d.SetCachedToken(ctx, tokenHash, token)
}

// InvalidateCachedToken invalidates access token cache.
func InvalidateCachedToken(ctx context.Context, tokenHash string) {
	d, _, _ := getDefaultRuntime()
	d.InvalidateCachedToken(ctx, tokenHash)
}

// GetCachedUser reads cached user.
func GetCachedUser(ctx context.Context, userID uint64) (*contracts.UserDTO, error) {
	d, _, _ := getDefaultRuntime()
	return d.GetCachedUser(ctx, userID)
}

// SetCachedUser stores user into cache.
func SetCachedUser(ctx context.Context, userID uint64, u *contracts.UserDTO) {
	d, _, _ := getDefaultRuntime()
	d.SetCachedUser(ctx, userID, u)
}

// InvalidateCachedUser invalidates user cache.
func InvalidateCachedUser(ctx context.Context, userID uint64) {
	d, _, _ := getDefaultRuntime()
	d.InvalidateCachedUser(ctx, userID)
}

// ResetAuthRAMCacheForTest clears RAM caches.
func ResetAuthRAMCacheForTest() {
	dao.ResetRAMCacheForTest()
}

// StopAuthCacheListener compatibility stub.
func StopAuthCacheListener() {}

// SetCapSecret sets CAPTCHA secret.
func SetCapSecret(secret []byte) {
	_, s, _ := getDefaultRuntime()
	s.CapManager.SetSecret(secret)
}

// GetDefaultCapManager returns the singleton CAPTCHA manager.
func GetDefaultCapManager() *CaptchaManager {
	_, s, _ := getDefaultRuntime()
	return s.CapManager
}

// CurrentCapSettings returns current CAPTCHA runtime settings.
func CurrentCapSettings(ctx context.Context) (CapRuntimeSettings, error) {
	_, s, _ := getDefaultRuntime()
	return s.CapSettings.Current(ctx)
}

// CapProtectionEnabled checks if CAPTCHA is enabled.
func CapProtectionEnabled(ctx context.Context) bool {
	_, s, _ := getDefaultRuntime()
	return s.CapSettings.CapProtectionEnabled(ctx)
}

// InvalidateCapRuntimeSettings invalidates runtime CAPTCHA settings cache.
func InvalidateCapRuntimeSettings() {
	_, s, _ := getDefaultRuntime()
	s.CapSettings.Invalidate()
}

// ResetCapRuntimeSettingsForTest clears test CAPTCHA settings.
func ResetCapRuntimeSettingsForTest() {
	InvalidateCapRuntimeSettings()
}

// InstallCapTestRuntimeSettings installs a test snapshot.
func InstallCapTestRuntimeSettings(settings CapRuntimeSettings) func() {
	_, s, _ := getDefaultRuntime()
	return s.CapSettings.InstallTestSnapshot(settings)
}

// VerifyCaptchaMiddleware returns captcha verification middleware.
func VerifyCaptchaMiddleware(mgr *service.CaptchaManager, scope string) gin.HandlerFunc {
	_, s, _ := getDefaultRuntime()
	return controller.VerifyCaptchaMiddleware(mgr, s.CapSettings, scope)
}

// Challenge HTTP handler.
func Challenge(c *gin.Context) {
	_, _, ctrl := getDefaultRuntime()
	ctrl.Captcha.Challenge(c)
}

// Redeem HTTP handler.
func Redeem(c *gin.Context) {
	_, _, ctrl := getDefaultRuntime()
	ctrl.Captcha.Redeem(c)
}

// GetLoginSources HTTP handler.
func GetLoginSources(c *gin.Context) {
	_, _, ctrl := getDefaultRuntime()
	ctrl.OAuth.GetLoginSources(c)
}

// GetLoginURL HTTP handler.
func GetLoginURL(c *gin.Context) {
	_, _, ctrl := getDefaultRuntime()
	ctrl.OAuth.GetLoginURL(c)
}

// Authorize HTTP handler.
func Authorize(c *gin.Context) {
	_, _, ctrl := getDefaultRuntime()
	ctrl.OAuth.Authorize(c)
}

// Callback HTTP handler.
func Callback(c *gin.Context) {
	_, _, ctrl := getDefaultRuntime()
	ctrl.OAuth.Callback(c)
}

// Logout HTTP handler.
func Logout(c *gin.Context) {
	_, _, ctrl := getDefaultRuntime()
	ctrl.OAuth.Logout(c)
}

// UserInfo HTTP handler.
func UserInfo(c *gin.Context) {
	_, _, ctrl := getDefaultRuntime()
	ctrl.UserInfo.UserInfo(c)
}

// ListExternalAccounts HTTP handler.
func ListExternalAccounts(c *gin.Context) {
	_, _, ctrl := getDefaultRuntime()
	ctrl.OAuth.ListExternalAccounts(c)
}

// DeleteExternalAccount HTTP handler.
func DeleteExternalAccount(c *gin.Context) {
	_, _, ctrl := getDefaultRuntime()
	ctrl.OAuth.DeleteExternalAccount(c)
}

// InvalidateOIDCProviderCache invalidates OIDC provider cache entry.
func InvalidateOIDCProviderCache(issuer string) {
	_, s, _ := getDefaultRuntime()
	s.OIDCProviderCache.Invalidate(issuer)
}
