package validation_test

import (
	"encoding/json"
	"testing"

	validation "github.com/kryovyx/rextension-validation"
)

// --- JSONCodec ---

func TestJSONCodec_ContentType(t *testing.T) {
	c := validation.JSONCodec{}
	if ct := c.ContentType(); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
}

func TestJSONCodec_Marshal_Success(t *testing.T) {
	c := validation.JSONCodec{}
	type sample struct {
		Name string `json:"name"`
	}
	data, err := c.Marshal(sample{Name: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"name":"test"}` {
		t.Errorf("unexpected marshal result: %s", data)
	}
}

func TestJSONCodec_Marshal_Failure(t *testing.T) {
	c := validation.JSONCodec{}
	// Channels cannot be marshalled to JSON.
	_, err := c.Marshal(make(chan int))
	if err == nil {
		t.Error("expected error marshalling a channel")
	}
}

func TestJSONCodec_Unmarshal_Success(t *testing.T) {
	c := validation.JSONCodec{}
	type sample struct {
		Name string `json:"name"`
	}
	var s sample
	err := c.Unmarshal([]byte(`{"name":"hello"}`), &s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "hello" {
		t.Errorf("expected name hello, got %s", s.Name)
	}
}

func TestJSONCodec_Unmarshal_Failure(t *testing.T) {
	c := validation.JSONCodec{}
	type sample struct {
		Name string `json:"name"`
	}
	var s sample
	err := c.Unmarshal([]byte(`{invalid`), &s)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- codecRegistry findByContentType ---

func TestCodecRegistry_FindByContentType_ExactMatch(t *testing.T) {
	reg := validation.ExportNewCodecRegistry([]validation.Codec{validation.JSONCodec{}})
	c := reg.TestFindByContentType("application/json")
	if c == nil {
		t.Fatal("expected to find codec for application/json")
	}
	if c.ContentType() != "application/json" {
		t.Errorf("expected application/json, got %s", c.ContentType())
	}
}

func TestCodecRegistry_FindByContentType_WithParams(t *testing.T) {
	reg := validation.ExportNewCodecRegistry([]validation.Codec{validation.JSONCodec{}})
	c := reg.TestFindByContentType("application/json; charset=utf-8")
	if c == nil {
		t.Fatal("expected to find codec for application/json; charset=utf-8")
	}
	if c.ContentType() != "application/json" {
		t.Errorf("expected application/json, got %s", c.ContentType())
	}
}

func TestCodecRegistry_FindByContentType_NoMatch(t *testing.T) {
	reg := validation.ExportNewCodecRegistry([]validation.Codec{validation.JSONCodec{}})
	c := reg.TestFindByContentType("text/xml")
	if c != nil {
		t.Errorf("expected nil for unregistered content type, got %v", c)
	}
}

func TestCodecRegistry_FindByContentType_CaseInsensitive(t *testing.T) {
	reg := validation.ExportNewCodecRegistry([]validation.Codec{validation.JSONCodec{}})
	c := reg.TestFindByContentType("Application/JSON")
	if c == nil {
		t.Fatal("expected to find codec with case-insensitive lookup")
	}
}

// --- codecRegistry negotiate ---

func TestCodecRegistry_Negotiate_EmptyAccept(t *testing.T) {
	reg := validation.ExportNewCodecRegistry([]validation.Codec{validation.JSONCodec{}})
	c := reg.TestNegotiate("")
	if c == nil {
		t.Fatal("expected first codec for empty accept")
	}
	if c.ContentType() != "application/json" {
		t.Errorf("expected application/json, got %s", c.ContentType())
	}
}

func TestCodecRegistry_Negotiate_Wildcard(t *testing.T) {
	reg := validation.ExportNewCodecRegistry([]validation.Codec{validation.JSONCodec{}})
	c := reg.TestNegotiate("*/*")
	if c == nil {
		t.Fatal("expected first codec for */*")
	}
	if c.ContentType() != "application/json" {
		t.Errorf("expected application/json, got %s", c.ContentType())
	}
}

func TestCodecRegistry_Negotiate_ExactMatch(t *testing.T) {
	xml := &xmlCodec{}
	reg := validation.ExportNewCodecRegistry([]validation.Codec{validation.JSONCodec{}, xml})
	c := reg.TestNegotiate("text/xml")
	if c == nil {
		t.Fatal("expected to find xml codec")
	}
	if c.ContentType() != "text/xml" {
		t.Errorf("expected text/xml, got %s", c.ContentType())
	}
}

func TestCodecRegistry_Negotiate_QualityValues(t *testing.T) {
	xml := &xmlCodec{}
	reg := validation.ExportNewCodecRegistry([]validation.Codec{validation.JSONCodec{}, xml})
	// JSON has lower quality, XML has higher.
	c := reg.TestNegotiate("application/json;q=0.5, text/xml;q=1.0")
	if c == nil {
		t.Fatal("expected to find codec")
	}
	if c.ContentType() != "text/xml" {
		t.Errorf("expected text/xml (higher quality), got %s", c.ContentType())
	}
}

func TestCodecRegistry_Negotiate_QualityValues_JSONHigher(t *testing.T) {
	xml := &xmlCodec{}
	reg := validation.ExportNewCodecRegistry([]validation.Codec{validation.JSONCodec{}, xml})
	c := reg.TestNegotiate("application/json;q=1.0, text/xml;q=0.5")
	if c == nil {
		t.Fatal("expected to find codec")
	}
	if c.ContentType() != "application/json" {
		t.Errorf("expected application/json (higher quality), got %s", c.ContentType())
	}
}

func TestCodecRegistry_Negotiate_TypeWildcard(t *testing.T) {
	reg := validation.ExportNewCodecRegistry([]validation.Codec{validation.JSONCodec{}})
	c := reg.TestNegotiate("application/*")
	if c == nil {
		t.Fatal("expected to match application/* wildcard")
	}
	if c.ContentType() != "application/json" {
		t.Errorf("expected application/json, got %s", c.ContentType())
	}
}

func TestCodecRegistry_Negotiate_TypeWildcard_NoMatch(t *testing.T) {
	reg := validation.ExportNewCodecRegistry([]validation.Codec{validation.JSONCodec{}})
	c := reg.TestNegotiate("text/*")
	if c != nil {
		t.Errorf("expected nil for text/* wildcard with no text codecs, got %v", c)
	}
}

func TestCodecRegistry_Negotiate_NoMatch(t *testing.T) {
	reg := validation.ExportNewCodecRegistry([]validation.Codec{validation.JSONCodec{}})
	c := reg.TestNegotiate("text/xml")
	if c != nil {
		t.Errorf("expected nil for unregistered type, got %v", c)
	}
}

func TestCodecRegistry_Negotiate_MultipleEntries_SortedByQuality(t *testing.T) {
	xml := &xmlCodec{}
	reg := validation.ExportNewCodecRegistry([]validation.Codec{validation.JSONCodec{}, xml})
	// Three entries with different qualities; text/xml is highest.
	c := reg.TestNegotiate("application/json;q=0.8, text/xml;q=0.9, */*;q=0.1")
	if c == nil {
		t.Fatal("expected to find codec")
	}
	if c.ContentType() != "text/xml" {
		t.Errorf("expected text/xml, got %s", c.ContentType())
	}
}

func TestCodecRegistry_Negotiate_EmptyRegistry_EmptyAccept(t *testing.T) {
	reg := validation.ExportNewCodecRegistry([]validation.Codec{})
	c := reg.TestNegotiate("")
	if c != nil {
		t.Errorf("expected nil for empty registry, got %v", c)
	}
}

func TestCodecRegistry_Negotiate_WildcardInEntries(t *testing.T) {
	reg := validation.ExportNewCodecRegistry([]validation.Codec{validation.JSONCodec{}})
	// Wildcard in the entries list among other types.
	c := reg.TestNegotiate("text/html, */*;q=0.5")
	if c == nil {
		t.Fatal("expected first codec for fallback */*")
	}
	if c.ContentType() != "application/json" {
		t.Errorf("expected application/json, got %s", c.ContentType())
	}
}

// --- parseFloat / parseFloatBytes ---

func TestParseFloat_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"1.0", 1.0},
		{"0.5", 0.5},
		{"0.9", 0.9},
		{"0", 0.0},
		{"1", 1.0},
	}
	for _, tc := range tests {
		v, _, err := validation.ExportParseFloat(tc.input)
		if err != nil {
			t.Errorf("parseFloat(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if v != tc.expected {
			t.Errorf("parseFloat(%q): expected %f, got %f", tc.input, tc.expected, v)
		}
	}
}

func TestParseFloat_Invalid(t *testing.T) {
	_, _, err := validation.ExportParseFloat("abc")
	if err == nil {
		t.Error("expected error for non-numeric string")
	}
}

func TestParseFloatBytes_Valid(t *testing.T) {
	v, n, err := validation.ExportParseFloatBytes([]byte("0.75"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 0.75 {
		t.Errorf("expected 0.75, got %f", v)
	}
	if n != 4 {
		t.Errorf("expected consumed 4 bytes, got %d", n)
	}
}

func TestParseFloatBytes_Empty(t *testing.T) {
	_, _, err := validation.ExportParseFloatBytes([]byte(""))
	if err == nil {
		t.Error("expected error for empty input")
	}
}

// xmlCodec is a test helper for a second codec.
type xmlCodec struct{}

func (x *xmlCodec) ContentType() string { return "text/xml" }
func (x *xmlCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v) // just reuse JSON for testing
}
func (x *xmlCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
