// Package voto defines voto sources: where the base voto of each player comes
// from. The engine never calls a source; the caller resolves voti first and
// passes them to the engine as data.
package voto

import (
	"context"

	"github.com/openfanta/openfanta/internal/domain"
)

// Source provides base voti for the players of a match. A player missing from
// the result has no voto (senza voto, unless a rule set assigns one).
type Source interface {
	// Name identifies the source in computation runs, e.g. "provider-rating".
	Name() string
	Votes(ctx context.Context, facts domain.MatchFacts) (map[domain.PlayerID]domain.Points, error)
}

// ProviderRating turns the upstream provider's player rating into a voto by
// rounding to the nearest half point and clamping to [Min, Max].
type ProviderRating struct {
	Min, Max domain.Points
}

// NewProviderRating returns the default provider-rating source: voti between 3 and 10.
func NewProviderRating() ProviderRating { return ProviderRating{Min: 300, Max: 1000} }

// Name implements Source.
func (ProviderRating) Name() string { return "provider-rating" }

// Votes implements Source. Players without a rating get no voto.
func (s ProviderRating) Votes(_ context.Context, facts domain.MatchFacts) (map[domain.PlayerID]domain.Points, error) {
	out := make(map[domain.PlayerID]domain.Points)
	for _, a := range facts.Appearances {
		if a.Rating == nil {
			continue
		}
		out[a.Player] = min(max(RoundHalf(*a.Rating), s.Min), s.Max)
	}
	return out, nil
}

// RoundHalf rounds to the nearest 0.5, with exact quarters rounding up:
// 6.24 -> 6, 6.25 -> 6.5, 6.74 -> 6.5, 6.75 -> 7.
func RoundHalf(p domain.Points) domain.Points {
	const half = domain.Scale / 2
	if p < 0 {
		return -RoundHalf(-p)
	}
	return (p + half/2) / half * half
}
