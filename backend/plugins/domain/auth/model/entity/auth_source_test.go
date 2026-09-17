// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package entity_test

import (
	"Wavelet/plugins/domain/auth/model/entity"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthSourceValidation(t *testing.T) {
	t.Run("Valid OIDC Source", func(t *testing.T) {
		src := entity.AuthSource{
			Name:               "google",
			Type:               "oidc",
			DisplayName:        "Google Sign-In",
			ClientID:           "client-123",
			ClientSecret:       "secret-456",
			OpenIDDiscoveryURL: "https://accounts.google.com",
			IsActive:           true,
		}
		require.NoError(t, src.Validate())
		assert.Equal(t, "openid profile email", src.Scopes)
		assert.Equal(t, "w_auth_sources", src.TableName())

		src.Sanitize()
		assert.True(t, src.ClientSecretConfigured)
		assert.Empty(t, src.ClientSecret)
	})

	t.Run("Empty Name Fails", func(t *testing.T) {
		src := entity.AuthSource{
			Name: "",
			Type: "oidc",
		}
		assert.Error(t, src.Validate())
	})

	t.Run("Invalid Name Format Fails", func(t *testing.T) {
		src := entity.AuthSource{
			Name: "invalid name with spaces!",
			Type: "oidc",
		}
		assert.Error(t, src.Validate())
	})

	t.Run("Unsupported Type Fails", func(t *testing.T) {
		src := entity.AuthSource{
			Name: "ldap_source",
			Type: "ldap",
		}
		assert.Error(t, src.Validate())
	})

	t.Run("Missing Discovery URL Fails", func(t *testing.T) {
		src := entity.AuthSource{
			Name: "google",
			Type: "oidc",
		}
		assert.Error(t, src.Validate())
	})

	t.Run("Active Source Missing Credentials Fails", func(t *testing.T) {
		src := entity.AuthSource{
			Name:               "google",
			Type:               "oidc",
			OpenIDDiscoveryURL: "https://accounts.google.com",
			IsActive:           true,
		}
		assert.Error(t, src.Validate())
	})
}
