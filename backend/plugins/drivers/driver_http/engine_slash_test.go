// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_http

import (
	"Wavelet/core"
	"Wavelet/core/extpoints"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildEngineDefaultRedirectsTrailingSlash(t *testing.T) {
	eng, err := BuildEngineWithConfig(httpAppConfig{}, httpRedisConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !eng.RedirectTrailingSlash {
		t.Fatal("default RedirectTrailingSlash must be true")
	}
}

func TestBuildEngineCanDisableRedirectTrailingSlash(t *testing.T) {
	eng, err := BuildEngineWithConfig(httpAppConfig{RedirectTrailingSlash: boolPtr(false)}, httpRedisConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if eng.RedirectTrailingSlash {
		t.Fatal("RedirectTrailingSlash must honor false")
	}
}

func TestBindAppConfigDefaultKeepsTrailingSlashRedirect(t *testing.T) {
	cfg := bindAppConfig(t, map[string]any{}, map[string]string{})
	if cfg.RedirectTrailingSlash != nil {
		t.Fatal("absent redirect_trailing_slash must leave *bool nil")
	}
	eng, err := BuildEngineWithConfig(cfg, httpRedisConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !eng.RedirectTrailingSlash {
		t.Fatal("default RedirectTrailingSlash must be true after Bind")
	}
}

func TestBindAppConfigCanDisableTrailingSlashRedirect(t *testing.T) {
	cfg := bindAppConfig(t, map[string]any{"app.redirect_trailing_slash": false}, nil)
	if cfg.RedirectTrailingSlash == nil || *cfg.RedirectTrailingSlash {
		t.Fatal("yaml false must bind *false")
	}
	eng, err := BuildEngineWithConfig(cfg, httpRedisConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if eng.RedirectTrailingSlash {
		t.Fatal("RedirectTrailingSlash must honor bound false")
	}
}

func TestBindAppConfigEnvCanDisableTrailingSlashRedirect(t *testing.T) {
	cfg := bindAppConfig(t, nil, map[string]string{"APP_REDIRECT_TRAILING_SLASH": "false"})
	if cfg.RedirectTrailingSlash == nil || *cfg.RedirectTrailingSlash {
		t.Fatal("env false must bind *false")
	}
	eng, err := BuildEngineWithConfig(cfg, httpRedisConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if eng.RedirectTrailingSlash {
		t.Fatal("RedirectTrailingSlash must honor env-bound false")
	}
}

type slashConfigSource struct {
	values map[string]any
	env    map[string]string
}

func (s slashConfigSource) Lookup(path string) (any, bool) {
	v, ok := s.values[path]
	return v, ok
}

func (s slashConfigSource) LookupEnv(name string) (string, bool) {
	v, ok := s.env[name]
	return v, ok
}

func (s slashConfigSource) Describe() string { return "slash-test" }

func bindAppConfig(t *testing.T, values map[string]any, env map[string]string) httpAppConfig {
	t.Helper()
	if values == nil {
		values = map[string]any{}
	}
	if env == nil {
		env = map[string]string{}
	}
	r := extpoints.NewConfigRegistry(slashConfigSource{values: values, env: env})
	if err := r.Declare("driver_http", extpoints.ConfigBinding{Prefix: "app", Target: &httpAppConfig{}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Resolve(); err != nil {
		t.Fatal(err)
	}
	var cfg httpAppConfig
	if err := r.Bind("app", &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func boolPtr(v bool) *bool { return &v }

func TestDriverHTTPSwaggerMount(t *testing.T) {
	ctx := core.NewContext(t.Context())
	ctx.Config().SetSource(core.NewMapSource(map[string]any{"app.env": "development"}))
	if err := ctx.Config().Resolve(); err != nil {
		t.Fatal(err)
	}
	p := New(WithAddr("127.0.0.1:0"))
	if err := p.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	startCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if err := p.Start(startCtx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Stop(t.Context()) }()

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/swagger/index.html", nil)
	req.RequestURI = "/swagger/index.html"
	p.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /swagger/index.html, got %d (body: %s)", w.Code, w.Body.String())
	}
}
