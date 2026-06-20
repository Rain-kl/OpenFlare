package openresty

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderWAFConfigIncludesAllRouteSiteNames(t *testing.T) {
	doc := Document{
		Routes: []Route{
			{ID: 1, SiteName: "", Domain: "Example.COM", Domains: []string{"example.com", "www.example.com"}},
			{ID: 2, SiteName: "named-site", Domain: "other.example.com"},
		},
		WAF: WAFDocument{
			RuleGroups: []WAFRuleGroup{
				{
					ID:         1,
					Name:       "pow-group",
					Enabled:    true,
					PoWEnabled: true,
					PoWConfig:  &PoWConfig{Difficulty: 4, Algorithm: "fast", SessionTTL: 600, ChallengeTTL: 300},
				},
			},
			Bindings: []WAFBinding{
				{RouteID: 1, SiteName: "example.com", RuleGroupIDs: []uint{1}},
				{RouteID: 2, SiteName: "named-site", RuleGroupIDs: []uint{1}},
			},
		},
	}

	wafConfig, err := RenderWAFConfig(doc.WAF)
	if err != nil {
		t.Fatalf("RenderWAFConfig() error = %v", err)
	}

	var decoded struct {
		SiteRuleGroups map[string][]uint `json:"site_rule_groups"`
	}
	if err := json.Unmarshal([]byte(wafConfig), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	for _, route := range doc.Routes {
		siteName := resolveRouteSiteName(route)
		if _, ok := decoded.SiteRuleGroups[siteName]; !ok {
			t.Fatalf("site_rule_groups missing site %q, got %#v", siteName, decoded.SiteRuleGroups)
		}
	}

	routeConfig, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatalf("RenderRouteConfig() error = %v", err)
	}
	if !strings.Contains(routeConfig, `set $openflare_waf_site "example.com"`) {
		t.Fatalf("expected route config to use normalized site name example.com, got:\n%s", routeConfig)
	}
	if !strings.Contains(routeConfig, `require("pow.runtime").check()`) {
		t.Fatalf("expected route config to enable pow runtime, got:\n%s", routeConfig)
	}
}

func TestRenderPagesAPIProxyLocationBlock(t *testing.T) {
	tests := []struct {
		name       string
		deployment *PagesDeployment
		expected   []string
		unexpected []string
	}{
		{
			name:       "nil deployment",
			deployment: nil,
			expected:   []string{""},
		},
		{
			name: "disabled proxy",
			deployment: &PagesDeployment{
				APIProxyEnabled: false,
				APIProxyPath:    "/api",
				APIProxyPass:    "http://127.0.0.1:8080",
			},
			expected: []string{""},
		},
		{
			name: "enabled proxy without rewrite",
			deployment: &PagesDeployment{
				APIProxyEnabled: true,
				APIProxyPath:    "/api",
				APIProxyPass:    "http://127.0.0.1:8080",
				APIProxyRewrite: "",
			},
			expected: []string{
				"location /api {",
				"proxy_pass http://127.0.0.1:8080;",
				"proxy_http_version 1.1;",
				"proxy_set_header Host $http_host;",
			},
			unexpected: []string{
				"rewrite",
			},
		},
		{
			name: "enabled proxy with rewrite to root",
			deployment: &PagesDeployment{
				APIProxyEnabled: true,
				APIProxyPath:    "/api",
				APIProxyPass:    "http://127.0.0.1:8080",
				APIProxyRewrite: "/",
			},
			expected: []string{
				"location /api {",
				"rewrite ^/api/(.*)$ /$1 break;",
				"rewrite ^/api$ / break;",
				"proxy_pass http://127.0.0.1:8080;",
			},
		},
		{
			name: "enabled proxy with rewrite to subpath",
			deployment: &PagesDeployment{
				APIProxyEnabled: true,
				APIProxyPath:    "/api",
				APIProxyPass:    "http://127.0.0.1:8080",
				APIProxyRewrite: "/v2",
			},
			expected: []string{
				"location /api {",
				"rewrite ^/api/(.*)$ /v2/$1 break;",
				"rewrite ^/api$ /v2 break;",
				"proxy_pass http://127.0.0.1:8080;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderPagesAPIProxyLocationBlock(tt.deployment)
			if len(tt.expected) == 1 && tt.expected[0] == "" {
				if got != "" {
					t.Fatalf("expected empty output, got: %q", got)
				}
				return
			}
			for _, exp := range tt.expected {
				if !strings.Contains(got, exp) {
					t.Errorf("expected output to contain %q, but got:\n%s", exp, got)
				}
			}
			for _, unexp := range tt.unexpected {
				if strings.Contains(got, unexp) {
					t.Errorf("expected output NOT to contain %q, but got:\n%s", unexp, got)
				}
			}
		})
	}
}
