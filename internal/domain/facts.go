package domain

// PlayerID is an opaque player identifier. Inside MatchFacts it only needs to
// be unique within the match; ingest maps it to a stable OpenFanta ID.
type PlayerID string

// TeamID is an opaque team identifier, scoped like PlayerID.
type TeamID string

// Role is the fantacalcio-relevant role of a player in a match. Classic rules
// only distinguish goalkeepers from everyone else.
type Role string

// Roles.
const (
	RoleGoalkeeper Role = "goalkeeper"
	RoleOutfield   Role = "outfield"
)

// EventKind is a match event that matters for fantacalcio.
type EventKind string

// Event kinds. Penalty shoot-outs are never represented.
const (
	EventGoal          EventKind = "goal"           // Player scored from open play; Related is the assist, if any.
	EventPenaltyGoal   EventKind = "penalty_goal"   // Player scored a penalty; Related is the assist, if any.
	EventOwnGoal       EventKind = "own_goal"       // Player scored into their own net; Team is the player's team.
	EventPenaltyMissed EventKind = "penalty_missed" // Player missed the target or hit the woodwork.
	EventPenaltySaved  EventKind = "penalty_saved"  // Player's penalty was saved; Related is the goalkeeper, if known.
	EventSubstitution  EventKind = "substitution"   // Player went off, Related came on.
	EventYellowCard    EventKind = "yellow_card"
	EventSecondYellow  EventKind = "second_yellow" // A second yellow: counts as a yellow and a sending off.
	EventRedCard       EventKind = "red_card"      // A straight red.
)

// AssistClass says what kind of assist a goal's Related player made. The zero
// value is an open-play assist.
type AssistClass string

// Assist classes.
const (
	AssistOpenPlay     AssistClass = ""             // the default: not a set piece, not a mere contribution
	AssistSetPiece     AssistClass = "set_piece"    // the pass came from a free kick, corner or throw-in
	AssistContribution AssistClass = "contribution" // a pass that helped the goal but was not judged an assist
)

// Valid reports whether c is a known class.
func (c AssistClass) Valid() bool {
	return c == AssistOpenPlay || c == AssistSetPiece || c == AssistContribution
}

// Position is the position a player held in a match, as the provider reports
// it. It is not the fantacalcio listone role, and the zero value means unknown.
type Position string

// Positions.
const (
	PositionGoalkeeper Position = "gk"
	PositionDefender   Position = "def"
	PositionMidfielder Position = "mid"
	PositionForward    Position = "fwd"
)

// StatKey names a per-player match statistic. Goals, assists, cards, own
// goals, penalties scored or missed and minutes are not here: events are the
// single source of truth for those.
type StatKey string

// Stat keys. All are non-negative counts for the match.
const (
	StatPasses             StatKey = "passes"
	StatAccuratePasses     StatKey = "accurate_passes"
	StatKeyPasses          StatKey = "key_passes"
	StatChancesCreated     StatKey = "chances_created"
	StatTouches            StatKey = "touches"
	StatShotsOnTarget      StatKey = "shots_on_target"
	StatShotsOffTarget     StatKey = "shots_off_target"
	StatBlockedShots       StatKey = "blocked_shots"
	StatDribbles           StatKey = "dribbles"
	StatDribblesWon        StatKey = "dribbles_won"
	StatDribbledPast       StatKey = "dribbled_past"
	StatDispossessed       StatKey = "dispossessed"
	StatPossessionLost     StatKey = "possession_lost"
	StatDuels              StatKey = "duels"
	StatDuelsWon           StatKey = "duels_won"
	StatAerialsWon         StatKey = "aerials_won"
	StatAerialsLost        StatKey = "aerials_lost"
	StatTackles            StatKey = "tackles"
	StatTacklesWon         StatKey = "tackles_won"
	StatInterceptions      StatKey = "interceptions"
	StatClearances         StatKey = "clearances"
	StatBallRecoveries     StatKey = "ball_recoveries"
	StatFouls              StatKey = "fouls"
	StatFoulsDrawn         StatKey = "fouls_drawn"
	StatPenaltiesWon       StatKey = "penalties_won"
	StatPenaltiesCommitted StatKey = "penalties_committed"
	StatErrorsToGoal       StatKey = "errors_to_goal"
	StatErrorsToShot       StatKey = "errors_to_shot"
	StatSaves              StatKey = "saves"
	StatSavesInsideBox     StatKey = "saves_inside_box"
	StatPunches            StatKey = "punches"
)

// Event is one match event. Events are ordered by Seq, which must be unique in
// a match; Minute is only used to measure minutes played.
type Event struct {
	Seq     int       `json:"seq"`
	Minute  int       `json:"minute"`
	Kind    EventKind `json:"kind"`
	Team    TeamID    `json:"team"`
	Player  PlayerID  `json:"player"`
	Related PlayerID  `json:"related,omitempty"`
	// AssistClass classifies the assist by Related on a goal or penalty goal.
	AssistClass AssistClass `json:"assist_class,omitempty"`
}

// Appearance lists a player in a matchday squad. Bench players who never come
// on may be listed; they do not count as having played.
type Appearance struct {
	Player  PlayerID `json:"player"`
	Team    TeamID   `json:"team"`
	Role    Role     `json:"role"`
	Starter bool     `json:"starter"`
	// Rating is the provider's own player rating, if any. The engine ignores it;
	// a voto source may use it.
	Rating *Points `json:"rating,omitempty"`
	// PlayerOfTheMatch is set when the provider names this player man of the match.
	PlayerOfTheMatch bool `json:"player_of_the_match,omitempty"`
	// Position is the provider's match position; "" when unknown.
	Position Position `json:"position,omitempty"`
	// HasStats is set when the provider sent a statistics block for this
	// player. Providers leave out stats that are zero, so for such a player a
	// key missing from Stats is 0. Without the block (a bench player who never
	// played) every stat is unknown.
	HasStats bool `json:"has_stats,omitempty"`
	// Stats holds the non-zero statistics the provider sent.
	Stats map[StatKey]int64 `json:"stats,omitempty"`
}

// Stat returns the value of k and whether it is known. A player with a
// statistics block has every stat known, with 0 for those the provider omitted.
func (a Appearance) Stat(k StatKey) (int64, bool) {
	if !a.HasStats {
		return 0, false
	}
	return a.Stats[k], true
}

// MatchFacts is the provider-agnostic description of a finished match: the
// single input from which voti and fantavoti are derived.
type MatchFacts struct {
	HomeTeam    TeamID       `json:"home_team"`
	AwayTeam    TeamID       `json:"away_team"`
	Length      int          `json:"length"` // regulation length in minutes, usually 90
	Appearances []Appearance `json:"appearances"`
	Events      []Event      `json:"events"`
}

// PlayerLine is what one player did in a match, as counted from MatchFacts.
// It is the engine's per-player input.
type PlayerLine struct {
	Player          PlayerID `json:"player"`
	Team            TeamID   `json:"team"`
	Role            Role     `json:"role"`
	Minutes         int      `json:"minutes"`
	Goals           int      `json:"goals"` // open play only
	PenaltiesScored int      `json:"penalties_scored"`
	PenaltiesMissed int      `json:"penalties_missed"` // missed or saved, as the taker
	PenaltiesSaved  int      `json:"penalties_saved"`  // as the goalkeeper
	OwnGoals        int      `json:"own_goals"`
	Assists         int      `json:"assists"` // every class together
	// AssistsSetPiece and AssistsContribution are the parts of Assists of
	// those classes; the rest are open-play assists.
	AssistsSetPiece     int  `json:"assists_set_piece"`
	AssistsContribution int  `json:"assists_contribution"`
	YellowCards         int  `json:"yellow_cards"`
	SentOff             bool `json:"sent_off"`
	GoalsConceded       int  `json:"goals_conceded"` // by the player's team while they were on the pitch
	// DecisiveGoalsWin is 1 for the goal that decided a won match, DecisiveGoalsDraw
	// for the goal that levelled a drawn one. See timeline.decisive.
	DecisiveGoalsWin  int  `json:"decisive_goals_win"`
	DecisiveGoalsDraw int  `json:"decisive_goals_draw"`
	PlayerOfTheMatch  bool `json:"player_of_the_match"`
}

// CleanSheet reports whether a goalkeeper played and their team conceded no
// goal while they were on the pitch. Whether it earns a bonus is up to the
// engine and the league settings.
func (l PlayerLine) CleanSheet() bool {
	return l.Role == RoleGoalkeeper && l.Minutes > 0 && l.GoalsConceded == 0
}
