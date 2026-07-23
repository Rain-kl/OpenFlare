package openresty

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderOpenRestyUsesDedicatedWAFIPGroupSharedDict(t *testing.T) {
	block := renderOpenRestyObservabilityTemplateBlock()
	if !strings.Contains(block, "lua_shared_dict openflare_waf_config 1m;") {
		t.Fatal("expected general WAF coordination dictionary to remain available")
	}
	if !strings.Contains(block, "lua_shared_dict openflare_waf_ip_groups 64m;") {
		t.Fatalf("expected dedicated 64m WAF IP group dictionary, got:\n%s", block)
	}
}

func TestRenderWAFConfigIncludesAllRouteSiteNames(t *testing.T) {
	doc := Document{
		Routes: []Route{
			{ID: 1, SiteName: "example.com", Domains: []string{"example.com", "www.example.com"}},
			{ID: 2, SiteName: "named-site", Domains: []string{"other.example.com"}},
		},
		WAF: WAFDocument{
			RuleGroups: []WAFRuleGroup{
				{
					ID: 1, Name: "pow-group", Enabled: true,
					Graph: WAFRuleGraph{Entry: "pow", Nodes: map[string]WAFRuleNode{"pow": {Type: "pow"}}},
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

	var decoded WAFDocument
	if err := json.Unmarshal([]byte(wafConfig), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(decoded.Bindings) != 2 || decoded.Bindings[0].SiteName != "example.com" || decoded.Bindings[1].SiteName != "named-site" {
		t.Fatalf("bindings did not preserve route site names: %#v", decoded.Bindings)
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

func TestRenderWAFConfigDoesNotSynthesizeLegacyPoWConfig(t *testing.T) {
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

	var decoded WAFDocument
	if err := json.Unmarshal([]byte(wafConfig), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(decoded.RuleGroups) != 1 {
		t.Fatalf("expected 1 rule group, got %d", len(decoded.RuleGroups))
	}
	if decoded.RuleGroups[0].PoWConfig != nil {
		t.Fatalf("expected renderer not to synthesize legacy PoW config, got %#v", decoded.RuleGroups[0].PoWConfig)
	}
}

func TestGetPoWConfigForRouteUsesGlobalGroupWithoutExplicitBinding(t *testing.T) {
	snapshot := WAFDocument{
		RuleGroups: []WAFRuleGroup{
			{
				ID: 1, Name: "global", Enabled: true, IsGlobal: true,
				Graph: WAFRuleGraph{Entry: "pow", Nodes: map[string]WAFRuleNode{"pow": {Type: "pow"}}},
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
	if config != nil {
		t.Fatalf("expected node config to stay in runtime graph, got legacy config %#v", config)
	}
}

func TestRenderRouteConfigEnablesPoWLocationsFromRuntimeGraph(t *testing.T) {
	doc := Document{
		Routes: []Route{{ID: 1, SiteName: "pow.example.com", Domains: []string{"pow.example.com"}, OriginURL: "http://127.0.0.1:8080", Enabled: true}},
		WAF: WAFDocument{
			RuleGroups: []WAFRuleGroup{{
				ID: 1, Name: "graph-pow", Enabled: true, IsGlobal: true,
				Graph: WAFRuleGraph{Entry: "start", Nodes: map[string]WAFRuleNode{
					"start": {Type: "start", Next: map[string]string{"next": "pow"}},
					"pow":   {Type: "pow", Config: json.RawMessage(`{"algorithm":"fast","difficulty":4,"session_ttl":600,"challenge_ttl":300}`), Next: map[string]string{"next": "allow"}},
					"allow": {Type: "allow"},
				}},
			}},
			Bindings: []WAFBinding{{RouteID: 1, SiteName: "pow.example.com", RuleGroupIDs: []uint{}}},
		},
	}

	rendered, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatalf("RenderRouteConfig() error = %v", err)
	}
	for _, expected := range []string{
		`location = /.within.website/x/cmd/anubis/api/make-challenge`,
		`location = /.within.website/x/cmd/anubis/api/pass-challenge`,
		`location /.within.website/x/cmd/anubis/static/`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected graph PoW route to contain %q, got:\n%s", expected, rendered)
		}
	}
}

func TestRenderWAFConfigPreservesRuntimeGraphAndBindingOrder(t *testing.T) {
	doc := WAFDocument{
		RuleGroups: []WAFRuleGroup{{
			ID: 9, Name: "graph", Enabled: true,
			Graph: WAFRuleGraph{Entry: "start", Nodes: map[string]WAFRuleNode{
				"start": {Type: "start", Next: map[string]string{"next": "allow"}},
				"allow": {Type: "allow"},
			}},
		}},
		Bindings: []WAFBinding{{RouteID: 3, SiteName: "ordered.example.com", RuleGroupIDs: []uint{9, 4, 7}}},
	}

	raw, err := RenderWAFConfig(doc)
	if err != nil {
		t.Fatalf("RenderWAFConfig() error = %v", err)
	}
	var decoded WAFDocument
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.RuleGroups[0].Graph.Entry != "start" {
		t.Fatalf("runtime graph was not preserved: %#v", decoded.RuleGroups[0].Graph)
	}
	if got := decoded.Bindings[0].RuleGroupIDs; len(got) != 3 || got[0] != 9 || got[1] != 4 || got[2] != 7 {
		t.Fatalf("binding order changed: %#v", got)
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
					LocalRoot:          "/data/var/lib/openflare/pages/projects/1/current",
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
					LocalRoot:          "/data/var/lib/openflare/pages/projects/1/current",
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

func TestRenderRouteCachePolicyConditionStaticDefault(t *testing.T) {
	staticBlock := renderRouteCachePolicyCondition(routeCacheConfig{Enabled: true, Policy: "static"})
	if staticBlock == "" {
		t.Fatal("static policy should emit a path condition")
	}
	if !strings.Contains(staticBlock, "css") || !strings.Contains(staticBlock, "woff2") {
		t.Fatalf("static policy should include default extensions, got:\n%s", staticBlock)
	}
	if !strings.Contains(staticBlock, "map") || !strings.Contains(staticBlock, "mjs") {
		t.Fatalf("static policy should include map and mjs, got:\n%s", staticBlock)
	}
	if strings.Contains(staticBlock, "html") {
		t.Fatalf("static policy must not include html, got:\n%s", staticBlock)
	}
	// Pattern is \.(?:css|js|...)$ — reject bare "json" as an alternation token.
	if strings.Contains(staticBlock, "|json|") || strings.Contains(staticBlock, "|json)") || strings.Contains(staticBlock, "(?:json|") {
		t.Fatalf("static policy must not include json (CF default), got:\n%s", staticBlock)
	}

	// Legacy empty/url = all (wide cache after method bypass).
	emptyPolicy := renderRouteCachePolicyCondition(routeCacheConfig{Enabled: true, Policy: ""})
	if emptyPolicy != "" {
		t.Fatalf("empty policy should map to all (no path filter), got %q", emptyPolicy)
	}

	allBlock := renderRouteCachePolicyCondition(routeCacheConfig{Enabled: true, Policy: "all"})
	if allBlock != "" {
		t.Fatalf("all policy should not add path condition, got %q", allBlock)
	}
	urlBlock := renderRouteCachePolicyCondition(routeCacheConfig{Enabled: true, Policy: "url"})
	if urlBlock != "" {
		t.Fatalf("legacy url policy should map to all, got %q", urlBlock)
	}
}

func TestRenderRouteCacheBlockAlignsCloudflareDefaults(t *testing.T) {
	block := renderRouteCacheBlock(
		routeCacheConfig{Enabled: true, Policy: "static"},
		ConfigSnapshot{CacheEnabled: true},
	)
	if !strings.Contains(block, "proxy_cache openflare_cache") {
		t.Fatalf("expected proxy_cache, got:\n%s", block)
	}
	if !strings.Contains(block, "\\.(?:") {
		t.Fatalf("expected static suffix pattern, got:\n%s", block)
	}
	if !strings.Contains(block, "request_method != GET") {
		t.Fatalf("expected method bypass for non-GET, got:\n%s", block)
	}
	if strings.Contains(block, "$http_authorization") {
		t.Fatalf("must not bypass on Authorization (CF-aligned), got:\n%s", block)
	}
	if strings.Contains(block, "$http_cookie") {
		t.Fatalf("must not bypass on Cookie (CF-aligned), got:\n%s", block)
	}
	if strings.Contains(block, "$http_cache_control") {
		t.Fatalf("must not bypass on request Cache-Control (CF-aligned), got:\n%s", block)
	}
	if !strings.Contains(block, "proxy_no_cache $openflare_skip_cache $upstream_http_set_cookie") {
		t.Fatalf("expected Set-Cookie no-cache gate, got:\n%s", block)
	}
	if !strings.Contains(block, "proxy_cache_valid 200 206 301 120m") {
		t.Fatalf("expected default Edge TTL for 200/206/301, got:\n%s", block)
	}
	if !strings.Contains(block, "proxy_cache_valid 302 303 20m") {
		t.Fatalf("expected default Edge TTL for 302/303, got:\n%s", block)
	}
	if !strings.Contains(block, "proxy_cache_valid 404 410 3m") {
		t.Fatalf("expected default Edge TTL for 404/410, got:\n%s", block)
	}
	if !strings.Contains(block, "proxy_cache_bypass $openflare_skip_cache") {
		t.Fatalf("expected proxy_cache_bypass on skip flag only, got:\n%s", block)
	}
}

func TestMergeRouteLimitConfig(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		route Route
		cfg   ConfigSnapshot
		want  routeLimitConfig
	}{
		{
			name:  "both zero off",
			route: Route{},
			cfg:   ConfigSnapshot{},
			want:  routeLimitConfig{},
		},
		{
			name:  "inherit all defaults",
			route: Route{},
			cfg: ConfigSnapshot{
				DefaultLimitConnPerServer: 100,
				DefaultLimitConnPerIP:     10,
				DefaultLimitRate:          "512k",
				DefaultLimitReqPerIP:      "10r/s",
			},
			want: routeLimitConfig{LimitConnPerServer: 100, LimitConnPerIP: 10, LimitRate: "512k", LimitReqPerIP: "10r/s"},
		},
		{
			name:  "explicit off ignores default",
			route: Route{LimitConnPerServer: -1, LimitConnPerIP: -1, LimitRate: "-1", LimitReqPerIP: "-1"},
			cfg: ConfigSnapshot{
				DefaultLimitConnPerServer: 100,
				DefaultLimitConnPerIP:     10,
				DefaultLimitRate:          "512k",
				DefaultLimitReqPerIP:      "10r/s",
			},
			want: routeLimitConfig{},
		},
		{
			name:  "route overrides default",
			route: Route{LimitConnPerServer: 50, LimitConnPerIP: 5, LimitRate: "1m"},
			cfg: ConfigSnapshot{
				DefaultLimitConnPerServer: 100,
				DefaultLimitConnPerIP:     10,
				DefaultLimitRate:          "512k",
				DefaultLimitReqPerIP:      "10r/s",
			},
			want: routeLimitConfig{LimitConnPerServer: 50, LimitConnPerIP: 5, LimitRate: "1m", LimitReqPerIP: "10r/s"},
		},
		{
			name:  "partial inherit",
			route: Route{LimitConnPerServer: 0, LimitConnPerIP: -1, LimitRate: ""},
			cfg: ConfigSnapshot{
				DefaultLimitConnPerServer: 100,
				DefaultLimitConnPerIP:     10,
				DefaultLimitRate:          "256k",
				DefaultLimitReqPerIP:      "10r/s",
			},
			want: routeLimitConfig{LimitConnPerServer: 100, LimitConnPerIP: 0, LimitRate: "256k", LimitReqPerIP: "10r/s"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := mergeRouteLimitConfig(tc.route, tc.cfg)
			if got != tc.want {
				t.Fatalf("mergeRouteLimitConfig() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestRenderRouteConfigAppliesDefaultLimits(t *testing.T) {
	doc := Document{
		Routes: []Route{{
			SiteName:  "example.com",
			Domains:   []string{"example.com"},
			Enabled:   true,
			OriginURL: "http://127.0.0.1:8080",
			Upstreams: []string{"http://127.0.0.1:8080"},
		}},
		OpenRestyConfig: ConfigSnapshot{
			DefaultLimitConnPerServer: 120,
			DefaultLimitConnPerIP:     12,
			DefaultLimitRate:          "512k",
			DefaultLimitReqPerIP:      "10r/s",
		},
	}
	rendered, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatalf("RenderRouteConfig() error = %v", err)
	}
	for _, want := range []string{
		"limit_conn openflare_conn_per_server 120;",
		"limit_conn openflare_conn_per_ip 12;",
		"limit_rate 512k;",
		"limit_req zone=openflare_req_10rs burst=20 nodelay;",
		"limit_req_status 429;",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in route config, got:\n%s", want, rendered)
		}
	}
}

func TestRenderMainConfigEmitsLimitReqZonesByEffectiveRate(t *testing.T) {
	doc := Document{
		Routes: []Route{
			{
				SiteName:  "a.example.com",
				Domains:   []string{"a.example.com"},
				Enabled:   true,
				OriginURL: "http://127.0.0.1:8080",
				Upstreams: []string{"http://127.0.0.1:8080"},
			},
			{
				SiteName:      "b.example.com",
				Domains:       []string{"b.example.com"},
				Enabled:       true,
				OriginURL:     "http://127.0.0.1:8081",
				Upstreams:     []string{"http://127.0.0.1:8081"},
				LimitReqPerIP: "5r/s",
			},
			{
				SiteName:      "c.example.com",
				Domains:       []string{"c.example.com"},
				Enabled:       true,
				OriginURL:     "http://127.0.0.1:8082",
				Upstreams:     []string{"http://127.0.0.1:8082"},
				LimitReqPerIP: "-1",
			},
		},
		OpenRestyConfig: ConfigSnapshot{
			DefaultLimitReqPerIP: "10r/s",
		},
	}
	mainConfig := RenderMainConfig(doc)
	for _, want := range []string{
		"limit_req_zone $openflare_waf_site$binary_remote_addr zone=openflare_req_10rs:10m rate=10r/s;",
		"limit_req_zone $openflare_waf_site$binary_remote_addr zone=openflare_req_5rs:10m rate=5r/s;",
	} {
		if !strings.Contains(mainConfig, want) {
			t.Fatalf("expected %q in main config, got:\n%s", want, mainConfig)
		}
	}
	if strings.Contains(mainConfig, "openflare_req_per_ip") {
		t.Fatalf("unexpected legacy zone name in main config:\n%s", mainConfig)
	}

	routeConfig, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatalf("RenderRouteConfig() error = %v", err)
	}
	if !strings.Contains(routeConfig, "limit_req zone=openflare_req_10rs burst=20 nodelay;") {
		t.Fatalf("expected inherited zone on route a, got:\n%s", routeConfig)
	}
	if !strings.Contains(routeConfig, "limit_req zone=openflare_req_5rs burst=10 nodelay;") {
		t.Fatalf("expected custom zone on route b, got:\n%s", routeConfig)
	}
	// route c is off: count limit_req lines should equal 2 routes * (http+https? depends) — assert c server has no limit_req by site name block is hard; ensure -1 route does not force extra zones
	if strings.Count(mainConfig, "limit_req_zone") != 2 {
		t.Fatalf("expected exactly 2 limit_req_zone lines, got main:\n%s", mainConfig)
	}
}

func TestRenderRouteConfigExplicitOffSkipsDefaultLimits(t *testing.T) {
	doc := Document{
		Routes: []Route{{
			SiteName:           "example.com",
			Domains:            []string{"example.com"},
			Enabled:            true,
			OriginURL:          "http://127.0.0.1:8080",
			Upstreams:          []string{"http://127.0.0.1:8080"},
			LimitConnPerServer: -1,
			LimitConnPerIP:     -1,
			LimitRate:          "-1",
			LimitReqPerIP:      "-1",
		}},
		OpenRestyConfig: ConfigSnapshot{
			DefaultLimitConnPerServer: 120,
			DefaultLimitConnPerIP:     12,
			DefaultLimitRate:          "512k",
			DefaultLimitReqPerIP:      "10r/s",
		},
	}
	rendered, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatalf("RenderRouteConfig() error = %v", err)
	}
	if strings.Contains(rendered, "limit_conn") || strings.Contains(rendered, "limit_rate") || strings.Contains(rendered, "limit_req") {
		t.Fatalf("expected no limit directives, got:\n%s", rendered)
	}
}
