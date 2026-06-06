package router

import (
	"github.com/rain-kl/openflare/openflare-server/controller"
	"github.com/rain-kl/openflare/openflare-server/middleware"

	"github.com/gin-gonic/gin"
)

func SetApiRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	{
		apiRouter.GET("/status", controller.GetStatus)
		apiRouter.GET("/notice", controller.GetNotice)
		apiRouter.GET("/about", controller.GetAbout)
		apiRouter.GET("/verification", middleware.CriticalRateLimit(), controller.SendEmailVerification)
		apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), controller.SendPasswordResetEmail)
		apiRouter.POST("/user/reset", middleware.CriticalRateLimit(), controller.ResetPassword)
		apiRouter.GET("/oauth/github", middleware.CriticalRateLimit(), controller.GitHubOAuth)
		apiRouter.GET("/oauth/wechat", middleware.CriticalRateLimit(), controller.WeChatAuth)
		apiRouter.GET("/oauth/wechat/bind", middleware.CriticalRateLimit(), middleware.UserAuth(), controller.WeChatBind)
		apiRouter.GET("/oauth/email/bind", middleware.CriticalRateLimit(), middleware.UserAuth(), controller.EmailBind)
		apiRouter.GET("/oauth/:source/authorize", middleware.CriticalRateLimit(), controller.OAuthAuthorize)
		apiRouter.GET("/oauth/:source/callback", middleware.CriticalRateLimit(), controller.OAuthCallback)
		apiRouter.POST("/oauth/link-existing", middleware.CriticalRateLimit(), controller.LinkExistingOAuthAccount)
		externalAccountRoute := apiRouter.Group("/oauth/external-accounts")
		externalAccountRoute.Use(middleware.UserAuth(), middleware.NoTokenAuth())
		{
			externalAccountRoute.GET("/", controller.ListExternalAccounts)
			externalAccountRoute.POST("/:id/delete", controller.DeleteExternalAccount)
		}

		capRoute := apiRouter.Group("/cap")
		{
			capRoute.POST("/:scope/challenge", middleware.CriticalRateLimit(), controller.GetCapChallenge)
			capRoute.POST("/:scope/redeem", middleware.CriticalRateLimit(), controller.RedeemCapChallenge)
		}

		userRoute := apiRouter.Group("/user")
		{
			userRoute.POST("/register", middleware.CriticalRateLimit(), controller.Register)
			userRoute.POST("/login", middleware.CriticalRateLimit(), middleware.CapAuth("login"), controller.Login)
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
			optionRoute.POST("/update-batch", controller.UpdateOptionsBatch)
			optionRoute.POST("/geoip/lookup", controller.LookupGeoIP)
			optionRoute.POST("/database/cleanup", controller.CleanupDatabaseObservability)
		}
		uptimekumaRoute := apiRouter.Group("/uptimekuma")
		uptimekumaRoute.Use(middleware.RootAuth(), middleware.NoTokenAuth())
		{
			uptimekumaRoute.POST("/sync", controller.SyncUptimeKuma)
		}
		authSourceRoute := apiRouter.Group("/auth-sources")
		authSourceRoute.Use(middleware.RootAuth(), middleware.NoTokenAuth())
		{
			authSourceRoute.GET("/", controller.ListAuthSources)
			authSourceRoute.POST("/", controller.CreateAuthSource)
			authSourceRoute.POST("/:id/update", controller.UpdateAuthSource)
			authSourceRoute.POST("/:id/delete", controller.DeleteAuthSource)
			authSourceRoute.POST("/:id/toggle", controller.ToggleAuthSource)
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
		proxyRoute := apiRouter.Group("/proxy-routes")
		proxyRoute.Use(middleware.AdminAuth())
		{
			proxyRoute.GET("/", controller.GetProxyRoutes)
			proxyRoute.GET("/:id", controller.GetProxyRoute)
			proxyRoute.POST("/", controller.CreateProxyRoute)
			proxyRoute.POST("/:id/update", controller.UpdateProxyRoute)
			proxyRoute.POST("/:id/delete", controller.DeleteProxyRoute)
		}
		wafRoute := apiRouter.Group("/waf")
		wafRoute.Use(middleware.AdminAuth())
		{
			wafRoute.GET("/ip-groups", controller.ListWAFIPGroups)
			wafRoute.GET("/ip-groups/:id", controller.GetWAFIPGroup)
			wafRoute.POST("/ip-groups", controller.CreateWAFIPGroup)
			wafRoute.POST("/ip-groups/test", controller.TestWAFIPGroupAutoConfig)
			wafRoute.POST("/ip-groups/:id/update", controller.UpdateWAFIPGroup)
			wafRoute.POST("/ip-groups/:id/delete", controller.DeleteWAFIPGroup)
			wafRoute.POST("/ip-groups/:id/sync", controller.SyncWAFIPGroup)
			wafRoute.GET("/rule-groups", controller.ListWAFRuleGroups)
			wafRoute.GET("/rule-groups/:id", controller.GetWAFRuleGroup)
			wafRoute.POST("/rule-groups", controller.CreateWAFRuleGroup)
			wafRoute.POST("/rule-groups/:id/update", controller.UpdateWAFRuleGroup)
			wafRoute.POST("/rule-groups/:id/delete", controller.DeleteWAFRuleGroup)
			wafRoute.POST("/rule-groups/:id/sites", controller.ReplaceWAFRuleGroupSites)
			wafRoute.GET("/sites/:route_id/rule-groups", controller.GetWAFSiteRuleGroups)
			wafRoute.POST("/sites/:route_id/rule-groups", controller.ReplaceWAFSiteRuleGroups)
		}
		originRoute := apiRouter.Group("/origins")
		originRoute.Use(middleware.AdminAuth())
		{
			originRoute.GET("/", controller.GetOrigins)
			originRoute.GET("/:id", controller.GetOrigin)
			originRoute.POST("/", controller.CreateOrigin)
			originRoute.POST("/:id/update", controller.UpdateOrigin)
			originRoute.POST("/:id/delete", controller.DeleteOrigin)
		}
		pagesRoute := apiRouter.Group("/pages")
		pagesRoute.Use(middleware.AdminAuth())
		{
			pagesRoute.GET("/", controller.ListPagesProjects)
			pagesRoute.GET("/:id", controller.GetPagesProject)
			pagesRoute.POST("/", controller.CreatePagesProject)
			pagesRoute.POST("/:id/update", controller.UpdatePagesProject)
			pagesRoute.POST("/:id/delete", controller.DeletePagesProject)
			pagesRoute.GET("/:id/deployments", controller.ListPagesDeployments)
			pagesRoute.POST("/:id/deployments/upload", controller.UploadPagesDeployment)
			pagesRoute.POST("/:id/deployments/:deployment_id/activate", controller.ActivatePagesDeployment)
			pagesRoute.POST("/:id/deployments/:deployment_id/delete", controller.DeletePagesDeployment)
			pagesRoute.GET("/deployments/:deployment_id/files", controller.ListPagesDeploymentFiles)
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
			tlsCertificateRoute.POST("/:id/update-acme", controller.UpdateAcmeCertificate)
			tlsCertificateRoute.POST("/:id/convert-acme", controller.ConvertTLSCertificateToAcme)
			tlsCertificateRoute.POST("/import-file", controller.ImportTLSCertificateFile)
			tlsCertificateRoute.POST("/:id/delete", controller.DeleteTLSCertificate)
			tlsCertificateRoute.POST("/apply", controller.ApplyTLSCertificate)
			tlsCertificateRoute.POST("/:id/renew", controller.RenewTLSCertificate)
		}
		acmeAccountRoute := apiRouter.Group("/acme-accounts")
		acmeAccountRoute.Use(middleware.AdminAuth())
		{
			acmeAccountRoute.GET("/default", controller.GetDefaultAcmeAccount)
		}
		dnsAccountRoute := apiRouter.Group("/dns-accounts")
		dnsAccountRoute.Use(middleware.AdminAuth())
		{
			dnsAccountRoute.GET("/", controller.GetDnsAccounts)
			dnsAccountRoute.POST("/", controller.CreateDnsAccount)
			dnsAccountRoute.POST("/:id/update", controller.UpdateDnsAccount)
			dnsAccountRoute.POST("/:id/delete", controller.DeleteDnsAccount)
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
			configVersionRoute.POST("/cleanup", controller.CleanupConfigVersions)
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
			nodeRoute.POST("/:id/update", controller.UpdateNode)
			nodeRoute.POST("/:id/delete", controller.DeleteNode)
			nodeRoute.POST("/:id/agent-update", controller.RequestNodeAgentUpdate)
			nodeRoute.POST("/:id/openresty-restart", controller.RequestNodeOpenrestyRestart)
			nodeRoute.POST("/:id/force-sync", controller.RequestNodeForceSync)
			nodeRoute.GET("/:id/observability", controller.GetNodeObservability)
			nodeRoute.POST("/:id/observability/cleanup", controller.CleanupNodeHealthEvents)
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
			accessLogRoute.GET("/folds/ip-summary", controller.GetFoldedAccessLogIPs)
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
				authorizedRoute.GET("/ws", controller.AgentWebSocket)
				authorizedRoute.POST("/nodes/heartbeat", controller.AgentHeartbeat)
				authorizedRoute.GET("/config-versions/active", controller.AgentGetActiveConfig)
				authorizedRoute.GET("/pages/deployments/:deployment_id/package", controller.AgentDownloadPagesDeploymentPackage)
				authorizedRoute.POST("/waf/ip-groups/sync", controller.AgentSyncWAFIPGroups)
				authorizedRoute.POST("/apply-logs", controller.AgentReportApplyLog)
			}
		}
		relayRoute := apiRouter.Group("/relay")
		relayRoute.Use(middleware.RelayAuth())
		{
			relayRoute.POST("/heartbeat", controller.RelayHeartbeat)
			relayRoute.GET("/ws", controller.RelayWebSocket)
		}
		flaredRoute := apiRouter.Group("/flared")
		flaredRoute.Use(middleware.TunnelAuth())
		{
			flaredRoute.POST("/heartbeat", controller.FlaredHeartbeat)
			flaredRoute.GET("/config/active", controller.FlaredGetActiveConfig)
			flaredRoute.POST("/apply-log", controller.FlaredReportApplyLog)
			flaredRoute.GET("/ws", controller.FlaredWebSocket)
		}

	}
}
