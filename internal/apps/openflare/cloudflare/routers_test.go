// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
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
	if err := db.DB(ctx).Exec("DELETE FROM of_zone_domains WHERE id = ?", member.ZoneDomainID).Error; err != nil {
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

func TestListGroupsHandlerWithMissingNodeStillSucceeds(t *testing.T) {
	ctx, _ := setupCloudflareLogicDB(t)
	if err := db.DB(ctx).Create(&model.CFPointingGroup{
		Name:          "KR",
		PrimaryNodeID: 12,
		ActiveNodeID:  12,
		Enabled:       true,
	}).Error; err != nil {
		t.Fatalf("Create(missing-node group) error = %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(response.ErrorHandlerMiddleware())
	router.GET("/groups", ListGroupsHandler)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/groups", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("ListGroupsHandler status = %d, body = %s, want %d", recorder.Code, recorder.Body.String(), http.StatusOK)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"name":"KR"`) {
		t.Fatalf("ListGroupsHandler body = %s, want group KR", body)
	}
	if !strings.Contains(body, `"name":"primary"`) {
		t.Fatalf("ListGroupsHandler body = %s, want intact group primary", body)
	}
}

func TestGetGroupHandlerWithMissingNodeStillSucceeds(t *testing.T) {
	ctx, _ := setupCloudflareLogicDB(t)
	group := model.CFPointingGroup{Name: "KR", PrimaryNodeID: 12, ActiveNodeID: 12, Enabled: true}
	if err := db.DB(ctx).Create(&group).Error; err != nil {
		t.Fatalf("Create(missing-node group) error = %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(response.ErrorHandlerMiddleware())
	router.GET("/groups/:id", GetGroupHandler)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/groups/%d", group.ID), nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GetGroupHandler status = %d, body = %s, want %d", recorder.Code, recorder.Body.String(), http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"name":"KR"`) {
		t.Fatalf("GetGroupHandler body = %s, want group KR", recorder.Body.String())
	}
}
