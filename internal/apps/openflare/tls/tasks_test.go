// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

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
			errMsg:  "任务参数不能为空",
		},
		{
			name:    "invalid JSON",
			payload: []byte(`{`),
			wantErr: true,
			errMsg:  "无效的 JSON 格式",
		},
		{
			name:    "zero ID",
			payload: []byte(`{"id":0}`),
			wantErr: true,
			errMsg:  "证书 ID 不能为空或零",
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
		assert.Contains(t, err.Error(), "获取证书记录失败")
	})

	t.Run("certificate provider is not ACME", func(t *testing.T) {
		cert := &model.TLSCertificate{
			Name:           "custom-cert",
			Provider:       "custom",
			PrimaryDomain:  "example.com",
		}
		err := model.CreateTLSCertificateRecord(ctx, cert)
		require.NoError(t, err)

		payload, err := json.Marshal(SSLSingleRenewPayload{ID: cert.ID})
		require.NoError(t, err)

		_, err = handler.Execute(ctx, payload)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "不是 ACME 托管证书")
	})

	t.Run("successful renewal", func(t *testing.T) {
		cert := &model.TLSCertificate{
			Name:          "acme-cert-success",
			Provider:      tlsProviderACME,
			PrimaryDomain: "success.example.com",
		}
		err := model.CreateTLSCertificateRecord(ctx, cert)
		require.NoError(t, err)

		// Mock obtainCertificate to succeed
		restore := SetObtainCertificateFuncForTest(func(ctx context.Context, c *model.TLSCertificate) error {
			c.ApplyStatus = tlsApplyStatusReady
			return model.SaveTLSCertificate(ctx, c)
		})
		defer restore()

		payload, err := json.Marshal(SSLSingleRenewPayload{ID: cert.ID})
		require.NoError(t, err)

		res, err := handler.Execute(ctx, payload)
		require.NoError(t, err)
		assert.Contains(t, res.Message, "续签成功")

		updated, err := model.GetTLSCertificateByID(ctx, cert.ID)
		require.NoError(t, err)
		assert.Equal(t, tlsApplyStatusReady, updated.ApplyStatus)
	})

	t.Run("failed renewal in obtain", func(t *testing.T) {
		cert := &model.TLSCertificate{
			Name:          "acme-cert-fail",
			Provider:      tlsProviderACME,
			PrimaryDomain: "fail.example.com",
		}
		err := model.CreateTLSCertificateRecord(ctx, cert)
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
