// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package uptimekuma

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactSensitiveJSON(t *testing.T) {
	payload := []byte(`["login",{"username":"admin","password":"s3cret","nested":{"token":"abc","label":"keep"}},"plain"]`)
	var decoded any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	out, err := json.Marshal(redactSensitiveJSON(decoded))
	require.NoError(t, err)

	s := string(out)
	assert.NotContains(t, s, "s3cret")
	assert.NotContains(t, s, "abc")
	assert.Contains(t, s, `"password":"***"`)
	assert.Contains(t, s, `"token":"***"`)
	assert.Contains(t, s, `"label":"keep"`)
	assert.Contains(t, s, `"username":"admin"`)
}
