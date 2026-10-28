package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Change operations.
const (
	OpAdd    = "add"
	OpRemove = "remove"
	OpChange = "change"
)

// Change is one difference between two documents. Path is a JSON Pointer
// (RFC 6901). Old and New hold canonical JSON, and are nil where the value
// does not exist (Old for add, New for remove).
type Change struct {
	Path string          `json:"path"`
	Op   string          `json:"op"`
	Old  json.RawMessage `json:"old,omitempty"`
	New  json.RawMessage `json:"new,omitempty"`
}

// Diff returns the differences between two canonical documents, ordered by
// path so the result is deterministic. Arrays are compared as a whole, since
// their order carries meaning (for example substitution priority). The change
// log is derived, never stored as the source of truth.
func Diff(prev, next []byte) ([]Change, error) {
	a, err := decodeAny(prev)
	if err != nil {
		return nil, fmt.Errorf("previous document: %w", err)
	}
	b, err := decodeAny(next)
	if err != nil {
		return nil, fmt.Errorf("next document: %w", err)
	}
	var out []Change
	diffValue("", a, b, &out)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func decodeAny(b []byte) (any, error) {
	canon, err := Canonicalize(b)
	if err != nil {
		return nil, err
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(canon))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func diffValue(path string, a, b any, out *[]Change) {
	am, aok := a.(map[string]any)
	bm, bok := b.(map[string]any)
	if aok && bok {
		for k, av := range am {
			p := path + "/" + escapePointer(k)
			if bv, ok := bm[k]; ok {
				diffValue(p, av, bv, out)
			} else {
				*out = append(*out, Change{Path: p, Op: OpRemove, Old: encode(av)})
			}
		}
		for k, bv := range bm {
			if _, ok := am[k]; !ok {
				*out = append(*out, Change{Path: path + "/" + escapePointer(k), Op: OpAdd, New: encode(bv)})
			}
		}
		return
	}
	if x, y := encode(a), encode(b); !bytes.Equal(x, y) {
		*out = append(*out, Change{Path: path, Op: OpChange, Old: x, New: y})
	}
}

func encode(v any) json.RawMessage {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v) // v came from json decoding: cannot fail
	canon, _ := Canonicalize(bytes.TrimSpace(buf.Bytes()))
	return canon
}

func escapePointer(k string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(k)
}
