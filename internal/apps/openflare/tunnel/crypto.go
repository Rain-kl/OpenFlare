// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

const (
	bootstrapTokenPrefix = "oftok1_"
	bootstrapKindRelay   = "relay"
	bootstrapKindTunnel  = "tunnel"
)

// BootstrapTokenPayload packs control-plane URL, credential role, secret token, and optional pool key
// into a single opaque string that users can pass directly to openflare-relay or openflared --token.
type BootstrapTokenPayload struct {
	Version   int    `json:"v"`
	ServerURL string `json:"s,omitempty"`
	Kind      string `json:"k"`
	Token     string `json:"t"`
	PoolKey   string `json:"p,omitempty"`
}

// GenerateNoiseKeypair creates a Curve25519 keypair base64-encoded for Rathole Noise_NK_25519_ChaChaPoly_BLAKE2s.
func GenerateNoiseKeypair() (privateB64 string, publicB64 string, err error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(priv.Bytes()),
		base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes()),
		nil
}

// GenerateRandomToken creates a cryptographically random hex token with an optional prefix.
func GenerateRandomToken(prefix string, numBytes int) (string, error) {
	if numBytes <= 0 {
		numBytes = 24
	}
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(buf), nil
}

// EncodeBootstrapToken serializes a self-contained CLI bootstrap token string.
func EncodeBootstrapToken(serverURL, kind, token, poolKey string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("token cannot be empty")
	}
	payload := BootstrapTokenPayload{
		Version:   1,
		ServerURL: strings.TrimRight(strings.TrimSpace(serverURL), "/"),
		Kind:      strings.TrimSpace(kind),
		Token:     token,
		PoolKey:   strings.TrimSpace(poolKey),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return bootstrapTokenPrefix + base64.RawURLEncoding.EncodeToString(encoded), nil
}

// DecodeBootstrapToken parses an oftok1_* token (or raw base64url JSON) and returns its payload.
func DecodeBootstrapToken(raw string) (*BootstrapTokenPayload, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("token cannot be empty")
	}
	trimmed = strings.TrimPrefix(trimmed, bootstrapTokenPrefix)
	decoded, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, err
	}
	var payload BootstrapTokenPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, err
	}
	if payload.Version != 1 || strings.TrimSpace(payload.Token) == "" {
		return nil, errors.New("invalid bootstrap token payload")
	}
	return &payload, nil
}

// ExtractRawToken extracts the underlying secret token whether caller supplied a raw token or oftok1_ wrapper.
func ExtractRawToken(raw string, expectedKind string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, bootstrapTokenPrefix) {
		if payload, err := DecodeBootstrapToken(trimmed); err == nil {
			if expectedKind == "" || payload.Kind == "" || payload.Kind == expectedKind {
				return strings.TrimSpace(payload.Token)
			}
		}
	}
	return trimmed
}

func constantTimeTokenEqual(expected, supplied string) bool {
	if expected == "" || supplied == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(supplied)) == 1
}
