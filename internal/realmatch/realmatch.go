// Package realmatch is stage 0 of scoring: what happens to the players of a
// real match that did not go the usual way (postponed, suspended), and which
// single voto a player keeps when a matchday gives them more than one.
//
// It is pure. Applying it when a matchday closes, and showing it in the
// explain trace, belong to the matchday lifecycle.
package realmatch

import (
	"errors"
	"fmt"
	"time"

	d "github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/settings"
)

// Action is what to do with the players of a real match.
type Action string

// Actions.
const (
	ActionScore     Action = "score"          // score the match from its own facts, as usual
	ActionWait      Action = "wait"           // nothing yet: the players have no voto from this match
	ActionAutomatic Action = "automatic_voto" // every player involved gets Outcome.Voto
)

// Origin says which rule produced an outcome, for the explain trace.
type Origin string

// Origins.
const (
	OriginRegular  Origin = "regular"           // a normal match: no exception, no override
	OriginLeague   Origin = "league_policy"     // the league's real_match settings
	OriginOverride Origin = "matchday_override" // an admin's override for this matchday
)

// Override is an admin's decision for one fixture of one matchday. It wins
// over the league policy and applies to any fixture, so it also covers a match
// played outside its round window.
type Override struct {
	Policy string // settings.PolicyWait or settings.PolicyAutomaticVoto
	// Voto replaces the league's automatic voto; nil uses the league's.
	Voto *d.Points
}

// Outcome is the decision for one fixture.
type Outcome struct {
	Action    Action
	Voto      d.Points // only for ActionAutomatic
	Origin    Origin
	Exception d.Exception
}

// ErrNoAutomaticVoto is returned when the automatic voto is asked for but
// neither the override nor the league settings give one.
var ErrNoAutomaticVoto = errors.New("no automatic voto configured")

// Decide applies the real-match policy to a fixture with exception e.
func Decide(rm settings.RealMatch, e d.Exception, ov *Override) (Outcome, error) {
	out := Outcome{Exception: e}
	var policy string
	switch {
	case ov != nil:
		policy, out.Origin = ov.Policy, OriginOverride
	case e != d.ExceptionNone:
		policy, out.Origin = rm.PolicyFor(e), OriginLeague
	default:
		out.Action, out.Origin = ActionScore, OriginRegular
		return out, nil
	}

	switch policy {
	case settings.PolicyWait:
		out.Action = ActionWait
	case settings.PolicyAutomaticVoto:
		out.Action = ActionAutomatic
		switch {
		case ov != nil && ov.Voto != nil:
			out.Voto = *ov.Voto
		case rm.AutomaticVoto != nil:
			out.Voto = *rm.AutomaticVoto
		default:
			return Outcome{}, ErrNoAutomaticVoto
		}
	default:
		return Outcome{}, fmt.Errorf("unknown real-match policy %q", policy)
	}
	return out, nil
}

// Candidate is a voto a player holds for a matchday from one fixture.
type Candidate struct {
	Fixture   string
	Kickoff   time.Time // zero when unknown
	Voto      d.Points
	Automatic bool // an automatic voto, not a rating of what the player did
}

// PickVoto returns the one voto a player keeps for a matchday when more than
// one fixture gives them one (a transfer between two clubs that both play in
// the round, a match resumed in a later round). A voto earned on the pitch
// beats an automatic one; among equals the later kickoff wins, since that is
// the club the player ends the round with; a remaining tie goes to the higher
// fixture ID so the choice never depends on input order. ok is false when
// there are no candidates.
func PickVoto(cands []Candidate) (best Candidate, ok bool) {
	for i, c := range cands {
		if i == 0 || better(c, best) {
			best, ok = c, true
		}
	}
	return best, ok
}

func better(a, b Candidate) bool {
	switch {
	case a.Automatic != b.Automatic:
		return !a.Automatic
	case !a.Kickoff.Equal(b.Kickoff):
		return a.Kickoff.After(b.Kickoff)
	default:
		return a.Fixture > b.Fixture
	}
}
