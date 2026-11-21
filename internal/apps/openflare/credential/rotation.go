// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"context"
	"errors"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/util"
)

// MinSessionSecretBytes is the minimum length of a secret used for automation.
const MinSessionSecretBytes = 32

// Rotate re-encrypts the stored credentials with a new session secret. In check
// mode, it verifies the operation without applying database updates.
func Rotate(ctx context.Context, oldSecret, newSecret string, apply bool) ([]repository.CredentialRotationCount, error) {
	if len(newSecret) < MinSessionSecretBytes {
		return nil, errors.New("the new session secret must be at least 32 bytes")
	}
	oldKey, newKey := deriveKey(oldSecret), deriveKey(newSecret)
	return repository.RotateSealedCredentials(ctx, Prefix, apply, func(value string) (string, error) {
		return rewrapCredential(value, oldKey, newKey)
	})
}

func rewrapCredential(stored, oldKey, newKey string) (string, error) {
	ciphertext := strings.TrimPrefix(strings.TrimSpace(stored), Prefix)
	if _, err := util.Decrypt(newKey, ciphertext); err == nil {
		return stored, nil
	}
	if oldKey == "" {
		return "", errors.New("the current configuration has no old session secret")
	}
	plaintext, err := util.Decrypt(oldKey, ciphertext)
	if err != nil {
		return "", errors.New("unable to decrypt with the current or new session secret")
	}
	replacement, err := sealWithKey(newKey, plaintext)
	if err != nil {
		return "", errors.New("unable to encrypt with the new session secret")
	}
	decrypted, err := util.Decrypt(newKey, strings.TrimPrefix(replacement, Prefix))
	if err != nil || decrypted != plaintext {
		return "", errors.New("could not verify the encrypted replacement")
	}
	return replacement, nil
}
