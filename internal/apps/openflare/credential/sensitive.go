// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package credential seals and opens OpenFlare integration credentials.
package credential

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/Rain-kl/Wavelet/pkg/util"
)

// Prefix identifies values encrypted with the current credential format.
const Prefix = "enc:v1:"

func encryptionKey() string {
	if config.Config == nil {
		return ""
	}
	return deriveKey(config.Config.App.SessionSecret)
}

func deriveKey(secret string) string {
	if strings.TrimSpace(secret) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// Seal trims and encrypts plaintext when a session secret is configured.
// Plaintext storage is preserved for installations without a session secret.
func Seal(plaintext string) (string, error) {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return "", nil
	}
	key := encryptionKey()
	if key == "" {
		return plaintext, nil
	}
	return sealWithKey(key, plaintext)
}

func sealWithKey(key, plaintext string) (string, error) {
	encrypted, err := util.Encrypt(key, plaintext)
	if err != nil {
		return "", err
	}
	return Prefix + encrypted, nil
}

// Open decrypts a sealed value and accepts legacy plaintext values.
func Open(stored string) (string, error) {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, Prefix) {
		return stored, nil
	}
	key := encryptionKey()
	if key == "" {
		return "", errors.New("cannot decrypt sensitive field without session secret")
	}
	return util.Decrypt(key, strings.TrimPrefix(stored, Prefix))
}
