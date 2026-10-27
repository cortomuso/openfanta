// Package timeline turns MatchFacts into per-player lines: minutes played,
// goals, cards, and goals conceded while on the pitch. It is pure: no I/O, no
// clock, and the output depends only on the input (never on slice order).
package timeline

import (
	"errors"
	"fmt"
	"sort"

	d "github.com/openfanta/openfanta/internal/domain"
)

// ErrInvalidFacts wraps every inconsistency found in the input.
var ErrInvalidFacts = errors.New("invalid match facts")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidFacts, fmt.Sprintf(format, args...))
}

// stint is a continuous spell on the pitch. Seq bounds are exclusive on entry
// and on exit: a player who comes on at seq 5 is on the pitch for events with
// seq > 5, and a player who goes off at seq 9 is on for events with seq < 9.
type stint struct {
	onSeq, offSeq       int // offSeq == noExit while still on the pitch
	onMinute, offMinute int
}

const (
	beforeKickoff = -1 << 62
	noExit        = 1 << 62
)

type player struct {
	app    d.Appearance
	line   d.PlayerLine
	stint  *stint // nil if never on the pitch
	played bool
}

func (p *player) onPitchAt(seq int) bool {
	return p.stint != nil && p.stint.onSeq < seq && seq < p.stint.offSeq
}

// Derive computes one PlayerLine for every player who appeared in the match,
// ordered by team (home first) then player ID.
func Derive(f d.MatchFacts) ([]d.PlayerLine, error) {
	if f.HomeTeam == f.AwayTeam {
		return nil, invalid("home and away team are both %q", f.HomeTeam)
	}
	if f.Length <= 0 {
		return nil, invalid("match length %d", f.Length)
	}
	opponent := map[d.TeamID]d.TeamID{f.HomeTeam: f.AwayTeam, f.AwayTeam: f.HomeTeam}

	players := make(map[d.PlayerID]*player, len(f.Appearances))
	for _, a := range f.Appearances {
		if _, ok := opponent[a.Team]; !ok {
			return nil, invalid("player %q has unknown team %q", a.Player, a.Team)
		}
		if a.Role != d.RoleGoalkeeper && a.Role != d.RoleOutfield {
			return nil, invalid("player %q has unknown role %q", a.Player, a.Role)
		}
		if _, dup := players[a.Player]; dup {
			return nil, invalid("player %q listed twice", a.Player)
		}
		p := &player{app: a, line: d.PlayerLine{Player: a.Player, Team: a.Team, Role: a.Role}}
		if a.Starter {
			p.stint = &stint{onSeq: beforeKickoff, offSeq: noExit, onMinute: 0}
			p.played = true
		}
		players[a.Player] = p
	}

	events := append([]d.Event(nil), f.Events...)
	sort.Slice(events, func(i, j int) bool { return events[i].Seq < events[j].Seq })
	for i, e := range events {
		if i > 0 && events[i-1].Seq == e.Seq {
			return nil, invalid("duplicate event seq %d", e.Seq)
		}
		if err := apply(e, players, opponent, min(e.Minute, f.Length)); err != nil {
			return nil, fmt.Errorf("event seq %d (%s): %w", e.Seq, e.Kind, err)
		}
	}

	lines := make([]d.PlayerLine, 0, len(players))
	for _, p := range players {
		if !p.played {
			continue
		}
		off := f.Length
		if p.stint.offSeq != noExit {
			off = p.stint.offMinute
		}
		p.line.Minutes = max(off-p.stint.onMinute, 0)
		lines = append(lines, p.line)
	}
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].Team != lines[j].Team {
			return lines[i].Team == f.HomeTeam
		}
		return lines[i].Player < lines[j].Player
	})
	return lines, nil
}

func apply(e d.Event, players map[d.PlayerID]*player, opponent map[d.TeamID]d.TeamID, minute int) error {
	if e.Minute < 0 {
		return invalid("negative minute %d", e.Minute)
	}
	if _, ok := opponent[e.Team]; !ok {
		return invalid("unknown team %q", e.Team)
	}
	p, ok := players[e.Player]
	if !ok {
		return invalid("unknown player %q", e.Player)
	}
	if p.app.Team != e.Team {
		return invalid("player %q does not play for %q", e.Player, e.Team)
	}
	var related *player
	if e.Related != "" {
		if related, ok = players[e.Related]; !ok {
			return invalid("unknown related player %q", e.Related)
		}
	}

	// Cards can be shown to bench players; every other event needs the player on the pitch.
	isCard := e.Kind == d.EventYellowCard || e.Kind == d.EventSecondYellow || e.Kind == d.EventRedCard
	if !isCard && !p.onPitchAt(e.Seq) {
		return invalid("player %q is not on the pitch", e.Player)
	}

	switch e.Kind {
	case d.EventGoal, d.EventPenaltyGoal:
		if e.Kind == d.EventGoal {
			p.line.Goals++
		} else {
			p.line.PenaltiesScored++
		}
		if related != nil {
			related.line.Assists++
		}
		concede(players, opponent[e.Team], e.Seq)
	case d.EventOwnGoal:
		p.line.OwnGoals++
		concede(players, e.Team, e.Seq)
	case d.EventPenaltyMissed:
		p.line.PenaltiesMissed++
	case d.EventPenaltySaved:
		p.line.PenaltiesMissed++
		if related == nil {
			related = keeperOnPitch(players, opponent[e.Team], e.Seq)
		}
		if related != nil {
			related.line.PenaltiesSaved++
		}
	case d.EventSubstitution:
		if related == nil {
			return invalid("substitution without incoming player")
		}
		if related.stint != nil {
			return invalid("incoming player %q already played", related.app.Player)
		}
		leave(p, e.Seq, minute)
		related.stint = &stint{onSeq: e.Seq, offSeq: noExit, onMinute: minute}
		related.played = true
	case d.EventYellowCard:
		p.line.YellowCards++
	case d.EventSecondYellow, d.EventRedCard:
		if e.Kind == d.EventSecondYellow {
			p.line.YellowCards++
		}
		p.line.SentOff = true
		if p.onPitchAt(e.Seq) {
			leave(p, e.Seq, minute)
		}
	default:
		return invalid("unknown event kind %q", e.Kind)
	}
	return nil
}

func leave(p *player, seq, minute int) {
	p.stint.offSeq = seq
	p.stint.offMinute = minute
}

// concede adds a goal conceded to every player of team on the pitch at seq.
func concede(players map[d.PlayerID]*player, team d.TeamID, seq int) {
	for _, p := range players {
		if p.app.Team == team && p.onPitchAt(seq) {
			p.line.GoalsConceded++
		}
	}
}

// keeperOnPitch returns team's goalkeeper on the pitch at seq, or nil if there
// is not exactly one.
func keeperOnPitch(players map[d.PlayerID]*player, team d.TeamID, seq int) *player {
	var found *player
	for _, p := range players {
		if p.app.Team == team && p.app.Role == d.RoleGoalkeeper && p.onPitchAt(seq) {
			if found != nil {
				return nil
			}
			found = p
		}
	}
	return found
}
