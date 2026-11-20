// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package edgeconfig

import (
	"strings"
	"testing"
)

func TestValidatePoWPathRules(t *testing.T) {
	tests := []struct {
		name    string
		rules   *PoWPathRules
		wantErr bool
	}{
		{name: "legacy defaults"},
		{name: "all paths", rules: &PoWPathRules{Mode: "all"}},
		{name: "explicit path", rules: &PoWPathRules{Mode: "blacklist", Blacklist: []PoWPathMatch{{Type: "exact", Value: "/login"}}}},
		{name: "regex", rules: &PoWPathRules{Mode: "blacklist", Blacklist: []PoWPathMatch{{Type: "regex", Value: `^/admin(?:/|$)`}}}},
		{name: "empty blacklist", rules: &PoWPathRules{Mode: "blacklist"}, wantErr: true},
		{name: "unknown mode", rules: &PoWPathRules{Mode: "allow"}, wantErr: true},
		{name: "invalid regex", rules: &PoWPathRules{Mode: "all", Whitelist: []PoWPathMatch{{Type: "regex", Value: "("}}}, wantErr: true},
		{name: "invalid type", rules: &PoWPathRules{Mode: "all", Whitelist: []PoWPathMatch{{Type: "contains", Value: "/"}}}, wantErr: true},
		{name: "missing slash", rules: &PoWPathRules{Mode: "all", Whitelist: []PoWPathMatch{{Type: "prefix", Value: "admin"}}}, wantErr: true},
		{name: "empty value", rules: &PoWPathRules{Mode: "all", Whitelist: []PoWPathMatch{{Type: "exact"}}}, wantErr: true},
		{name: "long regex", rules: &PoWPathRules{Mode: "all", Whitelist: []PoWPathMatch{{Type: "regex", Value: strings.Repeat("a", 1025)}}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultPoWConfig()
			config.PathRules = tt.rules
			if err := ValidatePoWConfig(config); (err != nil) != tt.wantErr {
				t.Errorf("ValidatePoWConfig(%+v) error = %v, want error %t", config, err, tt.wantErr)
			}
		})
	}
}

func TestBuiltInPoWTemplatesIncludeWidget(t *testing.T) {
	for _, style := range []string{"classic", "shield"} {
		if !strings.Contains(DefaultPoWPageHTML(style), "{{widget}}") {
			t.Errorf("template %s is missing its widget placeholder", style)
		}
	}
}
