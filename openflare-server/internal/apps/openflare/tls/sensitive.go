// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/pkg/util"
)

const sensitiveValuePrefix = "enc:v1:"

func sensitiveEncryptionKey() string {
	if config.Config == nil || strings.TrimSpace(config.Config.App.SessionSecret) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(config.Config.App.SessionSecret))
	return hex.EncodeToString(sum[:])
}

func sealSensitive(plaintext string) (string, error) {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return "", nil
	}
	key := sensitiveEncryptionKey()
	if key == "" {
		return plaintext, nil
	}
	encrypted, err := util.Encrypt(key, plaintext)
	if err != nil {
		return "", err
	}
	return sensitiveValuePrefix + encrypted, nil
}

func openSensitive(stored string) (string, error) {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, sensitiveValuePrefix) {
		return stored, nil
	}
	key := sensitiveEncryptionKey()
	if key == "" {
		return "", errors.New("cannot decrypt sensitive field without session secret")
	}
	return util.Decrypt(key, strings.TrimPrefix(stored, sensitiveValuePrefix))
}
