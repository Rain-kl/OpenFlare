// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
	"testing"

	openrestyrender "github.com/Rain-kl/Wavelet/pkg/render/openresty"
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

func TestLegacyAgentSiteAuthConfig(t *testing.T) {
	const source = `{"routes":[
		{"basic_auth_username":"visitor","basic_auth_password":"secret","site_auth_ttl_hours":24,"site_auth_key":"private","pages_deployment":{"api_proxy_enabled":true,"api_proxy_pass":"http://localhost"}},
		{"pages_deployment":{"api_proxy_enabled":true}}
	]}`
	config, err := legacyAgentSiteAuthConfig(source)
	if err != nil {
		t.Fatal(err)
	}

	var doc openrestyrender.Document
	if err := json.Unmarshal([]byte(config), &doc); err != nil {
		t.Fatal(err)
	}
	protected, public := doc.Routes[0], doc.Routes[1]
	if !protected.BasicAuthEnabled || !protected.RedirectHTTP || protected.BasicAuthUsername != "visitor" ||
		protected.BasicAuthPassword != "secret" || protected.SiteAuthKey != "" || protected.SiteAuthTTLHours != 0 ||
		protected.PagesDeployment == nil || protected.PagesDeployment.APIProxyEnabled {
		t.Error("legacy config must enforce Basic Auth over HTTPS and close the Pages API location")
	}
	if public.BasicAuthEnabled || public.PagesDeployment == nil || !public.PagesDeployment.APIProxyEnabled {
		t.Error("legacy config must preserve the public Pages API location")
	}
}
