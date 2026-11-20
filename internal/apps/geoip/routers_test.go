// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package geoip

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGeoIPHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/geoip", GetGeoIPHandler)

	cases := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		wantIP     string
	}{
		{
			name:       "direct remote addr",
			remoteAddr: "82.22.26.203:51234",
			wantIP:     "82.22.26.203",
		},
		{
			name:       "x-forwarded-for behind proxy",
			remoteAddr: "172.27.0.1:51234",
			headers:    map[string]string{"X-Forwarded-For": "138.2.50.10"},
			wantIP:     "138.2.50.10",
		},
		{
			name:       "x-real-ip behind proxy",
			remoteAddr: "172.27.0.1:51234",
			headers:    map[string]string{"X-Real-IP": "1.1.1.1"},
			wantIP:     "1.1.1.1",
		},
		{
			name:       "ipv4-mapped ipv6 normalized",
			remoteAddr: "[::ffff:8.8.8.8]:443",
			wantIP:     "8.8.8.8",
		},
		{
			name:       "ipv6 remote addr",
			remoteAddr: "[2606:4700:4700::1111]:443",
			wantIP:     "2606:4700:4700::1111",
		},
		{
			name:       "empty remote falls back to 0.0.0.0",
			remoteAddr: "",
			wantIP:     "0.0.0.0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/geoip", nil)
			req.RemoteAddr = tc.remoteAddr
			for key, value := range tc.headers {
				req.Header.Set(key, value)
			}
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

			var envelope struct {
				ErrorMsg string       `json:"error_msg"`
				Data     ClientIPView `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
			assert.Empty(t, envelope.ErrorMsg)
			assert.Equal(t, tc.wantIP, envelope.Data.IP)
		})
	}
}

func TestResolveRequesterIPDirect(t *testing.T) {
	view := ResolveRequesterIP("", "")
	assert.Equal(t, response.OK(ClientIPView{IP: "0.0.0.0"}).Data, *view)
}
