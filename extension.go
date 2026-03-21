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
	e.index = newRouteIndex()
	e.registry = newCodecRegistry(e.cfg.Codecs)
	e.validator = newRouteValidator()

	// Subscribe to route registration events to build the route index.
	r.EventBus().Subscribe(rxevent.EventTypeRouterRouteRegistered, func(ev rxevent.Event) {
			if routeEv, ok := rxevent.As[rxevent.RouterRouteRegisteredEvent](ev); ok {
			e.index.register(routeEv.Route)
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

// OnStart is a no-op for the validation extension.
func (e *ValidationExtension) OnStart(ctx context.Context, r rx.Rex) error { return nil }

// OnReady is a no-op for the validation extension.
func (e *ValidationExtension) OnReady(ctx context.Context, r rx.Rex) error { return nil }

// OnStop is a no-op for the validation extension.
func (e *ValidationExtension) OnStop(ctx context.Context, r rx.Rex) error { return nil }

// OnShutdown is a no-op for the validation extension.
func (e *ValidationExtension) OnShutdown(ctx context.Context, r rx.Rex) error { return nil }
