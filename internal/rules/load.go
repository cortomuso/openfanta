package rules

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"

	"github.com/openfanta/openfanta/rulesets"
)

// ErrInvalid wraps every validation failure returned by Parse.
var ErrInvalid = errors.New("invalid rule set")

// compiledSchema compiles the embedded schema once. The schema ships inside the
// binary and is exercised by tests, so a failure here is a programming error.
var compiledSchema = sync.OnceValue(func() *jsonschema.Schema {
	raw, err := rulesets.FS.ReadFile("schema.json")
	if err != nil {
		panic(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		panic(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", doc); err != nil {
		panic(err)
	}
	return c.MustCompile("schema.json")
})

// Parse decodes and validates a rule set from YAML (JSON is valid YAML too).
func Parse(src []byte) (*RuleSet, error) {
	var generic any
	if err := yaml.Unmarshal(src, &generic); err != nil {
		return nil, fmt.Errorf("%w: yaml: %w", ErrInvalid, err)
	}
	// Round-trip through JSON so the schema validator sees JSON-native types.
	// yaml.v3 decodes mappings to map[string]any, which always marshals.
	asJSON, _ := json.Marshal(generic)
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(asJSON))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := compiledSchema().Validate(doc); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	var rs RuleSet
	dec := json.NewDecoder(bytes.NewReader(asJSON))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rs); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return &rs, nil
}

// Registry holds rule sets by key ("id@version").
type Registry struct {
	byKey map[string]*RuleSet
}

// Get returns the rule set with the given id and version.
func (r *Registry) Get(id, version string) (*RuleSet, bool) {
	rs, ok := r.byKey[id+"@"+version]
	return rs, ok
}

// Latest returns the highest version of the rule set with the given id.
func (r *Registry) Latest(id string) (*RuleSet, bool) {
	var best *RuleSet
	for _, rs := range r.byKey {
		if rs.ID == id && (best == nil || compareSemver(rs.Version, best.Version) > 0) {
			best = rs
		}
	}
	return best, best != nil
}

// All returns every rule set ordered by id, then version.
func (r *Registry) All() []*RuleSet {
	out := make([]*RuleSet, 0, len(r.byKey))
	for _, rs := range r.byKey {
		out = append(out, rs)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return compareSemver(out[i].Version, out[j].Version) < 0
	})
	return out
}

// LoadFS parses every *.yaml / *.yml file at the root of fsys. Duplicate keys are an error.
func LoadFS(fsys fs.FS) (*Registry, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	reg := &Registry{byKey: map[string]*RuleSet{}}
	for _, e := range entries {
		ext := path.Ext(e.Name())
		if e.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		src, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, err
		}
		rs, err := Parse(src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if _, dup := reg.byKey[rs.Key()]; dup {
			return nil, fmt.Errorf("%s: %w: duplicate rule set %s", e.Name(), ErrInvalid, rs.Key())
		}
		reg.byKey[rs.Key()] = rs
	}
	return reg, nil
}

// Merge combines registries. The same id@version in two registries is an error.
func Merge(regs ...*Registry) (*Registry, error) {
	out := &Registry{byKey: map[string]*RuleSet{}}
	for _, r := range regs {
		for k, rs := range r.byKey {
			if _, dup := out.byKey[k]; dup {
				return nil, fmt.Errorf("%w: duplicate rule set %s", ErrInvalid, k)
			}
			out.byKey[k] = rs
		}
	}
	return out, nil
}

// LatestOfEach returns the highest version of every rule set family, ordered by id.
func (r *Registry) LatestOfEach() []*RuleSet {
	var out []*RuleSet
	for _, rs := range r.All() {
		if n := len(out); n > 0 && out[n-1].ID == rs.ID {
			out[n-1] = rs // All is sorted by version within an id
			continue
		}
		out = append(out, rs)
	}
	return out
}

// LoadBuiltin loads the rule sets embedded in the binary.
func LoadBuiltin() (*Registry, error) { return LoadFS(rulesets.FS) }

// compareSemver compares two schema-validated MAJOR.MINOR.PATCH strings.
func compareSemver(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := range 3 {
		if len(pa[i]) != len(pb[i]) {
			return len(pa[i]) - len(pb[i])
		}
		if c := strings.Compare(pa[i], pb[i]); c != 0 {
			return c
		}
	}
	return 0
}
