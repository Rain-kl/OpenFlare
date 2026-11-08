// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openresty

import (
	"strings"
	"testing"
)

func TestWildcardCertificateCoverage(t *testing.T) {
	tests := []struct {
		certDomain string
		domain     string
		wantErr    bool
	}{
		{certDomain: "*.s3.example.com", domain: "*.s3.example.com"},
		{certDomain: "*.s3.example.com", domain: "bucket.s3.example.com"},
		{certDomain: "*.example.com", domain: "*.s3.example.com", wantErr: true},
		{certDomain: "bucket.s3.example.com", domain: "*.s3.example.com", wantErr: true},
		{certDomain: "s3.example.com", domain: "*.s3.example.com", wantErr: true},
		{certDomain: "*.s3.example.com", domain: "s3.example.com", wantErr: true},
		{certDomain: "*.s3.example.com", domain: "nested.bucket.s3.example.com", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.certDomain+"_for_"+tt.domain, func(t *testing.T) {
			err := validateCertificateCoverage(testCertificatePEMForDomain(t, tt.certDomain), []string{tt.domain})
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCertificateCoverage(cert=%q, domain=%q) error = %v, want error=%t", tt.certDomain, tt.domain, err, tt.wantErr)
			}
		})
	}
}

func TestRenderWildcardAndExactRoutes(t *testing.T) {
	for _, https := range []bool{false, true} {
		t.Run(map[bool]string{false: "HTTP", true: "HTTPS"}[https], func(t *testing.T) {
			doc := Document{Routes: []Route{
				{ID: 1, SiteName: "wildcard", Domains: []string{"*.s3.example.com"}, OriginURL: "http://wildcard-origin:8080", EnableHTTPS: https, RedirectHTTP: https, DomainCertIDs: []uint{1}},
				{ID: 2, SiteName: "exact", Domains: []string{"special.s3.example.com"}, OriginURL: "http://exact-origin:8080", EnableHTTPS: https, RedirectHTTP: https, DomainCertIDs: []uint{1}},
			}}
			files := []SupportFile{{Path: "1.crt", Content: testCertificatePEMForDomain(t, "*.s3.example.com")}}
			got, err := RenderRouteConfig(doc, files)
			if err != nil {
				t.Fatalf("RenderRouteConfig(wildcard+exact, HTTPS=%t) error = %v", https, err)
			}
			for _, want := range []string{"server_name *.s3.example.com;", "server_name special.s3.example.com;", "server wildcard-origin:8080", "server exact-origin:8080"} {
				if !strings.Contains(got, want) {
					t.Errorf("RenderRouteConfig(wildcard+exact) missing %q", want)
				}
			}
			if https && !strings.Contains(got, "return 301 https://$host$request_uri;") {
				t.Error("RenderRouteConfig(wildcard HTTPS) must redirect using the actual request host")
			}
		})
	}
}
