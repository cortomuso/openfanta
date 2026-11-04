package sportmonks

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/provider"
)

// parserVersion must change whenever parsing can produce different output
// from the same payload.
const parserVersion = "6"

// ErrParse wraps every parsing failure.
var ErrParse = errors.New("sportmonks: cannot parse payload")

// Sportmonks type IDs used by the parser. See docs.sportmonks.com/v3 "Types".
const (
	eventGoal          = 14
	eventOwnGoal       = 15
	eventPenalty       = 16 // a scored penalty
	eventMissedPenalty = 17
	eventSubstitution  = 18
	eventYellow        = 19
	eventRed           = 20
	eventYellowRed     = 21

	subTypePenaltySaved = 1509

	lineupStarter = 11
	lineupBench   = 12

	positionGoalkeeper = 24
	positionDefender   = 25
	positionMidfielder = 26
	positionAttacker   = 27

	detailRating     = 118
	detailManOfMatch = 1490 // data.value is 1 for the man of the match; most fixtures have none
)

// positions maps Sportmonks position types (verified against /core/types).
var positions = map[int]domain.Position{
	positionGoalkeeper: domain.PositionGoalkeeper,
	positionDefender:   domain.PositionDefender,
	positionMidfielder: domain.PositionMidfielder,
	positionAttacker:   domain.PositionForward,
}

// statTypes maps the lineups.details types kept as player stats. The IDs and
// names were checked against /core/types and a real finished Serie A fixture.
// Types that duplicate events (goals 52, assists 79, cards 83/84/85, minutes
// 119, penalties scored 111 and missed 112, own goals 324) are deliberately
// absent: events are the single source of truth for those.
var statTypes = map[int]domain.StatKey{
	80:    domain.StatPasses,
	116:   domain.StatAccuratePasses,
	117:   domain.StatKeyPasses,
	9706:  domain.StatChancesCreated,
	120:   domain.StatTouches,
	86:    domain.StatShotsOnTarget,
	41:    domain.StatShotsOffTarget,
	97:    domain.StatBlockedShots,
	108:   domain.StatDribbles, // "Dribble Attempts"
	109:   domain.StatDribblesWon,
	110:   domain.StatDribbledPast,
	94:    domain.StatDispossessed,
	27273: domain.StatPossessionLost,
	105:   domain.StatDuels,
	106:   domain.StatDuelsWon,
	107:   domain.StatAerialsWon,
	27266: domain.StatAerialsLost,
	78:    domain.StatTackles,
	27267: domain.StatTacklesWon,
	100:   domain.StatInterceptions,
	101:   domain.StatClearances,
	27271: domain.StatBallRecoveries,
	56:    domain.StatFouls,
	96:    domain.StatFoulsDrawn,
	115:   domain.StatPenaltiesWon,
	114:   domain.StatPenaltiesCommitted,
	571:   domain.StatErrorsToGoal,
	48997: domain.StatErrorsToShot,
	57:    domain.StatSaves,
	104:   domain.StatSavesInsideBox,
	103:   domain.StatPunches,
}

// Fixture states after which a match is over.
var finalStates = map[string]bool{"FT": true, "AET": true, "FT_PEN": true}

// exceptionStates maps the states of a match that did not go the usual way.
// A match stopped part-way (suspended, interrupted, abandoned) can be resumed
// or replayed, so all three are suspensions. Cancelled, walk-over and awarded
// matches are not covered by the real-match policy.
var exceptionStates = map[string]domain.Exception{
	"POSTPONED":   domain.ExceptionPostponed,
	"SUSPENDED":   domain.ExceptionSuspended,
	"INTERRUPTED": domain.ExceptionSuspended,
	"ABANDONED":   domain.ExceptionSuspended,
}

// periods maps fixture states to how far the match has got. Other states (postponed,
// suspended, cancelled, delayed, ...) say nothing about a period.
var periods = map[string]domain.Period{
	"NS": domain.PeriodNotStarted, "TBA": domain.PeriodNotStarted,
	"INPLAY_1ST_HALF":  domain.PeriodFirstHalf,
	"HT":               domain.PeriodHalfTime,
	"INPLAY_2ND_HALF":  domain.PeriodSecondHalf,
	"EXTRA_TIME_BREAK": domain.PeriodExtraTime, "INPLAY_ET": domain.PeriodExtraTime,
	"INPLAY_ET_SECOND_HALF": domain.PeriodExtraTime, "BREAK": domain.PeriodExtraTime,
	"PEN_BREAK": domain.PeriodPenalties, "INPLAY_PENALTIES": domain.PeriodPenalties,
	"FT": domain.PeriodFinished, "AET": domain.PeriodFinished, "FT_PEN": domain.PeriodFinished,
}

// currentScore is the description of the running or final score among a fixture's scores
// (the others are per half, extra time and shoot-out).
const currentScore = "CURRENT"

type id = json.Number // Sportmonks IDs are integers; keep them as exact strings

type envelope[T any] struct {
	Data T `json:"data"`
}

func decode[T any](r provider.Raw) (T, error) {
	var env envelope[T]
	if err := json.Unmarshal(r.Body, &env); err != nil {
		return env.Data, fmt.Errorf("%w (%s): %w", ErrParse, r.Key, err)
	}
	return env.Data, nil
}

// ParseSeason implements provider.Provider.
func (c *Client) ParseSeason(r provider.Raw) (provider.Season, error) { return ParseSeason(r) }

// ParseSchedule implements provider.Provider.
func (c *Client) ParseSchedule(r provider.Raw) (provider.Schedule, error) { return ParseSchedule(r) }

// ParseFixture implements provider.Provider.
func (c *Client) ParseFixture(r provider.Raw) (provider.Fixture, error) { return ParseFixture(r) }

// ParseSeason parses GET /seasons/{id}?include=league.country.
func ParseSeason(r provider.Raw) (provider.Season, error) {
	s, err := decode[struct {
		ID      id     `json:"id"`
		Name    string `json:"name"`
		Current bool   `json:"is_current"`
		League  struct {
			ID      id     `json:"id"`
			Name    string `json:"name"`
			Country struct {
				Name string `json:"name"`
			} `json:"country"`
		} `json:"league"`
	}](r)
	if err != nil {
		return provider.Season{}, err
	}
	if s.ID == "" || s.League.ID == "" {
		return provider.Season{}, fmt.Errorf("%w (%s): missing season or league id", ErrParse, r.Key)
	}
	return provider.Season{
		Ref: s.ID.String(), Name: s.Name,
		CompetitionRef: s.League.ID.String(), CompetitionName: s.League.Name, CompetitionCountry: s.League.Country.Name,
		Current: s.Current,
	}, nil
}

// ParseSchedule parses GET /schedules/seasons/{id}: stages → rounds → fixtures.
// Round names are matchday numbers in league competitions.
func ParseSchedule(r provider.Raw) (provider.Schedule, error) {
	stages, err := decode[[]struct {
		Rounds []struct {
			Name     string `json:"name"`
			Fixtures []struct {
				ID id `json:"id"`
			} `json:"fixtures"`
		} `json:"rounds"`
	}](r)
	if err != nil {
		return provider.Schedule{}, err
	}
	out := provider.Schedule{Matchdays: map[int][]string{}}
	for _, st := range stages {
		for _, rd := range st.Rounds {
			n, err := strconv.Atoi(rd.Name)
			if err != nil || n <= 0 {
				continue // not a numbered matchday (e.g. a play-off round)
			}
			for _, f := range rd.Fixtures {
				out.Matchdays[n] = append(out.Matchdays[n], f.ID.String())
			}
		}
	}
	for n := range out.Matchdays {
		sort.Strings(out.Matchdays[n])
	}
	return out, nil
}

type smFixture struct {
	ID         id      `json:"id"`
	StartingAt *string `json:"starting_at"`
	Length     *int    `json:"length"`
	State      struct {
		DeveloperName string `json:"developer_name"`
	} `json:"state"`
	Participants []struct {
		ID   id     `json:"id"`
		Name string `json:"name"`
		Meta struct {
			Location string `json:"location"`
		} `json:"meta"`
	} `json:"participants"`
	Lineups []struct {
		PlayerID   id     `json:"player_id"`
		TeamID     id     `json:"team_id"`
		PositionID *int   `json:"position_id"`
		TypeID     int    `json:"type_id"`
		PlayerName string `json:"player_name"`
		Details    []struct {
			TypeID int `json:"type_id"`
			Data   struct {
				Value json.RawMessage `json:"value"`
			} `json:"data"`
		} `json:"details"`
	} `json:"lineups"`
	Events []smEvent `json:"events"`
	Scores []struct {
		Description string `json:"description"`
		Score       struct {
			Goals       *int   `json:"goals"`
			Participant string `json:"participant"`
		} `json:"score"`
	} `json:"scores"`
}

type smEvent struct {
	ID              id   `json:"id"`
	TypeID          int  `json:"type_id"`
	SubTypeID       *int `json:"sub_type_id"`
	PlayerID        *id  `json:"player_id"`
	RelatedPlayerID *id  `json:"related_player_id"`
	Minute          *int `json:"minute"`
	ExtraMinute     *int `json:"extra_minute"`
	SortOrder       *int `json:"sort_order"`
	Rescinded       bool `json:"rescinded"`
}

// ParseFixture parses GET /fixtures/{id}?include=participants;lineups.details;events;state.
func ParseFixture(r provider.Raw) (provider.Fixture, error) {
	f, err := decode[smFixture](r)
	if err != nil {
		return provider.Fixture{}, err
	}
	fail := func(format string, args ...any) (provider.Fixture, error) {
		return provider.Fixture{}, fmt.Errorf("%w (%s): %s", ErrParse, r.Key, fmt.Sprintf(format, args...))
	}

	out := provider.Fixture{
		Ref:       f.ID.String(),
		Status:    f.State.DeveloperName,
		Final:     finalStates[f.State.DeveloperName],
		Players:   map[string]string{},
		Exception: exceptionStates[f.State.DeveloperName],
		Period:    periods[f.State.DeveloperName],
	}
	out.Score = currentScoreOf(f)
	if f.StartingAt != nil {
		t, err := time.Parse(time.DateTime, *f.StartingAt) // Sportmonks returns UTC
		if err != nil {
			return fail("starting_at %q: %v", *f.StartingAt, err)
		}
		out.Kickoff = &t
	}
	for _, p := range f.Participants {
		team := provider.Team{Ref: p.ID.String(), Name: p.Name}
		switch p.Meta.Location {
		case "home":
			out.Home = team
		case "away":
			out.Away = team
		}
	}
	if out.Home.Ref == "" || out.Away.Ref == "" {
		return fail("missing home or away participant")
	}

	facts := domain.MatchFacts{HomeTeam: domain.TeamID(out.Home.Ref), AwayTeam: domain.TeamID(out.Away.Ref), Length: 90}
	if f.Length != nil && *f.Length > 0 {
		facts.Length = *f.Length
	}

	teamOf := map[string]domain.TeamID{}
	for _, l := range f.Lineups {
		if l.TypeID != lineupStarter && l.TypeID != lineupBench {
			continue
		}
		ref := l.PlayerID.String()
		if ref == "" {
			continue
		}
		role := domain.RoleOutfield
		if l.PositionID != nil && *l.PositionID == positionGoalkeeper {
			role = domain.RoleGoalkeeper
		}
		app := domain.Appearance{
			Player: domain.PlayerID(ref), Team: domain.TeamID(l.TeamID.String()), Role: role, Starter: l.TypeID == lineupStarter,
		}
		if l.PositionID != nil {
			app.Position = positions[*l.PositionID]
		}
		// A statistics block means the player took part; the provider leaves
		// out the stats that are zero, so the block itself is what matters.
		app.HasStats = len(l.Details) > 0
		for _, d := range l.Details {
			if key, ok := statTypes[d.TypeID]; ok {
				if n, ok := parseCount(d.Data.Value); ok && n > 0 {
					if app.Stats == nil {
						app.Stats = map[domain.StatKey]int64{}
					}
					app.Stats[key] = n
				}
			}
			if d.TypeID == detailRating {
				if p, ok := parseRating(d.Data.Value); ok {
					app.Rating = &p
				}
			}
			if d.TypeID == detailManOfMatch {
				if v, ok := parseRating(d.Data.Value); ok && v > 0 {
					app.PlayerOfTheMatch = true
				}
			}
		}
		facts.Appearances = append(facts.Appearances, app)
		teamOf[ref] = app.Team
		out.Players[ref] = l.PlayerName
	}
	sort.Slice(facts.Appearances, func(i, j int) bool { return facts.Appearances[i].Player < facts.Appearances[j].Player })

	events := append([]smEvent(nil), f.Events...)
	sort.SliceStable(events, func(i, j int) bool { return eventLess(events[i], events[j]) })
	for _, e := range events {
		ev, keep, err := convertEvent(e, teamOf)
		if err != nil {
			return fail("event %s: %v", e.ID, err)
		}
		if keep {
			ev.Seq = len(facts.Events) + 1
			facts.Events = append(facts.Events, ev)
		}
	}
	out.Facts = facts
	return out, nil
}

// convertEvent maps one Sportmonks event to a domain event. Every assist is an
// open-play assist: the sub_type_id of a goal is how it was struck (left foot
// 1521, right foot 1522, header 1694), not how it was made, and no other
// structured field tells set pieces apart. keep is false for
// events that do not matter for fantacalcio or were rescinded (e.g. by VAR).
func convertEvent(e smEvent, teamOf map[string]domain.TeamID) (domain.Event, bool, error) {
	if e.Rescinded || e.PlayerID == nil {
		return domain.Event{}, false, nil // rescinded, or a card shown to a coach
	}
	var kind domain.EventKind
	player, related := e.PlayerID.String(), ""
	if e.RelatedPlayerID != nil {
		related = e.RelatedPlayerID.String()
	}
	switch e.TypeID {
	case eventGoal:
		kind = domain.EventGoal
	case eventPenalty:
		kind = domain.EventPenaltyGoal
	case eventOwnGoal:
		kind, related = domain.EventOwnGoal, ""
	case eventMissedPenalty:
		kind, related = domain.EventPenaltyMissed, ""
		if e.SubTypeID != nil && *e.SubTypeID == subTypePenaltySaved {
			kind = domain.EventPenaltySaved
		}
	case eventSubstitution:
		// Sportmonks: player_id comes on, related_player_id goes off.
		kind, player, related = domain.EventSubstitution, related, player
	case eventYellow:
		kind, related = domain.EventYellowCard, ""
	case eventRed:
		kind, related = domain.EventRedCard, ""
	case eventYellowRed:
		kind, related = domain.EventSecondYellow, ""
	default:
		return domain.Event{}, false, nil
	}

	team, ok := teamOf[player]
	if !ok {
		if kind == domain.EventYellowCard || kind == domain.EventRedCard || kind == domain.EventSecondYellow {
			return domain.Event{}, false, nil // a card shown to a coach: player_id is the coach's person ID
		}
		return domain.Event{}, false, fmt.Errorf("player %s not in lineups", player)
	}
	if related != "" {
		if _, ok := teamOf[related]; !ok {
			return domain.Event{}, false, fmt.Errorf("related player %s not in lineups", related)
		}
	}
	minute := 0
	if e.Minute != nil {
		minute = *e.Minute
	}
	return domain.Event{
		Minute: minute, Kind: kind, Team: team,
		Player: domain.PlayerID(player), Related: domain.PlayerID(related),
	}, true, nil
}

func eventLess(a, b smEvent) bool {
	ka := [3]int{deref(a.Minute), deref(a.ExtraMinute), deref(a.SortOrder)}
	kb := [3]int{deref(b.Minute), deref(b.ExtraMinute), deref(b.SortOrder)}
	if ka != kb {
		for i := range ka {
			if ka[i] != kb[i] {
				return ka[i] < kb[i]
			}
		}
	}
	ia, _ := a.ID.Int64()
	ib, _ := b.ID.Int64()
	return ia < ib
}

func deref(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// parseRating converts a rating such as 7.23 to hundredths, rounding half away
// from zero (6.835 -> 6.84). This is the only place where provider precision
// beyond two decimals is dropped; nothing downstream rounds voti.
func parseRating(raw json.RawMessage) (domain.Points, bool) {
	var n json.Number
	if json.Unmarshal(raw, &n) != nil {
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return 0, false
		}
		n = json.Number(s)
	}
	v, err := n.Float64()
	if err != nil || v <= 0 || v > 10 {
		return 0, false
	}
	return domain.Points(math.Round(v * 100)), true
}

// parseCount reads a non-negative whole number such as 12 or "12". Anything
// else (a fraction, a negative, text) is not a count and is ignored.
func parseCount(raw json.RawMessage) (int64, bool) {
	var n json.Number
	if json.Unmarshal(raw, &n) != nil {
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return 0, false
		}
		n = json.Number(s)
	}
	v, err := n.Float64()
	if err != nil || v < 0 || v != math.Trunc(v) || v > 1<<31 {
		return 0, false
	}
	return int64(v), true
}

// currentScoreOf returns the CURRENT score, or nil unless both sides have one.
func currentScoreOf(f smFixture) *domain.Score {
	var home, away *int
	for _, s := range f.Scores {
		if s.Description != currentScore {
			continue
		}
		switch s.Score.Participant {
		case "home":
			home = s.Score.Goals
		case "away":
			away = s.Score.Goals
		}
	}
	if home == nil || away == nil || *home < 0 || *away < 0 {
		return nil
	}
	return &domain.Score{Home: *home, Away: *away}
}
