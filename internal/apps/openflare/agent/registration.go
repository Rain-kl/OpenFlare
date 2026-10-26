// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

const registrationKeyBytes = 32

// discoveryIdentity derives separate public identity and private credential from a random retry secret.
// Public node IDs cannot be used as registration keys to recover credentials.
func discoveryIdentity(key string) (string, string, error) {
	decoded, err := hex.DecodeString(key)
	if err != nil || len(decoded) != registrationKeyBytes {
		return "", "", errors.New(errRegistrationKeyInvalid)
	}
	nodeHash := sha256.Sum256(append([]byte("openflare:discovery:node:v1:"), decoded...))
	tokenHash := sha256.Sum256(append([]byte("openflare:discovery:token:v1:"), decoded...))
	return "node-" + base64.RawURLEncoding.EncodeToString(nodeHash[:]), hex.EncodeToString(tokenHash[:]), nil
}
