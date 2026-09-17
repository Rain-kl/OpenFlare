// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Wavelet/openflare/plugins/server/kernel/repository"
	"Wavelet/pkg/response"

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

func TestGetGroupWithOrphanedMemberHealsAndSucceeds(t *testing.T) {
	ctx, memberID := setupCloudflareLogicDB(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(response.ErrorHandlerMiddleware())
	router.GET("/groups/:id", GetGroupHandler)

	member, err := repository.GetCFPointingMemberByID(ctx, memberID)
	if err != nil {
		t.Fatalf("GetCFPointingMemberByID() error = %v", err)
	}

	// Simulate orphaned member by deleting the ZoneDomain directly
	if err := repository.DB(ctx).Exec("DELETE FROM of_zone_domains WHERE id = ?", member.ZoneDomainID).Error; err != nil {
		t.Fatalf("DELETE FROM of_zone_domains error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/groups/1", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /groups/1 status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	// Verify the orphaned member has been removed
	_, err = repository.GetCFPointingMemberByID(ctx, memberID)
	if err == nil {
		t.Errorf("GetCFPointingMemberByID() should return not found after healing")
	}
}
