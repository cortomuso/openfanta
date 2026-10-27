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

// Event is one match event. Events are ordered by Seq, which must be unique in
// a match; Minute is only used to measure minutes played.
type Event struct {
	Seq     int       `json:"seq"`
	Minute  int       `json:"minute"`
	Kind    EventKind `json:"kind"`
	Team    TeamID    `json:"team"`
	Player  PlayerID  `json:"player"`
	Related PlayerID  `json:"related,omitempty"`
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
	Assists         int      `json:"assists"`
	YellowCards     int      `json:"yellow_cards"`
	SentOff         bool     `json:"sent_off"`
	GoalsConceded   int      `json:"goals_conceded"` // by the player's team while they were on the pitch
}
