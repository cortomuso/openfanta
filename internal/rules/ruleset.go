// Package rules defines rule sets: the bonus and malus values used to compute
// fantavoti. Rule sets are data (YAML validated by rulesets/schema.json), so a
// new league rule set needs no code change.
package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/openfanta/openfanta/internal/domain"
)

// Code identifies a scoring item. Codes double as breakdown codes in results.
type Code string

// Scoring codes. Each one matches a key under `scores` in a rule set file.
const (
	Goal           Code = "goal"
	PenaltyScored  Code = "penalty_scored"
	PenaltyMissed  Code = "penalty_missed"
	PenaltySaved   Code = "penalty_saved"
	OwnGoal        Code = "own_goal"
	Assist         Code = "assist"
	YellowCard     Code = "yellow_card"
	RedCard        Code = "red_card"
	GKGoalConceded Code = "gk_goal_conceded"
	GKCleanSheet   Code = "gk_clean_sheet"
)

// RuleSet is a parsed, validated rule set. Treat it as immutable.
type RuleSet struct {
	ID          string    `json:"id" yaml:"id"`
	Version     string    `json:"version" yaml:"version"`
	Name        string    `json:"name" yaml:"name"`
	Description string    `json:"description,omitempty" yaml:"description"`
	Scores      Scores    `json:"scores" yaml:"scores"`
	Options     Options   `json:"options" yaml:"options"`
	SenzaVoto   SenzaVoto `json:"senza_voto" yaml:"senza_voto"`
}

// Scores holds the value of each scoring item, in hundredths.
type Scores struct {
	Goal           domain.Points `json:"goal" yaml:"goal"`
	PenaltyScored  domain.Points `json:"penalty_scored" yaml:"penalty_scored"`
	PenaltyMissed  domain.Points `json:"penalty_missed" yaml:"penalty_missed"`
	PenaltySaved   domain.Points `json:"penalty_saved" yaml:"penalty_saved"`
	OwnGoal        domain.Points `json:"own_goal" yaml:"own_goal"`
	Assist         domain.Points `json:"assist" yaml:"assist"`
	YellowCard     domain.Points `json:"yellow_card" yaml:"yellow_card"`
	RedCard        domain.Points `json:"red_card" yaml:"red_card"`
	GKGoalConceded domain.Points `json:"gk_goal_conceded" yaml:"gk_goal_conceded"`
	GKCleanSheet   domain.Points `json:"gk_clean_sheet" yaml:"gk_clean_sheet"`
}

// Value returns the configured value for code, and false for an unknown code.
func (s Scores) Value(code Code) (domain.Points, bool) {
	switch code {
	case Goal:
		return s.Goal, true
	case PenaltyScored:
		return s.PenaltyScored, true
	case PenaltyMissed:
		return s.PenaltyMissed, true
	case PenaltySaved:
		return s.PenaltySaved, true
	case OwnGoal:
		return s.OwnGoal, true
	case Assist:
		return s.Assist, true
	case YellowCard:
		return s.YellowCard, true
	case RedCard:
		return s.RedCard, true
	case GKGoalConceded:
		return s.GKGoalConceded, true
	case GKCleanSheet:
		return s.GKCleanSheet, true
	}
	return 0, false
}

// Options are behavioural switches that are not plain point values.
type Options struct {
	// RedReplacesYellow drops yellow card malus for a player who was sent off.
	RedReplacesYellow bool `json:"red_replaces_yellow" yaml:"red_replaces_yellow"`
	// GKCleanSheetMinMinutes is the minimum minutes a goalkeeper must play to earn the clean sheet bonus.
	GKCleanSheetMinMinutes int `json:"gk_clean_sheet_min_minutes" yaml:"gk_clean_sheet_min_minutes"`
}

// SenzaVoto controls players without a voto (s.v.).
type SenzaVoto struct {
	// MinMinutes below which a player is s.v. even if the voto source provided a voto.
	MinMinutes int `json:"min_minutes" yaml:"min_minutes"`
	// VotoUfficio assigns a voto to an s.v. player with a qualifying event; first match wins.
	VotoUfficio []VotoUfficio `json:"voto_ufficio" yaml:"voto_ufficio"`
}

// VotoUfficio is one "s.v. player with event When gets voto Voto" rule.
type VotoUfficio struct {
	When Code          `json:"when" yaml:"when"`
	Voto domain.Points `json:"voto" yaml:"voto"`
}

// Key returns "id@version", the unique identifier of a rule set.
func (r *RuleSet) Key() string { return r.ID + "@" + r.Version }

// Hash returns the sha256 of the rule set's canonical JSON encoding. It ignores
// comments and formatting in the source file, so it changes only when values do.
func (r *RuleSet) Hash() string {
	b, _ := json.Marshal(r.canonical()) // only strings, ints and bools: cannot fail
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// canonical mirrors RuleSet with numeric points, so the hash input is stable
// regardless of how Points is formatted for display.
func (r *RuleSet) canonical() any {
	type vu struct {
		When Code  `json:"when"`
		Voto int64 `json:"voto"`
	}
	scores := map[Code]int64{}
	for _, c := range AllScoreCodes {
		v, _ := r.Scores.Value(c)
		scores[c] = int64(v)
	}
	ufficio := make([]vu, len(r.SenzaVoto.VotoUfficio))
	for i, u := range r.SenzaVoto.VotoUfficio {
		ufficio[i] = vu{u.When, int64(u.Voto)}
	}
	return map[string]any{
		"id": r.ID, "version": r.Version, "name": r.Name, "description": r.Description,
		"scores":  scores,
		"options": r.Options,
		"senza_voto": map[string]any{
			"min_minutes": r.SenzaVoto.MinMinutes, "voto_ufficio": ufficio,
		},
	}
}

// AllScoreCodes lists every scoring code in a stable order.
var AllScoreCodes = []Code{
	Goal, PenaltyScored, PenaltyMissed, PenaltySaved, OwnGoal,
	Assist, YellowCard, RedCard, GKGoalConceded, GKCleanSheet,
}
