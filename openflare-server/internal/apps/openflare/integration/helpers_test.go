// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/common/response"
	v1 "github.com/Rain-kl/Wavelet/internal/router/v1"
	ofrouter "github.com/Rain-kl/Wavelet/internal/router/v1/openflare"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func decodeAPIResponse(t *testing.T, rec *httptest.ResponseRecorder) response.Any {
	t.Helper()

	var resp response.Any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp
}

func requireAPIOK(t *testing.T, rec *httptest.ResponseRecorder) response.Any {
	t.Helper()

	resp := decodeAPIResponse(t, rec)
	require.Empty(t, resp.ErrorMsg, "unexpected API error: %s", resp.ErrorMsg)
	return resp
}

func unmarshalAPIData(t *testing.T, data any, target any) {
	t.Helper()

	payload, err := json.Marshal(data)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(payload, target))
}

func unmarshalAPIMap(t *testing.T, data any) map[string]any {
	t.Helper()

	var result map[string]any
	unmarshalAPIData(t, data, &result)
	return result
}

func unmarshalAPISlice(t *testing.T, data any) []any {
	t.Helper()

	var result []any
	unmarshalAPIData(t, data, &result)
	return result
}

func mountOpenFlareTestRoutes(engine *gin.Engine) {
	api := engine.Group("/api")
	apiV1 := api.Group("/v1")
	v1.RegisterV1Routes(apiV1, api)
}

func apiPath(subpath string) string {
	return ofrouter.V1BasePath + subpath
}

func performJSONRequest(
	t *testing.T,
	engine http.Handler,
	method, path string,
	body any,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		require.NoError(t, err)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func adminAuthHeaders(token string) map[string]string {
	return map[string]string{
		"X-Access-Token": token,
	}
}