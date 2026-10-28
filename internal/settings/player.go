package settings

import (
	"bytes"
	"encoding/json"
	"fmt"

	d "github.com/openfanta/openfanta/internal/domain"
)

// Event ids, as they appear under player.events.
const (
	EventGoal         = "goal"
	EventPenaltyGoal  = "penalty_goal"
	EventAssist       = "assist.open_play"
	EventYellow       = "yellow"
	EventRed          = "red"
	EventOwnGoal      = "own_goal"
	EventGoalConceded = "goal_conceded"
	EventPenaltySaved = "penalty_saved"
	EventPenaltyMiss  = "penalty_missed"
	EventCleanSheet   = "clean_sheet"
)

// Penalty goal modes.
const (
	PenaltySameAsGoal = "same_as_goal"
	PenaltyReplace    = "replace_goal"
	PenaltyAdd        = "add_to_goal"
)

// goalkeeperRole is the by_role key that applies to goalkeepers. The engine
// does not tell D, C and A apart yet, so every other player gets the default.
const goalkeeperRole = "P"

// Player holds the per-player scoring rules (stage 1). All values are
// integer hundredths.
type Player struct {
	Events    map[string]Event `json:"events"`
	Cards     Cards            `json:"cards"`
	SenzaVoto SenzaVoto        `json:"senza_voto"`
}

// Cards holds card rules that are not plain point values.
type Cards struct {
	// RedReplacesYellow drops the yellow malus of a player who was sent off.
	RedReplacesYellow bool `json:"red_replaces_yellow"`
}

// SenzaVoto controls players without a voto (s.v.).
type SenzaVoto struct {
	// MinMinutes below which a player is s.v. even with a voto. Null means 0.
	MinMinutes *int `json:"min_minutes"`
	// WithRed gives an s.v. player who was sent off a fixed voto.
	WithRed *struct {
		Fixed d.Points `json:"fixed"`
	} `json:"with_red"`
	// WithOtherEvents gives an s.v. player with any of Events a base voto.
	WithOtherEvents *struct {
		Base   d.Points `json:"base"`
		Events []string `json:"events"`
	} `json:"with_other_events"`
}

// Event is the scoring rule of one event. In JSON it is either a plain
// integer (the value) or an object.
type Event struct {
	Value      *d.Points           `json:"value"`
	ByRole     map[string]d.Points `json:"by_role"`
	MinMinutes *int                `json:"min_minutes"`
	Mode       string              `json:"mode"`
}

// UnmarshalJSON accepts the integer shorthand and the object form.
func (e *Event) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] != '{' {
		var v d.Points
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*e = Event{Value: &v}
		return nil
	}
	type plain Event // no methods: avoids recursion
	var p plain
	if err := decodeStrict(b, &p); err != nil {
		return err
	}
	*e = Event(p)
	return nil
}

// Points returns the value of event id for a player of the given role, and
// zero when the document does not mention the event. A by_role entry for the
// goalkeeper wins over the default value.
func (p Player) Points(id string, role d.Role) d.Points {
	e, ok := p.Events[id]
	if !ok {
		return 0
	}
	if role == d.RoleGoalkeeper {
		if v, ok := e.ByRole[goalkeeperRole]; ok {
			return v
		}
	}
	if e.Value != nil {
		return *e.Value
	}
	return 0
}

// PenaltyGoalPoints returns what a scored penalty is worth, following the
// penalty_goal mode: same as a goal, instead of it, or on top of it.
func (p Player) PenaltyGoalPoints(role d.Role) d.Points {
	e := p.Events[EventPenaltyGoal]
	switch {
	case e.Mode == PenaltyAdd:
		return p.Points(EventGoal, role) + p.Points(EventPenaltyGoal, role)
	case e.Mode == PenaltyReplace || (e.Mode == "" && e.Value != nil):
		return p.Points(EventPenaltyGoal, role)
	default: // same_as_goal, and an absent penalty_goal
		return p.Points(EventGoal, role)
	}
}

// CleanSheetMinMinutes is the minutes a goalkeeper must play to earn the
// clean sheet bonus (0 when not set).
func (p Player) CleanSheetMinMinutes() int {
	if m := p.Events[EventCleanSheet].MinMinutes; m != nil {
		return *m
	}
	return 0
}

// MinPlayed is the minutes below which a player is s.v. (0 when not set).
func (s SenzaVoto) MinPlayed() int {
	if s.MinMinutes != nil {
		return *s.MinMinutes
	}
	return 0
}

func (p Player) validate() error {
	for id, e := range p.Events {
		if e.Mode != "" && id != EventPenaltyGoal {
			return fmt.Errorf("player.events.%s: mode only applies to %s", id, EventPenaltyGoal)
		}
		if e.MinMinutes != nil && id != EventCleanSheet {
			return fmt.Errorf("player.events.%s: min_minutes only applies to %s", id, EventCleanSheet)
		}
		if e.Value == nil && len(e.ByRole) == 0 && e.Mode != PenaltySameAsGoal {
			return fmt.Errorf("player.events.%s: needs a value, by_role or mode same_as_goal", id)
		}
		if (e.Mode == PenaltyReplace || e.Mode == PenaltyAdd) && e.Value == nil {
			return fmt.Errorf("player.events.%s: mode %s needs a value", id, e.Mode)
		}
		if e.Mode == PenaltySameAsGoal && (e.Value != nil || len(e.ByRole) > 0) {
			return fmt.Errorf("player.events.%s: mode %s takes no value", id, e.Mode)
		}
	}
	return nil
}
