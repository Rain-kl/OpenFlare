// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints

import (
	"strings"
	"sync"
)

// RouteDefinition holds the metadata and handler list for a single HTTP route.
type RouteDefinition struct {
	ID          uint64
	Method      string
	Path        string
	Handlers    []any
	Middlewares []any
}

// RouterExtension defines the interface for registering routes and middlewares.
type RouterExtension interface {
	Use(middlewares ...any)
	Group(prefix string, middlewares ...any) RouterExtension
	Handle(method, path string, handlers ...any) RouteDefinition
	// HandleRaw joins path with the group prefix but preserves a trailing slash,
	// so `/resource` and `/resource/` can coexist as distinct routes.
	HandleRaw(method, path string, handlers ...any) RouteDefinition
	// BasePath reports this group's absolute prefix ("" for the root registry).
	BasePath() string
	GET(path string, handlers ...any) RouteDefinition
	POST(path string, handlers ...any) RouteDefinition
	PUT(path string, handlers ...any) RouteDefinition
	DELETE(path string, handlers ...any) RouteDefinition
	PATCH(path string, handlers ...any) RouteDefinition
	HEAD(path string, handlers ...any) RouteDefinition
	OPTIONS(path string, handlers ...any) RouteDefinition
	Any(path string, handlers ...any) []RouteDefinition
	Routes() []RouteDefinition
	Middlewares() []any
	Unregister(method, path string) bool
	UnregisterByID(id uint64) bool
	UnregisterMiddlewareByID(id uint64) bool
	RegisterWhitelist(patterns ...string)
	UnregisterWhitelist(patterns ...string)
	Whitelist() []string
	IsWhitelisted(path string) bool
}

// middlewareDefinition holds an assigned ID and handler for registered middleware.
type middlewareDefinition struct {
	ID      uint64
	Handler any
}

// RouterRegistry implements RouterExtension as the root route and middleware collector.
type RouterRegistry struct {
	mu          sync.RWMutex
	nextID      uint64
	nextMWID    uint64
	routes      []RouteDefinition
	middlewares []middlewareDefinition
	whitelist   PathWhitelist
}

// NewRouterRegistry creates a new root router collector.
func NewRouterRegistry() *RouterRegistry {
	return &RouterRegistry{}
}

// Use registers global middlewares to the router.
func (r *RouterRegistry) Use(middlewares ...any) {
	r.UseWithID(middlewares...)
}

// UseWithID registers global middlewares to the router and returns their assigned IDs.
func (r *RouterRegistry) UseWithID(middlewares ...any) []uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	ids := make([]uint64, 0, len(middlewares))
	for _, mw := range middlewares {
		r.nextMWID++
		r.middlewares = append(r.middlewares, middlewareDefinition{
			ID:      r.nextMWID,
			Handler: mw,
		})
		ids = append(ids, r.nextMWID)
	}
	return ids
}

// UnregisterMiddlewareByID removes a registered global middleware by its unique ID.
func (r *RouterRegistry) UnregisterMiddlewareByID(id uint64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, mw := range r.middlewares {
		if mw.ID == id {
			r.middlewares = append(r.middlewares[:i], r.middlewares[i+1:]...)
			return true
		}
	}
	return false
}

// Middlewares returns a copy of registered root middleware handlers.
func (r *RouterRegistry) Middlewares() []any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]any, len(r.middlewares))
	for i, mw := range r.middlewares {
		res[i] = mw.Handler
	}
	return res
}

// Group creates a new RouteGroup under the router.
func (r *RouterRegistry) Group(prefix string, middlewares ...any) RouterExtension {
	return &RouterGroup{
		registry:    r,
		prefix:      cleanPath(prefix),
		middlewares: middlewares,
	}
}

// Handle registers a route with a custom HTTP method and handlers.
func (r *RouterRegistry) Handle(method, path string, handlers ...any) RouteDefinition {
	return r.addRoute(method, cleanPath(path), handlers...)
}

// addRoute appends a route whose path is already normalised.
func (r *RouterRegistry) addRoute(method, fullPath string, handlers ...any) RouteDefinition {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	rd := RouteDefinition{
		ID:       r.nextID,
		Method:   strings.ToUpper(method),
		Path:     fullPath,
		Handlers: handlers,
		// Global Router.Use middlewares are applied at HTTP Start from
		// Router.Middlewares(), so late-registered plugins still wrap earlier routes.
	}
	r.routes = append(r.routes, rd)
	return rd
}

// Unregister removes a route matching method and path from the registry.
func (r *RouterRegistry) Unregister(method, path string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	targetMethod := strings.ToUpper(method)
	targetPath := cleanPath(path)

	for i, rd := range r.routes {
		if rd.Method == targetMethod && rd.Path == targetPath {
			r.routes = append(r.routes[:i], r.routes[i+1:]...)
			return true
		}
	}
	return false
}

// UnregisterByID removes a route by its unique ID.
func (r *RouterRegistry) UnregisterByID(id uint64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, rd := range r.routes {
		if rd.ID == id {
			r.routes = append(r.routes[:i], r.routes[i+1:]...)
			return true
		}
	}
	return false
}

// GET registers a GET route.
func (r *RouterRegistry) GET(path string, handlers ...any) RouteDefinition {
	return r.Handle("GET", path, handlers...)
}

// POST registers a POST route.
func (r *RouterRegistry) POST(path string, handlers ...any) RouteDefinition {
	return r.Handle("POST", path, handlers...)
}

// PUT registers a PUT route.
func (r *RouterRegistry) PUT(path string, handlers ...any) RouteDefinition {
	return r.Handle("PUT", path, handlers...)
}

// DELETE registers a DELETE route.
func (r *RouterRegistry) DELETE(path string, handlers ...any) RouteDefinition {
	return r.Handle("DELETE", path, handlers...)
}

// PATCH registers a PATCH route.
func (r *RouterRegistry) PATCH(path string, handlers ...any) RouteDefinition {
	return r.Handle("PATCH", path, handlers...)
}

// HEAD registers a HEAD route.
func (r *RouterRegistry) HEAD(path string, handlers ...any) RouteDefinition {
	return r.Handle("HEAD", path, handlers...)
}

// OPTIONS registers an OPTIONS route.
func (r *RouterRegistry) OPTIONS(path string, handlers ...any) RouteDefinition {
	return r.Handle("OPTIONS", path, handlers...)
}

// Any registers a route for standard HTTP methods.
func (r *RouterRegistry) Any(path string, handlers ...any) []RouteDefinition {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	defs := make([]RouteDefinition, 0, len(methods))
	for _, m := range methods {
		defs = append(defs, r.Handle(m, path, handlers...))
	}
	return defs
}

// Routes returns a copy of all collected RouteDefinitions.
func (r *RouterRegistry) Routes() []RouteDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]RouteDefinition, len(r.routes))
	copy(res, r.routes)
	return res
}

// RegisterWhitelist adds path patterns to the whitelist.
func (r *RouterRegistry) RegisterWhitelist(patterns ...string) {
	r.whitelist.Add(patterns...)
}

// UnregisterWhitelist removes path patterns from the whitelist.
func (r *RouterRegistry) UnregisterWhitelist(patterns ...string) {
	r.whitelist.Remove(patterns...)
}

// Whitelist returns a copy of all registered whitelist path patterns.
func (r *RouterRegistry) Whitelist() []string {
	return r.whitelist.Patterns()
}

// IsWhitelisted checks if the given path matches any registered whitelist pattern.
func (r *RouterRegistry) IsWhitelisted(path string) bool {
	return r.whitelist.Match(path)
}

// RouterGroup represents a scoped route group with a path prefix and group-level middlewares.
type RouterGroup struct {
	registry    *RouterRegistry
	prefix      string
	middlewares []any
}

// Use adds middlewares to this group.
func (g *RouterGroup) Use(middlewares ...any) {
	g.middlewares = append(g.middlewares, middlewares...)
}

// Group creates a nested RouteGroup.
func (g *RouterGroup) Group(prefix string, middlewares ...any) RouterExtension {
	combinedPrefix := joinPaths(g.prefix, prefix)
	combinedMiddlewares := make([]any, 0, len(g.middlewares)+len(middlewares))
	combinedMiddlewares = append(combinedMiddlewares, g.middlewares...)
	combinedMiddlewares = append(combinedMiddlewares, middlewares...)

	return &RouterGroup{
		registry:    g.registry,
		prefix:      combinedPrefix,
		middlewares: combinedMiddlewares,
	}
}

// Handle registers a route under this group.
func (g *RouterGroup) Handle(method, path string, handlers ...any) RouteDefinition {
	return g.addRoute(method, joinPaths(g.prefix, path), handlers...)
}

// addRoute appends a route under this group whose path is already joined.
func (g *RouterGroup) addRoute(method, fullPath string, handlers ...any) RouteDefinition {
	g.registry.mu.Lock()
	defer g.registry.mu.Unlock()

	allMiddlewares := append([]any(nil), g.middlewares...)

	g.registry.nextID++
	rd := RouteDefinition{
		ID:          g.registry.nextID,
		Method:      strings.ToUpper(method),
		Path:        fullPath,
		Handlers:    handlers,
		Middlewares: allMiddlewares,
	}
	g.registry.routes = append(g.registry.routes, rd)
	return rd
}

// Unregister removes a route under this group prefix matching method and path.
func (g *RouterGroup) Unregister(method, path string) bool {
	fullPath := joinPaths(g.prefix, path)
	return g.registry.Unregister(method, fullPath)
}

// UnregisterByID removes a route by its unique ID.
func (g *RouterGroup) UnregisterByID(id uint64) bool {
	return g.registry.UnregisterByID(id)
}

// UnregisterMiddlewareByID removes a middleware by ID via the root registry.
func (g *RouterGroup) UnregisterMiddlewareByID(id uint64) bool {
	return g.registry.UnregisterMiddlewareByID(id)
}

// GET registers a GET route in this group.
func (g *RouterGroup) GET(path string, handlers ...any) RouteDefinition {
	return g.Handle("GET", path, handlers...)
}

// POST registers a POST route in this group.
func (g *RouterGroup) POST(path string, handlers ...any) RouteDefinition {
	return g.Handle("POST", path, handlers...)
}

// PUT registers a PUT route in this group.
func (g *RouterGroup) PUT(path string, handlers ...any) RouteDefinition {
	return g.Handle("PUT", path, handlers...)
}

// DELETE registers a DELETE route in this group.
func (g *RouterGroup) DELETE(path string, handlers ...any) RouteDefinition {
	return g.Handle("DELETE", path, handlers...)
}

// PATCH registers a PATCH route in this group.
func (g *RouterGroup) PATCH(path string, handlers ...any) RouteDefinition {
	return g.Handle("PATCH", path, handlers...)
}

// HEAD registers a HEAD route in this group.
func (g *RouterGroup) HEAD(path string, handlers ...any) RouteDefinition {
	return g.Handle("HEAD", path, handlers...)
}

// OPTIONS registers an OPTIONS route in this group.
func (g *RouterGroup) OPTIONS(path string, handlers ...any) RouteDefinition {
	return g.Handle("OPTIONS", path, handlers...)
}

// Any registers a route in this group for standard HTTP methods.
func (g *RouterGroup) Any(path string, handlers ...any) []RouteDefinition {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	defs := make([]RouteDefinition, 0, len(methods))
	for _, m := range methods {
		defs = append(defs, g.Handle(m, path, handlers...))
	}
	return defs
}

// Routes returns all routes from the parent registry.
func (g *RouterGroup) Routes() []RouteDefinition {
	return g.registry.Routes()
}

// Middlewares returns a copy of the group's middlewares.
func (g *RouterGroup) Middlewares() []any {
	res := make([]any, len(g.middlewares))
	copy(res, g.middlewares)
	return res
}

// RegisterWhitelist adds path patterns under this group prefix to the whitelist.
func (g *RouterGroup) RegisterWhitelist(patterns ...string) {
	for _, p := range patterns {
		g.registry.RegisterWhitelist(joinPaths(g.prefix, p))
	}
}

// UnregisterWhitelist removes path patterns under this group prefix from the whitelist.
func (g *RouterGroup) UnregisterWhitelist(patterns ...string) {
	for _, p := range patterns {
		g.registry.UnregisterWhitelist(joinPaths(g.prefix, p))
	}
}

// Whitelist returns a copy of all registered whitelist path patterns.
func (g *RouterGroup) Whitelist() []string {
	return g.registry.Whitelist()
}

// IsWhitelisted checks if the given path matches any registered whitelist pattern.
func (g *RouterGroup) IsWhitelisted(path string) bool {
	return g.registry.IsWhitelisted(path)
}

func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}

func joinPaths(base, relative string) string {
	if base == "" || base == "/" {
		return cleanPath(relative)
	}
	if relative == "" || relative == "/" {
		return cleanPath(base)
	}
	base = strings.TrimSuffix(base, "/")
	relative = strings.TrimPrefix(relative, "/")
	return cleanPath(base + "/" + relative)
}

// MatchPathPattern checks if a URL path matches a pattern (supports exact match and wildcards).
func MatchPathPattern(pattern, path string) bool {
	pattern = cleanPath(pattern)
	path = cleanPath(path)

	if pattern == path {
		return true
	}

	// Suffix wildcard: /api/v1/oauth/* matches /api/v1/oauth and /api/v1/oauth/...
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}

	// Parameter wildcard: /api/v1/oauth/*/authorize or /api/v1/oauth/:source/authorize
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")
	if len(patternParts) == len(pathParts) {
		matched := true
		for i, part := range patternParts {
			if part == "*" || strings.HasPrefix(part, ":") {
				continue
			}
			if part != pathParts[i] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}

	return false
}

// compiledPattern holds a whitelist pattern with its per-request work already done.
type compiledPattern struct {
	raw    string   // normalised pattern, reported back by Patterns
	prefix string   // non-empty when the pattern ends in "/*"
	parts  []string // normalised pattern split on "/"
}

// PathWhitelist matches request paths against a fixed set of patterns.
//
// Patterns are registered once during plugin Apply and never change afterwards, so
// normalising and splitting them on every request is wasted work. PathWhitelist
// does that once at registration instead. The zero value is ready to use.
type PathWhitelist struct {
	mu       sync.RWMutex
	patterns []compiledPattern
}

// NewPathWhitelist returns a whitelist pre-populated with the given patterns.
func NewPathWhitelist(patterns ...string) *PathWhitelist {
	w := &PathWhitelist{}
	w.Add(patterns...)
	return w
}

// compilePatterns normalises and splits each pattern once, ahead of any request.
func compilePatterns(patterns []string) []compiledPattern {
	compiled := make([]compiledPattern, 0, len(patterns))
	for _, p := range patterns {
		clean := cleanPath(p)
		cp := compiledPattern{raw: clean, parts: strings.Split(clean, "/")}
		if strings.HasSuffix(clean, "/*") {
			cp.prefix = strings.TrimSuffix(clean, "/*")
		}
		compiled = append(compiled, cp)
	}
	return compiled
}

// Add appends patterns, normalising and splitting each now rather than per request.
func (w *PathWhitelist) Add(patterns ...string) {
	if len(patterns) == 0 {
		return
	}
	compiled := compilePatterns(patterns)

	w.mu.Lock()
	defer w.mu.Unlock()
	w.patterns = append(w.patterns, compiled...)
}

// Replace discards any existing patterns and installs the given ones, for callers
// whose configuration is a full swap rather than an incremental registration.
func (w *PathWhitelist) Replace(patterns ...string) {
	compiled := compilePatterns(patterns)

	w.mu.Lock()
	defer w.mu.Unlock()
	w.patterns = compiled
}

// Remove removes matching patterns from the whitelist.
func (w *PathWhitelist) Remove(patterns ...string) {
	if len(patterns) == 0 {
		return
	}
	targets := make(map[string]struct{}, len(patterns))
	for _, p := range patterns {
		targets[cleanPath(p)] = struct{}{}
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	filtered := w.patterns[:0]
	for _, p := range w.patterns {
		if _, remove := targets[p.raw]; !remove {
			filtered = append(filtered, p)
		}
	}
	w.patterns = filtered
}

// Match reports whether path matches any registered pattern. Equivalent to calling
// MatchPathPattern for every pattern, except the path is normalised and split once.
func (w *PathWhitelist) Match(path string) bool {
	clean := cleanPath(path)
	pathParts := strings.Split(clean, "/")

	w.mu.RLock()
	defer w.mu.RUnlock()
	for i := range w.patterns {
		p := &w.patterns[i]
		if p.raw == clean {
			return true
		}
		// A suffix wildcard matches both the bare prefix and anything below it.
		if p.prefix != "" && (clean == p.prefix || strings.HasPrefix(clean, p.prefix+"/")) {
			return true
		}
		if len(p.parts) != len(pathParts) {
			continue
		}
		if matchSegments(p.parts, pathParts) {
			return true
		}
	}
	return false
}

// matchSegments compares an already-split pattern against an already-split path.
func matchSegments(patternParts, pathParts []string) bool {
	for i, part := range patternParts {
		if part == "*" || strings.HasPrefix(part, ":") {
			continue
		}
		if part != pathParts[i] {
			return false
		}
	}
	return true
}

// Patterns returns a copy of the registered patterns in registration order.
func (w *PathWhitelist) Patterns() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	res := make([]string, len(w.patterns))
	for i := range w.patterns {
		res[i] = w.patterns[i].raw
	}
	return res
}

// ─── Raw path registration ────────────────────────────────────────────────────

// ensureLeadingSlash normalises a path to start with exactly one "/" while
// preserving any trailing slash (unlike cleanPath).
func ensureLeadingSlash(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

// joinPathPreservingTrailing joins a group prefix and a relative path without
// stripping a trailing slash, so a group "/x" can serve both "/x" and "/x/".
func joinPathPreservingTrailing(base, relative string) string {
	rel := ensureLeadingSlash(relative)
	if base == "" || base == "/" {
		return rel
	}
	return strings.TrimSuffix(cleanPath(base), "/") + rel
}

// HandleRaw registers a route on the root registry, preserving a trailing slash.
func (r *RouterRegistry) HandleRaw(method, path string, handlers ...any) RouteDefinition {
	return r.addRoute(method, ensureLeadingSlash(path), handlers...)
}

// BasePath returns "" because the root registry has no prefix.
func (r *RouterRegistry) BasePath() string { return "" }

// HandleRaw registers a route under this group, preserving a trailing slash.
func (g *RouterGroup) HandleRaw(method, path string, handlers ...any) RouteDefinition {
	return g.addRoute(method, joinPathPreservingTrailing(g.prefix, path), handlers...)
}

// BasePath returns this group's absolute prefix.
func (g *RouterGroup) BasePath() string { return g.prefix }
