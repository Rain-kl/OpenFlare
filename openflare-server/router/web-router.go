package router

import (
	"embed"
	"io/fs"
	"net/http"
	pathpkg "path"
	"strings"

	"github.com/rain-kl/openflare/openflare-server/middleware"
	"github.com/rain-kl/openflare/openflare-server/utils/embedfs"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

func setWebRouter(router *gin.Engine, buildFS embed.FS, indexPage []byte) {
	exportedBuildFS, err := fs.Sub(buildFS, "web/build")
	if err != nil {
		panic(err)
	}

	router.Use(middleware.GlobalWebRateLimit())
	router.Use(normalizeStaticExportDataNavigation())
	router.Use(middleware.Cache())
	router.Use(static.Serve("/", embedfs.EmbedFolder(buildFS, "web/build")))
	router.NoRoute(func(c *gin.Context) {
		if serveExportedPage(c, exportedBuildFS) {
			return
		}

		if isStaticAssetRequest(c.Request.URL.Path) {
			c.Status(http.StatusNotFound)
			return
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", indexPage)
	})
}

func serveExportedPage(c *gin.Context, buildFS fs.FS) bool {
	requestPath := strings.Trim(c.Request.URL.Path, "/")
	if isOAuthCallbackPath(requestPath) {
		requestPath = "oauth/callback"
	}

	candidates := []string{"index.html"}
	if requestPath != "" {
		candidates = []string{
			requestPath + ".html",
			pathpkg.Join(requestPath, "index.html"),
		}
	}

	for _, candidate := range candidates {
		content, err := fs.ReadFile(buildFS, candidate)
		if err == nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8", content)
			return true
		}
	}

	return false
}

func isOAuthCallbackPath(requestPath string) bool {
	if !strings.HasPrefix(requestPath, "oauth/") || strings.Count(requestPath, "/") != 1 {
		return false
	}
	source := strings.TrimPrefix(requestPath, "oauth/")
	switch source {
	case "", "callback", "link":
		return false
	default:
		return true
	}
}

func normalizeStaticExportDataNavigation() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if strings.HasSuffix(requestPath, ".txt") && isDocumentNavigationRequest(c.Request) {
			normalizedPath := strings.TrimSuffix(requestPath, ".txt")
			if normalizedPath == "" {
				normalizedPath = "/"
			}
			c.Request.URL.Path = normalizedPath
		}

		c.Next()
	}
}

func isDocumentNavigationRequest(request *http.Request) bool {
	if request.Header.Get("Sec-Fetch-Mode") == "navigate" || request.Header.Get("Sec-Fetch-Dest") == "document" {
		return true
	}

	return strings.Contains(request.Header.Get("Accept"), "text/html")
}

func isStaticAssetRequest(requestPath string) bool {
	return strings.HasPrefix(requestPath, "/_next/") || pathpkg.Ext(requestPath) != ""
}
