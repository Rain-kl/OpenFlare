package root

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/automation"
	"github.com/Rain-kl/Wavelet/internal/automationcontract"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/gin-gonic/gin"
)

func registerAutomationRoutes(engine *gin.Engine) {
	handler := automation.MCPHandler()
	engine.POST(automationcontract.MCPPath, handler)
	engine.GET(automationcontract.MCPPath, handler)
	engine.DELETE(automationcontract.MCPPath, handler)
	engine.GET("/.well-known/oauth-authorization-server", automation.AuthorizationMetadata())
	engine.GET("/.well-known/oauth-protected-resource", automation.ProtectedResourceMetadata(true))
	engine.GET("/.well-known/oauth-protected-resource/mcp", automation.ProtectedResourceMetadata(true))
	engine.GET("/.well-known/oauth-protected-resource"+config.Config.App.APIPrefix+automationcontract.APISuffix, automation.ProtectedResourceMetadata(false))
	automation.RegisterOAuth(engine.Group(automationcontract.OAuthPath))
	automation.RegisterBrowser(engine.Group("/automation"))
}
