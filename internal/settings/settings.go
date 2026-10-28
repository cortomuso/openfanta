// Package settings defines the league settings document: the single, versioned
// description of how a league scores. A document is validated against the
// embedded JSON Schema, stored as canonical JSON and identified by its sha256.
//
// The package is pure: no I/O, no clock. The schema is in schema/v1.json and
// the Classic default is classic.json.
package settings

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	d "github.com/openfanta/openfanta/internal/domain"
)

// ErrInvalid wraps every validation failure returned by this package.
var ErrInvalid = errors.New("invalid settings document")

// Engines lists the engine versions this build can execute. A document pins
// one of them; an old version stays here while a season still uses it.
var Engines = []string{"2026.10"}

// Features lists the `requires` identifiers this build supports. A document
// that requires anything else is rejected instead of being misread.
var Features = []string{"scoring.penalty_mode"}

//go:embed schema/v1.json
var schemaJSON []byte

//go:embed classic.json
var classicJSON []byte

// compiledSchema compiles the embedded schema once. It ships inside the binary
// and is exercised by tests, so a failure here is a programming error.
var compiledSchema = sync.OnceValue(func() *jsonschema.Schema {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		panic(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("v1.json", doc); err != nil {
		panic(err)
	}
	return c.MustCompile("v1.json")
})

// Settings is a validated document. Canonical is the exact stored form; the
// hash and the typed view are derived from it. Treat it as immutable.
type Settings struct {
	Canonical []byte
	SHA256    string
	Doc       Document
}

// Document is the typed view of a settings document. Fields the engine ignores
// (`meta` and `x-*` keys) are not decoded here: they live only in Canonical,
// byte for byte as submitted.
type Document struct {
	Schema   string   `json:"schema"`
	Engine   string   `json:"engine"`
	Requires []string `json:"requires"`
	Game     string   `json:"game"`
	Roles    *Roles   `json:"roles"`
	Player   Player   `json:"player"`
}

// Roles selects the role catalog and groups its roles.
type Roles struct {
	Catalog string              `json:"catalog"`
	Groups  map[string][]string `json:"groups"`
}

// Parse validates raw and returns the settings it describes. It checks, in
// order: canonical form (integers only, no duplicate keys), the JSON Schema,
// then the rules a schema cannot express (supported engine, supported
// `requires`, mode and value consistency).
func Parse(raw []byte) (*Settings, error) {
	canon, err := Canonicalize(raw)
	if err != nil {
		return nil, err
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(canon))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := compiledSchema().Validate(inst); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	var doc Document
	if err := decodeStrict(withoutOpaque(canon), &doc); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := doc.validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return &Settings{Canonical: canon, SHA256: d.SHA256Hex(canon), Doc: doc}, nil
}

// Classic returns the built-in Classic default.
func Classic() *Settings {
	s, err := Parse(classicJSON)
	if err != nil {
		panic(err) // the embedded document is covered by tests
	}
	return s
}

// withoutOpaque drops the top-level `meta` and `x-*` keys, which the typed
// decoder must not see because it rejects unknown fields.
func withoutOpaque(canon []byte) []byte {
	var top map[string]json.RawMessage
	_ = json.Unmarshal(canon, &top) // canon already parsed as an object
	for k := range top {
		if k == "meta" || strings.HasPrefix(k, "x-") {
			delete(top, k)
		}
	}
	out, _ := json.Marshal(top)
	return out
}

func decodeStrict(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func (doc *Document) validate() error {
	if !slices.Contains(Engines, doc.Engine) {
		return fmt.Errorf("engine %q is not supported (supported: %s)", doc.Engine, strings.Join(Engines, ", "))
	}
	for _, f := range doc.Requires {
		if !slices.Contains(Features, f) {
			return fmt.Errorf("requires %q, which this server does not support", f)
		}
	}
	return doc.Player.validate()
}
