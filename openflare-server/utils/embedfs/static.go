package embedfs

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-contrib/static"
)

// Credit: https://github.com/gin-contrib/static/issues/19

type fileSystem struct {
	http.FileSystem
}

func (e fileSystem) Exists(prefix string, path string) bool {
	cleanPath := strings.TrimPrefix(path, prefix)
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	if cleanPath == "" {
		return false
	}

	_, err := e.Open(cleanPath)
	if err != nil {
		return false
	}
	return true
}

func EmbedFolder(fsEmbed embed.FS, targetPath string) static.ServeFileSystem {
	efs, err := fs.Sub(fsEmbed, targetPath)
	if err != nil {
		panic(err)
	}
	return fileSystem{
		FileSystem: http.FS(efs),
	}
}
