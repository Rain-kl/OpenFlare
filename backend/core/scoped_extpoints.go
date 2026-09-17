// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"Wavelet/core/extpoints"
)

// scopedRouterExtension wraps a RouterExtension to automatically register
// teardown disposers on the associated Context when routes are declared.
type scopedRouterExtension struct {
	underlying extpoints.RouterExtension
	ctx        *Context
}

func newScopedRouterExtension(ctx *Context, underlying extpoints.RouterExtension) extpoints.RouterExtension {
	return &scopedRouterExtension{
		underlying: underlying,
		ctx:        ctx,
	}
}

func (s *scopedRouterExtension) Use(middlewares ...any) {
	if len(middlewares) == 0 {
		return
	}
	if reg, ok := s.underlying.(*extpoints.RouterRegistry); ok {
		ids := reg.UseWithID(middlewares...)
		s.ctx.OnDispose(func() error {
			for _, id := range ids {
				reg.UnregisterMiddlewareByID(id)
			}
			return nil
		})
		return
	}
	s.underlying.Use(middlewares...)
}

func (s *scopedRouterExtension) Group(prefix string, middlewares ...any) extpoints.RouterExtension {
	subGroup := s.underlying.Group(prefix, middlewares...)
	return newScopedRouterExtension(s.ctx, subGroup)
}

func (s *scopedRouterExtension) Handle(method, path string, handlers ...any) extpoints.RouteDefinition {
	rd := s.underlying.Handle(method, path, handlers...)
	routeID := rd.ID
	s.ctx.OnDispose(func() error {
		s.underlying.UnregisterByID(routeID)
		return nil
	})
	return rd
}

// HandleRaw registers a trailing-slash-preserving route and tears it down with the scope.
func (s *scopedRouterExtension) HandleRaw(method, path string, handlers ...any) extpoints.RouteDefinition {
	rd := s.underlying.HandleRaw(method, path, handlers...)
	routeID := rd.ID
	s.ctx.OnDispose(func() error {
		s.underlying.UnregisterByID(routeID)
		return nil
	})
	return rd
}

// BasePath delegates to the wrapped group prefix.
func (s *scopedRouterExtension) BasePath() string { return s.underlying.BasePath() }

func (s *scopedRouterExtension) GET(path string, handlers ...any) extpoints.RouteDefinition {
	return s.Handle("GET", path, handlers...)
}

func (s *scopedRouterExtension) POST(path string, handlers ...any) extpoints.RouteDefinition {
	return s.Handle("POST", path, handlers...)
}

func (s *scopedRouterExtension) PUT(path string, handlers ...any) extpoints.RouteDefinition {
	return s.Handle("PUT", path, handlers...)
}

func (s *scopedRouterExtension) DELETE(path string, handlers ...any) extpoints.RouteDefinition {
	return s.Handle("DELETE", path, handlers...)
}

func (s *scopedRouterExtension) PATCH(path string, handlers ...any) extpoints.RouteDefinition {
	return s.Handle("PATCH", path, handlers...)
}

func (s *scopedRouterExtension) HEAD(path string, handlers ...any) extpoints.RouteDefinition {
	return s.Handle("HEAD", path, handlers...)
}

func (s *scopedRouterExtension) OPTIONS(path string, handlers ...any) extpoints.RouteDefinition {
	return s.Handle("OPTIONS", path, handlers...)
}

func (s *scopedRouterExtension) Any(path string, handlers ...any) []extpoints.RouteDefinition {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	defs := make([]extpoints.RouteDefinition, 0, len(methods))
	for _, m := range methods {
		defs = append(defs, s.Handle(m, path, handlers...))
	}
	return defs
}

func (s *scopedRouterExtension) Routes() []extpoints.RouteDefinition {
	return s.underlying.Routes()
}

func (s *scopedRouterExtension) Middlewares() []any {
	return s.underlying.Middlewares()
}

func (s *scopedRouterExtension) Unregister(method, path string) bool {
	return s.underlying.Unregister(method, path)
}

func (s *scopedRouterExtension) UnregisterByID(id uint64) bool {
	return s.underlying.UnregisterByID(id)
}

func (s *scopedRouterExtension) UnregisterMiddlewareByID(id uint64) bool {
	return s.underlying.UnregisterMiddlewareByID(id)
}

func (s *scopedRouterExtension) RegisterWhitelist(patterns ...string) {
	s.underlying.RegisterWhitelist(patterns...)
	if len(patterns) > 0 {
		s.ctx.OnDispose(func() error {
			s.underlying.UnregisterWhitelist(patterns...)
			return nil
		})
	}
}

func (s *scopedRouterExtension) UnregisterWhitelist(patterns ...string) {
	s.underlying.UnregisterWhitelist(patterns...)
}

func (s *scopedRouterExtension) Whitelist() []string {
	return s.underlying.Whitelist()
}

func (s *scopedRouterExtension) IsWhitelisted(path string) bool {
	return s.underlying.IsWhitelisted(path)
}

// scopedTaskExtension wraps a TaskExtension to automatically register
// teardown disposers on the associated Context when task handlers are declared.
type scopedTaskExtension struct {
	underlying extpoints.TaskExtension
	ctx        *Context
}

func newScopedTaskExtension(ctx *Context, underlying extpoints.TaskExtension) extpoints.TaskExtension {
	return &scopedTaskExtension{
		underlying: underlying,
		ctx:        ctx,
	}
}

func (s *scopedTaskExtension) Register(pattern string, handler any, opts ...extpoints.TaskOption) {
	s.underlying.Register(pattern, handler, opts...)
	s.ctx.OnDispose(func() error {
		s.underlying.Unregister(pattern)
		return nil
	})
}

func (s *scopedTaskExtension) Tasks() []extpoints.TaskDefinition {
	return s.underlying.Tasks()
}

func (s *scopedTaskExtension) Get(pattern string) (extpoints.TaskDefinition, bool) {
	return s.underlying.Get(pattern)
}

func (s *scopedTaskExtension) Unregister(pattern string) bool {
	return s.underlying.Unregister(pattern)
}

// scopedScheduleExtension wraps a ScheduleExtension to automatically register
// teardown disposers on the associated Context when cron/scheduled tasks are declared.
type scopedScheduleExtension struct {
	underlying extpoints.ScheduleExtension
	ctx        *Context
}

func newScopedScheduleExtension(ctx *Context, underlying extpoints.ScheduleExtension) extpoints.ScheduleExtension {
	return &scopedScheduleExtension{
		underlying: underlying,
		ctx:        ctx,
	}
}

func (s *scopedScheduleExtension) Register(spec, taskType string, payload any, opts ...extpoints.ScheduleOption) {
	s.underlying.Register(spec, taskType, payload, opts...)
	s.ctx.OnDispose(func() error {
		s.underlying.Unregister(taskType)
		return nil
	})
}

func (s *scopedScheduleExtension) RegisterCron(spec, taskType string, payload any, opts ...extpoints.ScheduleOption) {
	s.Register(spec, taskType, payload, opts...)
}

func (s *scopedScheduleExtension) Schedules() []extpoints.ScheduleDefinition {
	return s.underlying.Schedules()
}

func (s *scopedScheduleExtension) Get(taskType string) (extpoints.ScheduleDefinition, bool) {
	return s.underlying.Get(taskType)
}

func (s *scopedScheduleExtension) Unregister(taskType string) bool {
	return s.underlying.Unregister(taskType)
}

// scopedSettingExtension wraps a SettingExtension to automatically register
// teardown disposers on the associated Context when settings schemas are declared.
type scopedSettingExtension struct {
	underlying extpoints.SettingExtension
	ctx        *Context
}

func newScopedSettingExtension(ctx *Context, underlying extpoints.SettingExtension) extpoints.SettingExtension {
	return &scopedSettingExtension{
		underlying: underlying,
		ctx:        ctx,
	}
}

func (s *scopedSettingExtension) Register(schema extpoints.SettingSchema) {
	s.underlying.Register(schema)
	key := schema.Key
	s.ctx.OnDispose(func() error {
		s.underlying.Unregister(key)
		return nil
	})
}

func (s *scopedSettingExtension) Schemas() []extpoints.SettingSchema {
	return s.underlying.Schemas()
}

func (s *scopedSettingExtension) Get(key string) (extpoints.SettingSchema, bool) {
	return s.underlying.Get(key)
}

func (s *scopedSettingExtension) Unregister(key string) bool {
	return s.underlying.Unregister(key)
}
