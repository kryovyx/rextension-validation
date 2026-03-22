package validation

import (
	rxroute "github.com/kryovyx/rextension/route"
)

// NewTestMiddlewareConfig exposes MiddlewareConfig construction for tests.
func NewTestMiddlewareConfig(codecs []Codec, strict bool, validateResp bool) MiddlewareConfig {
	idx := newRouteIndex()
	reg := newCodecRegistry(codecs)
	v := newRouteValidator()
	return MiddlewareConfig{RouteIndex: idx, Registry: reg, Validator: v, Strict: strict, ValidateResp: validateResp}
}

// RegisterTestRoute registers a route in the middleware config's route index.
func RegisterTestRoute(cfg *MiddlewareConfig, rt rxroute.Route) {
	cfg.RouteIndex.register(rt)
}

// ExportNewCodecRegistry exposes codecRegistry for tests.
func ExportNewCodecRegistry(codecs []Codec) *codecRegistry {
	return newCodecRegistry(codecs)
}

// TestFindByContentType exposes codecRegistry.findByContentType for tests.
func (r *codecRegistry) TestFindByContentType(ct string) Codec {
	return r.findByContentType(ct)
}

// TestNegotiate exposes codecRegistry.negotiate for tests.
func (r *codecRegistry) TestNegotiate(accept string) Codec {
	return r.negotiate(accept)
}

// ExportFormatValidationErrors exposes formatValidationErrors for tests.
func ExportFormatValidationErrors(err error) []ValidationError {
	return formatValidationErrors(err)
}

// ExportHasBody exposes hasBody for tests.
func ExportHasBody(method string) bool {
	return hasBody(method)
}

// ExportCopyHeaders exposes copyHeaders for tests.
func ExportCopyHeaders(dst, src map[string][]string) {
	copyHeaders(dst, src)
}

// ExportParseFloat exposes parseFloat for tests.
func ExportParseFloat(s string) (float64, int, error) {
	var out float64
	n, err := parseFloat(s, &out)
	return out, n, err
}

// ExportParseFloatBytes exposes parseFloatBytes for tests.
func ExportParseFloatBytes(b []byte) (float64, int, error) {
	var out float64
	n, err := parseFloatBytes(b, &out)
	return out, n, err
}
