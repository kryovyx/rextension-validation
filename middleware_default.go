// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

// Package validation provides a Rex extension for request/response body
// validation, content negotiation, and pluggable codec support.
//
// This file implements the validation middleware that handles:
//   - Content-Type check for request bodies (415)
//   - Accept header negotiation (406)
//   - Request body decoding and struct validation (422)
//   - Response body validation with strict mode (500)
//   - Response re-encoding through the negotiated codec
package validation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	rxroute "github.com/kryovyx/rextension/route"
)

// routeValidator wraps the go-playground validator instance.
type routeValidator struct {
	v *validator.Validate
}

// newRouteValidator creates a new validator instance.
func newRouteValidator() routeValidator {
	return routeValidator{v: validator.New(validator.WithRequiredStructEnabled())}
}

// validateStruct validates a struct value using the go-playground validator.
func (rv routeValidator) validateStruct(v interface{}) error {
	return rv.v.Struct(v)
}

// ValidationError represents a structured validation error returned to clients.
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value,omitempty"`
	Message string `json:"message"`
}

// ValidationErrorResponse is the 422 error response body.
type ValidationErrorResponse struct {
	Status  int               `json:"status"`
	Message string            `json:"message"`
	Errors  []ValidationError `json:"errors"`
}

// formatValidationErrors converts go-playground validation errors to our response format.
func formatValidationErrors(err error) []ValidationError {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return []ValidationError{{Message: err.Error()}}
	}
	out := make([]ValidationError, 0, len(ve))
	for _, fe := range ve {
		out = append(out, ValidationError{
			Field:   fe.Field(),
			Tag:     fe.Tag(),
			Value:   fmt.Sprintf("%v", fe.Value()),
			Message: fmt.Sprintf("field '%s' failed on the '%s' tag", fe.Field(), fe.Tag()),
		})
	}
	return out
}

// ValidationMiddleware creates a standard HTTP middleware that validates
// request and response bodies according to the route's ValidatableRoute contract.
func ValidationMiddleware(cfg MiddlewareConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			vr, found := cfg.RouteIndex.lookup(r.Method, r.URL.Path)
			if !found {
				if rt, ok := rxroute.GetMatchedRoute(r); ok {
					if validatable, isValidatable := rt.(ValidatableRoute); isValidatable {
						vr = validatable
						found = true
					}
				}
			}
			if !found {
				next.ServeHTTP(w, r)
				return
			}

			// 1. Content negotiation — determine response codec from Accept header.
			acceptCodec := cfg.Registry.negotiate(r.Header.Get("Accept"))
			if acceptCodec == nil {
				http.Error(w, "406 Not Acceptable: no supported media type in Accept header", http.StatusNotAcceptable)
				return
			}

			// Store the chosen codec in context for downstream use.
			r = r.WithContext(context.WithValue(r.Context(), ContextKeyAcceptCodec, acceptCodec))

			// 2. Request body validation.
			reqSchema := vr.RequestBody()
			if reqSchema != nil {
				// Check Content-Type.
				ct := r.Header.Get("Content-Type")
				if ct == "" {
					http.Error(w, "415 Unsupported Media Type: Content-Type header required", http.StatusUnsupportedMediaType)
					return
				}
				reqCodec := cfg.Registry.findByContentType(ct)
				if reqCodec == nil {
					http.Error(w, "415 Unsupported Media Type: "+ct, http.StatusUnsupportedMediaType)
					return
				}

				// Read request body.
				body, err := io.ReadAll(r.Body)
				r.Body.Close()
				if err != nil {
					http.Error(w, "400 Bad Request: failed to read request body", http.StatusBadRequest)
					return
				}

				// Decode + validate according to schema kind.
				decoded, err := decodeAndValidate(body, reqSchema, reqCodec, cfg.Validator)
				if err != nil {
					writeValidationError(w, acceptCodec, err)
					return
				}

				// Store decoded body in context.
				r = r.WithContext(context.WithValue(r.Context(), ContextKeyRequestBody, decoded))
			}

			// 3. If response validation is enabled, wrap the writer.
			if cfg.ValidateResp && vr.Responses() != nil {
				rec := &responseRecorder{
					header: make(http.Header),
					body:   &bytes.Buffer{},
					status: http.StatusOK,
				}
				next.ServeHTTP(rec, r)

				// Validate and flush.
				flushRecordedResponse(w, rec, vr, acceptCodec, cfg)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// decodeAndValidate tries to decode the body according to the schema kind.
// For OneOf: tries each type, succeeds on exactly one.
// For AnyOf: tries each type, succeeds on at least one.
// For AllOf: tries all types, all must succeed.
// For Scalar: decodes into the single type.
func decodeAndValidate(body []byte, schema BodySchema, codec Codec, val routeValidator) (interface{}, error) {
	types := schema.Types()
	if len(types) == 0 {
		return nil, fmt.Errorf("body schema has no types")
	}

	switch schema.Kind() {
	case SchemaScalar:
		return decodeAndValidateOne(body, types[0], codec, val)

	case SchemaOneOf:
		var matched interface{}
		matchCount := 0
		var lastErr error
		for _, t := range types {
			v, err := decodeAndValidateOne(body, t, codec, val)
			if err == nil {
				matched = v
				matchCount++
			} else {
				lastErr = err
			}
		}
		if matchCount == 1 {
			return matched, nil
		}
		if matchCount == 0 {
			return nil, fmt.Errorf("body did not match any oneOf schema: %v", lastErr)
		}
		return nil, fmt.Errorf("body matched %d of %d oneOf schemas (exactly 1 required)", matchCount, len(types))

	case SchemaAnyOf:
		var matched interface{}
		var lastErr error
		for _, t := range types {
			v, err := decodeAndValidateOne(body, t, codec, val)
			if err == nil {
				matched = v
				// Return first match.
				return matched, nil
			}
			lastErr = err
		}
		return nil, fmt.Errorf("body did not match any anyOf schema: %v", lastErr)

	case SchemaAllOf:
		// For allOf, we validate against each type; return the first decoded value.
		var first interface{}
		for i, t := range types {
			v, err := decodeAndValidateOne(body, t, codec, val)
			if err != nil {
				return nil, fmt.Errorf("body failed allOf schema %d: %v", i, err)
			}
			if first == nil {
				first = v
			}
		}
		return first, nil

	default:
		return nil, fmt.Errorf("unknown schema kind: %v", schema.Kind())
	}
}

// decodeAndValidateOne decodes the body into a new value of the same type as
// zeroVal and runs struct validation on it.
func decodeAndValidateOne(body []byte, zeroVal interface{}, codec Codec, val routeValidator) (interface{}, error) {
	t := reflect.TypeOf(zeroVal)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	ptr := reflect.New(t).Interface()

	if err := codec.Unmarshal(body, ptr); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	if err := val.validateStruct(ptr); err != nil {
		return nil, err
	}

	return ptr, nil
}

// writeValidationError writes a 422 response with structured validation errors.
func writeValidationError(w http.ResponseWriter, codec Codec, err error) {
	resp := ValidationErrorResponse{
		Status:  http.StatusUnprocessableEntity,
		Message: "Validation failed",
		Errors:  formatValidationErrors(err),
	}
	data, marshalErr := codec.Marshal(resp)
	if marshalErr != nil {
		http.Error(w, "422 Unprocessable Entity", http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", codec.ContentType())
	w.WriteHeader(http.StatusUnprocessableEntity)
	w.Write(data)
}

// responseRecorder captures the handler's response for deferred validation.
type responseRecorder struct {
	header http.Header
	body   *bytes.Buffer
	status int
}

func (rr *responseRecorder) Header() http.Header        { return rr.header }
func (rr *responseRecorder) WriteHeader(statusCode int) { rr.status = statusCode }
func (rr *responseRecorder) Write(b []byte) (int, error) {
	return rr.body.Write(b)
}

// flushRecordedResponse validates the captured response and flushes it.
func flushRecordedResponse(
	w http.ResponseWriter,
	rec *responseRecorder,
	vr ValidatableRoute,
	acceptCodec Codec,
	cfg MiddlewareConfig,
) {
	responses := vr.Responses()
	schema, documented := responses[rec.status]

	if !documented {
		if cfg.Strict {
			// Strict mode: undocumented status code → 500.
			writeStrictError(w, acceptCodec, fmt.Sprintf(
				"response status %d is not documented for this route", rec.status,
			))
			return
		}
		// Non-strict: passthrough as-is.
		copyHeaders(w.Header(), rec.header)
		w.WriteHeader(rec.status)
		w.Write(rec.body.Bytes())
		return
	}

	// Validate the response body against the schema.
	if schema != nil && rec.body.Len() > 0 {
		// Determine codec from response Content-Type (handler may have set it).
		respCT := rec.header.Get("Content-Type")
		respCodec := cfg.Registry.findByContentType(respCT)
		if respCodec == nil {
			respCodec = acceptCodec
		}

		_, err := decodeAndValidate(rec.body.Bytes(), schema, respCodec, cfg.Validator)
		if err != nil {
			if cfg.Strict {
				writeStrictError(w, acceptCodec, fmt.Sprintf(
					"response body validation failed: %v", err,
				))
				return
			}
			// Non-strict: passthrough.
		}
	}

	// Re-encode through the accepted codec if different from what the handler used.
	// For now, passthrough the original bytes — re-encoding requires parsing the
	// original value, which we've already done for validation.
	copyHeaders(w.Header(), rec.header)
	// Ensure Content-Type matches the negotiated codec.
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", acceptCodec.ContentType())
	}
	w.WriteHeader(rec.status)
	w.Write(rec.body.Bytes())
}

// writeStrictError writes a 500 Internal Server Error for strict mode violations.
func writeStrictError(w http.ResponseWriter, codec Codec, msg string) {
	resp := struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	}{
		Status:  http.StatusInternalServerError,
		Message: msg,
	}
	data, err := codec.Marshal(resp)
	if err != nil {
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", codec.ContentType())
	w.WriteHeader(http.StatusInternalServerError)
	w.Write(data)
}

// copyHeaders copies all headers from src to dst.
func copyHeaders(dst, src http.Header) {
	for k, vs := range src {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

// hasBody returns true if the request method typically carries a body.
func hasBody(method string) bool {
	switch strings.ToUpper(method) {
	case "POST", "PUT", "PATCH":
		return true
	default:
		return false
	}
}
