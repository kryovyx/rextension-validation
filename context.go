// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

// Package validation provides a Rex extension for request/response body
// validation, content negotiation, and pluggable codec support.
//
// This file provides request context helpers for handlers to retrieve
// decoded request bodies and the negotiated codec.
package validation

import (
	"net/http"
)

// GetRequestBody retrieves the decoded and validated request body from the
// request context. Returns the value and true if found, or the zero value
// and false otherwise.
//
// Usage:
//
//	body, ok := validation.GetRequestBody[MyRequestType](r)
func GetRequestBody[T any](r *http.Request) (T, bool) {
	val := r.Context().Value(ContextKeyRequestBody)
	if val == nil {
		var zero T
		return zero, false
	}
	if typed, ok := val.(*T); ok {
		return *typed, true
	}
	// Also try non-pointer.
	if typed, ok := val.(T); ok {
		return typed, true
	}
	var zero T
	return zero, false
}

// GetAcceptCodec retrieves the negotiated response codec from the request
// context. Returns nil if no codec was negotiated (e.g., route has no
// ValidatableRoute contract).
func GetAcceptCodec(r *http.Request) Codec {
	val := r.Context().Value(ContextKeyAcceptCodec)
	if val == nil {
		return nil
	}
	c, _ := val.(Codec)
	return c
}
