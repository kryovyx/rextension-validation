# Rex Validation Extension (rextension-validation)

A Rex extension for automatic request/response body validation, content negotiation, and pluggable codec support.

[![Go Version](https://img.shields.io/badge/go-1.26+-blue.svg)](https://golang.org/dl/)
[![Coverage](https://img.shields.io/badge/coverage-87.9%25-green.svg)](#)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## Overview

`rextension-validation` is a Rex extension that provides:

- **Automatic request body decoding** using a pluggable `Codec` interface
- **Struct validation** via [go-playground/validator/v10](https://github.com/go-playground/validator)
- **Content-Type checking** — returns `415 Unsupported Media Type` when the request body uses an unregistered content type
- **Accept header negotiation** with quality values — returns `406 Not Acceptable` when no registered codec matches
- **Response body validation** with optional strict mode — returns `500 Internal Server Error` for undocumented status codes
- **Union schemas** — `OneOf`, `AnyOf`, `AllOf` for advanced request/response contracts
- **Per-status-code response schemas** — validate outgoing bodies per HTTP status
- **Context helpers** — retrieve decoded bodies and negotiated codecs in handlers

## Installation

```bash
go get github.com/kryovyx/rextension-validation
```

## Quick Start

Define a validated route with a `Scalar` request body:

```go
package main

import (
    "net/http"

    "github.com/kryovyx/rex"
    "github.com/kryovyx/rex/route"
    validation "github.com/kryovyx/rextension-validation"
)

// CreateUserRequest is the expected request body.
type CreateUserRequest struct {
    Name  string `json:"name"  validate:"required,min=2"`
    Email string `json:"email" validate:"required,email"`
}

// CreateUserResponse is the response body.
type CreateUserResponse struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

// CreateUserRoute implements both route.Route and ValidatableRoute.
type CreateUserRoute struct {
    route.Route
}

func (r *CreateUserRoute) RequestBody() validation.BodySchema {
    return validation.Scalar(CreateUserRequest{})
}

func (r *CreateUserRoute) Responses() map[int]validation.BodySchema {
    return map[int]validation.BodySchema{
        201: validation.Scalar(CreateUserResponse{}),
    }
}

func main() {
    app := rex.New()

    // Add the validation extension
    app.WithOptions(
        validation.WithValidation(nil),
    )

    // Register a validated route
    app.RegisterRoute(&CreateUserRoute{
        Route: route.New("POST", "/users", func(ctx route.Context) {
            body, ok := validation.GetRequestBody[CreateUserRequest](ctx.Request())
            if !ok {
                ctx.Text(http.StatusBadRequest, "missing body")
                return
            }
            resp := CreateUserResponse{ID: 1, Name: body.Name, Email: body.Email}
            codec := validation.GetAcceptCodec(ctx.Request())
            data, _ := codec.Marshal(resp)
            ctx.Respond(http.StatusCreated, codec.ContentType(), data)
        }),
    })

    if err := app.Run(); err != nil {
        panic(err)
    }
}
```

## Core Concepts

### Codecs

A `Codec` encodes and decodes values for a specific content type. The built-in `JSONCodec` handles `application/json`.

```go
// The Codec interface
type Codec interface {
    ContentType() string                       // e.g., "application/json"
    Marshal(v interface{}) ([]byte, error)      // Serialize
    Unmarshal(data []byte, v interface{}) error  // Deserialize
}
```

`JSONCodec` is registered by default. Add more codecs via configuration:

```go
validation.WithValidation(validation.NewConfig(
    validation.WithCodec(myXMLCodec),
    validation.WithCodec(myYAMLCodec),
))
```

The first codec in the list serves as the default when the client does not specify an `Accept` header.

### Content Negotiation

The middleware performs two content negotiation checks on every request to a `ValidatableRoute`:

| Check | Header | Failure | HTTP Status |
|-------|--------|---------|-------------|
| Request body decoding | `Content-Type` | Content type not registered | `415 Unsupported Media Type` |
| Response encoding | `Accept` | No registered codec matches | `406 Not Acceptable` |

The `Accept` header supports quality values (`q=...`). For example, `Accept: application/xml;q=0.9, application/json;q=1.0` prefers JSON.

### Body Schemas

Body schemas describe the shape of request and response bodies. Four schema kinds are available:

| Constructor | Kind | Description |
|-------------|------|-------------|
| `Scalar(v)` | `SchemaScalar` | A single concrete type |
| `OneOf(vs...)` | `SchemaOneOf` | Exactly one of the listed types must match |
| `AnyOf(vs...)` | `SchemaAnyOf` | One or more of the listed types may match (first match wins) |
| `AllOf(vs...)` | `SchemaAllOf` | All of the listed types must match (merged) |

```go
// Single type
validation.Scalar(CreateUserRequest{})

// Exactly one must match
validation.OneOf(AdminRequest{}, UserRequest{})

// Any may match
validation.AnyOf(JSONPayload{}, XMLPayload{})

// All must match (merged fields)
validation.AllOf(BaseRequest{}, ExtendedFields{})
```

### Validation

Struct validation uses [go-playground/validator/v10](https://github.com/go-playground/validator) tags. Add `validate` struct tags to your request types:

```go
type CreateUserRequest struct {
    Name  string `json:"name"  validate:"required,min=2,max=100"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age"   validate:"omitempty,gte=0,lte=150"`
}
```

When validation fails, the middleware returns a structured `422 Unprocessable Entity` response (see [Error Responses](#error-responses)).

## ValidatableRoute Interface

Routes opt into validation by implementing the `ValidatableRoute` interface. Routes that do not implement it are passed through without validation.

```go
type ValidatableRoute interface {
    // RequestBody returns the body schema for the request, or nil to skip.
    RequestBody() BodySchema

    // Responses returns a map of HTTP status code → body schema.
    // Return nil to skip response validation.
    Responses() map[int]BodySchema
}
```

Example with multiple response codes:

```go
func (r *GetUserRoute) RequestBody() validation.BodySchema {
    return nil // GET — no request body
}

func (r *GetUserRoute) Responses() map[int]validation.BodySchema {
    return map[int]validation.BodySchema{
        200: validation.Scalar(UserResponse{}),
        404: validation.Scalar(ErrorResponse{}),
    }
}
```

## Context Helpers

After the middleware processes a request, decoded values are stored in the request context. Two generic helpers retrieve them in handlers:

### GetRequestBody

```go
body, ok := validation.GetRequestBody[CreateUserRequest](ctx.Request())
if !ok {
    // No decoded body — route may not implement ValidatableRoute
}
```

### GetAcceptCodec

```go
codec := validation.GetAcceptCodec(ctx.Request())
if codec != nil {
    data, _ := codec.Marshal(response)
    ctx.Respond(http.StatusOK, codec.ContentType(), data)
}
```

## Response Validation

When enabled (default), the middleware validates outgoing response bodies against the schemas declared in `Responses()`. The response is captured, validated, and then flushed to the client.

### Strict Mode

With `StrictResponses` enabled, any status code **not** present in the `Responses()` map causes a `500 Internal Server Error` instead of passing through:

```go
validation.WithValidation(validation.NewConfig(
    validation.WithStrictResponses(true),
))
```

| Scenario | `StrictResponses: false` (default) | `StrictResponses: true` |
|----------|------------------------------------|-------------------------|
| Status code in `Responses()` map | Validate body against schema | Validate body against schema |
| Status code **not** in `Responses()` map | Pass through unvalidated | Return `500 Internal Server Error` |
| Response body fails validation | Return `500 Internal Server Error` | Return `500 Internal Server Error` |

Disable response validation entirely:

```go
validation.WithValidation(validation.NewConfig(
    validation.WithValidateResponses(false),
))
```

## Configuration Reference

All configuration is optional. Pass `nil` to `WithValidation` to use the defaults.

### Config Struct

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Codecs` | `[]Codec` | `[JSONCodec{}]` | Ordered list of registered codecs. The first codec is the default. |
| `StrictResponses` | `bool` | `false` | Return `500` for undocumented status codes. |
| `ValidateResponses` | `bool` | `true` | Enable response body validation against declared schemas. |

### Functional Options

```go
cfg := validation.NewConfig(
    validation.WithCodec(myXMLCodec),
    validation.WithStrictResponses(true),
    validation.WithValidateResponses(true),
)

app.WithOptions(validation.WithValidation(cfg))
```

| Option | Description |
|--------|-------------|
| `WithCodec(c)` | Appends a codec to the codec list. |
| `WithStrictResponses(strict)` | Enables or disables strict response checking. |
| `WithValidateResponses(validate)` | Enables or disables response body validation. |

## Custom Codec Example (XML)

Implement the `Codec` interface to support additional content types:

```go
package main

import "encoding/xml"

// XMLCodec handles application/xml.
type XMLCodec struct{}

func (XMLCodec) ContentType() string                       { return "application/xml" }
func (XMLCodec) Marshal(v interface{}) ([]byte, error)     { return xml.Marshal(v) }
func (XMLCodec) Unmarshal(data []byte, v interface{}) error { return xml.Unmarshal(data, v) }
```

Register it alongside the default JSON codec:

```go
validation.WithValidation(validation.NewConfig(
    validation.WithCodec(XMLCodec{}),
))
```

Clients can now send `Content-Type: application/xml` and negotiate responses via `Accept: application/xml`.

## Error Responses

When request body validation fails, the middleware returns a `422 Unprocessable Entity` response with a structured JSON body:

```json
{
  "status": 422,
  "message": "Validation failed",
  "errors": [
    {
      "field": "Name",
      "tag": "required",
      "value": "",
      "message": "field 'Name' failed on the 'required' tag"
    },
    {
      "field": "Email",
      "tag": "email",
      "value": "not-an-email",
      "message": "field 'Email' failed on the 'email' tag"
    }
  ]
}
```

The response types are:

```go
type ValidationError struct {
    Field   string `json:"field"`
    Tag     string `json:"tag"`
    Value   string `json:"value,omitempty"`
    Message string `json:"message"`
}

type ValidationErrorResponse struct {
    Status  int               `json:"status"`
    Message string            `json:"message"`
    Errors  []ValidationError `json:"errors"`
}
```

## Best Practices

1. **Use struct tags consistently** — always pair `json` tags with `validate` tags to keep serialization and validation aligned.
2. **Start without strict mode** — enable `StrictResponses` once all response codes are fully documented in `Responses()`.
3. **Keep schemas in sync** — when adding a new status code to a handler, add it to `Responses()` as well.
4. **Prefer `Scalar` for most routes** — use `OneOf`/`AnyOf`/`AllOf` only when your API genuinely accepts polymorphic payloads.
5. **Register codecs once** — add all codecs at startup via `WithCodec`; avoid modifying the codec list after the application starts.
6. **Use context helpers** — always use `GetRequestBody[T]` and `GetAcceptCodec` instead of manually decoding; the middleware has already done the work.
7. **Return the negotiated codec's content type** — use `codec.ContentType()` from `GetAcceptCodec` in your `ctx.Respond` calls so the response matches what the client asked for.

## Contributing

**At this time, this project is in active development and is not open for external contributions.** The framework is still being refined and major interfaces may change.

Once the framework reaches a stable architecture and API, contributions from the community will be welcome. Please check back later or open an issue if you have feature requests or feedback.

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.

## Copyright

© 2026 Kryovyx
