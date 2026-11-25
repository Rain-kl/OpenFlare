// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package option

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateOriginErrorPageStatusCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{
			name:  "合法单码与区间",
			value: `["522","500-502"]`,
		},
		{
			name:  "默认区间",
			value: `["500-599"]`,
		},
		{
			name:    "非法标签",
			value:   `["abc"]`,
			wantErr: "invalid status code",
		},
		{
			name:    "非 JSON 数组",
			value:   `500-599`,
			wantErr: "must be a JSON array of strings",
		},
		{
			name:    "空数组",
			value:   `[]`,
			wantErr: "at least one status code tag",
		},
		{
			name:    "越界状态码",
			value:   `["399"]`,
			wantErr: "status code must be between",
		},
		{
			name:    "空字符串",
			value:   "",
			wantErr: "must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateEdgeOption(model.ConfigKeyOriginErrorPageStatusCodes, tt.value)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidateEdgeTrustedProxyCIDRs(t *testing.T) {
	require.NoError(t, validateEdgeOption(model.ConfigKeyEdgeTrustedProxyCIDRs, `[]`))
	require.NoError(t, validateEdgeOption(model.ConfigKeyEdgeTrustedProxyCIDRs, `["173.245.48.0/20","10.2.0.0/24"]`))
	for _, value := range []string{`null`, `10.2.0.0/24`, `["10.2.0.0/24;return 200"]`, `["]`} {
		require.Error(t, validateEdgeOption(model.ConfigKeyEdgeTrustedProxyCIDRs, value), "value %q must be rejected", value)
	}
}

func TestValidateOriginErrorPageHTML(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateEdgeOption(model.ConfigKeyOriginErrorPageHTML, ""))
	require.NoError(t, validateEdgeOption(model.ConfigKeyOriginErrorPageHTML, "<html>ok</html>"))

	oversized := strings.Repeat("a", maxOriginErrorPageHTMLBytes+1)
	err := validateEdgeOption(model.ConfigKeyOriginErrorPageHTML, oversized)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not exceed")

	// 恰好上限应通过
	atLimit := strings.Repeat("b", maxOriginErrorPageHTMLBytes)
	require.NoError(t, validateEdgeOption(model.ConfigKeyOriginErrorPageHTML, atLimit))
}

func TestValidateOriginErrorPageEnabled(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateEdgeOption(model.ConfigKeyOriginErrorPageEnabled, "true"))
	require.NoError(t, validateEdgeOption(model.ConfigKeyOriginErrorPageEnabled, "false"))
	err := validateEdgeOption(model.ConfigKeyOriginErrorPageEnabled, "yes")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "true or false")
}

func TestValidateSWOfflineDomains(t *testing.T) {
	cases := []struct {
		name  string
		value string
		ok    bool
	}{
		{"empty array", `[]`, true},
		{"single", `["example.com"]`, true},
		{"multiple", `["example.com","api.example.com"]`, true},
		{"invalid json", `not-json`, false},
		{"null", "null", false},
		{"empty element", `[""]`, false},
		{"duplicate", `["example.com","example.com"]`, false},
		{"whitespace dedup", `[" Example.com ","example.com"]`, false},
		{"over limit", fmt.Sprintf(`[%s]`, strings.Repeat(`"a.com",`, maxSWOfflineDomains)+`"a.com"`), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSWOfflineDomains("sw_offline_domains", tc.value)
			if tc.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

func TestValidatePoWPageOption(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{
			name:  "valid custom html",
			value: `{"template":"shield","widget":"turnstile","html":"<main>{{host}}{{widget}}</main>"}`,
		},
		{
			name:  "empty and whitespace html",
			value: `{"template":"classic","widget":"progress","html":"   "}`,
		},
		{
			name:    "invalid template",
			value:   `{"template":"unknown","widget":"progress"}`,
			wantErr: true,
		},
		{
			name:    "invalid widget",
			value:   `{"template":"shield","widget":"captcha"}`,
			wantErr: true,
		},
		{
			name:    "unknown field",
			value:   `{"template":"shield","widget":"progress","extra":true}`,
			wantErr: true,
		},
		{
			name:    "null json",
			value:   `null`,
			wantErr: true,
		},
		{
			name:    "non-object json",
			value:   `[]`,
			wantErr: true,
		},
		{
			name:    "oversized html",
			value:   fmt.Sprintf(`{"template":"shield","widget":"progress","html":"%s"}`, strings.Repeat("a", (256<<10)+1)),
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateEdgeOption(model.ConfigKeyPoWPage, tc.value)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
