// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"io/fs"
	"strings"
)

//nolint:unused // Used by frontend.go in embed_frontend builds; default lint runs without that tag.
var nextExportDynamicPrefixes = []string{"websites", "cloudflare/groups"}

//nolint:unused // Used by frontend.go in embed_frontend builds; default lint runs without that tag.
func resolveNextExportDynamicFallback(subFS fs.FS, cleanPath string) (string, bool) {
	templatePath, ok := nextExportDynamicTemplate(cleanPath)
	if !ok {
		return "", false
	}
	if _, err := fs.Stat(subFS, templatePath); err != nil {
		return "", false
	}
	return templatePath, true
}

//nolint:unused // Used by frontend.go in embed_frontend builds; default lint runs without that tag.
func nextExportDynamicTemplate(cleanPath string) (string, bool) {
	for _, prefix := range nextExportDynamicPrefixes {
		if path, ok := nextExportIDTemplate(cleanPath, prefix); ok {
			return path, true
		}
	}
	return "", false
}

//nolint:unused // Used by frontend.go in embed_frontend builds; default lint runs without that tag.
func nextExportIDTemplate(cleanPath, prefix string) (string, bool) {
	if cleanPath == prefix || !strings.HasPrefix(cleanPath, prefix+"/") {
		return "", false
	}
	rest := strings.TrimPrefix(cleanPath, prefix+"/")
	if rest == "" {
		return "", false
	}
	parts := strings.Split(rest, "/")
	switch {
	case len(parts) == 1 && !strings.Contains(parts[0], "."):
		return prefix + "/1.html", true
	case len(parts) == 1 && strings.HasSuffix(parts[0], ".txt"):
		return prefix + "/1.txt", true
	case len(parts) == 2 && parts[1] != "" && strings.HasPrefix(parts[1], "__next.") && strings.HasSuffix(parts[1], ".txt"):
		return prefix + "/1/" + parts[1], true
	default:
		return "", false
	}
}
