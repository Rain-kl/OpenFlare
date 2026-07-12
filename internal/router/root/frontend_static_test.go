// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"testing"
	"testing/fstest"
)

func TestResolveNextExportDynamicFallbackUsesZoneTemplate(t *testing.T) {
	subFS := fstest.MapFS{
		"index.html":      &fstest.MapFile{Data: []byte("dashboard")},
		"websites/1.html": &fstest.MapFile{Data: []byte("zone detail")},
		"websites/1.txt":  &fstest.MapFile{Data: []byte("zone flight")},
		"websites/1/__next.!KG1haW4p.websites.$d$zoneId.__PAGE__.txt": &fstest.MapFile{Data: []byte("zone segment")},
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "html", input: "websites/3", want: "websites/1.html"},
		{name: "route payload", input: "websites/3.txt", want: "websites/1.txt"},
		{
			name:  "segment payload",
			input: "websites/3/__next.!KG1haW4p.websites.$d$zoneId.__PAGE__.txt",
			want:  "websites/1/__next.!KG1haW4p.websites.$d$zoneId.__PAGE__.txt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := resolveNextExportDynamicFallback(subFS, tt.input)
			if !ok {
				t.Fatal("expected dynamic websites route fallback")
			}
			if got != tt.want {
				t.Fatalf("expected %q fallback, got %q", tt.want, got)
			}
		})
	}
}

func TestResolveNextExportDynamicFallbackRejectsNestedOrAssetPath(t *testing.T) {
	subFS := fstest.MapFS{
		"websites/1.html": &fstest.MapFile{Data: []byte("zone detail")},
	}

	tests := []string{
		"websites",
		"websites/3/settings",
		"websites/3.js",
		"websites/3/",
		"websites/3/missing.txt",
	}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			if got, ok := resolveNextExportDynamicFallback(subFS, tt); ok {
				t.Fatalf("expected no fallback, got %q", got)
			}
		})
	}
}

func TestResolveNextExportDynamicFallbackRequiresGeneratedTemplate(t *testing.T) {
	subFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("dashboard")},
	}

	if got, ok := resolveNextExportDynamicFallback(subFS, "websites/3"); ok {
		t.Fatalf("expected no fallback without generated template, got %q", got)
	}
}
