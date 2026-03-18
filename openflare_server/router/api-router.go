package router

import (
	"openflare/controller"
	"openflare/middleware"

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
				selfRoute.POST("/self/update", controller.UpdateSelf)
				selfRoute.POST("/self/delete", controller.DeleteSelf)
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
				adminRoute.POST("/update", controller.UpdateUser)
				adminRoute.POST("/:id/delete", controller.DeleteUser)
			}
		}
		optionRoute := apiRouter.Group("/option")
		optionRoute.Use(middleware.RootAuth(), middleware.NoTokenAuth())
		{
			optionRoute.GET("/", controller.GetOptions)
			optionRoute.POST("/update", controller.UpdateOption)
			optionRoute.POST("/geoip/lookup", controller.LookupGeoIP)
		}
		updateRoute := apiRouter.Group("/update")
		updateRoute.Use(middleware.RootAuth(), middleware.NoTokenAuth())
		{
			updateRoute.GET("/latest-release", controller.GetLatestRelease)
			updateRoute.GET("/logs/ws", controller.StreamServerUpgradeLogs)
			updateRoute.POST("/manual-upload", controller.UploadManualServerBinary)
			updateRoute.POST("/manual-upgrade", controller.ConfirmManualServerUpgrade)
			updateRoute.POST("/upgrade", controller.UpgradeServer)
		}
		fileRoute := apiRouter.Group("/file")
		fileRoute.Use(middleware.AdminAuth())
		{
			fileRoute.GET("/", controller.GetAllFiles)
			fileRoute.GET("/search", controller.SearchFiles)
			fileRoute.POST("/", middleware.UploadRateLimit(), controller.UploadFile)
			fileRoute.POST("/:id/delete", controller.DeleteFile)
		}
		proxyRoute := apiRouter.Group("/proxy-routes")
		proxyRoute.Use(middleware.AdminAuth())
		{
			proxyRoute.GET("/", controller.GetProxyRoutes)
			proxyRoute.POST("/", controller.CreateProxyRoute)
			proxyRoute.POST("/:id/update", controller.UpdateProxyRoute)
			proxyRoute.POST("/:id/delete", controller.DeleteProxyRoute)
		}
		managedDomainRoute := apiRouter.Group("/managed-domains")
		managedDomainRoute.Use(middleware.AdminAuth())
		{
			managedDomainRoute.GET("/", controller.GetManagedDomains)
			managedDomainRoute.GET("/match", controller.MatchManagedDomainCertificate)
			managedDomainRoute.POST("/", controller.CreateManagedDomain)
			managedDomainRoute.POST("/:id/update", controller.UpdateManagedDomain)
			managedDomainRoute.POST("/:id/delete", controller.DeleteManagedDomain)
		}
		tlsCertificateRoute := apiRouter.Group("/tls-certificates")
		tlsCertificateRoute.Use(middleware.AdminAuth())
		{
			tlsCertificateRoute.GET("/", controller.GetTLSCertificates)
			tlsCertificateRoute.GET("/:id", controller.GetTLSCertificate)
			tlsCertificateRoute.GET("/:id/content", controller.GetTLSCertificateContent)
			tlsCertificateRoute.POST("/", controller.CreateTLSCertificate)
			tlsCertificateRoute.POST("/:id/update", controller.UpdateTLSCertificate)
			tlsCertificateRoute.POST("/import-file", controller.ImportTLSCertificateFile)
			tlsCertificateRoute.POST("/:id/delete", controller.DeleteTLSCertificate)
		}
		configVersionRoute := apiRouter.Group("/config-versions")
		configVersionRoute.Use(middleware.AdminAuth())
		{
			configVersionRoute.GET("/", controller.GetConfigVersions)
			configVersionRoute.GET("/active", controller.GetActiveConfigVersion)
			configVersionRoute.GET("/preview", controller.PreviewConfigVersion)
			configVersionRoute.GET("/diff", controller.DiffConfigVersion)
			configVersionRoute.GET("/:id", controller.GetConfigVersion)
			configVersionRoute.POST("/publish", controller.PublishConfigVersion)
			configVersionRoute.POST("/:id/activate", controller.ActivateConfigVersion)
		}
		dashboardRoute := apiRouter.Group("/dashboard")
		dashboardRoute.Use(middleware.AdminAuth())
		{
			dashboardRoute.GET("/overview", controller.GetDashboardOverview)
		}
		nodeRoute := apiRouter.Group("/nodes")
		nodeRoute.Use(middleware.AdminAuth())
		{
			nodeRoute.GET("/bootstrap-token", controller.GetNodeBootstrapToken)
			nodeRoute.POST("/bootstrap-token/rotate", controller.RotateNodeBootstrapToken)
			nodeRoute.GET("/", controller.GetNodes)
			nodeRoute.POST("/", controller.CreateNode)
			nodeRoute.GET("/:id/agent-release", controller.GetNodeAgentRelease)
			nodeRoute.GET("/:id/observability", controller.GetNodeObservability)
			nodeRoute.POST("/:id/observability/cleanup", controller.CleanupNodeHealthEvents)
			nodeRoute.POST("/:id/agent-update", controller.RequestNodeAgentUpdate)
			nodeRoute.POST("/:id/openresty-restart", controller.RequestNodeOpenrestyRestart)
			nodeRoute.POST("/:id/update", controller.UpdateNode)
			nodeRoute.POST("/:id/delete", controller.DeleteNode)
		}
		applyLogRoute := apiRouter.Group("/apply-logs")
		applyLogRoute.Use(middleware.AdminAuth())
		{
			applyLogRoute.GET("/", controller.GetApplyLogs)
			applyLogRoute.POST("/cleanup", controller.CleanupApplyLogs)
		}
		accessLogRoute := apiRouter.Group("/access-logs")
		accessLogRoute.Use(middleware.AdminAuth())
		{
			accessLogRoute.GET("/", controller.GetAccessLogs)
			accessLogRoute.GET("/folds", controller.GetFoldedAccessLogs)
			accessLogRoute.GET("/ip-summary", controller.GetAccessLogIPSummaries)
			accessLogRoute.GET("/ip-summary/trend", controller.GetAccessLogIPTrend)
			accessLogRoute.POST("/cleanup", controller.CleanupAccessLogs)
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
