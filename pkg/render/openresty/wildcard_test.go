// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openresty

import (
	"regexp"
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

			for _, tt := range []struct{ domain, upstream, origin string }{
				{domain: "*.s3.example.com", upstream: "backend_wildcard_1", origin: "wildcard-origin:8080"},
				{domain: "special.s3.example.com", upstream: "backend_exact_2", origin: "exact-origin:8080"},
			} {
				upstreamPattern := `(?ms)^upstream ` + regexp.QuoteMeta(tt.upstream) + ` \{.*?^\}`
				upstream := regexp.MustCompile(upstreamPattern).FindString(got)
				if !strings.Contains(upstream, "server "+tt.origin+" ") {
					t.Errorf("upstream %s = %q, want origin %s", tt.upstream, upstream, tt.origin)
				}
				proxyCount, redirectCount := 0, 0
				for _, block := range regexp.MustCompile(`(?ms)^server \{.*?^\}`).FindAllString(got, -1) {
					if !strings.Contains(block, "server_name "+tt.domain+";") {
						continue
					}
					if https && strings.Contains(block, "listen 80;") {
						redirectCount++
						if !strings.Contains(block, "return 301 https://$host$request_uri;") {
							t.Errorf("redirect for %s = %q, want actual request host", tt.domain, block)
						}
					} else {
						proxyCount++
						if !strings.Contains(block, "proxy_pass http://"+tt.upstream+";") {
							t.Errorf("proxy for %s = %q, want upstream %s", tt.domain, block, tt.upstream)
						}
					}
				}
				wantRedirects := 0
				if https {
					wantRedirects = 1
				}
				if proxyCount != 1 || redirectCount != wantRedirects {
					t.Errorf("server blocks for %s: proxy=%d redirect=%d, want proxy=1 redirect=%d", tt.domain, proxyCount, redirectCount, wantRedirects)
				}
			}
		})
	}
}
