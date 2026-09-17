// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core

import "context"

type appContextKey struct{}

// WithAppContext attaches the micro-kernel Context to a standard context.Context
// so request and worker handlers can Inject services without package-level setters.
func WithAppContext(ctx context.Context, app *Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if app == nil {
		return ctx
	}
	return context.WithValue(ctx, appContextKey{}, app.Root())
}

// AppContext extracts the micro-kernel Context from ctx, if present.
func AppContext(ctx context.Context) *Context {
	if ctx == nil {
		return nil
	}
	if c, ok := ctx.(*Context); ok {
		return c
	}
	app, _ := ctx.Value(appContextKey{}).(*Context)
	return app
}

// InjectFrom resolves T from ctx when it carries a micro-kernel Context
// (*Context itself, or a value attached by WithAppContext).
func InjectFrom[T any](ctx context.Context) (T, error) {
	var zero T
	app := AppContext(ctx)
	if app == nil {
		return zero, ErrNilContext
	}
	return Inject[T](app)
}
