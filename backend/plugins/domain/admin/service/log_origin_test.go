// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service_test

import (
	"Wavelet/plugins/domain/admin/service"
	"context"
	"testing"
)

func TestIsAllowedLogOrigin(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		origin     string
		host       string
		extraHosts []string
		want       bool
	}{
		{name: "empty origin", origin: "", host: "localhost:8000", want: true},
		{name: "same host", origin: "http://localhost:8000", host: "localhost:8000", want: true},
		{name: "same host different case", origin: "http://LocalHost:8000", host: "localhost:8000", want: true},
		{
			name:   "next rewrite different port same hostname",
			origin: "http://localhost:3000",
			host:   "localhost:8000",
			want:   true,
		},
		{
			name:       "x-forwarded-host matches origin",
			origin:     "http://localhost:3000",
			host:       "backend:8080",
			extraHosts: []string{"localhost:3000"},
			want:       true,
		},
		{
			name:   "unrelated origin",
			origin: "https://evil.example",
			host:   "localhost:8000",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.IsAllowedLogOrigin(ctx, tt.origin, tt.host, tt.extraHosts...)
			if got != tt.want {
				t.Errorf("IsAllowedLogOrigin(%q, %q, %v) = %v, want %v",
					tt.origin, tt.host, tt.extraHosts, got, tt.want)
			}
		})
	}
}
