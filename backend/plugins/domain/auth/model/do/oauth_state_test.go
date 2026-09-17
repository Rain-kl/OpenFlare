// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package do_test

import (
	"Wavelet/plugins/domain/auth/model/do"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuthStatePayload(t *testing.T) {
	payload := do.OAuthStatePayload{
		SourceName:  "github",
		Purpose:     "login",
		UserID:      12345,
		SessionHash: "hash-abc-123",
	}

	encoded, err := payload.Encode()
	require.NoError(t, err)
	assert.NotEmpty(t, encoded)

	decoded, err := do.DecodeOAuthStatePayload(encoded)
	require.NoError(t, err)
	assert.Equal(t, payload, decoded)

	_, err = do.DecodeOAuthStatePayload("invalid-json")
	assert.Error(t, err)
}
