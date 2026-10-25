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
			err := validateOpenRestyOption(model.ConfigKeyOriginErrorPageStatusCodes, tt.value)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidateOpenRestyTrustedProxyCIDRs(t *testing.T) {
	require.NoError(t, validateOpenRestyOption(model.ConfigKeyOpenRestyTrustedProxyCIDRs, `[]`))
	require.NoError(t, validateOpenRestyOption(model.ConfigKeyOpenRestyTrustedProxyCIDRs, `["173.245.48.0/20","10.2.0.0/24"]`))
	for _, value := range []string{`null`, `10.2.0.0/24`, `["10.2.0.0/24;return 200"]`, `["]`} {
		require.Error(t, validateOpenRestyOption(model.ConfigKeyOpenRestyTrustedProxyCIDRs, value), "value %q must be rejected", value)
	}
}

func TestValidateOriginErrorPageHTML(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateOpenRestyOption(model.ConfigKeyOriginErrorPageHTML, ""))
	require.NoError(t, validateOpenRestyOption(model.ConfigKeyOriginErrorPageHTML, "<html>ok</html>"))

	oversized := strings.Repeat("a", maxOriginErrorPageHTMLBytes+1)
	err := validateOpenRestyOption(model.ConfigKeyOriginErrorPageHTML, oversized)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not exceed")

	// 恰好上限应通过
	atLimit := strings.Repeat("b", maxOriginErrorPageHTMLBytes)
	require.NoError(t, validateOpenRestyOption(model.ConfigKeyOriginErrorPageHTML, atLimit))
}

func TestValidateOriginErrorPageEnabled(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateOpenRestyOption(model.ConfigKeyOriginErrorPageEnabled, "true"))
	require.NoError(t, validateOpenRestyOption(model.ConfigKeyOriginErrorPageEnabled, "false"))
	err := validateOpenRestyOption(model.ConfigKeyOriginErrorPageEnabled, "yes")
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
