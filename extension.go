// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

// Package validation provides a Rex extension for request/response body
// validation, content negotiation, and pluggable codec support.
//
// The extension provides:
//   - Automatic request body decoding using a pluggable Codec interface
//   - Struct validation via go-playground/validator/v10
//   - Content-Type checking (415 Unsupported Media Type)
//   - Accept header negotiation (406 Not Acceptable)
//   - Response body validation with optional strict mode (500)
//   - Support for OneOf/AnyOf/AllOf union schemas
//   - Per-status-code response schema mapping
package validation

import (
	"context"

	rx "github.com/kryovyx/rextension"
	rxevent "github.com/kryovyx/rextension/event"
)

// ValidationExtension implements the Rex extension contract for body validation
// and content negotiation.
type ValidationExtension struct {
	cfg       Config
	logger    rx.Logger
	index     *routeIndex
	registry  *codecRegistry
	validator routeValidator
	rex       rx.Rex // Store reference to Rex for router access
}

// NewValidationExtension constructs a validation extension instance.
func NewValidationExtension(cfg *Config) rx.Extension {
	c := NewDefaultConfig()
	if cfg != nil {
		if len(cfg.Codecs) > 0 {
			c.Codecs = cfg.Codecs
		}
		c.StrictResponses = cfg.StrictResponses
		c.ValidateResponses = cfg.ValidateResponses
	}
	return &ValidationExtension{cfg: *c}
}

// WithValidation is a helper Option to attach the validation extension to Rex.
func WithValidation(cfg *Config) rx.Option {
	return rx.WithExtension(NewValidationExtension(cfg))
}

// OnInitialize sets up the validation infrastructure and event subscriptions.
func (e *ValidationExtension) OnInitialize(ctx context.Context, r rx.Rex) error {
	e.logger = r.Logger()
	e.rex = r // Store reference to Rex
	e.index = newRouteIndex()
	e.registry = newCodecRegistry(e.cfg.Codecs)
	e.validator = newRouteValidator()

	// Subscribe to route registration events to build the route index.
	r.EventBus().Subscribe(rxevent.EventTypeRouterRouteRegistered, func(ev rxevent.Event) {
		if routeEv, ok := rxevent.As[rxevent.RouterRouteRegisteredEvent](ev); ok {
			routerName := e.getRouterName(routeEv)
			routerBaseURL := e.getRouterBaseURL(routeEv)

			e.index.register(routeEv.Route, routerName, routerBaseURL)
			if _, isValidatable := routeEv.Route.(ValidatableRoute); isValidatable {
				e.logger.Info("Registered validation schema for route %s %s",
					routeEv.Route.Method(), routeEv.Route.Path())
			}
		}
	})

	// Register the validation middleware.
	mwCfg := MiddlewareConfig{
		RouteIndex:   e.index,
		Registry:     e.registry,
		Validator:    e.validator,
		Strict:       e.cfg.StrictResponses,
		ValidateResp: e.cfg.ValidateResponses,
	}
	r.Use(ValidationMiddleware(mwCfg))

	// Expose the codec registry via DI so other extensions (e.g., OpenAPI) can access it.
	r.Container().Instance(e.registry)

	e.logger.Info("Validation extension initialized with %d codec(s), strict=%v",
		len(e.cfg.Codecs), e.cfg.StrictResponses)

	return nil
}

// getRouterName extracts the router name from a RouterRouteRegisteredEvent.
func (e *ValidationExtension) getRouterName(routeEv rxevent.RouterRouteRegisteredEvent) string {
	if routerName := routeEv.RouterName; routerName != "" {
		return routerName
	}
	return "default"
}

// getRouterBaseURL attempts to extract the router's base URL.
// For now, this returns empty string as the event doesn't carry this information.
// The middleware's stripSegment function handles the base path matching.
func (e *ValidationExtension) getRouterBaseURL(routeEv rxevent.RouterRouteRegisteredEvent) string {
	return ""
}

// OnStart is a no-op for the validation extension.
func (e *ValidationExtension) OnStart(ctx context.Context, r rx.Rex) error { return nil }

// OnReady is a no-op for the validation extension.
func (e *ValidationExtension) OnReady(ctx context.Context, r rx.Rex) error { return nil }

// OnStop is a no-op for the validation extension.
func (e *ValidationExtension) OnStop(ctx context.Context, r rx.Rex) error { return nil }

// OnShutdown is a no-op for the validation extension.
func (e *ValidationExtension) OnShutdown(ctx context.Context, r rx.Rex) error { return nil }
