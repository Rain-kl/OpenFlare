// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"github.com/Rain-kl/Wavelet/internal/apps/admin"
	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apply_log"
	cf "github.com/Rain-kl/Wavelet/internal/apps/openflare/cloudflare"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/config_version"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/dashboard"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/node"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/observability"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/option"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/origin"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/pages"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/performance"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/pow"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/proxy_route"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/tls"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/waf"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/zone"
	"github.com/Rain-kl/Wavelet/internal/shared/httpx"
	"github.com/gin-gonic/gin"
)

// OpenFlareV1BasePath is the OpenFlare console API prefix under /api/v1.
const OpenFlareV1BasePath = "/api/v1/d"

func openFlareAdminMiddlewares() []gin.HandlerFunc {
	return []gin.HandlerFunc{oauth.LoginRequired(), admin.LoginAdminRequired()}
}

func registerOpenFlareConsoleRoutes(apiV1Router *gin.RouterGroup) {
	group := apiV1Router.Group("/d", config_version.WakeAfterWrite)
	registerOpenFlareSystemRoutes(group)
	registerOpenFlareTrafficRoutes(group)
	registerOpenFlareNodeRoutes(group)
	registerOpenFlareSecurityRoutes(group)
	registerOpenFlareTLSRoutes(group)
	registerOpenFlareCloudflareRoutes(group)
	registerOpenFlarePagesRoutes(group)
}

func registerOpenFlareSystemRoutes(apiGroup *gin.RouterGroup) {
	apiGroup.GET("/status", option.GetStatusHandler)

	optionRoute := apiGroup.Group("/option")
	optionRoute.Use(openFlareAdminMiddlewares()...)
	{
		httpx.RegisterCollection(optionRoute, "GET", option.ListOptionsHandler)
		optionRoute.POST("/update", option.UpdateOptionHandler)
		optionRoute.POST("/update-batch", option.UpdateOptionsBatchHandler)
		optionRoute.POST("/geoip/lookup", option.LookupGeoIPHandler)
	}

	uptimeKumaRoute := apiGroup.Group("/uptimekuma")
	uptimeKumaRoute.Use(openFlareAdminMiddlewares()...)
	{
		uptimeKumaRoute.POST("/sync", option.SyncUptimeKumaHandler)
	}

	dashboardRoute := apiGroup.Group("/dashboard")
	dashboardRoute.Use(openFlareAdminMiddlewares()...)
	{
		dashboardRoute.GET("/overview", dashboard.GetOverviewHandler)
	}

	applyLogRoute := apiGroup.Group("/apply-logs")
	applyLogRoute.Use(openFlareAdminMiddlewares()...)
	{
		httpx.RegisterCollection(applyLogRoute, "GET", apply_log.GetApplyLogs)
		applyLogRoute.POST("/cleanup", apply_log.CleanupApplyLogs)
	}

	accessLogRoute := apiGroup.Group("/access-logs")
	accessLogRoute.Use(openFlareAdminMiddlewares()...)
	{
		httpx.RegisterCollection(accessLogRoute, "GET", observability.GetAccessLogsHandler)
		accessLogRoute.GET("/overview", observability.GetAccessLogOverviewHandler)
		accessLogRoute.GET("/folds", observability.GetFoldedAccessLogsHandler)
		accessLogRoute.GET("/folds/ip-summary", observability.GetFoldedAccessLogIPsHandler)
		accessLogRoute.GET("/ip-summary", observability.GetAccessLogIPSummariesHandler)
		accessLogRoute.GET("/ip-summary/trend", observability.GetAccessLogIPTrendHandler)
		accessLogRoute.GET("/ip-summary/analysis", observability.GetAccessLogIPAnalysisHandler)
		accessLogRoute.POST("/cleanup", observability.CleanupAccessLogsHandler)
	}
}

func registerOpenFlareTrafficRoutes(apiGroup *gin.RouterGroup) {
	originRoute := apiGroup.Group("/origins")
	originRoute.Use(openFlareAdminMiddlewares()...)
	{
		httpx.RegisterCollection(originRoute, "GET", origin.GetOrigins)
		originRoute.GET("/:id", origin.GetOrigin)
		httpx.RegisterCollection(originRoute, "POST", origin.CreateOriginHandler)
		originRoute.POST("/:id/update", origin.UpdateOriginHandler)
		originRoute.POST("/:id/delete", origin.DeleteOriginHandler)
	}

	proxyRouteGroup := apiGroup.Group("/proxy-routes")
	proxyRouteGroup.Use(openFlareAdminMiddlewares()...)
	{
		httpx.RegisterCollection(proxyRouteGroup, "GET", proxy_route.GetProxyRoutes)
		proxyRouteGroup.GET("/:id", proxy_route.GetProxyRouteHandler)
		httpx.RegisterCollection(proxyRouteGroup, "POST", proxy_route.CreateProxyRouteHandler)
		proxyRouteGroup.POST("/:id/update", proxy_route.UpdateProxyRouteHandler)
		proxyRouteGroup.POST("/:id/delete", proxy_route.DeleteProxyRouteHandler)
	}

	zoneGroup := apiGroup.Group("/zones")
	zoneGroup.Use(openFlareAdminMiddlewares()...)
	httpx.RegisterCollection(zoneGroup, "GET", zone.ListHandler)
	httpx.RegisterCollection(zoneGroup, "POST", zone.CreateHandler)
	zoneGroup.GET("/:id/overview", zone.GetOverviewHandler)
	zoneGroup.GET("/:id/stats", zone.GetStatsHandler)
	zoneGroup.POST("/:id/update", zone.UpdateHandler)
	zoneGroup.POST("/:id/delete", zone.DeleteHandler)
	zoneGroup.POST("/:id/domains", zone.CreateDomainHandler)
	zoneGroup.POST("/:id/domains/cleanup-unlinked", zone.CleanupUnlinkedDomainsHandler)
	zoneGroup.POST("/:id/domains/:domainId/update", zone.UpdateDomainHandler)
	zoneGroup.POST("/:id/domains/:domainId/delete", zone.DeleteDomainHandler)

}

func registerOpenFlareNodeRoutes(apiGroup *gin.RouterGroup) {
	policies := apiGroup.Group("/performance-policies", openFlareAdminMiddlewares()...)
	httpx.RegisterCollection(policies, "GET", performance.ListHandler)
	httpx.RegisterCollection(policies, "POST", performance.CreateHandler)
	policies.POST("/:id/update", performance.UpdateHandler)
	policies.POST("/:id/assign", performance.AssignHandler)
	policies.POST("/:id/delete", performance.DeleteHandler)

	nodeRoute := apiGroup.Group("/nodes")
	nodeRoute.Use(openFlareAdminMiddlewares()...)
	{
		nodeRoute.GET("/page", node.ListNodePageHandler)
		nodeRoute.GET("/:id", node.GetNodeHandler)
		nodeRoute.GET("/bootstrap-token", node.GetBootstrapTokenHandler)
		nodeRoute.POST("/bootstrap-token/rotate", node.RotateBootstrapTokenHandler)
		httpx.RegisterCollection(nodeRoute, "GET", node.ListNodesHandler)
		httpx.RegisterCollection(nodeRoute, "POST", node.CreateNodeHandler)
		nodeRoute.GET("/:id/agent-release", node.GetAgentReleaseHandler)
		nodeRoute.POST("/:id/update", node.UpdateNodeHandler)
		nodeRoute.POST("/:id/delete", node.DeleteNodeHandler)
		nodeRoute.POST("/:id/agent-update", node.RequestAgentUpdateHandler)
		nodeRoute.POST("/:id/edge-restart", node.RequestEdgeRestartHandler)
		nodeRoute.POST("/:id/force-sync", node.RequestForceSyncHandler)
		nodeRoute.GET("/:id/observability", node.GetObservabilityHandler)
		nodeRoute.POST("/:id/observability/cleanup", node.CleanupHealthEventsHandler)
	}
}

func registerOpenFlareSecurityRoutes(apiGroup *gin.RouterGroup) {
	wafRoute := apiGroup.Group("/waf")
	wafRoute.Use(openFlareAdminMiddlewares()...)
	{
		wafRoute.GET("/ip-groups", waf.ListIPGroupsHandler)
		wafRoute.GET("/ip-groups/:id", waf.GetIPGroupHandler)
		wafRoute.POST("/ip-groups", waf.CreateIPGroupHandler)
		wafRoute.POST("/ip-groups/test", waf.TestIPGroupAutoConfigHandler)
		wafRoute.POST("/ip-groups/:id/update", waf.UpdateIPGroupHandler)
		wafRoute.POST("/ip-groups/:id/delete", waf.DeleteIPGroupHandler)
		wafRoute.POST("/ip-groups/:id/sync", waf.SyncIPGroupHandler)

		wafRoute.GET("/attack-events", waf.ListAttackEventsHandler)
		wafRoute.GET("/attack-logs", waf.ListAttackLogsHandler)
		wafRoute.GET("/attack-logs/:id/request", waf.GetAttackLogRequestHandler)
		wafRoute.GET("/attack-groups", waf.ListAttackGroupsHandler)
		wafRoute.POST("/attack-groups", waf.CreateAttackGroupHandler)
		wafRoute.POST("/attack-groups/:id/update", waf.UpdateAttackGroupHandler)

		wafRoute.GET("/rule-groups", waf.ListRulesHandler)
		wafRoute.GET("/rule-groups/:id", waf.GetRuleHandler)
		wafRoute.GET("/rule-groups/:id/sites", waf.ListRuleGroupSitesHandler)
		wafRoute.POST("/rule-groups/:id/sites/remove", waf.RemoveRuleGroupSitesHandler)
		wafRoute.POST("/rule-groups", waf.CreateRuleHandler)
		wafRoute.POST("/rule-groups/:id/meta", waf.UpdateRuleMetaHandler)
		wafRoute.POST("/rule-groups/:id/graph", waf.SaveRuleGraphHandler)
		wafRoute.POST("/rule-groups/:id/delete", waf.DeleteRuleHandler)

		wafRoute.GET("/sites/:route_id/rule-groups", waf.GetSiteRuleGroupsHandler)
		wafRoute.POST("/sites/:route_id/rule-groups", waf.ReplaceSiteRuleGroupsHandler)
	}

	powRoute := apiGroup.Group("/pow")
	powRoute.Use(openFlareAdminMiddlewares()...)
	powRoute.GET("/policies", pow.ListPoliciesHandler)
	powRoute.PUT("/policies/:id", pow.SavePolicyHandler)
}

func registerOpenFlareTLSRoutes(apiGroup *gin.RouterGroup) {
	tlsCertificateRoute := apiGroup.Group("/tls-certificates")
	tlsCertificateRoute.Use(openFlareAdminMiddlewares()...)
	{
		httpx.RegisterCollection(tlsCertificateRoute, "GET", tls.GetCertificates)
		tlsCertificateRoute.GET("/:id", tls.GetCertificateDetail)
		tlsCertificateRoute.GET("/:id/content", tls.GetCertificateContentHandler)
		httpx.RegisterCollection(tlsCertificateRoute, "POST", tls.CreateCertificateHandler)
		tlsCertificateRoute.POST("/:id/update", tls.UpdateCertificateHandler)
		tlsCertificateRoute.POST("/:id/update-acme", tls.UpdateACMECertificateHandler)
		tlsCertificateRoute.POST("/:id/convert-acme", tls.ConvertCertificateToACMEHandler)
		tlsCertificateRoute.POST("/import-file", tls.ImportCertificateFile)
		tlsCertificateRoute.POST("/:id/delete", tls.DeleteCertificateHandler)
		tlsCertificateRoute.POST("/apply", tls.ApplyCertificateHandler)
		tlsCertificateRoute.POST("/:id/renew", tls.RenewCertificateHandler)
	}

	acmeAccountRoute := apiGroup.Group("/acme-accounts")
	acmeAccountRoute.Use(openFlareAdminMiddlewares()...)
	{
		acmeAccountRoute.GET("/default", tls.GetDefaultAcmeAccountHandler)
	}

	dnsAccountRoute := apiGroup.Group("/dns-accounts")
	dnsAccountRoute.Use(openFlareAdminMiddlewares()...)
	{
		httpx.RegisterCollection(dnsAccountRoute, "GET", tls.GetDNSAccounts)
		httpx.RegisterCollection(dnsAccountRoute, "POST", tls.CreateDNSAccountHandler)
		dnsAccountRoute.POST("/:id/update", tls.UpdateDNSAccountHandler)
		dnsAccountRoute.POST("/:id/delete", tls.DeleteDNSAccountHandler)
	}
}

func registerOpenFlareCloudflareRoutes(apiGroup *gin.RouterGroup) {
	route := apiGroup.Group("/cloudflare")
	route.Use(openFlareAdminMiddlewares()...)
	route.GET("/connection", cf.GetConnectionHandler)
	route.PUT("/connection", cf.SaveConnectionHandler)
	route.POST("/connection/verify", cf.VerifyConnectionHandler)
	route.POST("/connection/clear", cf.ClearConnectionHandler)
	route.GET("/overview", cf.OverviewHandler)
	route.GET("/domains/available", cf.ListAvailableDomainsHandler)
	groups := route.Group("/groups")
	httpx.RegisterCollection(groups, "GET", cf.ListGroupsHandler)
	httpx.RegisterCollection(groups, "POST", cf.CreateGroupHandler)
	route.GET("/groups/:id", cf.GetGroupHandler)
	route.POST("/groups/:id/update", cf.UpdateGroupHandler)
	route.POST("/groups/:id/delete", cf.DeleteGroupHandler)
	route.POST("/groups/:id/sync", cf.SyncGroupHandler)
	route.GET("/groups/:id/members", cf.ListMembersHandler)
	route.POST("/groups/:id/members", cf.CreateMemberHandler)
	route.POST("/groups/:id/members/:memberId/update", cf.UpdateMemberHandler)
	route.POST("/groups/:id/members/:memberId/move", cf.MoveMemberHandler)
	route.POST("/groups/:id/members/:memberId/remove", cf.RemoveMemberHandler)
	route.POST("/groups/:id/members/:memberId/sync", cf.SyncMemberHandler)
	route.POST("/groups/:id/members/batch-move", cf.BatchMoveMembersHandler)
	route.POST("/groups/:id/members/batch-remove", cf.BatchRemoveMembersHandler)
	route.POST("/groups/:id/members/batch-proxy", cf.BatchEnableProxyHandler)
}

func registerOpenFlarePagesRoutes(apiGroup *gin.RouterGroup) {
	pagesRoute := apiGroup.Group("/pages")
	pagesRoute.Use(openFlareAdminMiddlewares()...)
	{
		httpx.RegisterCollection(pagesRoute, "GET", pages.ListProjectsHandler)
		pagesRoute.GET("/:id", pages.GetProjectHandler)
		httpx.RegisterCollection(pagesRoute, "POST", pages.CreateProjectHandler)
		pagesRoute.POST("/:id/update", pages.UpdateProjectHandler)
		pagesRoute.POST("/:id/delete", pages.DeleteProjectHandler)
		pagesRoute.GET("/:id/source", pages.GetSourceHandler)
		pagesRoute.POST("/:id/source/update", pages.UpdateSourceHandler)
		pagesRoute.POST("/:id/source/delete", pages.DeleteSourceHandler)
		pagesRoute.POST("/:id/source/check", pages.CheckSourceHandler)
		pagesRoute.POST("/:id/source/sync", pages.SyncSourceHandler)
		pagesRoute.GET("/:id/deployments", pages.ListDeploymentsHandler)
		pagesRoute.POST("/:id/deployments/upload", pages.UploadDeploymentHandler)
		pagesRoute.POST("/:id/deployments/upload-from-url", pages.UploadDeploymentFromURLHandler)
		pagesRoute.POST("/:id/deployments/:deployment_id/activate", pages.ActivateDeploymentHandler)
		pagesRoute.POST("/:id/deployments/:deployment_id/delete", pages.DeleteDeploymentHandler)
		pagesRoute.GET("/deployments/:deployment_id/files", pages.ListDeploymentFilesHandler)
	}
}
