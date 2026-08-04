// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func TestConnectionHandlersNeverReturnAPIToken(t *testing.T) {
	setupCloudflareLogicDB(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(response.ErrorHandlerMiddleware())
	router.PUT("/connection", SaveConnectionHandler)
	router.GET("/connection", GetConnectionHandler)

	save := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/connection", strings.NewReader(`{"source":"standalone","api_token":"top-secret-token"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(save, request)
	if save.Code != http.StatusOK {
		t.Fatalf("PUT /connection status = %d, body = %s", save.Code, save.Body.String())
	}
	if strings.Contains(save.Body.String(), "top-secret-token") || strings.Contains(save.Body.String(), "api_token") {
		t.Fatalf("PUT /connection leaked token: %s", save.Body.String())
	}

	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/connection", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET /connection status = %d, body = %s", get.Code, get.Body.String())
	}
	if strings.Contains(get.Body.String(), "top-secret-token") || strings.Contains(get.Body.String(), "api_token") {
		t.Fatalf("GET /connection leaked token: %s", get.Body.String())
	}
}
