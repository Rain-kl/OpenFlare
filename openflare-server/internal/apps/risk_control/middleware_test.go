// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package risk_control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

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
		logChan = make(chan *UserAccessLog, defaultQueueSize)
		defer func() {
			config.Config.ClickHouse.Enabled = false
			logChan = nil
		}()

		r := gin.New()
		r.Use(func(c *gin.Context) {
			// Mock authentication middleware placing user in context
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
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "ok", w.Body.String())

		// Verify log is enqueued
		select {
		case logItem := <-logChan:
			assert.Equal(t, uint64(12345), logItem.UserID)
			assert.Equal(t, "/test", logItem.Path)
			assert.Equal(t, http.MethodGet, logItem.Method)
			assert.Equal(t, int32(http.StatusOK), logItem.Status)
			assert.NotEmpty(t, logItem.Headers)
			assert.Contains(t, logItem.Headers, "X-Test-Header")
			assert.NotContains(t, logItem.Headers, "Cookie")
		case <-time.After(100 * time.Millisecond):
			t.Fatal("expected log item in logChan, but got none")
		}
	})

	t.Run("ClickHouse enabled - Unauthenticated Request", func(t *testing.T) {
		config.Config.ClickHouse.Enabled = true
		logChan = make(chan *UserAccessLog, defaultQueueSize)
		defer func() {
			config.Config.ClickHouse.Enabled = false
			logChan = nil
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

		// Verify no log is enqueued
		select {
		case <-logChan:
			t.Fatal("expected no log item for unauthenticated request")
		case <-time.After(50 * time.Millisecond):
			// Success
		}
	})

	t.Run("ClickHouse enabled - Buffer Full Rate Limiting", func(t *testing.T) {
		config.Config.ClickHouse.Enabled = true
		logChan = make(chan *UserAccessLog, 2) // small capacity for quick fill
		defer func() {
			config.Config.ClickHouse.Enabled = false
			logChan = nil
		}()

		// fill logChan up to cap to simulate buffer full
		for len(logChan) < cap(logChan) {
			logChan <- &UserAccessLog{}
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
