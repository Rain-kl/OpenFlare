// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openresty

import (
	"encoding/json"
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

func TestIndependentPoWDoesNotRequireWAF(t *testing.T) {
	config := DefaultPoWConfig()
	route := Route{ID: 12, SiteName: "protected", Domains: []string{"example.com"}, Enabled: true, OriginURL: "http://127.0.0.1:8080", PoWEnabled: true, PoWConfig: &config}
	result, err := Render(Document{Routes: []Route{route}}, nil)
	if err != nil {
		t.Fatalf("Render(independent PoW) error = %v", err)
	}
	if !strings.Contains(result.RouteConfig, `require("pow.runtime").prepare()`) || !strings.Contains(result.RouteConfig, "pass-challenge") {
		t.Errorf("Render(independent PoW) missing runtime or challenge endpoints: %s", result.RouteConfig)
	}
	var sites map[string]struct {
		Enabled bool      `json:"enabled"`
		Config  PoWConfig `json:"config"`
	}
	for _, file := range result.SupportFiles {
		if file.Path == "pow_config.json" {
			if err := json.Unmarshal([]byte(file.Content), &sites); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !sites["protected"].Enabled || sites["protected"].Config.Difficulty != config.Difficulty {
		t.Errorf("Render(independent PoW) site policies = %+v, want protected enabled with difficulty %d", sites, config.Difficulty)
	}
	route.PoWEnabled = false
	raw, err := RenderPoWConfig([]Route{route})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(raw), &sites); err != nil {
		t.Fatal(err)
	}
	if sites["protected"].Enabled {
		t.Error("RenderPoWConfig(disabled independent policy) enabled = true, want false")
	}
}

func TestPoWPageDefaultsAndCustomization(t *testing.T) {
	for _, style := range []string{"classic", "shield"} {
		file, err := powPageSupportFile(PoWPageConfig{Template: style, Widget: "turnstile"})
		if err != nil {
			t.Fatal(err)
		}
		var cfg PoWPageConfig
		if err := json.Unmarshal([]byte(file.Content), &cfg); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(cfg.HTML, "{{widget}}") || cfg.Widget != "turnstile" {
			t.Errorf("powPageSupportFile(%s) = %+v, want reusable widget placeholder and turnstile", style, cfg)
		}
	}
	custom := `<main><h1>{{host}}</h1>{{widget}}</main>`
	file, err := powPageSupportFile(PoWPageConfig{HTML: custom})
	if err != nil {
		t.Fatal(err)
	}
	var cfg PoWPageConfig
	if err := json.Unmarshal([]byte(file.Content), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.HTML != custom {
		t.Errorf("powPageSupportFile(custom) HTML = %q, want %q", cfg.HTML, custom)
	}
}
