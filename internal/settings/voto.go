package settings

import (
	"fmt"
	"slices"

	d "github.com/openfanta/openfanta/internal/domain"
)

// Voto source names a document may select. The voto package implements them;
// its tests check that the two lists agree.
const (
	SourceProviderRating = "provider-rating"
	SourceOpenFantaStat  = "openfanta-stat"
	SourceAverage        = "average"
)

// VotoSources lists the voto source names this build accepts.
var VotoSources = []string{SourceProviderRating, SourceOpenFantaStat, SourceAverage}

// Rounding modes of a base voto.
const (
	RoundNone    = "none"    // keep the source's voto as is
	RoundNearest = "nearest" // to the nearest multiple of step, halves away from zero
	RoundUp      = "up"      // to the next multiple of step
	RoundDown    = "down"    // to the previous multiple of step
)

// minorWithVoto is the first schema minor that has the `voto` section.
const minorWithVoto = 2

// Voto says where a league's base voti come from and how they are rounded.
// The zero value is the Classic behaviour: provider rating, unrounded.
type Voto struct {
	Source   string   `json:"source"`
	Rounding Rounding `json:"rounding"`
}

// Rounding rounds a base voto to a multiple of Step hundredths (50 rounds to
// halves, 100 to whole points). Only the source's voto is rounded; a voto
// d'ufficio and the event points are exact values already.
type Rounding struct {
	Mode string   `json:"mode"`
	Step d.Points `json:"step"`
}

// SourceName is the selected voto source, provider-rating when none is set.
func (v Voto) SourceName() string {
	if v.Source == "" {
		return SourceProviderRating
	}
	return v.Source
}

// Apply rounds p. It is exact integer arithmetic, so equal inputs give equal
// results on every platform.
func (r Rounding) Apply(p d.Points) d.Points {
	if r.Mode == "" || r.Mode == RoundNone || r.Step <= 1 {
		return p
	}
	rem := p % r.Step
	if rem < 0 {
		rem += r.Step // floor remainder, so negatives round consistently
	}
	down := p - rem
	switch r.Mode {
	case RoundDown:
		return down
	case RoundUp:
		if rem == 0 {
			return down
		}
		return down + r.Step
	default: // nearest, halves away from zero
		if 2*rem > r.Step || (2*rem == r.Step && p > 0) {
			return down + r.Step
		}
		return down
	}
}

// ApplyAll returns a copy of votes with every voto rounded.
func (r Rounding) ApplyAll(votes map[d.PlayerID]d.Points) map[d.PlayerID]d.Points {
	out := make(map[d.PlayerID]d.Points, len(votes))
	for id, v := range votes {
		out[id] = r.Apply(v)
	}
	return out
}

func (doc *Document) validateVoto() error {
	v := doc.Voto
	if v == (Voto{}) {
		return nil
	}
	if minor := schemaMinor(doc.Schema); minor < minorWithVoto {
		return fmt.Errorf("voto needs schema openfanta.settings/1.%d or later, not %s", minorWithVoto, doc.Schema)
	}
	if v.Source != "" && !slices.Contains(VotoSources, v.Source) {
		return fmt.Errorf("voto.source %q is not supported (supported: %v)", v.Source, VotoSources)
	}
	r := v.Rounding
	switch {
	case r.Mode == RoundNone && r.Step != 0:
		return fmt.Errorf("voto.rounding: mode %s takes no step", RoundNone)
	case r.Mode != "" && r.Mode != RoundNone && r.Step < 1:
		return fmt.Errorf("voto.rounding: mode %s needs a step", r.Mode)
	case r.Mode == "" && r.Step != 0:
		return fmt.Errorf("voto.rounding: step needs a mode")
	}
	return nil
}
