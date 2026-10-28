package settings

import (
	"errors"
	"strings"
	"testing"
)

func TestCanonicalize(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"sorts keys, drops whitespace", "{ \"b\": 1,\n \"a\": [ 2 , 1 ] }", `{"a":[2,1],"b":1}`},
		{"nested", `{"z":{"b":true,"a":null},"a":"x"}`, `{"a":"x","z":{"a":null,"b":true}}`},
		{"escapes", `{"k":"a\"b\\c\n\u0001\u00e9<>&"}`, `{"k":"a\"b\\c\n\u0001é<>&"}`},
		{"utf16 key order", `{"\ue000":1,"\ud83d\ude00":2}`, "{\"\U0001F600\":2,\"\ue000\":1}"},
		{"negative and zero", `[-5,0]`, `[-5,0]`},
		{"empty containers", `{"a":{},"b":[]}`, `{"a":{},"b":[]}`},
	}
	for _, c := range cases {
		got, err := Canonicalize([]byte(c.in))
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if string(got) != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
		again, err := Canonicalize(got)
		if err != nil || string(again) != string(got) {
			t.Errorf("%s: not idempotent: %s (%v)", c.name, again, err)
		}
	}
}

func TestCanonicalizeRejects(t *testing.T) {
	deep := strings.Repeat("[", maxDepth+2) + strings.Repeat("]", maxDepth+2)
	for name, in := range map[string]string{
		"float":         `{"a":1.5}`,
		"fraction zero": `{"a":1.0}`,
		"exponent":      `{"a":1e2}`,
		"negative zero": `{"a":-0}`,
		"too large":     `{"a":9007199254740992}`,
		"duplicate key": `{"a":1,"a":2}`,
		"nested dup":    `{"o":{"a":1,"a":2}}`,
		"trailing":      `{} []`,
		"truncated":     `{"a":`,
		"empty":         ``,
		"too deep":      deep,
	} {
		if _, err := Canonicalize([]byte(in)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got %v, want ErrInvalid", name, err)
		}
	}
}

func TestCanonicalizeKeepsLargestSafeInteger(t *testing.T) {
	got, err := Canonicalize([]byte(`[9007199254740991,-9007199254740991]`))
	if err != nil || string(got) != `[9007199254740991,-9007199254740991]` {
		t.Fatalf("got %s, %v", got, err)
	}
}

func TestCanonicalizeEscapesControlCharacters(t *testing.T) {
	got, err := Canonicalize([]byte(`["\b\t\n\f\r\u001f"]`))
	if err != nil {
		t.Fatal(err)
	}
	if want := `["\b\t\n\f\r\u001f"]`; string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestCanonicalizeRejectsMalformed(t *testing.T) {
	for _, in := range []string{`{1:2}`, `{"a":1`, `[1,`, `[1`, `{"a"`} {
		if _, err := Canonicalize([]byte(in)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: got %v, want ErrInvalid", in, err)
		}
	}
}
