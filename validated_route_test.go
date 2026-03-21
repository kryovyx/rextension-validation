package validation_test

import (
	"testing"

	validation "github.com/kryovyx/rextension-validation"
)

// --- SchemaKind constants ---

func TestSchemaKind_Values(t *testing.T) {
	if validation.SchemaScalar != 0 {
		t.Errorf("expected SchemaScalar=0, got %d", validation.SchemaScalar)
	}
	if validation.SchemaOneOf != 1 {
		t.Errorf("expected SchemaOneOf=1, got %d", validation.SchemaOneOf)
	}
	if validation.SchemaAnyOf != 2 {
		t.Errorf("expected SchemaAnyOf=2, got %d", validation.SchemaAnyOf)
	}
	if validation.SchemaAllOf != 3 {
		t.Errorf("expected SchemaAllOf=3, got %d", validation.SchemaAllOf)
	}
}

// --- SchemaKind.String() ---

func TestSchemaKind_String_Scalar(t *testing.T) {
	if s := validation.SchemaScalar.String(); s != "scalar" {
		t.Errorf("expected 'scalar', got %q", s)
	}
}

func TestSchemaKind_String_OneOf(t *testing.T) {
	if s := validation.SchemaOneOf.String(); s != "oneOf" {
		t.Errorf("expected 'oneOf', got %q", s)
	}
}

func TestSchemaKind_String_AnyOf(t *testing.T) {
	if s := validation.SchemaAnyOf.String(); s != "anyOf" {
		t.Errorf("expected 'anyOf', got %q", s)
	}
}

func TestSchemaKind_String_AllOf(t *testing.T) {
	if s := validation.SchemaAllOf.String(); s != "allOf" {
		t.Errorf("expected 'allOf', got %q", s)
	}
}

func TestSchemaKind_String_Unknown(t *testing.T) {
	unknown := validation.SchemaKind(99)
	if s := unknown.String(); s != "unknown" {
		t.Errorf("expected 'unknown', got %q", s)
	}
}

// --- Scalar ---

func TestScalar(t *testing.T) {
	type Foo struct{ Name string }
	schema := validation.Scalar(Foo{})

	if schema.Kind() != validation.SchemaScalar {
		t.Errorf("expected SchemaScalar, got %v", schema.Kind())
	}
	types := schema.Types()
	if len(types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(types))
	}
	if _, ok := types[0].(Foo); !ok {
		t.Errorf("expected Foo type, got %T", types[0])
	}
}

// --- OneOf ---

func TestOneOf(t *testing.T) {
	type A struct{ X int }
	type B struct{ Y int }
	schema := validation.OneOf(A{}, B{})

	if schema.Kind() != validation.SchemaOneOf {
		t.Errorf("expected SchemaOneOf, got %v", schema.Kind())
	}
	types := schema.Types()
	if len(types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(types))
	}
	if _, ok := types[0].(A); !ok {
		t.Errorf("expected A type at [0], got %T", types[0])
	}
	if _, ok := types[1].(B); !ok {
		t.Errorf("expected B type at [1], got %T", types[1])
	}
}

// --- AnyOf ---

func TestAnyOf(t *testing.T) {
	type A struct{ X int }
	type B struct{ Y int }
	type C struct{ Z int }
	schema := validation.AnyOf(A{}, B{}, C{})

	if schema.Kind() != validation.SchemaAnyOf {
		t.Errorf("expected SchemaAnyOf, got %v", schema.Kind())
	}
	types := schema.Types()
	if len(types) != 3 {
		t.Fatalf("expected 3 types, got %d", len(types))
	}
}

// --- AllOf ---

func TestAllOf(t *testing.T) {
	type A struct{ X int }
	type B struct{ Y int }
	schema := validation.AllOf(A{}, B{})

	if schema.Kind() != validation.SchemaAllOf {
		t.Errorf("expected SchemaAllOf, got %v", schema.Kind())
	}
	types := schema.Types()
	if len(types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(types))
	}
}
