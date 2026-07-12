package openresty

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderWAFConfigIncludesAllRouteSiteNames(t *testing.T) {
	doc := Document{
		Routes: []Route{
			{ID: 1, SiteName: "example.com", Domains: []string{"example.com", "www.example.com"}},
			{ID: 2, SiteName: "named-site", Domains: []string{"other.example.com"}},
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

func TestRenderWAFConfigUsesDefaultPoWConfigWhenEnabledWithoutPayload(t *testing.T) {
	doc := WAFDocument{
		RuleGroups: []WAFRuleGroup{
			{
				ID:         1,
				Name:       "global",
				Enabled:    true,
				IsGlobal:   true,
				PoWEnabled: true,
			},
		},
		Bindings: []WAFBinding{
			{RouteID: 1, SiteName: "example.com", RuleGroupIDs: []uint{}},
		},
	}

	wafConfig, err := RenderWAFConfig(doc)
	if err != nil {
		t.Fatalf("RenderWAFConfig() error = %v", err)
	}

	var decoded struct {
		RuleGroups []struct {
			PoWEnabled bool       `json:"pow_enabled"`
			PoWConfig  *PoWConfig `json:"pow_config"`
		} `json:"rule_groups"`
	}
	if err := json.Unmarshal([]byte(wafConfig), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(decoded.RuleGroups) != 1 {
		t.Fatalf("expected 1 rule group, got %d", len(decoded.RuleGroups))
	}
	if !decoded.RuleGroups[0].PoWEnabled {
		t.Fatal("expected pow_enabled=true")
	}
	if decoded.RuleGroups[0].PoWConfig == nil {
		t.Fatal("expected default pow_config to be emitted")
	}
	if decoded.RuleGroups[0].PoWConfig.Difficulty != 4 {
		t.Fatalf("expected default difficulty 4, got %d", decoded.RuleGroups[0].PoWConfig.Difficulty)
	}
}

func TestGetPoWConfigForRouteUsesGlobalGroupWithoutExplicitBinding(t *testing.T) {
	snapshot := WAFDocument{
		RuleGroups: []WAFRuleGroup{
			{
				ID:         1,
				Name:       "global",
				Enabled:    true,
				IsGlobal:   true,
				PoWEnabled: true,
			},
		},
		Bindings: []WAFBinding{
			{RouteID: 42, SiteName: "example.com", RuleGroupIDs: []uint{}},
		},
	}

	enabled, config := getPoWConfigForRoute(42, snapshot)
	if !enabled {
		t.Fatal("expected pow to be enabled via global rule group")
	}
	if config == nil || config.Difficulty != 4 {
		t.Fatalf("expected default pow config, got %#v", config)
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

func TestRenderPagesRootLocationBlock(t *testing.T) {
	tests := []struct {
		name       string
		deployment *PagesDeployment
		expected   []string
		unexpected []string
	}{
		{
			name: "spa fallback disabled serves entry file at root",
			deployment: &PagesDeployment{
				SPAFallbackEnabled: false,
				EntryFile:          "index.html",
			},
			expected: []string{
				"location = / {",
				"try_files /index.html =404;",
			},
		},
		{
			name: "spa fallback disabled with custom entry file",
			deployment: &PagesDeployment{
				SPAFallbackEnabled: false,
				EntryFile:          "app.html",
			},
			expected: []string{
				"location = / {",
				"try_files /app.html =404;",
			},
		},
		{
			name: "spa fallback enabled serves fallback file at root",
			deployment: &PagesDeployment{
				SPAFallbackEnabled: true,
				SPAFallbackPath:    "/index.html",
			},
			expected: []string{
				"location = / {",
				"try_files /index.html =404;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderPagesRootLocationBlock(tt.deployment, routeLimitConfig{}, false, "", "")
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

func TestRenderRouteConfigPagesWithoutSPAFallbackServesRoot(t *testing.T) {
	doc := Document{
		Routes: []Route{
			{
				ID:           1,
				SiteName:     "speedtest.example.com",
				Domains:      []string{"speedtest.example.com"},
				UpstreamType: "pages",
				EnableHTTPS:  false,
				PagesDeployment: &PagesDeployment{
					LocalRoot:          "/data/var/lib/openflare/pages/deployments/1/current",
					EntryFile:          "index.html",
					SPAFallbackEnabled: false,
				},
			},
		},
	}

	routeConfig, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatalf("RenderRouteConfig() error = %v", err)
	}
	if !strings.Contains(routeConfig, "location = / {") {
		t.Fatalf("expected root location block, got:\n%s", routeConfig)
	}
	if !strings.Contains(routeConfig, "try_files /index.html =404;") {
		t.Fatalf("expected root try_files for entry file, got:\n%s", routeConfig)
	}
	if !strings.Contains(routeConfig, "try_files $uri $uri/ =404;") {
		t.Fatalf("expected static file try_files in location /, got:\n%s", routeConfig)
	}
}

func TestRenderRouteConfigPagesWithSPAFallbackServesRoot(t *testing.T) {
	doc := Document{
		Routes: []Route{
			{
				ID:           1,
				SiteName:     "speedtest.example.com",
				Domains:      []string{"speedtest.example.com"},
				UpstreamType: "pages",
				EnableHTTPS:  false,
				PagesDeployment: &PagesDeployment{
					LocalRoot:          "/data/var/lib/openflare/pages/deployments/1/current",
					EntryFile:          "index.html",
					SPAFallbackEnabled: true,
					SPAFallbackPath:    "/index.html",
				},
			},
		},
	}

	routeConfig, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatalf("RenderRouteConfig() error = %v", err)
	}
	if !strings.Contains(routeConfig, "location = / {") {
		t.Fatalf("expected root location block for spa fallback, got:\n%s", routeConfig)
	}
	if !strings.Contains(routeConfig, "try_files $uri $uri/ /index.html;") {
		t.Fatalf("expected spa fallback try_files in location /, got:\n%s", routeConfig)
	}
}
