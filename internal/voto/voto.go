// Package voto defines voto sources: where the base voto of each player comes
// from. The engine never calls a source; the caller resolves voti first and
// passes them to the engine as data.
package voto

import (
	"context"
	"fmt"

	"github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/settings"
	"github.com/openfanta/openfanta/internal/timeline"
)

// Source provides base voti for the players of a match. A player missing from
// the result has no voto (senza voto, unless a rule set assigns one).
type Source interface {
	// Name identifies the source in computation runs, e.g. "provider-rating".
	Name() string
	// Description says in one sentence where the voti come from.
	Description() string
	Votes(ctx context.Context, facts domain.MatchFacts) (map[domain.PlayerID]domain.Points, error)
}

// ProviderRating uses the upstream provider's player rating as the voto,
// unchanged, at two-decimal precision (e.g. 6.38). Rounding to halves or
// whole points is a league setting, never done here. The provider parser is
// the only place where extra digits are dropped.
type ProviderRating struct{}

// NewProviderRating returns the provider-rating source.
func NewProviderRating() ProviderRating { return ProviderRating{} }

// Name implements Source.
func (ProviderRating) Name() string { return settings.SourceProviderRating }

// Description implements Source.
func (ProviderRating) Description() string {
	return "The data provider's own player rating, unchanged."
}

// Votes implements Source. Players without a rating get no voto.
func (ProviderRating) Votes(_ context.Context, facts domain.MatchFacts) (map[domain.PlayerID]domain.Points, error) {
	out := make(map[domain.PlayerID]domain.Points)
	for _, a := range facts.Appearances {
		if a.Rating == nil {
			continue
		}
		out[a.Player] = *a.Rating
	}
	return out, nil
}

// Statistical derives a voto from the match alone, without any rating: every
// player who took the pitch starts from a 6, plus or minus a quarter point
// for their team winning or losing. Goals, cards and the rest are scored by
// the league's event values, so they are not counted twice here. Because it
// needs nothing but the facts, it covers players the provider did not rate.
type Statistical struct{}

// Statistical voto parameters, in hundredths.
const (
	statBase       domain.Points = 600
	statResultStep domain.Points = 25
)

// NewStatistical returns the statistical source.
func NewStatistical() Statistical { return Statistical{} }

// Name implements Source.
func (Statistical) Name() string { return settings.SourceStatistical }

// Description implements Source.
func (Statistical) Description() string {
	return "A 6 for every player who played, plus or minus 0.25 when their team won or lost; needs no provider rating."
}

// Votes implements Source. A player who never took the pitch gets no voto.
func (Statistical) Votes(_ context.Context, facts domain.MatchFacts) (map[domain.PlayerID]domain.Points, error) {
	lines, err := timeline.Derive(facts)
	if err != nil {
		return nil, err
	}
	home, away := score(facts)
	out := make(map[domain.PlayerID]domain.Points, len(lines))
	for _, l := range lines {
		if l.Minutes <= 0 {
			continue
		}
		var diff int
		switch l.Team {
		case facts.HomeTeam:
			diff = home - away
		case facts.AwayTeam:
			diff = away - home
		}
		v := statBase
		switch {
		case diff > 0:
			v += statResultStep
		case diff < 0:
			v -= statResultStep
		}
		out[l.Player] = v
	}
	return out, nil
}

// score counts the goals of each side from the regulation events.
func score(f domain.MatchFacts) (home, away int) {
	credit := func(team domain.TeamID) {
		switch team {
		case f.HomeTeam:
			home++
		case f.AwayTeam:
			away++
		}
	}
	for _, e := range f.Events {
		switch e.Kind {
		case domain.EventGoal, domain.EventPenaltyGoal:
			credit(e.Team)
		case domain.EventOwnGoal: // the event's team is the scorer's; the other side gets the goal
			if e.Team == f.HomeTeam {
				credit(f.AwayTeam)
			} else {
				credit(f.HomeTeam)
			}
		}
	}
	return home, away
}

// Averaged is the mean of the provider rating and the statistical voto, which
// pulls an outlier rating toward the baseline. A player without a provider
// rating gets no voto.
type Averaged struct {
	rating ProviderRating
	stat   Statistical
}

// NewAveraged returns the averaged source.
func NewAveraged() Averaged { return Averaged{} }

// Name implements Source.
func (Averaged) Name() string { return settings.SourceAveraged }

// Description implements Source.
func (Averaged) Description() string {
	return "The mean of the provider rating and the statistical voto; players without a rating have no voto."
}

// Votes implements Source. The mean is rounded half up in hundredths.
func (a Averaged) Votes(ctx context.Context, facts domain.MatchFacts) (map[domain.PlayerID]domain.Points, error) {
	rated, err := a.rating.Votes(ctx, facts)
	if err != nil {
		return nil, err
	}
	stat, err := a.stat.Votes(ctx, facts)
	if err != nil {
		return nil, err
	}
	out := make(map[domain.PlayerID]domain.Points)
	for id, r := range rated {
		if s, ok := stat[id]; ok {
			out[id] = (r + s + 1) / 2
		}
	}
	return out, nil
}

// All returns every source, in the order the API lists them.
func All() []Source {
	return []Source{NewProviderRating(), NewStatistical(), NewAveraged()}
}

// ByName returns the source called name.
func ByName(name string) (Source, error) {
	for _, s := range All() {
		if s.Name() == name {
			return s, nil
		}
	}
	return nil, fmt.Errorf("unknown voto source %q", name)
}
