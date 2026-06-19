// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package v1 contains router registrations for API V1
package v1

import (
	capApp "github.com/Rain-kl/Wavelet/internal/apps/cap"
	publicconfig "github.com/Rain-kl/Wavelet/internal/apps/config"
	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/apps/upload"
	"github.com/Rain-kl/Wavelet/internal/apps/user"
	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes registers all user-related, oauth, upload, and public routes.
func RegisterUserRoutes(apiV1Router *gin.RouterGroup, apiGroup *gin.RouterGroup) {
	// 1. CAPTCHA
	registerCaptchaRoutes(apiGroup)

	// 2. Config (public)
	registerConfigRoutes(apiV1Router)

	// 3. OAuth
	registerOAuthRoutes(apiV1Router)

	// 4. User
	registerUserRoutes(apiV1Router)

	// 5. Upload
	registerUploadRoutes(apiV1Router)
}

func registerCaptchaRoutes(apiGroup *gin.RouterGroup) {
	capGroup := apiGroup.Group("/cap")
	{
		capGroup.POST("/challenge", capApp.Challenge)
		capGroup.POST("/redeem", capApp.Redeem)
	}
}

func registerConfigRoutes(apiV1Router *gin.RouterGroup) {
	configRouter := apiV1Router.Group("/config")
	{
		configRouter.GET("/public", publicconfig.GetPublicConfig)
	}
}

func registerOAuthRoutes(apiV1Router *gin.RouterGroup) {
	apiV1Router.GET("/oauth/sources", oauth.GetLoginSources)
	apiV1Router.GET("/oauth/login", oauth.GetLoginURL)
	apiV1Router.GET("/oauth/:source/authorize", oauth.Authorize)
	apiV1Router.GET("/oauth/logout", oauth.Logout)
	apiV1Router.POST("/oauth/callback", oauth.Callback)
	apiV1Router.GET("/oauth/user-info", oauth.LoginRequired(), oauth.UserInfo)
	apiV1Router.GET("/user-info", oauth.LoginRequired(), oauth.UserInfo)
	apiV1Router.GET("/oauth/external-accounts", oauth.LoginRequired(), oauth.ListExternalAccounts)
	apiV1Router.POST("/oauth/external-accounts/:id/delete", oauth.LoginRequired(), oauth.DeleteExternalAccount)
}

func registerUserRoutes(apiV1Router *gin.RouterGroup) {
	userRouter := apiV1Router.Group("/user")
	{
		userRouter.POST("/login", capApp.VerifyMiddleware(capApp.GetDefaultManager(), "login"), user.Login)
		userRouter.POST("/register", capApp.VerifyMiddleware(capApp.GetDefaultManager(), "register"), user.Register)
		userRouter.POST("/send-email-code", capApp.VerifyMiddleware(capApp.GetDefaultManager(), "send_email_code"), user.SendEmailCode)
		userRouter.GET("/logout", user.Logout)
		userRouter.GET("/self", oauth.LoginRequired(), oauth.UserInfo)
		userRouter.POST("/change-password", oauth.LoginRequired(), user.ChangePassword)
		userRouter.PUT("/profile", oauth.LoginRequired(), user.UpdateProfile)

		// Access Token
		tokenRouter := userRouter.Group("/access-tokens")
		tokenRouter.Use(oauth.LoginRequired(), oauth.DisallowTokenAuth())
		{
			tokenRouter.GET("", user.ListAccessTokens)
			tokenRouter.POST("", user.CreateAccessToken)
			tokenRouter.DELETE("/:id", user.DeleteAccessToken)
			tokenRouter.POST("/:id/rotate", user.RotateAccessToken)
		}
	}
}

func registerUploadRoutes(apiV1Router *gin.RouterGroup) {
	uploadRouter := apiV1Router.Group("/upload")
	uploadRouter.Use(oauth.LoginRequired())
	{
		uploadRouter.POST("", upload.UploadFile)
		uploadRouter.GET("/my", upload.ListMyFiles)
		uploadRouter.DELETE("/:id", upload.DeleteMyFile)
		uploadRouter.PUT("/:id", upload.UpdateMyFile)
		uploadRouter.GET("/download/:id", upload.DownloadFile)
		uploadRouter.POST("/download/batch", upload.BatchDownloadFiles)
	}
}
