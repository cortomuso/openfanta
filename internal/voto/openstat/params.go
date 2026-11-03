package openstat

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"

	d "github.com/openfanta/openfanta/internal/domain"
)

//go:embed params_v1.json
var paramsV1 []byte

// Params are the numbers of the model. Every value is an integer in
// hundredths of a point (per unit of the term). The weights are engineering
// priors, not published constants; changing any of them means a new Version.
type Params struct {
	Version string `json:"version"`
	// Base is the voto of a player with nothing to their name; Min and Max
	// bound the result.
	Base d.Points `json:"base"`
	Min  d.Points `json:"min"`
	Max  d.Points `json:"max"`
	// ShrinkMinutes pulls a short appearance toward Base: the sum of terms is
	// scaled by m/(m+ShrinkMinutes) for m minutes played.
	ShrinkMinutes int64 `json:"shrink_minutes"`
	// CleanSheetMinMinutes is the time on the pitch that earns the clean sheet term.
	CleanSheetMinMinutes int `json:"clean_sheet_min_minutes"`
	// Curve is a piecewise-linear map applied to the scaled sum; knots are
	// [x, y] pairs with strictly increasing x. The identity in v1.
	Curve   [][2]int64                  `json:"curve"`
	Context Context                     `json:"context"`
	Weights map[string]map[string]int64 `json:"weights"` // term -> position -> hundredths
}

// Context holds the terms that depend on the team's result.
type Context struct {
	Win            int64               `json:"win"`
	Loss           int64               `json:"loss"`
	GoalDifference map[string]GoalDiff `json:"goal_difference"` // position -> rule
}

// GoalDiff is PerGoal points per goal of final difference, capped at ±Cap.
type GoalDiff struct {
	PerGoal int64 `json:"per_goal"`
	Cap     int64 `json:"cap"`
}

var positions = []string{"gk", "def", "mid", "fwd"}

// LoadParams decodes and validates a params document. Unknown keys, unknown
// terms and unknown positions are errors, so a typo cannot silently drop a weight.
func LoadParams(raw []byte) (Params, error) {
	var p Params
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Params{}, fmt.Errorf("openstat params: %w", err)
	}
	if err := p.validate(); err != nil {
		return Params{}, fmt.Errorf("openstat params %q: %w", p.Version, err)
	}
	return p, nil
}

func (p Params) validate() error {
	if p.Version == "" {
		return fmt.Errorf("no version")
	}
	if p.Min > p.Base || p.Base > p.Max {
		return fmt.Errorf("need min <= base <= max, got %d, %d, %d", p.Min, p.Base, p.Max)
	}
	if p.ShrinkMinutes < 0 {
		return fmt.Errorf("negative shrink_minutes")
	}
	if len(p.Curve) < 2 {
		return fmt.Errorf("curve needs at least two knots")
	}
	for i := 1; i < len(p.Curve); i++ {
		if p.Curve[i][0] <= p.Curve[i-1][0] {
			return fmt.Errorf("curve knots must have increasing x")
		}
	}
	known := map[string]bool{}
	for _, t := range terms {
		known[t] = true
	}
	for term, byPos := range p.Weights {
		if !known[term] {
			return fmt.Errorf("unknown term %q", term)
		}
		for pos := range byPos {
			if !validPosition(pos) {
				return fmt.Errorf("term %q: unknown position %q", term, pos)
			}
		}
	}
	for pos, g := range p.Context.GoalDifference {
		if !validPosition(pos) {
			return fmt.Errorf("goal_difference: unknown position %q", pos)
		}
		if g.Cap < 0 {
			return fmt.Errorf("goal_difference %s: negative cap", pos)
		}
	}
	return nil
}

func validPosition(s string) bool {
	for _, p := range positions {
		if p == s {
			return true
		}
	}
	return false
}

// terms lists every term a weight may name, in the order they are summed and
// reported, so a breakdown reads naturally.
var terms = []string{
	"goal", "assist", "penalty_missed", "own_goal", "yellow", "sent_off", "penalty_saved",
	"goal_conceded", "clean_sheet",
	"key_pass", "shot_on_target", "shot_off_target", "accurate_pass", "inaccurate_pass",
	"dribble_won", "failed_dribble", "dispossessed", "duel_won", "duel_lost",
	"aerial_won", "aerial_lost", "tackle_won", "interception", "clearance", "blocked_shot",
	"ball_recovery", "dribbled_past", "foul", "foul_drawn", "penalty_won", "penalty_committed",
	"error_to_shot", "error_to_goal", "save", "save_inside_box", "punch",
}
