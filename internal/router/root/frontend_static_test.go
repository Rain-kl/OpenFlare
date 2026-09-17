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
		"cloudflare/groups.html":   &fstest.MapFile{Data: []byte("groups list redirect")},
		"cloudflare/groups/1.html": &fstest.MapFile{Data: []byte("group detail")},
		"cloudflare/groups/1.txt":  &fstest.MapFile{Data: []byte("group flight")},
		"cloudflare/groups/1/__next.!KG1haW4p.cloudflare.groups.$d$id.__PAGE__.txt": &fstest.MapFile{Data: []byte("group segment")},
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
		{name: "cloudflare group html", input: "cloudflare/groups/2", want: "cloudflare/groups/1.html"},
		{name: "cloudflare group route payload", input: "cloudflare/groups/2.txt", want: "cloudflare/groups/1.txt"},
		{
			name:  "cloudflare group segment payload",
			input: "cloudflare/groups/3/__next.!KG1haW4p.cloudflare.groups.$d$id.__PAGE__.txt",
			want:  "cloudflare/groups/1/__next.!KG1haW4p.cloudflare.groups.$d$id.__PAGE__.txt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := resolveNextExportDynamicFallback(subFS, tt.input)
			if !ok {
				t.Fatalf("resolveNextExportDynamicFallback(%q) ok = false, want true", tt.input)
			}
			if got != tt.want {
				t.Fatalf("resolveNextExportDynamicFallback(%q) = %q, want %q", tt.input, got, tt.want)
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
		"cloudflare",
		"cloudflare/groups",
		"cloudflare/groups.html",
		"cloudflare/groups/2/settings",
		"cloudflare/groups/2.js",
		"cloudflare/groups/2/",
		"cloudflare/groups/2/missing.txt",
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
		t.Fatalf("resolveNextExportDynamicFallback(%q) = %q, want no fallback", "websites/3", got)
	}
	if got, ok := resolveNextExportDynamicFallback(subFS, "cloudflare/groups/2"); ok {
		t.Fatalf("resolveNextExportDynamicFallback(%q) = %q, want no fallback", "cloudflare/groups/2", got)
	}
}
