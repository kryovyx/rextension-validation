// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

// Package validation provides a Rex extension for request/response body
// validation, content negotiation, and pluggable codec support.
//
// This file defines the extension configuration and functional options.
package validation

// Config controls the validation extension behavior.
type Config struct {
	// Codecs is the ordered list of registered codecs. The first codec is
	// used as the default when the client does not specify an Accept header.
	// Defaults to [JSONCodec{}].
	Codecs []Codec

	// StrictResponses when true causes the middleware to return 500 Internal
	// Server Error if a handler writes a response with a status code that
	// is not documented in the route's Responses() map. When false,
	// undocumented status codes are passed through without validation.
	StrictResponses bool

	// ValidateResponses enables response body validation against the
	// declared Responses() schemas. Default: true.
	ValidateResponses bool
}

// NewDefaultConfig returns the default configuration for the validation extension.
func NewDefaultConfig() *Config {
	return &Config{
		Codecs:            []Codec{JSONCodec{}},
		StrictResponses:   false,
		ValidateResponses: true,
	}
}

// ConfigOption allows functional configuration.
type ConfigOption func(*Config)

// WithCodec appends a codec to the codec list.
func WithCodec(c Codec) ConfigOption {
	return func(cfg *Config) {
		cfg.Codecs = append(cfg.Codecs, c)
	}
}

// WithStrictResponses enables or disables strict response checking.
// When enabled, responses with undocumented status codes cause a 500.
func WithStrictResponses(strict bool) ConfigOption {
	return func(cfg *Config) {
		cfg.StrictResponses = strict
	}
}

// WithValidateResponses enables or disables response body validation.
func WithValidateResponses(validate bool) ConfigOption {
	return func(cfg *Config) {
		cfg.ValidateResponses = validate
	}
}

// NewConfig creates a config with the given options applied on top of defaults.
func NewConfig(opts ...ConfigOption) *Config {
	c := NewDefaultConfig()
	for _, opt := range opts {
		opt(c)
	}
	return c
}
