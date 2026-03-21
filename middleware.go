// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

// Package validation provides a Rex extension for request/response body
// validation, content negotiation, and pluggable codec support.
//
// This file defines the middleware interfaces, route index, and context keys.
package validation

import (
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

// routeIndex stores the ValidatableRoute information for registered routes,
// keyed by "METHOD /path".
type routeIndex struct {
	mu     sync.RWMutex
	routes map[string]ValidatableRoute
}

func newRouteIndex() *routeIndex {
	return &routeIndex{routes: make(map[string]ValidatableRoute)}
}

func (ri *routeIndex) register(rt rxevent.Route) {
	if vr, ok := rt.(ValidatableRoute); ok {
		key := rt.Method() + " " + rt.Path()
		ri.mu.Lock()
		ri.routes[key] = vr
		ri.mu.Unlock()
	}
}

func (ri *routeIndex) lookup(method, path string) (ValidatableRoute, bool) {
	key := method + " " + path
	ri.mu.RLock()
	vr, ok := ri.routes[key]
	ri.mu.RUnlock()
	return vr, ok
}

// MiddlewareConfig holds runtime dependencies for the validation middleware.
type MiddlewareConfig struct {
	RouteIndex   *routeIndex
	Registry     *codecRegistry
	Validator    routeValidator
	Strict       bool
	ValidateResp bool
}
