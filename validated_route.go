// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

// Package validation provides a Rex extension for request/response body
// validation, content negotiation, and pluggable codec support.
//
// This file defines the ValidatableRoute interface and BodySchema types
// that routes implement to declare their request/response body contracts.
package validation

// SchemaKind describes the composition strategy for a body schema.
type SchemaKind int

const (
	// SchemaScalar indicates a single concrete type.
	SchemaScalar SchemaKind = iota
	// SchemaOneOf indicates exactly one of the listed types must match.
	SchemaOneOf
	// SchemaAnyOf indicates one or more of the listed types may match.
	SchemaAnyOf
	// SchemaAllOf indicates all of the listed types must match (merged).
	SchemaAllOf
)

// String returns a human-readable name for the schema kind.
func (k SchemaKind) String() string {
	switch k {
	case SchemaScalar:
		return "scalar"
	case SchemaOneOf:
		return "oneOf"
	case SchemaAnyOf:
		return "anyOf"
	case SchemaAllOf:
		return "allOf"
	default:
		return "unknown"
	}
}

// BodySchema describes the shape of a request or response body.
// It can represent a single type (Scalar) or a union (OneOf/AnyOf/AllOf).
type BodySchema interface {
	// Kind returns the composition strategy.
	Kind() SchemaKind
	// Types returns the zero-value struct(s) that define the schema.
	// For Scalar, len(Types()) == 1. For unions, len >= 2.
	Types() []interface{}
}

// bodySchema is the default implementation of BodySchema.
type bodySchema struct {
	kind  SchemaKind
	types []interface{}
}

func (b *bodySchema) Kind() SchemaKind     { return b.kind }
func (b *bodySchema) Types() []interface{} { return b.types }

// Scalar creates a BodySchema for a single concrete type.
func Scalar(v interface{}) BodySchema {
	return &bodySchema{kind: SchemaScalar, types: []interface{}{v}}
}

// OneOf creates a BodySchema where exactly one of the given types must match.
func OneOf(vs ...interface{}) BodySchema {
	return &bodySchema{kind: SchemaOneOf, types: vs}
}

// AnyOf creates a BodySchema where one or more of the given types may match.
func AnyOf(vs ...interface{}) BodySchema {
	return &bodySchema{kind: SchemaAnyOf, types: vs}
}

// AllOf creates a BodySchema where all of the given types must match (merged).
func AllOf(vs ...interface{}) BodySchema {
	return &bodySchema{kind: SchemaAllOf, types: vs}
}

// ValidatableRoute is an optional interface that a route.Route may implement
// to declare its request and response body schemas.
//
// The validation middleware type-asserts registered routes to this interface;
// routes that do not implement it are passed through without validation.
type ValidatableRoute interface {
	// RequestBody returns the body schema for the request, or nil if the
	// route does not accept a request body.
	RequestBody() BodySchema

	// Responses returns a map of HTTP status code → body schema.
	// The middleware uses this to validate outgoing responses.
	// Return nil to skip response validation entirely.
	Responses() map[int]BodySchema
}
