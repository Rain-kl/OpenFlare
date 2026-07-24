// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package response

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAbortWithError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	AbortWithError(c, http.StatusBadRequest, "invalid input")

	require.Len(t, c.Errors, 1)

	var apiErr *APIError
	require.True(t, errors.As(c.Errors.Last().Err, &apiErr))
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	assert.Equal(t, "invalid input", apiErr.Msg)
	assert.True(t, c.IsAborted())
}

func TestErrorHandlerMiddleware_APIErrorStatusCodes(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		message    string
		abort      func(*gin.Context, string)
	}{
		{"400 Bad Request", http.StatusBadRequest, "bad request", AbortBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized, "unauthorized", AbortUnauthorized},
		{"403 Forbidden", http.StatusForbidden, "forbidden", AbortForbidden},
		{"404 Not Found", http.StatusNotFound, "not found", AbortNotFound},
		{"409 Conflict", http.StatusConflict, "conflict", AbortConflict},
		{"429 Too Many Requests", http.StatusTooManyRequests, "too many requests", AbortTooManyRequests},
		{"500 Internal Server Error", http.StatusInternalServerError, "internal error", AbortInternal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Use(ErrorHandlerMiddleware())
			r.GET("/test", func(c *gin.Context) {
				tc.abort(c, tc.message)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.statusCode, w.Code)
			assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

			var body Response[any]
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tc.message, body.ErrorMsg)
			assert.Nil(t, body.Data)
		})
	}
}

func TestErrorHandlerMiddleware_SkipsWhenNoErrors(t *testing.T) {
	r := gin.New()
	r.Use(ErrorHandlerMiddleware())
	r.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, OK("success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body Response[string]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "success", body.Data)
	assert.Empty(t, body.ErrorMsg)
}

func TestErrorHandlerMiddleware_SkipsWhenResponseAlreadyWritten(t *testing.T) {
	r := gin.New()
	r.Use(ErrorHandlerMiddleware())
	r.GET("/written", func(c *gin.Context) {
		c.JSON(http.StatusOK, OKNil())
		_ = c.Error(NewError(http.StatusBadRequest, "should not overwrite"))
	})

	req := httptest.NewRequest(http.MethodGet, "/written", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body Response[any]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Empty(t, body.ErrorMsg)
	assert.Nil(t, body.Data)
}

func TestErrorHandlerMiddleware_FallbackForNonAPIError(t *testing.T) {
	r := gin.New()
	r.Use(ErrorHandlerMiddleware())
	r.GET("/plain", func(c *gin.Context) {
		_ = c.Error(errors.New("plain error"))
	})

	req := httptest.NewRequest(http.MethodGet, "/plain", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var body Response[any]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "内部系统错误", body.ErrorMsg)
	assert.Nil(t, body.Data)
}

func TestErrorHandlerMiddleware_RecordsSpanOnAPIError(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(trace.NewNoopTracerProvider())

	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "request")

	r := gin.New()
	r.Use(ErrorHandlerMiddleware())
	r.GET("/err", func(c *gin.Context) {
		c.Request = c.Request.WithContext(ctx)
		AbortBadRequest(c, "bad request")
	})

	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	span.End()

	require.Equal(t, http.StatusBadRequest, w.Code)

	spans := sr.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, codes.Error, spans[0].Status().Code)
	assert.Equal(t, "bad request", spans[0].Status().Description)
	require.NotEmpty(t, spans[0].Events())
}
