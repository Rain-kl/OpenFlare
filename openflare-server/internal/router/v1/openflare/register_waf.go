// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/waf"
	"github.com/gin-gonic/gin"
)

func registerWAFRoutes(apiGroup *gin.RouterGroup) {
	wafRoute := apiGroup.Group("/waf")
	wafRoute.Use(apiutil.AdminMiddlewares()...)
	{
		wafRoute.GET("/ip-groups", waf.ListIPGroupsHandler)
		wafRoute.GET("/ip-groups/:id", waf.GetIPGroupHandler)
		wafRoute.POST("/ip-groups", waf.CreateIPGroupHandler)
		wafRoute.POST("/ip-groups/test", waf.TestIPGroupAutoConfigHandler)
		wafRoute.POST("/ip-groups/:id/update", waf.UpdateIPGroupHandler)
		wafRoute.POST("/ip-groups/:id/delete", waf.DeleteIPGroupHandler)
		wafRoute.POST("/ip-groups/:id/sync", waf.SyncIPGroupHandler)

		wafRoute.GET("/rule-groups", waf.ListRuleGroupsHandler)
		wafRoute.GET("/rule-groups/:id", waf.GetRuleGroupHandler)
		wafRoute.POST("/rule-groups", waf.CreateRuleGroupHandler)
		wafRoute.POST("/rule-groups/:id/update", waf.UpdateRuleGroupHandler)
		wafRoute.POST("/rule-groups/:id/delete", waf.DeleteRuleGroupHandler)
		wafRoute.POST("/rule-groups/:id/sites", waf.ReplaceRuleGroupSitesHandler)

		wafRoute.GET("/sites/:route_id/rule-groups", waf.GetSiteRuleGroupsHandler)
		wafRoute.POST("/sites/:route_id/rule-groups", waf.ReplaceSiteRuleGroupsHandler)
	}
}
