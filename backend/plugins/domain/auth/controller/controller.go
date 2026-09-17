// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package controller provides HTTP handlers and middlewares for the auth plugin.
package controller

import (
	"Wavelet/core/extpoints"
	"Wavelet/plugins/domain/auth/service"

	"github.com/gin-gonic/gin"
)

// Controller aggregates all HTTP handlers and middlewares for the auth plugin.
type Controller struct {
	svc       *service.Service
	whitelist *extpoints.PathWhitelist

	OAuth    *OAuthHandler
	UserInfo *UserInfoHandler
	Captcha  *CaptchaHandler
}

// New creates a new Controller instance.
func New(svc *service.Service) *Controller {
	wl := extpoints.NewPathWhitelist()
	oauthHandler := NewOAuthHandler(svc.OAuth, svc.Session, svc.DAO)
	userInfoHandler := NewUserInfoHandler()
	captchaHandler := NewCaptchaHandler(svc.CapManager)

	c := &Controller{
		svc:       svc,
		whitelist: wl,
		OAuth:     oauthHandler,
		UserInfo:  userInfoHandler,
		Captcha:   captchaHandler,
	}

	// Wire middlewares into AuthService
	svc.AuthSvc.SetMiddlewareHandlers(
		c.LoginRequired(),
		c.AdminRequired(),
		DisallowTokenAuth(),
		CurrentUserIDFromRequestContext,
	)

	return c
}

// Whitelist returns the whitelist tracker.
func (c *Controller) Whitelist() *extpoints.PathWhitelist {
	return c.whitelist
}

// RegisterWhitelist adds path patterns that bypass authentication.
func (c *Controller) RegisterWhitelist(patterns ...string) {
	if c.whitelist != nil {
		c.whitelist.Add(patterns...)
	}
}

// LoginRequired returns the authentication middleware.
func (c *Controller) LoginRequired() gin.HandlerFunc {
	return LoginRequiredMiddleware(c.whitelist, c.svc.DAO)
}

// AdminRequired returns the admin authorization middleware.
func (c *Controller) AdminRequired() gin.HandlerFunc {
	return AdminRequiredMiddleware(c.svc.DAO)
}

// DisallowTokenAuth returns the token rejection middleware.
func (c *Controller) DisallowTokenAuth() gin.HandlerFunc {
	return DisallowTokenAuth()
}

// VerifyCaptcha returns the captcha challenge verification middleware.
func (c *Controller) VerifyCaptcha(scope string) gin.HandlerFunc {
	return VerifyCaptchaMiddleware(c.svc.CapManager, c.svc.CapSettings, scope)
}

// RegisterRoutes mounts all auth endpoints onto the router.
func (c *Controller) RegisterRoutes(router extpoints.RouterExtension) {
	loginReq := c.LoginRequired()

	// 1. OAuth endpoints
	oauthGroup := router.Group("/api/v1/oauth")
	{
		oauthGroup.GET("/sources", c.OAuth.GetLoginSources)
		oauthGroup.GET("/login", c.OAuth.GetLoginURL)
		oauthGroup.GET("/:source/authorize", c.OAuth.Authorize)
		oauthGroup.GET("/logout", c.OAuth.Logout)
		oauthGroup.POST("/callback", c.OAuth.Callback)
		oauthGroup.GET("/user-info", loginReq, c.UserInfo.UserInfo)
		oauthGroup.GET("/external-accounts", loginReq, c.OAuth.ListExternalAccounts)
		oauthGroup.POST("/external-accounts/:id/delete", loginReq, c.OAuth.DeleteExternalAccount)
	}

	// 2. Global user-info route alias
	router.GET("/api/v1/user-info", loginReq, c.UserInfo.UserInfo)

	// 3. CAPTCHA endpoints
	capGroup := router.Group("/api/v1/cap")
	{
		capGroup.GET("/challenge", c.Captcha.Challenge)
		capGroup.POST("/challenge", c.Captcha.Challenge)
		capGroup.POST("/redeem", c.Captcha.Redeem)
	}
}
