// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"testing"

	openrestyrender "github.com/rain-kl/openflare/pkg/render/openresty"
)

func TestIsRuntimeGeneratedSupportFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "pow_config.json", want: true},
		{path: "waf_config.json", want: true},
		{path: openrestyrender.SourceConfigFileName, want: true},
		{path: "runtime/custom.json", want: false},
		{path: "certs/example.pem", want: false},
	}
	for _, tc := range tests {
		if got := isRuntimeGeneratedSupportFile(tc.path); got != tc.want {
			t.Fatalf("isRuntimeGeneratedSupportFile(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestSourceSupportFilesFiltersRuntimeGeneratedFiles(t *testing.T) {
	files := []SupportFile{
		{Path: "certs/example.pem", Content: "pem"},
		{Path: "pow_config.json", Content: "{}"},
		{Path: "waf_config.json", Content: "{}"},
		{Path: openrestyrender.SourceConfigFileName, Content: "{}"},
		{Path: "routes/extra.json", Content: "{}"},
	}

	filtered := sourceSupportFiles(files)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 support files, got %d: %+v", len(filtered), filtered)
	}
	if filtered[0].Path != "certs/example.pem" || filtered[1].Path != "routes/extra.json" {
		t.Fatalf("unexpected filtered files: %+v", filtered)
	}
}
