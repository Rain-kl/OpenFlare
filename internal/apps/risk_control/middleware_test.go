// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package risk_control

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db/batchwriter"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

const testLogQueueSize = 10_000

func newTestLogWriter(t *testing.T, queueSize int) (*batchwriter.Writer[*analytics.UserAccessLog], chan *analytics.UserAccessLog) {
	t.Helper()

	received := make(chan *analytics.UserAccessLog, queueSize)
	cfg := batchwriter.Config{
		QueueSize:     queueSize,
		MaxBatchSize:  1,
		FlushInterval: 5 * time.Millisecond,
	}
	writer, err := batchwriter.New[*analytics.UserAccessLog](cfg, func(_ context.Context, items []*analytics.UserAccessLog) error {
		for _, item := range items {
			if item != nil {
				received <- item
			}
		}
		return nil
	})
	assert.NoError(t, err)
	writer.Start(context.Background())
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = writer.Stop(stopCtx)
	})
	return writer, received
}

func TestRiskControlMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ClickHouse disabled", func(t *testing.T) {
		config.Config.ClickHouse.Enabled = false
		defer func() { config.Config.ClickHouse.Enabled = false }()

		r := testhelper.NewTestGinEngine(RiskControlMiddleware())
		r.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "ok", w.Body.String())
	})

	t.Run("ClickHouse enabled - Normal Authenticated Request", func(t *testing.T) {
		config.Config.ClickHouse.Enabled = true
		writer, received := newTestLogWriter(t, testLogQueueSize)
		resetWriter := SetLogWriterForTest(writer)
		defer func() {
			config.Config.ClickHouse.Enabled = false
			resetWriter()
		}()

		r := gin.New()
		r.Use(func(c *gin.Context) {
			user := &model.User{ID: 12345}
			oauth.SetToContext(c, oauth.UserObjKey, user)
			c.Next()
		})
		r.Use(RiskControlMiddleware())
		r.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Test-Header", "hello")
		req.Header.Set("Cookie", "session_id=abcdef123456")
		req.Header.Set("Authorization", "Bearer secret-token")
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "ok", w.Body.String())

		select {
		case logItem := <-received:
			assert.Equal(t, uint64(12345), logItem.UserID)
			assert.Equal(t, "/test", logItem.Path)
			assert.Equal(t, http.MethodGet, logItem.Method)
			assert.Equal(t, int32(http.StatusOK), logItem.Status)
			assert.NotEmpty(t, logItem.Headers)
			assert.NotContains(t, logItem.Headers, "X-Test-Header")
			assert.Contains(t, logItem.Headers, "Content-Type")
			assert.Contains(t, logItem.Headers, "sha256:")
			assert.NotContains(t, logItem.Headers, "secret-token")
			assert.NotContains(t, logItem.Headers, "session_id=abcdef123456")
		case <-time.After(200 * time.Millisecond):
			t.Fatal("expected flushed log item, but got none")
		}
	})

	t.Run("ClickHouse enabled - Unauthenticated Request", func(t *testing.T) {
		config.Config.ClickHouse.Enabled = true
		writer, received := newTestLogWriter(t, testLogQueueSize)
		resetWriter := SetLogWriterForTest(writer)
		defer func() {
			config.Config.ClickHouse.Enabled = false
			resetWriter()
		}()

		r := testhelper.NewTestGinEngine(RiskControlMiddleware())
		r.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "ok", w.Body.String())

		select {
		case <-received:
			t.Fatal("expected no log item for unauthenticated request")
		case <-time.After(50 * time.Millisecond):
		}
	})

	t.Run("ClickHouse enabled - Buffer Full Rate Limiting", func(t *testing.T) {
		config.Config.ClickHouse.Enabled = true
		writer, _ := newTestLogWriter(t, 2)
		resetWriter := SetLogWriterForTest(writer)
		defer func() {
			config.Config.ClickHouse.Enabled = false
			resetWriter()
		}()

		for range 2 {
			assert.True(t, writer.TryEnqueue(&analytics.UserAccessLog{}))
		}

		r := testhelper.NewTestGinEngine(RiskControlMiddleware())
		r.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Contains(t, resp["error_msg"], "系统繁忙")
	})
}
