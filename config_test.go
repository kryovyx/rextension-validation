package validation_test

import (
	"testing"

	validation "github.com/kryovyx/rextension-validation"
)

func TestNewDefaultConfig(t *testing.T) {
	cfg := validation.NewDefaultConfig()

	if cfg == nil {
		t.Fatal("NewDefaultConfig returned nil")
	}
	if len(cfg.Codecs) != 1 {
		t.Fatalf("expected 1 default codec, got %d", len(cfg.Codecs))
	}
	if cfg.Codecs[0].ContentType() != "application/json" {
		t.Errorf("expected default codec content type application/json, got %s", cfg.Codecs[0].ContentType())
	}
	if cfg.StrictResponses {
		t.Error("expected StrictResponses false by default")
	}
	if !cfg.ValidateResponses {
		t.Error("expected ValidateResponses true by default")
	}
}

func TestWithCodec(t *testing.T) {
	cfg := validation.NewDefaultConfig()

	dummy := &dummyCodec{contentType: "text/xml"}
	opt := validation.WithCodec(dummy)
	opt(cfg)

	if len(cfg.Codecs) != 2 {
		t.Fatalf("expected 2 codecs after WithCodec, got %d", len(cfg.Codecs))
	}
	if cfg.Codecs[1].ContentType() != "text/xml" {
		t.Errorf("expected appended codec content type text/xml, got %s", cfg.Codecs[1].ContentType())
	}
}

func TestWithStrictResponses_True(t *testing.T) {
	cfg := validation.NewDefaultConfig()

	opt := validation.WithStrictResponses(true)
	opt(cfg)

	if !cfg.StrictResponses {
		t.Error("expected StrictResponses true after WithStrictResponses(true)")
	}
}

func TestWithStrictResponses_False(t *testing.T) {
	cfg := &validation.Config{StrictResponses: true}

	opt := validation.WithStrictResponses(false)
	opt(cfg)

	if cfg.StrictResponses {
		t.Error("expected StrictResponses false after WithStrictResponses(false)")
	}
}

func TestWithValidateResponses_True(t *testing.T) {
	cfg := &validation.Config{ValidateResponses: false}

	opt := validation.WithValidateResponses(true)
	opt(cfg)

	if !cfg.ValidateResponses {
		t.Error("expected ValidateResponses true after WithValidateResponses(true)")
	}
}

func TestWithValidateResponses_False(t *testing.T) {
	cfg := validation.NewDefaultConfig()

	opt := validation.WithValidateResponses(false)
	opt(cfg)

	if cfg.ValidateResponses {
		t.Error("expected ValidateResponses false after WithValidateResponses(false)")
	}
}

func TestNewConfig_NoOptions(t *testing.T) {
	cfg := validation.NewConfig()

	if cfg == nil {
		t.Fatal("NewConfig returned nil")
	}
	if len(cfg.Codecs) != 1 {
		t.Fatalf("expected 1 default codec, got %d", len(cfg.Codecs))
	}
	if cfg.StrictResponses {
		t.Error("expected StrictResponses false by default")
	}
	if !cfg.ValidateResponses {
		t.Error("expected ValidateResponses true by default")
	}
}

func TestNewConfig_WithOptions(t *testing.T) {
	dummy := &dummyCodec{contentType: "text/xml"}
	cfg := validation.NewConfig(
		validation.WithCodec(dummy),
		validation.WithStrictResponses(true),
		validation.WithValidateResponses(false),
	)

	if len(cfg.Codecs) != 2 {
		t.Fatalf("expected 2 codecs, got %d", len(cfg.Codecs))
	}
	if cfg.Codecs[1].ContentType() != "text/xml" {
		t.Errorf("expected second codec text/xml, got %s", cfg.Codecs[1].ContentType())
	}
	if !cfg.StrictResponses {
		t.Error("expected StrictResponses true")
	}
	if cfg.ValidateResponses {
		t.Error("expected ValidateResponses false")
	}
}

// dummyCodec is a test helper codec.
type dummyCodec struct {
	contentType string
}

func (d *dummyCodec) ContentType() string                        { return d.contentType }
func (d *dummyCodec) Marshal(v interface{}) ([]byte, error)      { return nil, nil }
func (d *dummyCodec) Unmarshal(data []byte, v interface{}) error { return nil }
