// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAntiCorruptionBoundary verifies that smartdns never couples directly to netquality
// or raw matrix/topology models, consuming network quality strictly via qualityapi.Provider.
func TestAntiCorruptionBoundary(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read smartdns dir: %v", err)
	}
	forbiddenSymbols := []string{
		"OpenFlareNetworkQualityMatrix",
		"OpenFlareASTopologyEdge",
		"LoadLivePredictor",
		"ComputeQualityScore",
	}

	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Clean(name)
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse imports of %s: %v", path, err)
		}
		for _, imp := range f.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(importPath, "/netquality") {
				t.Fatalf("forbidden import %q in %s: smartdns must consume quality only through qualityapi", importPath, path)
			}
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		src := string(raw)
		for _, sym := range forbiddenSymbols {
			if strings.Contains(src, sym) {
				t.Fatalf("forbidden netquality internal symbol %q referenced in %s", sym, path)
			}
		}
	}
}
