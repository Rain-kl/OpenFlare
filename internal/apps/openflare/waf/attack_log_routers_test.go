// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAttackLogRequestHandler(t *testing.T) {
	t.Cleanup(setupWAFTestDB(t))
	testhelper.SetupLogStoresForTest(t)
	ctx := context.Background()
	store, err := logstore.Active(ctx)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second).Add(-time.Minute)
	const id uint64 = 128401926193680388
	const preview = `{"message":"GET /?view=summary HTTP/1.1\r\n\r\n","truncated":false,"body_omitted":false}`
	require.NoError(t, store.AccessLogs.BatchInsertNodeAccessLogs(ctx, []analytics.NodeAccessLog{
		{ID: id, NodeID: "edge-a", LoggedAt: now.Add(250 * time.Millisecond), WAFAction: "blocked", WAFAudit: `{"action":"blocked","request":` + preview + `}`},
		{ID: id + 1, NodeID: "edge-a", LoggedAt: now, WAFAction: "blocked", WAFAudit: `{"action":"blocked"}`},
		{ID: id + 2, NodeID: "edge-a", LoggedAt: now},
	}))

	router := gin.New()
	router.Use(response.ErrorHandlerMiddleware())
	router.GET("/attack-logs", ListAttackLogsHandler)
	router.GET("/attack-logs/:id/request", GetAttackLogRequestHandler)
	get := func(path string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		return recorder
	}
	detail := func(record uint64) *httptest.ResponseRecorder {
		return get(fmt.Sprintf("/attack-logs/%d/request?node_id=edge-a&logged_at=%d", record, now.Unix()))
	}
	list := get("/attack-logs")
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.Contains(t, list.Body.String(), fmt.Sprintf(`"id":"%d"`, id))
	require.NotContains(t, list.Body.String(), `"request"`)
	request := detail(id)
	require.Equal(t, http.StatusOK, request.Code, request.Body.String())
	require.Equal(t, "no-store", request.Header().Get("Cache-Control"))
	require.JSONEq(t, `{"error_msg":"","data":{"request":`+preview+`}}`, request.Body.String())
	older := detail(id + 1)
	require.Equal(t, http.StatusOK, older.Code)
	require.Contains(t, older.Body.String(), `"request":null`)
	require.Equal(t, http.StatusNotFound, detail(id+2).Code)
	require.Equal(t, http.StatusBadRequest, get(fmt.Sprintf("/attack-logs/%d/request", id)).Code)
}
