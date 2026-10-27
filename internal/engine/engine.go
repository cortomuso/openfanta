// Package engine computes voti and fantavoti from per-player match lines, the
// voti provided by a voto source, and a rule set.
//
// The engine is pure and deterministic: it does no I/O, reads no clock, and
// returns the same output for the same input regardless of input order.
package engine

import (
	"sort"

	d "github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/rules"
)

// Version identifies the engine's scoring semantics. Bump it whenever a change
// can alter results for the same input; it is recorded on every computation run.
const Version = "1"

// BreakdownItem is one bonus or malus applied to a player.
type BreakdownItem struct {
	Code   rules.Code `json:"code"`
	Count  int        `json:"count"`
	Points d.Points   `json:"points"` // Count × the rule set value
}

// Score is the result for one player.
type Score struct {
	Line        d.PlayerLine    `json:"line"`
	Voto        *d.Points       `json:"voto"`      // nil when senza voto
	Fantavoto   *d.Points       `json:"fantavoto"` // nil when senza voto
	SenzaVoto   bool            `json:"senza_voto"`
	VotoUfficio bool            `json:"voto_ufficio"` // Voto was assigned by a senza_voto rule
	Breakdown   []BreakdownItem `json:"breakdown"`    // empty when senza voto
}

// Compute scores every line. votes holds the voto source's voti; a missing
// entry means the source gave no voto. The result is sorted by player ID.
func Compute(lines []d.PlayerLine, votes map[d.PlayerID]d.Points, rs *rules.RuleSet) []Score {
	out := make([]Score, 0, len(lines))
	for _, l := range lines {
		out = append(out, score(l, votes, rs))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Line.Player < out[j].Line.Player })
	return out
}

func score(l d.PlayerLine, votes map[d.PlayerID]d.Points, rs *rules.RuleSet) Score {
	s := Score{Line: l}
	events := eventCounts(l)

	voto, ok := votes[l.Player]
	if ok && l.Minutes > 0 && l.Minutes >= rs.SenzaVoto.MinMinutes {
		s.Voto = &voto
	} else {
		for _, u := range rs.SenzaVoto.VotoUfficio {
			if events[u.When] > 0 {
				v := u.Voto
				s.Voto, s.VotoUfficio = &v, true
				break
			}
		}
	}
	if s.Voto == nil {
		s.SenzaVoto = true
		return s
	}

	counts := events
	if l.SentOff && rs.Options.RedReplacesYellow {
		counts[rules.YellowCard] = 0
	}
	if l.Role == d.RoleGoalkeeper {
		counts[rules.GKGoalConceded] = l.GoalsConceded
		if l.GoalsConceded == 0 && l.Minutes >= rs.Options.GKCleanSheetMinMinutes {
			counts[rules.GKCleanSheet] = 1
		}
	}

	total := *s.Voto
	for _, code := range rules.AllScoreCodes {
		n := counts[code]
		if n == 0 {
			continue
		}
		v, _ := rs.Scores.Value(code)
		p := v * d.Points(n)
		s.Breakdown = append(s.Breakdown, BreakdownItem{Code: code, Count: n, Points: p})
		total += p
	}
	s.Fantavoto = &total
	return s
}

// eventCounts maps what the player did to scoring codes, before any rule set
// option is applied. Voto d'ufficio rules match against these raw counts.
func eventCounts(l d.PlayerLine) map[rules.Code]int {
	c := map[rules.Code]int{
		rules.Goal:          l.Goals,
		rules.PenaltyScored: l.PenaltiesScored,
		rules.PenaltyMissed: l.PenaltiesMissed,
		rules.PenaltySaved:  l.PenaltiesSaved,
		rules.OwnGoal:       l.OwnGoals,
		rules.Assist:        l.Assists,
		rules.YellowCard:    l.YellowCards,
	}
	if l.SentOff {
		c[rules.RedCard] = 1
	}
	return c
}
