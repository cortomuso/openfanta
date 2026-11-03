// Package voto defines voto sources: where the base voto of each player comes
// from. The engine never calls a source; the caller resolves voti first and
// passes them to the engine as data.
package voto

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/settings"
	"github.com/openfanta/openfanta/internal/voto/openstat"
)

// Source provides base voti for the players of a match. A player missing from
// the result has no voto (senza voto, unless a rule set assigns one).
type Source interface {
	// Name identifies the source in computation runs, e.g. "provider-rating".
	Name() string
	// Version identifies the parameters of the source. A run records it, and a
	// change to it makes a new run, so a tuned model never silently replaces
	// the numbers of an old one.
	Version() string
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

// Version implements Source. The source has no parameters.
func (ProviderRating) Version() string { return "1" }

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

// OpenFantaStat is OpenFanta's own statistical rating (see package openstat):
// events, per-player statistics and the team result, weighed by position and
// scaled by minutes played. It never looks at the provider's rating. A player
// the provider sent no statistics for gets no voto.
type OpenFantaStat struct{ model openstat.Model }

// NewOpenFantaStat returns the source with the embedded v1 parameters.
func NewOpenFantaStat() OpenFantaStat { return OpenFantaStat{model: openstat.Default()} }

// Name implements Source.
func (OpenFantaStat) Name() string { return settings.SourceOpenFantaStat }

// Version implements Source: the version of the parameters.
func (o OpenFantaStat) Version() string { return o.model.Version() }

// Description implements Source.
func (OpenFantaStat) Description() string {
	return "OpenFanta's own rating from match events, player statistics and the team result; independent of the provider's rating."
}

// Votes implements Source.
func (o OpenFantaStat) Votes(_ context.Context, facts domain.MatchFacts) (map[domain.PlayerID]domain.Points, error) {
	ratings, err := o.model.Rate(facts)
	if err != nil {
		return nil, err
	}
	out := make(map[domain.PlayerID]domain.Points, len(ratings))
	for id, r := range ratings {
		out[id] = r.Voto
	}
	return out, nil
}

// Average is the mean of its child sources, which pulls an outlier rating
// toward the others. A player missing from any child gets no voto, so the
// league's senza voto rules apply, exactly as with a single source.
type Average struct {
	children []Source
}

// NewAverage returns the mean of children. It needs at least two, and none may
// itself be an average.
func NewAverage(children ...Source) (Average, error) {
	if len(children) < 2 {
		return Average{}, fmt.Errorf("%s needs at least two sources, got %d", settings.SourceAverage, len(children))
	}
	for _, c := range children {
		if c.Name() == settings.SourceAverage {
			return Average{}, fmt.Errorf("%s cannot include itself", settings.SourceAverage)
		}
	}
	return Average{children: slices.Clone(children)}, nil
}

// Name implements Source.
func (Average) Name() string { return settings.SourceAverage }

// Version implements Source. It names the children and their versions, so a
// change to either is a different version.
func (a Average) Version() string {
	parts := make([]string, len(a.children))
	for i, c := range a.children {
		parts[i] = c.Name() + "@" + c.Version()
	}
	return strings.Join(parts, "+") + ";min=all"
}

// Description implements Source.
func (a Average) Description() string {
	names := make([]string, len(a.children))
	for i, c := range a.children {
		names[i] = c.Name()
	}
	return "The mean of " + strings.Join(names, " and ") + "; players missing from any of them have no voto."
}

// Votes implements Source. The mean is rounded half away from zero.
func (a Average) Votes(ctx context.Context, facts domain.MatchFacts) (map[domain.PlayerID]domain.Points, error) {
	sums := map[domain.PlayerID]domain.Points{}
	seen := map[domain.PlayerID]int{}
	for _, c := range a.children {
		votes, err := c.Votes(ctx, facts)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", c.Name(), err)
		}
		for id, v := range votes {
			sums[id] += v
			seen[id]++
		}
	}
	n := domain.Points(len(a.children))
	out := make(map[domain.PlayerID]domain.Points)
	for id, sum := range sums {
		if seen[id] != len(a.children) {
			continue
		}
		out[id] = divRound(sum, n)
	}
	return out, nil
}

// divRound divides rounding half away from zero.
func divRound(a, b domain.Points) domain.Points {
	if a < 0 {
		return -divRound(-a, b)
	}
	return (2*a + b) / (2 * b)
}

// DefaultNames are the sources computed when none are configured.
var DefaultNames = []string{settings.SourceProviderRating, settings.SourceOpenFantaStat, settings.SourceAverage}

// DefaultAverageOf are the children of the average when none are configured.
var DefaultAverageOf = []string{settings.SourceProviderRating, settings.SourceOpenFantaStat}

// Build returns the sources called names, in that order. averageOf names the
// children of the average, if it is among them. An empty list means the default.
func Build(names, averageOf []string) ([]Source, error) {
	if len(names) == 0 {
		names = DefaultNames
	}
	if len(averageOf) == 0 {
		averageOf = DefaultAverageOf
	}
	base := func(name string) (Source, error) {
		switch name {
		case settings.SourceProviderRating:
			return NewProviderRating(), nil
		case settings.SourceOpenFantaStat:
			return NewOpenFantaStat(), nil
		case settings.SourceAverage:
			return nil, fmt.Errorf("%s cannot be a child of %s", name, settings.SourceAverage)
		}
		return nil, fmt.Errorf("unknown voto source %q", name)
	}
	var out []Source
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			return nil, fmt.Errorf("voto source %q listed twice", name)
		}
		seen[name] = true
		if name != settings.SourceAverage {
			s, err := base(name)
			if err != nil {
				return nil, err
			}
			out = append(out, s)
			continue
		}
		children := make([]Source, len(averageOf))
		for i, c := range averageOf {
			s, err := base(c)
			if err != nil {
				return nil, fmt.Errorf("average of: %w", err)
			}
			children[i] = s
		}
		avg, err := NewAverage(children...)
		if err != nil {
			return nil, err
		}
		out = append(out, avg)
	}
	return out, nil
}

// Default returns the default sources.
func Default() []Source {
	out, err := Build(nil, nil)
	if err != nil {
		panic(err) // the defaults are covered by tests
	}
	return out
}
