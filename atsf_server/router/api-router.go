package router

import (
	"atsflare/controller"
	"atsflare/middleware"
	"github.com/gin-gonic/gin"
)

func SetApiRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	{
		apiRouter.GET("/status", controller.GetStatus)
		apiRouter.GET("/notice", controller.GetNotice)
		apiRouter.GET("/about", controller.GetAbout)
		apiRouter.GET("/verification", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendEmailVerification)
		apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendPasswordResetEmail)
		apiRouter.POST("/user/reset", middleware.CriticalRateLimit(), controller.ResetPassword)
		apiRouter.GET("/oauth/github", middleware.CriticalRateLimit(), controller.GitHubOAuth)
		apiRouter.GET("/oauth/wechat", middleware.CriticalRateLimit(), controller.WeChatAuth)
		apiRouter.GET("/oauth/wechat/bind", middleware.CriticalRateLimit(), middleware.UserAuth(), controller.WeChatBind)
		apiRouter.GET("/oauth/email/bind", middleware.CriticalRateLimit(), middleware.UserAuth(), controller.EmailBind)

		userRoute := apiRouter.Group("/user")
		{
			userRoute.POST("/register", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.Register)
			userRoute.POST("/login", middleware.CriticalRateLimit(), controller.Login)
			userRoute.GET("/logout", controller.Logout)

			selfRoute := userRoute.Group("/")
			selfRoute.Use(middleware.UserAuth(), middleware.NoTokenAuth())
			{
				selfRoute.GET("/self", controller.GetSelf)
				selfRoute.PUT("/self", controller.UpdateSelf)
				selfRoute.DELETE("/self", controller.DeleteSelf)
				selfRoute.GET("/token", controller.GenerateToken)
			}

			adminRoute := userRoute.Group("/")
			adminRoute.Use(middleware.AdminAuth(), middleware.NoTokenAuth())
			{
				adminRoute.GET("/", controller.GetAllUsers)
				adminRoute.GET("/search", controller.SearchUsers)
				adminRoute.GET("/:id", controller.GetUser)
				adminRoute.POST("/", controller.CreateUser)
				adminRoute.POST("/manage", controller.ManageUser)
				adminRoute.PUT("/", controller.UpdateUser)
				adminRoute.DELETE("/:id", controller.DeleteUser)
			}
		}
		optionRoute := apiRouter.Group("/option")
		optionRoute.Use(middleware.RootAuth(), middleware.NoTokenAuth())
		{
			optionRoute.GET("/", controller.GetOptions)
			optionRoute.PUT("/", controller.UpdateOption)
		}
		fileRoute := apiRouter.Group("/file")
		fileRoute.Use(middleware.AdminAuth())
		{
			fileRoute.GET("/", controller.GetAllFiles)
			fileRoute.GET("/search", controller.SearchFiles)
			fileRoute.POST("/", middleware.UploadRateLimit(), controller.UploadFile)
			fileRoute.DELETE("/:id", controller.DeleteFile)
		}
		proxyRoute := apiRouter.Group("/proxy-routes")
		proxyRoute.Use(middleware.AdminAuth())
		{
			proxyRoute.GET("/", controller.GetProxyRoutes)
			proxyRoute.POST("/", controller.CreateProxyRoute)
			proxyRoute.PUT("/:id", controller.UpdateProxyRoute)
			proxyRoute.DELETE("/:id", controller.DeleteProxyRoute)
		}
		managedDomainRoute := apiRouter.Group("/managed-domains")
		managedDomainRoute.Use(middleware.AdminAuth())
		{
			managedDomainRoute.GET("/", controller.GetManagedDomains)
			managedDomainRoute.GET("/match", controller.MatchManagedDomainCertificate)
			managedDomainRoute.POST("/", controller.CreateManagedDomain)
			managedDomainRoute.PUT("/:id", controller.UpdateManagedDomain)
			managedDomainRoute.DELETE("/:id", controller.DeleteManagedDomain)
		}
		tlsCertificateRoute := apiRouter.Group("/tls-certificates")
		tlsCertificateRoute.Use(middleware.AdminAuth())
		{
			tlsCertificateRoute.GET("/", controller.GetTLSCertificates)
			tlsCertificateRoute.POST("/", controller.CreateTLSCertificate)
			tlsCertificateRoute.POST("/import-file", controller.ImportTLSCertificateFile)
			tlsCertificateRoute.DELETE("/:id", controller.DeleteTLSCertificate)
		}
		configVersionRoute := apiRouter.Group("/config-versions")
		configVersionRoute.Use(middleware.AdminAuth())
		{
			configVersionRoute.GET("/", controller.GetConfigVersions)
			configVersionRoute.GET("/active", controller.GetActiveConfigVersion)
			configVersionRoute.GET("/preview", controller.PreviewConfigVersion)
			configVersionRoute.GET("/diff", controller.DiffConfigVersion)
			configVersionRoute.POST("/publish", controller.PublishConfigVersion)
			configVersionRoute.PUT("/:id/activate", controller.ActivateConfigVersion)
		}
		nodeRoute := apiRouter.Group("/nodes")
		nodeRoute.Use(middleware.AdminAuth())
		{
			nodeRoute.GET("/bootstrap-token", controller.GetNodeBootstrapToken)
			nodeRoute.POST("/bootstrap-token/rotate", controller.RotateNodeBootstrapToken)
			nodeRoute.GET("/", controller.GetNodes)
			nodeRoute.POST("/", controller.CreateNode)
			nodeRoute.PUT("/:id", controller.UpdateNode)
			nodeRoute.DELETE("/:id", controller.DeleteNode)
		}
		applyLogRoute := apiRouter.Group("/apply-logs")
		applyLogRoute.Use(middleware.AdminAuth())
		{
			applyLogRoute.GET("/", controller.GetApplyLogs)
		}
		agentRoute := apiRouter.Group("/agent")
		{
			discoveryRoute := agentRoute.Group("/")
			discoveryRoute.Use(middleware.AgentRegisterAuth())
			{
				discoveryRoute.POST("/nodes/register", controller.AgentRegister)
			}
			authorizedRoute := agentRoute.Group("/")
			authorizedRoute.Use(middleware.AgentAuth())
			{
				authorizedRoute.POST("/nodes/heartbeat", controller.AgentHeartbeat)
				authorizedRoute.GET("/config-versions/active", controller.AgentGetActiveConfig)
				authorizedRoute.POST("/apply-logs", controller.AgentReportApplyLog)
			}
		}
	}
}
