// Package openstat is OpenFanta's own statistical rating: a voto computed from
// the events of a match and the per-player statistics a provider sends, with
// no input from anyone else's rating.
//
// For every player who took part:
//
//	delta = Σ weight[position][term] × count   (events, stats, team result)
//	delta = delta × m / (m + shrink_minutes)    m = minutes played
//	delta = curve(delta)
//	voto  = clamp(base + delta, min, max)
//
// Everything is integer hundredths and every division rounds half away from
// zero, so the same facts always give the same voti. Events (goals, assists,
// cards, minutes, penalties) come from timeline.Derive; stats only from
// Appearance.Stats, which never repeats an event.
package openstat

import (
	"fmt"
	"sort"

	d "github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/timeline"
)

// Item is one term of a rating: how many of it, and the points it added
// before the minutes shrink and the curve.
type Item struct {
	Term   string   `json:"term"`
	Count  int64    `json:"count"`
	Points d.Points `json:"points"`
}

// Rating is the voto of one player and how it came about.
type Rating struct {
	Voto  d.Points `json:"voto"`
	Items []Item   `json:"items"`
	// Delta is the sum of the items before scaling; Scaled is after the minutes
	// shrink and the curve, i.e. what was added to the base.
	Delta  d.Points `json:"delta"`
	Scaled d.Points `json:"scaled"`
}

// Model rates players under one set of Params.
type Model struct{ p Params }

// New returns a Model for p, which must have passed LoadParams.
func New(p Params) Model { return Model{p: p} }

// Default returns the model with the embedded v1 parameters.
func Default() Model {
	p, err := LoadParams(paramsV1)
	if err != nil {
		panic(err) // the embedded document is covered by tests
	}
	return New(p)
}

// Version identifies the parameters.
func (m Model) Version() string { return m.p.Version }

// Rate rates every player who played and has a statistics block. A player the
// provider sent no statistics for is unknown, not average, so gets no entry.
func (m Model) Rate(facts d.MatchFacts) (map[d.PlayerID]Rating, error) {
	lines, err := timeline.Derive(facts)
	if err != nil {
		return nil, err
	}
	app := make(map[d.PlayerID]d.Appearance, len(facts.Appearances))
	for _, a := range facts.Appearances {
		app[a.Player] = a
	}
	home, away := Score(facts)
	out := make(map[d.PlayerID]Rating, len(lines))
	for _, l := range lines {
		a := app[l.Player]
		if l.Minutes <= 0 || !a.HasStats {
			continue
		}
		diff := home - away
		if l.Team == facts.AwayTeam {
			diff = -diff
		}
		out[l.Player] = m.rate(l, a, diff)
	}
	return out, nil
}

// Score counts the goals of each side from the regulation events. An own goal
// is credited to the other side.
func Score(f d.MatchFacts) (home, away int) {
	credit := func(team d.TeamID) {
		switch team {
		case f.HomeTeam:
			home++
		case f.AwayTeam:
			away++
		}
	}
	for _, e := range f.Events {
		switch e.Kind {
		case d.EventGoal, d.EventPenaltyGoal:
			credit(e.Team)
		case d.EventOwnGoal: // the event's team is the scorer's
			if e.Team == f.HomeTeam {
				credit(f.AwayTeam)
			} else {
				credit(f.HomeTeam)
			}
		}
	}
	return home, away
}

// positionOf picks the weight column: the provider's position, else goalkeeper
// for a keeper and midfielder for everyone else.
func positionOf(a d.Appearance, role d.Role) string {
	switch a.Position {
	case d.PositionGoalkeeper, d.PositionDefender, d.PositionMidfielder, d.PositionForward:
		return string(a.Position)
	}
	if role == d.RoleGoalkeeper {
		return string(d.PositionGoalkeeper)
	}
	return string(d.PositionMidfielder)
}

func (m Model) rate(l d.PlayerLine, a d.Appearance, goalDiff int) Rating {
	pos := positionOf(a, l.Role)
	counts := counts(l, a, m.p.CleanSheetMinMinutes)

	var r Rating
	add := func(term string, count int64, points int64) {
		if count == 0 || points == 0 {
			return
		}
		r.Items = append(r.Items, Item{Term: term, Count: count, Points: d.Points(points)})
		r.Delta += d.Points(points)
	}
	for _, term := range terms {
		add(term, counts[term], counts[term]*m.p.Weights[term][pos])
	}
	switch {
	case goalDiff > 0:
		add("result", 1, m.p.Context.Win)
	case goalDiff < 0:
		add("result", 1, m.p.Context.Loss)
	}
	if g, ok := m.p.Context.GoalDifference[pos]; ok {
		add("goal_difference", int64(goalDiff), clamp(g.PerGoal*int64(goalDiff), -g.Cap, g.Cap))
	}

	minutes := int64(l.Minutes)
	scaled := divRound(int64(r.Delta)*minutes, minutes+m.p.ShrinkMinutes)
	r.Scaled = d.Points(m.curve(scaled))
	r.Voto = d.Points(clamp(int64(m.p.Base)+int64(r.Scaled), int64(m.p.Min), int64(m.p.Max)))
	return r
}

// counts gathers the count of every term for one player. Events come from the
// timeline; the rest from the statistics block. A stat the provider omitted
// is 0 (the caller checked HasStats). Derived counts are floored at 0.
func counts(l d.PlayerLine, a d.Appearance, csMinutes int) map[string]int64 {
	stat := func(k d.StatKey) int64 { return a.Stats[k] }
	diff := func(total, part d.StatKey) int64 { return max(stat(total)-stat(part), 0) }

	keyPasses := stat(d.StatKeyPasses)
	if keyPasses == 0 { // the same idea under another name; never summed
		keyPasses = stat(d.StatChancesCreated)
	}
	// Unlike the fantacalcio clean-sheet bonus, the term applies to every
	// position: the team conceded nothing while the player was on the pitch.
	var cleanSheet int64
	if l.GoalsConceded == 0 && l.Minutes >= csMinutes {
		cleanSheet = 1
	}
	var sentOff int64
	if l.SentOff {
		sentOff = 1
	}
	return map[string]int64{
		"goal":              int64(l.Goals + l.PenaltiesScored),
		"assist":            int64(l.Assists),
		"penalty_missed":    int64(l.PenaltiesMissed),
		"own_goal":          int64(l.OwnGoals),
		"yellow":            int64(l.YellowCards),
		"sent_off":          sentOff,
		"penalty_saved":     int64(l.PenaltiesSaved),
		"goal_conceded":     int64(l.GoalsConceded),
		"clean_sheet":       cleanSheet,
		"key_pass":          keyPasses,
		"shot_on_target":    stat(d.StatShotsOnTarget),
		"shot_off_target":   stat(d.StatShotsOffTarget),
		"accurate_pass":     stat(d.StatAccuratePasses),
		"inaccurate_pass":   diff(d.StatPasses, d.StatAccuratePasses),
		"dribble_won":       stat(d.StatDribblesWon),
		"failed_dribble":    diff(d.StatDribbles, d.StatDribblesWon),
		"dispossessed":      stat(d.StatDispossessed),
		"duel_won":          stat(d.StatDuelsWon),
		"duel_lost":         diff(d.StatDuels, d.StatDuelsWon),
		"aerial_won":        stat(d.StatAerialsWon),
		"aerial_lost":       stat(d.StatAerialsLost),
		"tackle_won":        stat(d.StatTacklesWon),
		"interception":      stat(d.StatInterceptions),
		"clearance":         stat(d.StatClearances),
		"blocked_shot":      stat(d.StatBlockedShots),
		"ball_recovery":     stat(d.StatBallRecoveries),
		"dribbled_past":     stat(d.StatDribbledPast),
		"foul":              stat(d.StatFouls),
		"foul_drawn":        stat(d.StatFoulsDrawn),
		"penalty_won":       stat(d.StatPenaltiesWon),
		"penalty_committed": stat(d.StatPenaltiesCommitted),
		"error_to_shot":     stat(d.StatErrorsToShot),
		"error_to_goal":     stat(d.StatErrorsToGoal),
		"save":              stat(d.StatSaves),
		"save_inside_box":   stat(d.StatSavesInsideBox),
		"punch":             stat(d.StatPunches),
	}
}

// curve maps x through the piecewise-linear knots, clamping outside them.
func (m Model) curve(x int64) int64 {
	k := m.p.Curve
	if x <= k[0][0] {
		return k[0][1]
	}
	last := k[len(k)-1]
	if x >= last[0] {
		return last[1]
	}
	i := sort.Search(len(k), func(i int) bool { return k[i][0] > x }) // first knot right of x
	a, b := k[i-1], k[i]
	return a[1] + divRound((x-a[0])*(b[1]-a[1]), b[0]-a[0])
}

func clamp(v, lo, hi int64) int64 { return min(max(v, lo), hi) }

// divRound divides rounding half away from zero. b must be positive.
func divRound(a, b int64) int64 {
	if b <= 0 {
		panic(fmt.Sprintf("divRound by %d", b))
	}
	if a < 0 {
		return -divRound(-a, b)
	}
	return (2*a + b) / (2 * b)
}
