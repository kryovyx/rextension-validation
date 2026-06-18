// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

// Package validation provides a Rex extension for request/response body
// validation, content negotiation, and pluggable codec support.
//
// This file defines the middleware interfaces, route index, and context keys.
package validation

import (
	"strings"
	"sync"

	rxevent "github.com/kryovyx/rextension/event"
)

// Context keys for validation data in request context.
type contextKey string

const (
	// ContextKeyRequestBody stores the decoded+validated request body.
	ContextKeyRequestBody contextKey = "validation:request_body"
	// ContextKeyAcceptCodec stores the negotiated response codec.
	ContextKeyAcceptCodec contextKey = "validation:accept_codec"
)

// routeIndex stores the ValidatableRoute information for registered routes.
// It supports both exact path matching and base-path-aware lookups.
type routeIndex struct {
	mu           sync.RWMutex
	routes       map[string]ValidatableRoute // key: "METHOD /full/path"
	routerNames  map[string][]string         // key: router name, value: registered path prefixes for that router
	pathToRouter map[string]string           // key: route path key, value: router name (optional)
}

func newRouteIndex() *routeIndex {
	return &routeIndex{
		routes:       make(map[string]ValidatableRoute),
		routerNames:  make(map[string][]string),
		pathToRouter: make(map[string]string),
	}
}

func (ri *routeIndex) register(rt rxevent.Route, routerName string, routerBaseURL string) {
	if vr, ok := rt.(ValidatableRoute); ok {
		key := rt.Method() + " " + rt.Path()
		ri.mu.Lock()
		ri.routes[key] = vr
		ri.pathToRouter[key] = routerName

		// Store the full path with base URL for lookups
		if routerBaseURL != "" {
			fullPath := rt.Method() + " " + normalizeBaseURLForIndex(routerBaseURL) + rt.Path()
			ri.routes[fullPath] = vr
			ri.pathToRouter[fullPath] = routerName
		}

		ri.mu.Unlock()
	}
}

func (ri *routeIndex) lookup(method, path string) (ValidatableRoute, bool) {
	key := method + " " + path

	// Try exact match first
	ri.mu.RLock()
	vr, ok := ri.routes[key]
	ri.mu.RUnlock()
	if ok {
		return vr, true
	}

	// If not found, try to strip common base paths and retry
	// Try stripping the first path segment (common for base paths)
	pathToTry := stripSegment(path, 1)
	if pathToTry != path {
		key = method + " " + pathToTry
		ri.mu.RLock()
		vr, ok := ri.routes[key]
		ri.mu.RUnlock()
		if ok {
			return vr, true
		}
	}

	// Try stripping the first two segments (for base paths like /api/v1)
	pathToTry = stripSegment(path, 2)
	if pathToTry != path {
		key = method + " " + pathToTry
		ri.mu.RLock()
		vr, ok := ri.routes[key]
		ri.mu.RUnlock()
		if ok {
			return vr, true
		}
	}

	return nil, false
}

func normalizeBaseURLForIndex(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" || baseURL == "/" {
		return ""
	}
	if !strings.HasPrefix(baseURL, "/") {
		baseURL = "/" + baseURL
	}
	return strings.TrimRight(baseURL, "/")
}

// stripSegment strips the first N path segments from a path.
// For example: stripSegment("/a/b/c", 1) -> "/b/c", stripSegment("/a/b/c", 2) -> "/c"
func stripSegment(path string, numSegments int) string {
	if numSegments <= 0 {
		return path
	}

	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")

	// If path has fewer segments than requested to strip, return unchanged
	if len(parts) <= numSegments || (len(parts) == 1 && parts[0] == "") {
		return path
	}

	// Strip the requested number of segments
	remaining := parts[numSegments:]
	if len(remaining) == 0 || (len(remaining) == 1 && remaining[0] == "") {
		return "/"
	}

	return "/" + strings.Join(remaining, "/")
}

// MiddlewareConfig holds runtime dependencies for the validation middleware.
type MiddlewareConfig struct {
	RouteIndex   *routeIndex
	Registry     *codecRegistry
	Validator    routeValidator
	Strict       bool
	ValidateResp bool
}
