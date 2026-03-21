package validation_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kryovyx/rex/route"
	validation "github.com/kryovyx/rextension-validation"
)

// --- Test helpers ---

// testValidatableRoute implements both route.Route and validation.ValidatableRoute.
type testValidatableRoute struct {
	method  string
	path    string
	reqBody validation.BodySchema
	respMap map[int]validation.BodySchema
}

func (r *testValidatableRoute) Method() string                           { return r.method }
func (r *testValidatableRoute) Path() string                             { return r.path }
func (r *testValidatableRoute) Handler() route.HandlerFunc               { return nil }
func (r *testValidatableRoute) RequestBody() validation.BodySchema       { return r.reqBody }
func (r *testValidatableRoute) Responses() map[int]validation.BodySchema { return r.respMap }

// testPlainRoute implements route.Route but NOT ValidatableRoute.
type testPlainRoute struct {
	method string
	path   string
}

func (r *testPlainRoute) Method() string             { return r.method }
func (r *testPlainRoute) Path() string               { return r.path }
func (r *testPlainRoute) Handler() route.HandlerFunc { return nil }

// Test request/response types.
type createUserReq struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

type createUserResp struct {
	ID    int    `json:"id"`
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

type alternateReq struct {
	Code int `json:"code" validate:"required"`
}

type flexReqA struct {
	Alpha string `json:"alpha" validate:"required"`
}

type flexReqB struct {
	Beta string `json:"beta" validate:"required"`
}

func newMiddleware(codecs []validation.Codec, strict, validateResp bool, routes ...route.Route) func(http.Handler) http.Handler {
	cfg := validation.NewTestMiddlewareConfig(codecs, strict, validateResp)
	for _, rt := range routes {
		validation.RegisterTestRoute(&cfg, rt)
	}
	return validation.ValidationMiddleware(cfg)
}

// --- Route not in index ---

func TestMiddleware_RouteNotInIndex_Passthrough(t *testing.T) {
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/unregistered", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("expected body ok, got %s", rec.Body.String())
	}
}

// --- 415 Unsupported Media Type ---

func TestMiddleware_MissingContentType_415(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/users",
		reqBody: validation.Scalar(createUserReq{}),
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test","email":"test@test.com"}`))
	// No Content-Type header set.
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("expected 415, got %d", rec.Code)
	}
}

func TestMiddleware_WrongContentType_415(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/users",
		reqBody: validation.Scalar(createUserReq{}),
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`<user/>`))
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("expected 415, got %d", rec.Code)
	}
}

// --- 406 Not Acceptable ---

func TestMiddleware_UnacceptableAccept_406(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/users",
		reqBody: validation.Scalar(createUserReq{}),
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test","email":"test@test.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/xml")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotAcceptable {
		t.Errorf("expected 406, got %d", rec.Code)
	}
}

// --- 422 Validation fails ---

func TestMiddleware_ValidationFails_422(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/users",
		reqBody: validation.Scalar(createUserReq{}),
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called on validation failure")
	}))

	// Missing required "email" field.
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}

	var errResp validation.ValidationErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	if errResp.Status != 422 {
		t.Errorf("expected status 422 in body, got %d", errResp.Status)
	}
	if len(errResp.Errors) == 0 {
		t.Error("expected validation errors in response")
	}
}

// --- 200 Success with body in context ---

func TestMiddleware_ValidRequest_200_BodyInContext(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/users",
		reqBody: validation.Scalar(createUserReq{}),
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)

	var gotBody createUserReq
	var gotOk bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, gotOk = validation.GetRequestBody[createUserReq](r)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("created"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"Alice","email":"alice@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !gotOk {
		t.Fatal("expected body in context")
	}
	if gotBody.Name != "Alice" {
		t.Errorf("expected Name=Alice, got %s", gotBody.Name)
	}
	if gotBody.Email != "alice@example.com" {
		t.Errorf("expected Email=alice@example.com, got %s", gotBody.Email)
	}
}

// --- No request body schema → skip body validation ---

func TestMiddleware_NoRequestBody_SkipsBodyValidation(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "GET",
		path:    "/users",
		reqBody: nil,
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("list"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// --- OneOf schema ---

func TestMiddleware_OneOf_ExactlyOneMatch(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/flex",
		reqBody: validation.OneOf(flexReqA{}, flexReqB{}),
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Only flexReqA should match (alpha is required).
	req := httptest.NewRequest(http.MethodPost, "/flex", strings.NewReader(`{"alpha":"val"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for oneOf with exactly 1 match, got %d", rec.Code)
	}
}

func TestMiddleware_OneOf_ZeroMatches(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/flex",
		reqBody: validation.OneOf(flexReqA{}, flexReqB{}),
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	// Neither matches (both require alpha/beta).
	req := httptest.NewRequest(http.MethodPost, "/flex", strings.NewReader(`{"unrelated":"val"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for oneOf with 0 matches, got %d", rec.Code)
	}
}

func TestMiddleware_OneOf_MultipleMatches(t *testing.T) {
	// Use types where the same body matches both schemas.
	type X struct {
		Val string `json:"val" validate:"required"`
	}
	type Y struct {
		Val string `json:"val" validate:"required"`
	}
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/flex",
		reqBody: validation.OneOf(X{}, Y{}),
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for multiple oneOf matches")
	}))

	req := httptest.NewRequest(http.MethodPost, "/flex", strings.NewReader(`{"val":"both"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for oneOf with >1 matches, got %d", rec.Code)
	}
}

// --- AnyOf schema ---

func TestMiddleware_AnyOf_OneMatch(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/any",
		reqBody: validation.AnyOf(flexReqA{}, flexReqB{}),
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/any", strings.NewReader(`{"alpha":"val"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for anyOf with at least 1 match, got %d", rec.Code)
	}
}

func TestMiddleware_AnyOf_ZeroMatches(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/any",
		reqBody: validation.AnyOf(flexReqA{}, flexReqB{}),
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/any", strings.NewReader(`{"nothing":"here"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for anyOf with 0 matches, got %d", rec.Code)
	}
}

// --- AllOf schema ---

func TestMiddleware_AllOf_AllMatch(t *testing.T) {
	// Both types use the same JSON body, both are valid.
	type P1 struct {
		Name string `json:"name" validate:"required"`
	}
	type P2 struct {
		Name string `json:"name" validate:"required"`
	}
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/all",
		reqBody: validation.AllOf(P1{}, P2{}),
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/all", strings.NewReader(`{"name":"ok"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for allOf with all matching, got %d", rec.Code)
	}
}

func TestMiddleware_AllOf_SomeFail(t *testing.T) {
	type P1 struct {
		Name string `json:"name" validate:"required"`
	}
	type P2 struct {
		Age int `json:"age" validate:"required"`
	}
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/all",
		reqBody: validation.AllOf(P1{}, P2{}),
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	// P1 passes but P2 fails because age=0 is the zero value and required.
	req := httptest.NewRequest(http.MethodPost, "/all", strings.NewReader(`{"name":"ok"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for allOf with some failures, got %d", rec.Code)
	}
}

// --- Response validation ---

func TestMiddleware_ResponseValidation_StrictUndocumentedStatus_500(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/users",
		reqBody: nil,
		respMap: map[int]validation.BodySchema{
			200: validation.Scalar(createUserResp{}),
		},
	}
	// strict=true, validateResp=true.
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, true, true, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handler returns 201, which is NOT documented.
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":1,"name":"Alice","email":"alice@example.com"}`))
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for strict undocumented status, got %d", rec.Code)
	}
}

func TestMiddleware_ResponseValidation_NonStrictUndocumentedStatus_Passthrough(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/users",
		reqBody: nil,
		respMap: map[int]validation.BodySchema{
			200: validation.Scalar(createUserResp{}),
		},
	}
	// strict=false, validateResp=true.
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, true, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":1}`))
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201 passthrough for non-strict, got %d", rec.Code)
	}
}

func TestMiddleware_ResponseValidation_DocumentedStatus_ValidBody(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/users",
		reqBody: nil,
		respMap: map[int]validation.BodySchema{
			200: validation.Scalar(createUserResp{}),
		},
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, true, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"name":"Alice","email":"alice@example.com"}`))
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestMiddleware_ResponseValidation_DocumentedStatus_InvalidBody_Strict500(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/users",
		reqBody: nil,
		respMap: map[int]validation.BodySchema{
			200: validation.Scalar(createUserResp{}),
		},
	}
	// strict=true, validateResp=true.
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, true, true, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Missing required fields → invalid response body.
		w.Write([]byte(`{"id":1}`))
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for strict invalid response, got %d", rec.Code)
	}
}

func TestMiddleware_ResponseValidation_DocumentedStatus_InvalidBody_NonStrict_Passthrough(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/users",
		reqBody: nil,
		respMap: map[int]validation.BodySchema{
			200: validation.Scalar(createUserResp{}),
		},
	}
	// strict=false, validateResp=true.
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, true, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Missing required fields → invalid but non-strict so passthrough.
		w.Write([]byte(`{"id":1}`))
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 passthrough for non-strict invalid response, got %d", rec.Code)
	}
}

func TestMiddleware_ResponseValidation_Disabled_Passthrough(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "GET",
		path:    "/data",
		reqBody: nil,
		respMap: map[int]validation.BodySchema{
			200: validation.Scalar(createUserResp{}),
		},
	}
	// validateResp=false → skip response validation entirely.
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, true, false, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("raw data"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/data", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 when response validation disabled, got %d", rec.Code)
	}
}

func TestMiddleware_ResponseValidation_NilResponsesMap_Passthrough(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "GET",
		path:    "/simple",
		reqBody: nil,
		respMap: nil, // No response schemas at all.
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, true, true, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/simple", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with nil responses map, got %d", rec.Code)
	}
}

// --- AcceptCodec in context ---

func TestMiddleware_AcceptCodec_InContext(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "GET",
		path:    "/ctx",
		reqBody: nil,
		respMap: nil,
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)

	var gotCodec validation.Codec
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCodec = validation.GetAcceptCodec(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ctx", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotCodec == nil {
		t.Fatal("expected codec in context")
	}
	if gotCodec.ContentType() != "application/json" {
		t.Errorf("codec content type mismatch: got %s", gotCodec.ContentType())
	}
}

// --- Plain route (not ValidatableRoute) not registered ---

func TestMiddleware_PlainRoute_NotRegistered_Passthrough(t *testing.T) {
	pr := &testPlainRoute{method: "GET", path: "/plain"}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, pr)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("plain"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/plain", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// --- Bad request body (unreadable) ---

func TestMiddleware_UnmarshalError_422(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/users",
		reqBody: validation.Scalar(createUserReq{}),
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{not json`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for unmarshal error, got %d", rec.Code)
	}
}

// --- formatValidationErrors ---

func TestFormatValidationErrors_NonValidatorError(t *testing.T) {
	errs := validation.ExportFormatValidationErrors(fmt.Errorf("some generic error"))
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Message != "some generic error" {
		t.Errorf("expected message 'some generic error', got %s", errs[0].Message)
	}
}

// --- hasBody ---

func TestHasBody(t *testing.T) {
	tests := []struct {
		method   string
		expected bool
	}{
		{"POST", true},
		{"PUT", true},
		{"PATCH", true},
		{"GET", false},
		{"DELETE", false},
		{"HEAD", false},
		{"OPTIONS", false},
		{"post", true}, // lowercase
		{"get", false}, // lowercase
	}
	for _, tc := range tests {
		got := validation.ExportHasBody(tc.method)
		if got != tc.expected {
			t.Errorf("hasBody(%q): expected %v, got %v", tc.method, tc.expected, got)
		}
	}
}

// --- copyHeaders ---

func TestCopyHeaders(t *testing.T) {
	src := http.Header{
		"Content-Type": {"application/json"},
		"X-Custom":     {"val1", "val2"},
	}
	dst := http.Header{}
	validation.ExportCopyHeaders(dst, src)

	if got := dst.Get("Content-Type"); got != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", got)
	}
	vals := dst.Values("X-Custom")
	if len(vals) != 2 {
		t.Errorf("expected 2 X-Custom values, got %d", len(vals))
	}
}

func TestCopyHeaders_EmptySrc(t *testing.T) {
	src := http.Header{}
	dst := http.Header{"Existing": {"val"}}
	validation.ExportCopyHeaders(dst, src)

	if got := dst.Get("Existing"); got != "val" {
		t.Errorf("expected existing header preserved, got %s", got)
	}
}

// --- Response with nil schema for documented status → passthrough ---

func TestMiddleware_ResponseValidation_NilSchemaForStatus_Passthrough(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "DELETE",
		path:    "/users/1",
		reqBody: nil,
		respMap: map[int]validation.BodySchema{
			204: nil, // Documented but no body schema.
		},
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, true, true, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/users/1", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for nil schema, got %d", rec.Code)
	}
}

// --- Verify Content-Type header set in response ---

func TestMiddleware_ResponseSetsContentType(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "GET",
		path:    "/ct",
		reqBody: nil,
		respMap: map[int]validation.BodySchema{
			200: validation.Scalar(createUserResp{}),
		},
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, true, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"name":"X","email":"x@x.com"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/ct", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
}

// --- Request body with io.ReadAll failure is hard to simulate, but let's test empty body ---

func TestMiddleware_EmptyBody_ValidationFails(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/users",
		reqBody: validation.Scalar(createUserReq{}),
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for empty body, got %d", rec.Code)
	}
}

// --- Read body error simulation ---

type errReader struct{}

func (errReader) Read(p []byte) (int, error) { return 0, fmt.Errorf("read error") }
func (errReader) Close() error               { return nil }

func TestMiddleware_ReadBodyError_400(t *testing.T) {
	rt := &testValidatableRoute{
		method:  "POST",
		path:    "/users",
		reqBody: validation.Scalar(createUserReq{}),
	}
	mw := newMiddleware([]validation.Codec{validation.JSONCodec{}}, false, false, rt)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	req.Body = io.NopCloser(errReader{})
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for read body error, got %d", rec.Code)
	}
}
