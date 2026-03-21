package validation_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	validation "github.com/kryovyx/rextension-validation"
)

// --- GetRequestBody ---

func TestGetRequestBody_MatchingType(t *testing.T) {
	type Body struct{ Name string }
	body := &Body{Name: "hello"}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), validation.ContextKeyRequestBody, body))

	got, ok := validation.GetRequestBody[Body](req)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.Name != "hello" {
		t.Errorf("expected Name=hello, got %s", got.Name)
	}
}

func TestGetRequestBody_NonPointerValue(t *testing.T) {
	type Body struct{ Name string }
	body := Body{Name: "world"}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), validation.ContextKeyRequestBody, body))

	got, ok := validation.GetRequestBody[Body](req)
	if !ok {
		t.Fatal("expected ok=true for non-pointer value")
	}
	if got.Name != "world" {
		t.Errorf("expected Name=world, got %s", got.Name)
	}
}

func TestGetRequestBody_WrongType(t *testing.T) {
	type Body struct{ Name string }
	type Other struct{ Age int }

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), validation.ContextKeyRequestBody, Other{Age: 42}))

	_, ok := validation.GetRequestBody[Body](req)
	if ok {
		t.Error("expected ok=false for wrong type")
	}
}

func TestGetRequestBody_NilContextValue(t *testing.T) {
	type Body struct{ Name string }

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, ok := validation.GetRequestBody[Body](req)
	if ok {
		t.Error("expected ok=false when no value in context")
	}
}

// --- GetAcceptCodec ---

func TestGetAcceptCodec_WithCodec(t *testing.T) {
	codec := validation.JSONCodec{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), validation.ContextKeyAcceptCodec, codec))

	got := validation.GetAcceptCodec(req)
	if got == nil {
		t.Fatal("expected non-nil codec")
	}
	if got.ContentType() != "application/json" {
		t.Errorf("expected application/json, got %s", got.ContentType())
	}
}

func TestGetAcceptCodec_WithoutCodec(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	got := validation.GetAcceptCodec(req)
	if got != nil {
		t.Errorf("expected nil codec, got %v", got)
	}
}

func TestGetAcceptCodec_WrongType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), validation.ContextKeyAcceptCodec, "not a codec"))

	got := validation.GetAcceptCodec(req)
	if got != nil {
		t.Errorf("expected nil for wrong type, got %v", got)
	}
}
