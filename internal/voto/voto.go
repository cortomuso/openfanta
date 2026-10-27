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

// ProviderRating uses the upstream provider's player rating as the voto,
// unchanged, at two-decimal precision (e.g. 6.38). Rounding to halves or
// whole points is a league setting, never done here. The provider parser is
// the only place where extra digits are dropped.
type ProviderRating struct{}

// NewProviderRating returns the provider-rating source.
func NewProviderRating() ProviderRating { return ProviderRating{} }

// Name implements Source.
func (ProviderRating) Name() string { return "provider-rating" }

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
