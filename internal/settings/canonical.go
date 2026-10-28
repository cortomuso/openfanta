package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// maxDepth bounds nesting so a hostile document cannot exhaust the stack.
const maxDepth = 32

// maxInt is the largest integer RFC 8785 can represent exactly (2^53 - 1).
const maxInt = 1<<53 - 1

// Canonicalize returns the RFC 8785 (JCS) form of a settings document: object
// keys sorted, no insignificant whitespace, minimal string escapes.
//
// Settings documents hold integers only (points are hundredths), so anything
// that is not an integer is rejected instead of being rounded. Duplicate object
// keys are rejected too, because decoders disagree on which one wins.
func Canonicalize(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	v, err := decodeValue(dec, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: trailing data after the document", ErrInvalid)
	}
	var out bytes.Buffer
	writeValue(&out, v)
	return out.Bytes(), nil
}

func decodeValue(dec *json.Decoder, depth int) (any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("nesting deeper than %d", maxDepth)
	}
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		if t == '{' {
			return decodeObject(dec, depth)
		}
		return decodeArray(dec, depth)
	case json.Number:
		n, err := parseInteger(t)
		if err != nil {
			return nil, err
		}
		return n, nil
	default: // string, bool, nil
		return t, nil
	}
}

func decodeObject(dec *json.Decoder, depth int) (any, error) {
	m := map[string]any{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key := kt.(string) // the decoder only yields strings in key position
		if _, dup := m[key]; dup {
			return nil, fmt.Errorf("duplicate key %q", key)
		}
		if m[key], err = decodeValue(dec, depth+1); err != nil {
			return nil, err
		}
	}
	if _, err := dec.Token(); err != nil { // closing brace
		return nil, err
	}
	return m, nil
}

func decodeArray(dec *json.Decoder, depth int) (any, error) {
	a := []any{}
	for dec.More() {
		v, err := decodeValue(dec, depth+1)
		if err != nil {
			return nil, err
		}
		a = append(a, v)
	}
	if _, err := dec.Token(); err != nil { // closing bracket
		return nil, err
	}
	return a, nil
}

func parseInteger(n json.Number) (int64, error) {
	s := n.String()
	if strings.ContainsAny(s, ".eE") {
		return 0, fmt.Errorf("number %s is not an integer", s)
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil || i > maxInt || i < -maxInt {
		return 0, fmt.Errorf("integer %s out of range", s)
	}
	if s == "-0" {
		return 0, errors.New("negative zero")
	}
	return i, nil
}

func writeValue(b *bytes.Buffer, v any) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		// RFC 8785 sorts by UTF-16 code units, which differs from byte order
		// for characters outside the Basic Multilingual Plane.
		sort.Slice(keys, func(i, j int) bool { return lessUTF16(keys[i], keys[j]) })
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			writeString(b, k)
			b.WriteByte(':')
			writeValue(b, t[k])
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			writeValue(b, e)
		}
		b.WriteByte(']')
	case string:
		writeString(b, t)
	case int64:
		b.WriteString(strconv.FormatInt(t, 10))
	case bool:
		b.WriteString(strconv.FormatBool(t))
	case nil:
		b.WriteString("null")
	}
}

func lessUTF16(a, b string) bool {
	x, y := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(x) && i < len(y); i++ {
		if x[i] != y[i] {
			return x[i] < y[i]
		}
	}
	return len(x) < len(y)
}

func writeString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}
