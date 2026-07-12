// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"io/fs"
	"strings"
)

//nolint:unused // Used by frontend.go in embed_frontend builds; default lint runs without that tag.
func resolveNextExportDynamicFallback(subFS fs.FS, cleanPath string) (string, bool) {
	parts := strings.Split(cleanPath, "/")
	if len(parts) < 2 || parts[0] != "websites" || parts[1] == "" {
		return "", false
	}

	var templatePath string
	switch {
	case len(parts) == 2 && !strings.Contains(parts[1], "."):
		templatePath = "websites/1.html"
	case len(parts) == 2 && strings.HasSuffix(parts[1], ".txt"):
		templatePath = "websites/1.txt"
	case len(parts) == 3 && parts[2] != "" && strings.HasPrefix(parts[2], "__next.") && strings.HasSuffix(parts[2], ".txt"):
		templatePath = "websites/1/" + parts[2]
	default:
		return "", false
	}

	if _, err := fs.Stat(subFS, templatePath); err != nil {
		return "", false
	}
	return templatePath, true
}
