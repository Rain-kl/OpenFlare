// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/task"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

type sourceHandlerEnvelope struct {
	ErrorMsg string          `json:"error_msg"`
	Data     json.RawMessage `json:"data"`
}

func newPagesSourceTestRouter(userID uint64) *gin.Engine {
	router := testhelper.NewTestGinEngine(func(ctx *gin.Context) {
		oauth.SetToContext(ctx, oauth.UserObjKey, &model.User{ID: userID})
		ctx.Next()
	})
	router.GET("/api/v1/d/pages/:id/source", GetSourceHandler)
	router.POST("/api/v1/d/pages/:id/source/update", UpdateSourceHandler)
	router.POST("/api/v1/d/pages/:id/source/delete", DeleteSourceHandler)
	router.POST("/api/v1/d/pages/:id/source/check", CheckSourceHandler)
	router.POST("/api/v1/d/pages/:id/source/sync", SyncSourceHandler)
	return router
}

func performPagesSourceRequest(
	t *testing.T,
	router http.Handler,
	method string,
	path string,
	body []byte,
) (int, sourceHandlerEnvelope) {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	var envelope sourceHandlerEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(%s %s response %q) error = %v, want nil", method, path, recorder.Body.String(), err)
	}
	return recorder.Code, envelope
}

func setupPagesSourceDispatchTest(t *testing.T) {
	t.Helper()
	miniRedis, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v, want nil", err)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: miniRedis.Addr()})
	previousRedis := db.Redis
	previousAsynqClient := task.AsynqClient
	db.Redis = redisClient
	task.AsynqClient = asynqClient
	task.RegisterTaskMeta(PagesSourceActionMeta)
	t.Cleanup(func() {
		_ = asynqClient.Close()
		_ = redisClient.Close()
		miniRedis.Close()
		task.AsynqClient = previousAsynqClient
		db.Redis = previousRedis
	})
}

func TestPagesSourceHandlersReturnStableActionErrors(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	router := newPagesSourceTestRouter(42)

	manualProject := mustCreatePagesSourceProject(t, ctx, "handler-no-source")
	code, envelope := performPagesSourceRequest(
		t,
		router,
		http.MethodPost,
		fmt.Sprintf("/api/v1/d/pages/%d/source/sync", manualProject.ID),
		nil,
	)
	if got, want := code, http.StatusNotFound; got != want {
		t.Errorf("POST source/sync without source status = %d, want %d", got, want)
	}
	if got, want := envelope.ErrorMsg, errPagesSourceNotFound; got != want {
		t.Errorf("POST source/sync without source error = %q, want %q", got, want)
	}

	remoteProject := mustCreatePagesSourceProject(t, ctx, "handler-check")
	_, _ = mustConfigureRemoteSource(
		t,
		ctx,
		remoteProject.ID,
		"https://example.com/site.zip?token=handler-secret",
		false,
	)
	code, envelope = performPagesSourceRequest(
		t,
		router,
		http.MethodPost,
		fmt.Sprintf("/api/v1/d/pages/%d/source/check", remoteProject.ID),
		nil,
	)
	if got, want := code, http.StatusBadRequest; got != want {
		t.Errorf("POST remote source/check status = %d, want %d", got, want)
	}
	if got, want := envelope.ErrorMsg, errPagesSourceCheckUnsupported; got != want {
		t.Errorf("POST remote source/check error = %q, want %q", got, want)
	}
	if strings.Contains(string(envelope.Data), "handler-secret") || strings.Contains(envelope.ErrorMsg, "handler-secret") {
		t.Errorf("POST remote source/check response = %+v, want no URL secret", envelope)
	}

	busyProject := mustCreatePagesSourceProject(t, ctx, "handler-busy")
	busySource, _ := mustConfigureRemoteSource(
		t,
		ctx,
		busyProject.ID,
		"https://example.com/site.zip",
		false,
	)
	future := time.Now().Add(time.Minute)
	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", busySource.ID).
		Updates(map[string]any{
			"sync_status":      pagesSourceStatusSyncing,
			"lease_token":      "busy-owner",
			"lease_expires_at": &future,
		}).Error; err != nil {
		t.Fatalf("seed busy source runtime error = %v, want nil", err)
	}
	code, envelope = performPagesSourceRequest(
		t,
		router,
		http.MethodPost,
		fmt.Sprintf("/api/v1/d/pages/%d/source/sync", busyProject.ID),
		[]byte(`{}`),
	)
	if got, want := code, http.StatusConflict; got != want {
		t.Errorf("POST busy source/sync status = %d, want %d", got, want)
	}
	if got, want := envelope.ErrorMsg, errPagesSourceActionBusy; got != want {
		t.Errorf("POST busy source/sync error = %q, want %q", got, want)
	}
}

func TestSyncSourceHandlerAcceptsEmptyBodyAndEmptyObject(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	setupPagesSourceDispatchTest(t)
	router := newPagesSourceTestRouter(77)
	project := mustCreatePagesSourceProject(t, ctx, "handler-empty-sync")
	source, _ := mustConfigureRemoteSource(
		t,
		ctx,
		project.ID,
		"https://example.com/site.zip?token=dispatch-secret",
		false,
	)
	path := fmt.Sprintf("/api/v1/d/pages/%d/source/sync", project.ID)

	for _, test := range []struct {
		name string
		body []byte
	}{
		{name: "empty body", body: nil},
		{name: "empty object", body: []byte(`{}`)},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, envelope := performPagesSourceRequest(t, router, http.MethodPost, path, test.body)
			if got, want := code, http.StatusOK; got != want {
				t.Fatalf("POST source/sync (%s) status = %d, want %d; error=%q", test.name, got, want, envelope.ErrorMsg)
			}
			if envelope.ErrorMsg != "" {
				t.Errorf("POST source/sync (%s) error = %q, want empty", test.name, envelope.ErrorMsg)
			}
			var receipt SourceActionReceipt
			if err := json.Unmarshal(envelope.Data, &receipt); err != nil {
				t.Fatalf("json.Unmarshal(source/sync %s receipt) error = %v, want nil", test.name, err)
			}
			if receipt.TaskID == "" || receipt.ExecutionID == "" || receipt.Action != sourceActionSync {
				t.Errorf("POST source/sync (%s) receipt = %+v, want task/execution IDs and action %q", test.name, receipt, sourceActionSync)
			}
		})
	}

	var executions []model.TaskExecution
	if err := db.DB(ctx).Where("task_type = ?", PagesSourceActionTask).Order("id asc").Find(&executions).Error; err != nil {
		t.Fatalf("list Pages source task executions error = %v, want nil", err)
	}
	if got, want := len(executions), 2; got != want {
		t.Fatalf("Pages source task execution count = %d, want %d", got, want)
	}
	for _, execution := range executions {
		if strings.Contains(execution.Payload, "dispatch-secret") || strings.Contains(execution.Payload, "http") {
			t.Errorf("task execution %q payload = %s, want no Remote URL secret", execution.TaskID, execution.Payload)
		}
		var payload SourceActionPayload
		if err := json.Unmarshal([]byte(execution.Payload), &payload); err != nil {
			t.Errorf("json.Unmarshal(task execution %q payload) error = %v, want nil", execution.TaskID, err)
			continue
		}
		if payload.SourceID != source.ID || payload.ConfigVersion != source.ConfigVersion || payload.Actor != "user:77" {
			t.Errorf("task execution %q payload = %+v, want source=%d config=%d actor=user:77", execution.TaskID, payload, source.ID, source.ConfigVersion)
		}
	}
}
