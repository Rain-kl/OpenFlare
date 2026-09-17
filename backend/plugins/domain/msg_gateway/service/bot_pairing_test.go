// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service_test

import (
	"Wavelet/plugins/domain/msg_gateway/consts"
	"Wavelet/plugins/domain/msg_gateway/service"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCode_AlphabetAndLength(t *testing.T) {
	code, err := service.GenerateCode()
	require.NoError(t, err)
	assert.Len(t, code, consts.CodeLength)
	for _, r := range code {
		assert.Contains(t, consts.CodeAlphabet, string(r))
	}
}

func TestNormalizeAndFormat(t *testing.T) {
	assert.Equal(t, "ABCDEFGH", service.NormalizeCode("ab-cd-ef-gh"))
	assert.Equal(t, "ABCD-EFGH", service.FormatCode("ABCDEFGH"))
}
