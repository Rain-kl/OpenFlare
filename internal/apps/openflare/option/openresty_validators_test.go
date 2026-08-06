// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package option

import (
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
			wantErr: "无效状态码",
		},
		{
			name:    "非 JSON 数组",
			value:   `500-599`,
			wantErr: "必须为 JSON 字符串数组",
		},
		{
			name:    "空数组",
			value:   `[]`,
			wantErr: "至少包含一个状态码标签",
		},
		{
			name:    "越界状态码",
			value:   `["399"]`,
			wantErr: "状态码须在",
		},
		{
			name:    "空字符串",
			value:   "",
			wantErr: "不能为空",
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

func TestValidateOriginErrorPageHTML(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateOpenRestyOption(model.ConfigKeyOriginErrorPageHTML, ""))
	require.NoError(t, validateOpenRestyOption(model.ConfigKeyOriginErrorPageHTML, "<html>ok</html>"))

	oversized := strings.Repeat("a", maxOriginErrorPageHTMLBytes+1)
	err := validateOpenRestyOption(model.ConfigKeyOriginErrorPageHTML, oversized)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "长度不能超过")

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
	assert.Contains(t, err.Error(), "true 或 false")
}
