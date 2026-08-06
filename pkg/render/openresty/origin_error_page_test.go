package openresty

import (
	"strings"
	"testing"
)

func TestRenderOriginErrorPageEnabled(t *testing.T) {
	t.Parallel()
	doc := Document{
		Routes: []Route{{
			ID: 1, SiteName: "ex", Domains: []string{"ex.test"},
			OriginURL: "http://127.0.0.1:9", Enabled: true,
		}},
		OpenRestyConfig: ConfigSnapshot{
			OriginErrorPageEnabled:     true,
			OriginErrorPageStatusCodes: []string{"500-599"},
		},
	}
	out, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "proxy_intercept_errors on") {
		t.Fatal("missing intercept")
	}
	if !strings.Contains(out, "error_page") || !strings.Contains(out, "/__openflare_origin_error") {
		t.Fatal("missing error_page")
	}
	if !strings.Contains(out, "error_page 500") {
		t.Fatalf("expected expanded status codes in error_page, got:\n%s", out)
	}
	// Must NOT use `error_page … = /uri` (adopts error-URI status → often 200).
	// `location = /path` is unrelated and expected.
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "error_page ") && strings.Contains(trimmed, " = ") {
			t.Fatalf("error_page must not use '=' form, got: %s", trimmed)
		}
	}
	if !strings.Contains(out, "error_page ") || !strings.Contains(out, " /__openflare_origin_error;") {
		t.Fatal("error_page must redirect to internal location without '='")
	}
	if !strings.Contains(out, "resolve_error_status") || !strings.Contains(out, "ngx.status = code") {
		t.Fatal("internal location must resolve and set ngx.status to the original error code")
	}
	if !strings.Contains(out, ErrorPageTmplPlaceholder) {
		t.Fatal("missing error page template placeholder")
	}
	res, err := Render(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range res.SupportFiles {
		if f.Path == OriginErrorPageSupportPath {
			found = true
			if !strings.Contains(f.Content, "{{status}}") {
				t.Fatal("template missing placeholder")
			}
			if !strings.Contains(f.Content, "{{host}}") {
				t.Fatal("template missing host placeholder")
			}
		}
	}
	if !found {
		t.Fatal("missing support file")
	}
}

func TestRenderOriginErrorPageGetOnly(t *testing.T) {
	t.Parallel()
	doc := Document{
		Routes: []Route{{
			ID: 1, SiteName: "ex", Domains: []string{"ex.test"},
			OriginURL: "http://127.0.0.1:9", Enabled: true,
		}},
		OpenRestyConfig: ConfigSnapshot{
			OriginErrorPageEnabled:     true,
			OriginErrorPageStatusCodes: []string{"500-599"},
			OriginErrorPageGetOnly:     true,
		},
	}
	out, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "proxy_intercept_errors on") {
		t.Fatal("missing intercept on")
	}
	if !strings.Contains(out, "limit_except GET") {
		t.Fatal("get_only must emit limit_except GET")
	}
	if !strings.Contains(out, "proxy_intercept_errors off") {
		t.Fatal("get_only must turn intercept off for non-GET")
	}
	if !strings.Contains(out, `get_only = true`) {
		t.Fatal("internal location must set get_only = true")
	}
	if !strings.Contains(out, `ngx.req.get_method() ~= "GET"`) {
		t.Fatal("internal location must skip HTML for non-GET")
	}
}

func TestRenderOriginErrorPageDisabled(t *testing.T) {
	t.Parallel()
	doc := Document{
		Routes: []Route{{
			ID: 1, SiteName: "ex", Domains: []string{"ex.test"},
			OriginURL: "http://127.0.0.1:9", Enabled: true,
		}},
		OpenRestyConfig: ConfigSnapshot{OriginErrorPageEnabled: false},
	}
	out, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "proxy_intercept_errors") {
		t.Fatal("should not intercept when disabled")
	}
	if strings.Contains(out, "/__openflare_origin_error") {
		t.Fatal("should not emit internal error location when disabled")
	}
	res, err := Render(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range res.SupportFiles {
		if f.Path == OriginErrorPageSupportPath {
			t.Fatal("should not emit support file when disabled")
		}
	}
}

func TestRenderOriginErrorPageDefaultsEmptyHTMLAndStatusCodes(t *testing.T) {
	t.Parallel()
	doc := Document{
		Routes: []Route{{
			ID: 1, SiteName: "ex", Domains: []string{"ex.test"},
			OriginURL: "http://127.0.0.1:9", Enabled: true,
		}},
		OpenRestyConfig: ConfigSnapshot{
			OriginErrorPageEnabled: true,
		},
	}
	out, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "error_page 500") {
		t.Fatalf("empty status codes should default to 500-599, got:\n%s", out)
	}
	html := EffectiveOriginErrorPageHTML(doc.OpenRestyConfig)
	if html != DefaultOriginErrorPageHTML {
		t.Fatal("empty HTML should use default template")
	}
	if !strings.Contains(html, "{{status}}") || !strings.Contains(html, "{{host}}") {
		t.Fatal("default HTML must include placeholders")
	}
	if !strings.Contains(html, "OpenFlare") || !strings.Contains(html, "upstream server is unreachable") {
		t.Fatal("default HTML missing minimalist copy")
	}
}

func TestRenderOriginErrorPageCustomHTMLInSupportFile(t *testing.T) {
	t.Parallel()
	custom := "<html><body>custom {{status}} @ {{host}}</body></html>"
	doc := Document{
		Routes: []Route{{
			ID: 1, SiteName: "ex", Domains: []string{"ex.test"},
			OriginURL: "http://127.0.0.1:9", Enabled: true,
		}},
		OpenRestyConfig: ConfigSnapshot{
			OriginErrorPageEnabled:     true,
			OriginErrorPageStatusCodes: []string{"502"},
			OriginErrorPageHTML:        custom,
		},
	}
	res, err := Render(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range res.SupportFiles {
		if f.Path == OriginErrorPageSupportPath {
			found = true
			if f.Content != custom {
				t.Fatalf("support file content = %q, want custom HTML", f.Content)
			}
		}
	}
	if !found {
		t.Fatal("missing support file")
	}
	out, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "error_page 502 /__openflare_origin_error;") {
		t.Fatalf("expected single 502 error_page without '=', got:\n%s", out)
	}
}

func TestRenderOriginErrorPageSkipsPagesRoutes(t *testing.T) {
	t.Parallel()
	doc := Document{
		Routes: []Route{{
			ID: 1, SiteName: "pages", Domains: []string{"pages.test"},
			UpstreamType: "pages", Enabled: true,
			PagesDeployment: &PagesDeployment{
				ProjectID: 1, LocalRoot: PagesDirPlaceholder + "/projects/1/current",
				EntryFile: "index.html",
			},
		}},
		OpenRestyConfig: ConfigSnapshot{
			OriginErrorPageEnabled:     true,
			OriginErrorPageStatusCodes: []string{"500-599"},
		},
	}
	out, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "proxy_intercept_errors") {
		t.Fatal("pages routes must not get proxy_intercept_errors")
	}
	if strings.Contains(out, "/__openflare_origin_error") {
		t.Fatal("pages routes must not get origin error location")
	}
}
