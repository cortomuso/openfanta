package settings

import (
	"fmt"

	d "github.com/openfanta/openfanta/internal/domain"
)

// Real-match policies (stage 0): what a league does with a match that was
// postponed or suspended.
const (
	PolicyWait          = "wait"           // no voto until the match is played; the player has none at close
	PolicyAutomaticVoto = "automatic_voto" // everyone involved gets the automatic voto
)

// minorWithRealMatch is the first schema minor that has the `real_match` section.
const minorWithRealMatch = 3

// RealMatch is stage 0 of the scoring pipeline. The zero value is the Classic
// behaviour: wait for the real voti.
type RealMatch struct {
	OnPostponed string `json:"on_postponed"`
	OnSuspended string `json:"on_suspended"`
	// AutomaticVoto is the voto handed out by PolicyAutomaticVoto, whether set
	// here or by an admin's per-matchday override.
	AutomaticVoto *d.Points `json:"automatic_voto"`
}

// PolicyFor is the league's policy for a match with exception e. A match
// without one has no policy: it is scored when played.
func (r RealMatch) PolicyFor(e d.Exception) string {
	var p string
	switch e {
	case d.ExceptionPostponed:
		p = r.OnPostponed
	case d.ExceptionSuspended:
		p = r.OnSuspended
	}
	if p == "" {
		return PolicyWait
	}
	return p
}

func (doc *Document) validateRealMatch() error {
	r := doc.RealMatch
	if r == (RealMatch{}) {
		return nil
	}
	if minor := schemaMinor(doc.Schema); minor < minorWithRealMatch {
		return fmt.Errorf("real_match needs schema openfanta.settings/1.%d or later, not %s", minorWithRealMatch, doc.Schema)
	}
	if (r.OnPostponed == PolicyAutomaticVoto || r.OnSuspended == PolicyAutomaticVoto) && r.AutomaticVoto == nil {
		return fmt.Errorf("real_match: policy %s needs automatic_voto", PolicyAutomaticVoto)
	}
	return nil
}
