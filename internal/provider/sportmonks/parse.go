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
const parserVersion = "1"

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

	detailRating = 118
)

// Fixture states after which a match is over.
var finalStates = map[string]bool{"FT": true, "AET": true, "FT_PEN": true}

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
		ID     id     `json:"id"`
		Name   string `json:"name"`
		League struct {
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
		Ref:     f.ID.String(),
		Status:  f.State.DeveloperName,
		Final:   finalStates[f.State.DeveloperName],
		Players: map[string]string{},
	}
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
		for _, d := range l.Details {
			if d.TypeID == detailRating {
				if p, ok := parseRating(d.Data.Value); ok {
					app.Rating = &p
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

// convertEvent maps one Sportmonks event to a domain event. keep is false for
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
