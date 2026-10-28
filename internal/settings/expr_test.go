package settings

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

type goldenFile struct {
	Engine string                        `json:"engine"`
	Lineup map[string][]map[string]int64 `json:"lineup"`
	Cases  []struct {
		Name          string `json:"name"`
		CEL           string `json:"cel"`
		Want          int64  `json:"want"`
		WantRuleError string `json:"wantRuleError"`
	} `json:"cases"`
}

// TestExprGolden pins the results of the expression environment. Run it on
// every cel-go upgrade; a failure means existing documents would score
// differently.
func TestExprGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/cel/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g goldenFile
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	for _, c := range g.Cases {
		t.Run(c.Name, func(t *testing.T) {
			e, err := CompileExpr(g.Engine, c.CEL)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := e.Eval(map[string]any{"lineup": g.Lineup})
			if c.WantRuleError != "" {
				re, ok := IsRuleError(err)
				if !ok || re.Code != c.WantRuleError {
					t.Fatalf("got %d, %v; want rule error %s", got, err, c.WantRuleError)
				}
				return
			}
			if err != nil || got != c.Want {
				t.Fatalf("got %d, %v; want %d", got, err, c.Want)
			}
		})
	}
}

func TestCompileExprRejects(t *testing.T) {
	// 20 macro expansions stay under the source cap but over the node cap.
	manyNodes := "(" + strings.TrimSuffix(strings.Repeat("lineup.a.all(p,true)&&", 20), "&&") + ") ? 1 : 0"
	deep := strings.Repeat("(", MaxExprDepth+8) + "1" + strings.Repeat(")", MaxExprDepth+8)
	// Nested comprehensions multiply: this estimates far above the cost cap.
	costly := "lineup.starters.all(a, lineup.starters.all(b, lineup.starters.all(c, lineup.starters.all(d, a.voto_puro + b.voto_puro + c.voto_puro + d.voto_puro > 0)))) ? 1 : 0"
	cases := []struct{ name, src, reason string }{
		{"empty", ``, ""},
		{"syntax error", `1 +`, ""},
		{"undeclared variable", `roster.size()`, "undeclared reference"},
		{"unknown function", `nope(1)`, "undeclared reference"},
		{"bool result", `lineup.starters.size() > 1`, "must produce int"},
		{"string result", `"x"`, "must produce int"},
		{"list result", `[1, 2]`, "must produce int"},
		{"double result", `1.5`, "must produce int"},
		{"double inside an int expression", `int(1.5 * 2.0)`, "must not use double"},
		{"double function", `int(double(3) / 2.0)`, "must not use double"},
		{"source too long", strings.Repeat(" ", MaxExprSource+1) + `1`, "the limit is 512"},
		{"too many nodes", manyNodes, "expression count exceeds limit"},
		{"too deeply nested", deep, "recursion limit exceeded"},
		{"estimated cost too high", costly, "over the limit"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := CompileExpr("2026.10", c.src)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("got %v, want ErrInvalid", err)
			}
			if !strings.Contains(err.Error(), c.reason) {
				t.Errorf("got %q, want it to mention %q", err, c.reason)
			}
		})
	}
	if _, err := CompileExpr("1999.01", "1"); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown engine: got %v", err)
	}
}

func TestExprRuntimeBudget(t *testing.T) {
	// Cheap enough to pass the write-time cap on its hints, but expensive when
	// the caller supplies more data than the hints promise.
	e, err := CompileExpr("2026.10", `lineup.a.all(p, lineup.a.all(q, p.v + q.v >= 0)) ? 1 : 0`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	big := make([]map[string]int64, 400)
	for i := range big {
		big[i] = map[string]int64{"v": 1}
	}
	_, err = e.Eval(map[string]any{"lineup": map[string][]map[string]int64{"a": big}})
	if re, ok := IsRuleError(err); !ok || re.Code != RuleErrCostLimit {
		t.Fatalf("got %v, want rule error %s", err, RuleErrCostLimit)
	}
	small := map[string][]map[string]int64{"a": {{"v": 1}, {"v": 2}}}
	if got, err := e.Eval(map[string]any{"lineup": small}); err != nil || got != 1 {
		t.Errorf("small input: %d, %v", got, err)
	}
}

func TestRuleErrorIsDeterministic(t *testing.T) {
	e, err := CompileExpr("2026.10", `lineup.starters[0].voto_puro`)
	if err != nil {
		t.Fatal(err)
	}
	var first string
	for i := 0; i < 3; i++ {
		_, err := e.Eval(map[string]any{"lineup": map[string][]map[string]int64{}})
		re, ok := IsRuleError(err)
		if !ok {
			t.Fatalf("got %v, want a rule error", err)
		}
		if i == 0 {
			first = re.Error()
		} else if re.Error() != first {
			t.Errorf("rule error changed: %q vs %q", re.Error(), first)
		}
	}
}

func TestModulesInDocument(t *testing.T) {
	module := `"team": [{"id": "tutti_sufficienti", "type": "expr.cel/1", "stage": "team",
		"cel": "lineup.starters.all(p, p.voto_puro >= 600) ? 200 : 0", "description": "+2 se tutti sufficienti"}],`
	doc := func(edit func(string) string) []byte {
		s := strings.Replace(string(classicJSON), `"game"`, module+` "game"`, 1)
		s = strings.Replace(s, `settings/1.0`, `settings/1.1`, 1)
		s = strings.Replace(s, `["scoring.penalty_mode"]`, `["scoring.penalty_mode", "expr.cel/1"]`, 1)
		return []byte(edit(s))
	}
	id := func(s string) string { return s }

	s, err := Parse(doc(id))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Doc.Team) != 1 || s.Doc.Team[0].ID != "tutti_sufficienti" {
		t.Errorf("team = %+v", s.Doc.Team)
	}

	rejects := map[string]func(string) string{
		"not listed in requires": func(s string) string {
			return strings.Replace(s, `["scoring.penalty_mode", "expr.cel/1"]`, `["scoring.penalty_mode"]`, 1)
		},
		"schema 1.0": func(s string) string { return strings.Replace(s, `settings/1.1`, `settings/1.0`, 1) },
		"bad expression": func(s string) string {
			return strings.Replace(s, `lineup.starters.all(p, p.voto_puro >= 600) ? 200 : 0`, `lineup.starters.size() > 1`, 1)
		},
		"duplicate id": func(s string) string {
			return strings.Replace(s, `"team": [{`, `"team": [{"id": "tutti_sufficienti", "type": "expr.cel/1", "cel": "1"}, {`, 1)
		},
		"unknown module type": func(s string) string { return strings.Replace(s, `"expr.cel/1", "stage"`, `"band/1", "stage"`, 1) },
		"unknown key in module": func(s string) string {
			return strings.Replace(s, `"stage": "team",`, `"stage": "team", "extra": 1,`, 1)
		},
	}
	for name, edit := range rejects {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(doc(edit)); !errors.Is(err, ErrInvalid) {
				t.Fatalf("got %v, want ErrInvalid", err)
			}
		})
	}
}
