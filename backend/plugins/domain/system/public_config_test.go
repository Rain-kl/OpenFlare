// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Wavelet/core"
	"Wavelet/core/contracts"

	"github.com/gin-gonic/gin"
)

type stubPublic struct{ payload map[string]string }

func (s stubPublic) PublicConfig(context.Context) (map[string]string, error) {
	return s.payload, nil
}

type errPublic struct{ err error }

func (s errPublic) PublicConfig(context.Context) (map[string]string, error) {
	return nil, s.err
}

func TestPublicConfigUsesProviderWhenPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := core.NewContext(context.Background())
	core.Provide[contracts.PublicConfigProvider](ctx, stubPublic{payload: map[string]string{"k": "v"}})
	if err := New().Apply(ctx); err != nil {
		t.Fatal(err)
	}
	body := invokePublicConfig(t, publicConfigHandler(t, ctx))
	assertFlatKV(t, body)
}

func TestPublicConfigUsesProviderRegisteredAfterApply(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := core.NewContext(context.Background())
	if err := New().Apply(ctx); err != nil {
		t.Fatal(err)
	}
	core.Provide[contracts.PublicConfigProvider](ctx, stubPublic{payload: map[string]string{"k": "v"}})
	body := invokePublicConfig(t, publicConfigHandler(t, ctx))
	assertFlatKV(t, body)
}

func TestPublicConfigDefaultWithoutProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := core.NewContext(context.Background())
	if err := New().Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if !ctx.Router().IsWhitelisted("/api/v1/config/public") {
		t.Fatal("GET /api/v1/config/public not whitelisted")
	}
	raw := invokePublicConfig(t, publicConfigHandler(t, ctx))
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("data = %s, want empty flat map", raw)
	}
	if _, ok := data["configs"]; ok {
		t.Fatalf("data = %s, default payload must not wrap configs", raw)
	}
	if _, ok := data["app"]; ok {
		t.Fatalf("data = %s, default payload must not wrap app", raw)
	}
}

func TestPublicConfigProviderErrorAbortsInternal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := core.NewContext(context.Background())
	core.Provide[contracts.PublicConfigProvider](ctx, errPublic{err: errors.New("secret boom")})
	if err := New().Apply(ctx); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/config/public", nil)
	publicConfigHandler(t, ctx)(c)

	if !c.IsAborted() {
		t.Fatal("want request aborted on provider error")
	}
	if len(c.Errors) == 0 {
		t.Fatal("want gin error on provider failure")
	}
	if got := c.Errors.Last().Error(); got != "public config unavailable" {
		t.Fatalf("error = %q, want generic message", got)
	}
	if strings.Contains(w.Body.String(), "secret boom") {
		t.Fatalf("leaked provider error: %s", w.Body.String())
	}
}

func publicConfigHandler(t *testing.T, ctx *core.Context) gin.HandlerFunc {
	t.Helper()
	for _, rd := range ctx.Router().Routes() {
		if rd.Method != "GET" || rd.Path != "/api/v1/config/public" {
			continue
		}
		if len(rd.Handlers) == 0 {
			break
		}
		switch h := rd.Handlers[0].(type) {
		case gin.HandlerFunc:
			return h
		case func(*gin.Context):
			return h
		default:
			t.Fatalf("unexpected handler type %T", rd.Handlers[0])
		}
	}
	t.Fatal("public config route missing")
	return nil
}

func invokePublicConfig(t *testing.T, handler gin.HandlerFunc) json.RawMessage {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/config/public", nil)
	handler(c)
	var body struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Data
}

func assertFlatKV(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data["k"] != "v" {
		t.Fatalf("data = %s, want flat map", raw)
	}
	if _, ok := data["configs"]; ok {
		t.Fatalf("data = %s, want provider payload without default configs", raw)
	}
	if _, ok := data["app"]; ok {
		t.Fatalf("data = %s, want provider payload without default app", raw)
	}
}
