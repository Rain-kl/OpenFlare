package router

import (
	"atsflare/common"
	"atsflare/controller"
	"atsflare/middleware"
	"embed"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"io/fs"
	"net/http"
	pathpkg "path"
	"strings"
)

func setWebRouter(router *gin.Engine, buildFS embed.FS, indexPage []byte) {
	exportedBuildFS, err := fs.Sub(buildFS, "web/build")
	if err != nil {
		panic(err)
	}

	router.Use(middleware.GlobalWebRateLimit())
	fileDownloadRoute := router.Group("/")
	fileDownloadRoute.GET("/upload/:file", middleware.DownloadRateLimit(), controller.DownloadFile)
	router.Use(middleware.Cache())
	router.Use(static.Serve("/", common.EmbedFolder(buildFS, "web/build")))
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

func isStaticAssetRequest(requestPath string) bool {
	return strings.HasPrefix(requestPath, "/_next/") || pathpkg.Ext(requestPath) != ""
}
