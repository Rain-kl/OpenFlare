//go:build embed_frontend

// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var frontendFS embed.FS

func serveFileDirect(c *gin.Context, subFS fs.FS, filePath string) bool {
	file, err := subFS.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return false
	}

	if stat.IsDir() {
		return false
	}

	seeker, ok := file.(io.ReadSeeker)
	if !ok {
		return false
	}

	// 使用 http.ServeContent 直接输出文件内容，不进行路径规范化重定向
	http.ServeContent(c.Writer, c.Request, filePath, stat.ModTime(), seeker)
	return true
}

func init() {
	RegisterFrontend = func(r *gin.Engine) {
		subFS, err := fs.Sub(frontendFS, "dist")
		if err != nil {
			panic(err)
		}

		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path

			// API 接口路由或文件服务路由 -> 直接返回，由 Gin 处理标准 404
			if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/f/") {
				return
			}

			// 只处理 GET 和 HEAD 请求
			if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
				c.JSON(http.StatusMethodNotAllowed, gin.H{"error_msg": "Method not allowed"})
				return
			}

			// 移除开头的斜杠以在嵌入文件系统中查找
			cleanPath := strings.TrimPrefix(path, "/")

			// 1. 根路径 -> 直接输出 index.html
			if cleanPath == "" {
				if serveFileDirect(c, subFS, "index.html") {
					return
				}
			}

			// 2. 精确匹配（如果对应的文件存在，直接输出）
			if serveFileDirect(c, subFS, cleanPath) {
				return
			}

			// 如果是个目录（例如请求了 "/login"，同时 dist 目录下存在一个叫 "login" 的文件夹目录），
			// 则查找是否有对应的 ".html" 文件（例如 "login.html"）并进行输出。
			if cleanPath != "" {
				htmlPath := cleanPath + ".html"
				if serveFileDirect(c, subFS, htmlPath) {
					return
				}
			}

			// 3. Next.js Clean URLs 兜底逻辑（例如访问 /settings/security -> 实际映射输出 settings/security.html）
			if !strings.Contains(cleanPath, ".") {
				htmlPath := cleanPath + ".html"
				if serveFileDirect(c, subFS, htmlPath) {
					return
				}
				indexPath := cleanPath + "/index.html"
				if serveFileDirect(c, subFS, indexPath) {
					return
				}
			}

			// 4. 单页应用（SPA）前端路由兜底：返回 index.html
			if serveFileDirect(c, subFS, "index.html") {
				return
			}
		})
	}
}
