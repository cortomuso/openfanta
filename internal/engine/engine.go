// Package engine computes voti and fantavoti from per-player match lines, the
// voti provided by a voto source, and the player rules of a settings document.
//
// The engine is pure and deterministic: it does no I/O, reads no clock, and
// returns the same output for the same input regardless of input order.
package engine

import (
	"sort"

	d "github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/settings"
)

// Code identifies a scoring item. Codes double as breakdown codes in results.
type Code string

// Scoring codes. Each one maps to an event id of the settings document (see
// value); the codes themselves are part of the API.
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

// AllScoreCodes lists every scoring code in a stable order.
var AllScoreCodes = []Code{
	Goal, PenaltyScored, PenaltyMissed, PenaltySaved, OwnGoal,
	Assist, YellowCard, RedCard, GKGoalConceded, GKCleanSheet,
}

// value returns what one occurrence of code is worth to a player of the given
// role. Assists use assist.open_play for now: the facts do not say whether an
// assist came from a set piece.
func value(p settings.Player, code Code, role d.Role) d.Points {
	switch code {
	case Goal:
		return p.Points(settings.EventGoal, role)
	case PenaltyScored:
		return p.PenaltyGoalPoints(role)
	case PenaltyMissed:
		return p.Points(settings.EventPenaltyMiss, role)
	case PenaltySaved:
		return p.Points(settings.EventPenaltySaved, role)
	case OwnGoal:
		return p.Points(settings.EventOwnGoal, role)
	case Assist:
		return p.Points(settings.EventAssist, role)
	case YellowCard:
		return p.Points(settings.EventYellow, role)
	case RedCard:
		return p.Points(settings.EventRed, role)
	case GKGoalConceded:
		return p.Points(settings.EventGoalConceded, role)
	case GKCleanSheet:
		return p.Points(settings.EventCleanSheet, role)
	}
	return 0
}

// eventCode maps the event ids a senza_voto rule may list to scoring codes.
var eventCode = map[string]Code{
	settings.EventGoal:         Goal,
	settings.EventPenaltyGoal:  PenaltyScored,
	settings.EventPenaltySaved: PenaltySaved,
	settings.EventPenaltyMiss:  PenaltyMissed,
	settings.EventOwnGoal:      OwnGoal,
	settings.EventAssist:       Assist,
	"assist.set_piece":         Assist,
	settings.EventYellow:       YellowCard,
}

// Version identifies the engine's scoring semantics. Bump it whenever a change
// can alter results for the same input; it is recorded on every computation run.
const Version = "1"

// BreakdownItem is one bonus or malus applied to a player.
type BreakdownItem struct {
	Code   Code     `json:"code"`
	Count  int      `json:"count"`
	Points d.Points `json:"points"` // Count × the settings value
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
func Compute(lines []d.PlayerLine, votes map[d.PlayerID]d.Points, p settings.Player) []Score {
	out := make([]Score, 0, len(lines))
	for _, l := range lines {
		out = append(out, score(l, votes, p))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Line.Player < out[j].Line.Player })
	return out
}

func score(l d.PlayerLine, votes map[d.PlayerID]d.Points, p settings.Player) Score {
	s := Score{Line: l}
	events := eventCounts(l)

	voto, ok := votes[l.Player]
	if ok && l.Minutes > 0 && l.Minutes >= p.SenzaVoto.MinPlayed() {
		s.Voto = &voto
	} else if v, ok := votoUfficio(p.SenzaVoto, events); ok {
		s.Voto, s.VotoUfficio = &v, true
	}
	if s.Voto == nil {
		s.SenzaVoto = true
		return s
	}

	counts := events
	if l.SentOff && p.Cards.RedReplacesYellow {
		counts[YellowCard] = 0
	}
	if l.Role == d.RoleGoalkeeper {
		counts[GKGoalConceded] = l.GoalsConceded
		if l.GoalsConceded == 0 && l.Minutes >= p.CleanSheetMinMinutes() {
			counts[GKCleanSheet] = 1
		}
	}

	total := *s.Voto
	for _, code := range AllScoreCodes {
		n := counts[code]
		if n == 0 {
			continue
		}
		pts := value(p, code, l.Role) * d.Points(n)
		s.Breakdown = append(s.Breakdown, BreakdownItem{Code: code, Count: n, Points: pts})
		total += pts
	}
	s.Fantavoto = &total
	return s
}

// votoUfficio returns the voto an s.v. player gets from their events, if any:
// a red card first, then the first listed event the player has.
func votoUfficio(sv settings.SenzaVoto, events map[Code]int) (d.Points, bool) {
	if sv.WithRed != nil && events[RedCard] > 0 {
		return sv.WithRed.Fixed, true
	}
	if o := sv.WithOtherEvents; o != nil {
		for _, id := range o.Events {
			if events[eventCode[id]] > 0 {
				return o.Base, true
			}
		}
	}
	return 0, false
}

// eventCounts maps what the player did to scoring codes, before any rule set
// option is applied. Voto d'ufficio rules match against these raw counts.
func eventCounts(l d.PlayerLine) map[Code]int {
	c := map[Code]int{
		Goal:          l.Goals,
		PenaltyScored: l.PenaltiesScored,
		PenaltyMissed: l.PenaltiesMissed,
		PenaltySaved:  l.PenaltiesSaved,
		OwnGoal:       l.OwnGoals,
		Assist:        l.Assists,
		YellowCard:    l.YellowCards,
	}
	if l.SentOff {
		c[RedCard] = 1
	}
	return c
}
