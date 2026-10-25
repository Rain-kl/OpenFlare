// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSLSingleRenewHandler_ValidatePayload(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty payload",
			payload: nil,
			wantErr: true,
			errMsg:  "task parameters must not be empty",
		},
		{
			name:    "invalid JSON",
			payload: []byte(`{`),
			wantErr: true,
			errMsg:  "invalid JSON format",
		},
		{
			name:    "zero ID",
			payload: []byte(`{"id":0}`),
			wantErr: true,
			errMsg:  "certificate ID must be nonzero",
		},
		{
			name:    "valid ID",
			payload: []byte(`{"id":123}`),
			wantErr: false,
		},
	}

	handler := &SSLSingleRenewHandler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := handler.ValidatePayload(tt.payload)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				var payload SSLSingleRenewPayload
				err = json.Unmarshal(got, &payload)
				require.NoError(t, err)
				assert.Equal(t, uint(123), payload.ID)
			}
		})
	}
}

func TestSSLSingleRenewHandler_Execute(t *testing.T) {
	cleanup := setupTLSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	handler := &SSLSingleRenewHandler{}

	t.Run("invalid payload", func(t *testing.T) {
		_, err := handler.Execute(ctx, []byte(`{`))
		require.Error(t, err)
	})

	t.Run("certificate not found", func(t *testing.T) {
		payload, err := json.Marshal(SSLSingleRenewPayload{ID: 999})
		require.NoError(t, err)
		_, err = handler.Execute(ctx, payload)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get certificate record")
	})

	t.Run("certificate provider is not ACME", func(t *testing.T) {
		cert := &model.TLSCertificate{
			Name:          "custom-cert",
			Provider:      "custom",
			PrimaryDomain: "example.com",
		}
		err := repository.CreateTLSCertificateRecord(ctx, cert)
		require.NoError(t, err)

		payload, err := json.Marshal(SSLSingleRenewPayload{ID: cert.ID})
		require.NoError(t, err)

		_, err = handler.Execute(ctx, payload)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not ACME-managed")
	})

	t.Run("successful renewal", func(t *testing.T) {
		cert := &model.TLSCertificate{
			Name:          "acme-cert-success",
			Provider:      tlsProviderACME,
			PrimaryDomain: "success.example.com",
		}
		err := repository.CreateTLSCertificateRecord(ctx, cert)
		require.NoError(t, err)

		// Mock obtainCertificate to succeed
		restore := SetObtainCertificateFuncForTest(func(ctx context.Context, c *model.TLSCertificate) error {
			c.ApplyStatus = tlsApplyStatusReady
			return repository.SaveTLSCertificate(ctx, c)
		})
		defer restore()

		payload, err := json.Marshal(SSLSingleRenewPayload{ID: cert.ID})
		require.NoError(t, err)

		res, err := handler.Execute(ctx, payload)
		require.NoError(t, err)
		assert.Contains(t, res.Message, "renewed successfully")

		updated, err := repository.GetTLSCertificateByID(ctx, cert.ID)
		require.NoError(t, err)
		assert.Equal(t, tlsApplyStatusReady, updated.ApplyStatus)
	})

	t.Run("failed renewal in obtain", func(t *testing.T) {
		cert := &model.TLSCertificate{
			Name:          "acme-cert-fail",
			Provider:      tlsProviderACME,
			PrimaryDomain: "fail.example.com",
		}
		err := repository.CreateTLSCertificateRecord(ctx, cert)
		require.NoError(t, err)

		// Mock obtainCertificate to fail
		restore := SetObtainCertificateFuncForTest(func(ctx context.Context, c *model.TLSCertificate) error {
			return errors.New("ACME server timeout")
		})
		defer restore()

		payload, err := json.Marshal(SSLSingleRenewPayload{ID: cert.ID})
		require.NoError(t, err)

		_, err = handler.Execute(ctx, payload)
		require.Error(t, err)
		assert.Equal(t, "ACME server timeout", err.Error())
	})
}
