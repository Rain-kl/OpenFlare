// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/pkg/util"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
)

var (
	credentialSecretMu sync.RWMutex
	credentialSecret   string
)

// SetCredentialSecret sets the secret used to derive CredentialKey.
func SetCredentialSecret(secret string) {
	credentialSecretMu.Lock()
	defer credentialSecretMu.Unlock()
	credentialSecret = secret
}

// CredentialKey is AES-256 hex derived from the session secret.
func CredentialKey() string {
	credentialSecretMu.RLock()
	secret := credentialSecret
	credentialSecretMu.RUnlock()
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// EncryptCredentials encrypts a credential map as JSON ciphertext.
func EncryptCredentials(creds map[string]string) (string, error) {
	if creds == nil {
		creds = map[string]string{}
	}
	raw, err := json.Marshal(creds)
	if err != nil {
		return "", err
	}
	return util.Encrypt(CredentialKey(), string(raw))
}

// DecryptCredentials decrypts a credential map from ciphertext.
func DecryptCredentials(ciphertext string) (map[string]string, error) {
	if ciphertext == "" {
		return map[string]string{}, nil
	}
	plain, err := util.Decrypt(CredentialKey(), ciphertext)
	if err != nil {
		return nil, err
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(plain), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]string{}
	}
	return out, nil
}

// ParseExtra decodes optional extra JSON into a string map.
func ParseExtra(raw string) map[string]string {
	if raw == "" {
		return map[string]string{}
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil || out == nil {
		return map[string]string{}
	}
	return out
}

// EncodeExtra encodes extra fields as JSON string.
func EncodeExtra(extra map[string]string) string {
	if extra == nil {
		return ""
	}
	raw, err := json.Marshal(extra)
	if err != nil {
		return ""
	}
	return string(raw)
}

// MaskCredentials hides secret-bearing credential entries.
func MaskCredentials(_ string, in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		if k == "token" || k == "client_secret" || k == "app_secret" || k == "bot_token" {
			out[k] = MaskSecret(v)
		} else {
			out[k] = v
		}
	}
	return out
}

const minMaskSecretLength = 8

// MaskSecret keeps only a short visible prefix and suffix of a secret.
func MaskSecret(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= minMaskSecretLength {
		return "******"
	}
	return s[:4] + "..." + s[len(s)-4:]
}
