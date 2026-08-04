// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

import "github.com/Rain-kl/Wavelet/internal/apps/openflare/credential"

func sealSensitive(plaintext string) (string, error) {
	return credential.Seal(plaintext)
}

// OpenKeyPEM decrypts a stored certificate private key for runtime distribution.
func OpenKeyPEM(stored string) (string, error) {
	return openSensitive(stored)
}

func openSensitive(stored string) (string, error) {
	return credential.Open(stored)
}
