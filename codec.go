// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

// Package validation provides a Rex extension for request/response body
// validation, content negotiation, and pluggable codec support.
//
// This file defines the Codec interface and the built-in JSON codec.
package validation

import (
	"encoding/json"
	"strings"
)

// Codec encodes and decodes values for a specific content type.
// Register additional codecs via Config.Codecs or WithCodec() to support
// content types beyond JSON (e.g., XML, YAML, MessagePack).
type Codec interface {
	// ContentType returns the MIME type this codec handles (e.g., "application/json").
	ContentType() string
	// Marshal serializes v into bytes.
	Marshal(v interface{}) ([]byte, error)
	// Unmarshal deserializes data into v.
	Unmarshal(data []byte, v interface{}) error
}

// JSONCodec is the built-in codec for application/json.
type JSONCodec struct{}

// ContentType returns "application/json".
func (JSONCodec) ContentType() string { return "application/json" }

// Marshal encodes v as JSON.
func (JSONCodec) Marshal(v interface{}) ([]byte, error) { return json.Marshal(v) }

// Unmarshal decodes JSON data into v.
func (JSONCodec) Unmarshal(data []byte, v interface{}) error { return json.Unmarshal(data, v) }

// codecRegistry holds the ordered list of codecs and provides lookup methods.
type codecRegistry struct {
	codecs []Codec
	// byType maps lowercased content type → codec for O(1) lookup.
	byType map[string]Codec
}

// newCodecRegistry builds a registry from the given codecs.
func newCodecRegistry(codecs []Codec) *codecRegistry {
	r := &codecRegistry{
		codecs: codecs,
		byType: make(map[string]Codec, len(codecs)),
	}
	for _, c := range codecs {
		r.byType[strings.ToLower(c.ContentType())] = c
	}
	return r
}

// findByContentType returns the codec matching the content type, or nil.
// The content type string may include parameters (e.g., "application/json; charset=utf-8").
func (r *codecRegistry) findByContentType(ct string) Codec {
	// Strip parameters.
	if idx := strings.IndexByte(ct, ';'); idx >= 0 {
		ct = ct[:idx]
	}
	ct = strings.TrimSpace(strings.ToLower(ct))
	return r.byType[ct]
}

// negotiate attempts to match the Accept header to a registered codec.
// It supports quality values (q=...) and returns the best match.
// If no match is found, it returns nil.
func (r *codecRegistry) negotiate(accept string) Codec {
	if accept == "" || accept == "*/*" {
		// Default to first registered codec.
		if len(r.codecs) > 0 {
			return r.codecs[0]
		}
		return nil
	}

	type acceptEntry struct {
		mediaType string
		quality   float64
	}

	var entries []acceptEntry
	for _, part := range strings.Split(accept, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		mt := part
		q := 1.0
		if idx := strings.Index(part, ";"); idx >= 0 {
			mt = strings.TrimSpace(part[:idx])
			params := part[idx+1:]
			for _, p := range strings.Split(params, ";") {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(p, "q=") {
					var parsed float64
					if _, err := parseFloat(p[2:], &parsed); err == nil {
						q = parsed
					}
				}
			}
		}
		entries = append(entries, acceptEntry{mediaType: strings.ToLower(strings.TrimSpace(mt)), quality: q})
	}

	// Sort by quality descending (simple selection, typically short lists).
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].quality > entries[i].quality {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	for _, e := range entries {
		if e.mediaType == "*/*" {
			if len(r.codecs) > 0 {
				return r.codecs[0]
			}
			continue
		}
		// Support type/* wildcards.
		if strings.HasSuffix(e.mediaType, "/*") {
			prefix := e.mediaType[:len(e.mediaType)-1]
			for _, c := range r.codecs {
				if strings.HasPrefix(strings.ToLower(c.ContentType()), prefix) {
					return c
				}
			}
			continue
		}
		if c := r.byType[e.mediaType]; c != nil {
			return c
		}
	}

	return nil
}

// parseFloat is a minimal helper to parse a float64 from a string.
func parseFloat(s string, out *float64) (int, error) {
	var v float64
	n, err := parseFloatBytes([]byte(s), &v)
	if err != nil {
		return 0, err
	}
	*out = v
	return n, nil
}

// parseFloatBytes parses a float64 from a byte slice (trivial implementation).
func parseFloatBytes(b []byte, out *float64) (int, error) {
	s := strings.TrimSpace(string(b))
	var v float64
	var consumed int
	for i, c := range s {
		if (c >= '0' && c <= '9') || c == '.' {
			consumed = i + 1
		} else {
			break
		}
	}
	if consumed == 0 {
		return 0, &json.InvalidUnmarshalError{}
	}
	// Parse the number manually.
	whole := true
	var intPart, fracPart float64
	var fracDiv float64 = 1
	for _, c := range s[:consumed] {
		if c == '.' {
			whole = false
			continue
		}
		d := float64(c - '0')
		if whole {
			intPart = intPart*10 + d
		} else {
			fracPart = fracPart*10 + d
			fracDiv *= 10
		}
	}
	v = intPart + fracPart/fracDiv
	*out = v
	return consumed, nil
}
