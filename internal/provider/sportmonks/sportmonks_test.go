package sportmonks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	d "github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/provider"
	"github.com/openfanta/openfanta/internal/provider/httpx"
	"github.com/openfanta/openfanta/internal/timeline"
)

var update = flag.Bool("update", false, "rewrite golden files")

func raw(t *testing.T, name string) provider.Raw {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return provider.Raw{Key: name, Status: 200, Body: b}
}

func TestParseSeason(t *testing.T) {
	s, err := ParseSeason(raw(t, "season_777.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := provider.Season{Ref: "777", Name: "2026/2027", CompetitionRef: "384", CompetitionName: "Serie A", CompetitionCountry: "Italy"}
	if s != want {
		t.Errorf("season = %+v", s)
	}
}

func TestParseSchedule(t *testing.T) {
	s, err := ParseSchedule(raw(t, "schedule_777.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[int][]string{1: {"5001", "5002"}, 2: {"5003"}}
	if !reflect.DeepEqual(s.Matchdays, want) {
		t.Errorf("matchdays = %v (play-off round must be skipped)", s.Matchdays)
	}
}

func TestParseFixtureGolden(t *testing.T) {
	f, err := ParseFixture(raw(t, "fixture_5001_ft.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := json.MarshalIndent(f, "", "  ")
	golden := filepath.Join("testdata", "fixture_5001_ft.golden.json")
	if *update {
		if err := os.WriteFile(golden, append(got, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run with -update to create)", err)
	}
	if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
		t.Errorf("ParseFixture output differs from %s; rerun with -update and review the diff", golden)
	}
}

func TestParseFixtureSemantics(t *testing.T) {
	f, err := ParseFixture(raw(t, "fixture_5001_ft.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !f.Final || f.Status != "FT" || f.Home.Name != "Home FC" || f.Away.Ref != "200" {
		t.Errorf("header = final:%v status:%s home:%+v away:%+v", f.Final, f.Status, f.Home, f.Away)
	}
	if f.Kickoff == nil || !f.Kickoff.Equal(time.Date(2026, 8, 23, 18, 45, 0, 0, time.UTC)) {
		t.Errorf("kickoff = %v", f.Kickoff)
	}
	if f.Players["1013"] != "Home Super Sub" || len(f.Players) != 14 {
		t.Errorf("players = %v", f.Players)
	}

	lines, err := timeline.Derive(f.Facts)
	if err != nil {
		t.Fatalf("parsed facts are inconsistent: %v", err)
	}
	by := map[d.PlayerID]d.PlayerLine{}
	for _, l := range lines {
		by[l.Player] = l
	}
	if _, ok := by["2007"]; ok {
		t.Error("unused substitute must not play")
	}
	if _, ok := by["1012"]; ok {
		t.Error("unused reserve keeper must not play")
	}
	checks := []struct {
		player string
		got    any
		want   any
	}{
		{"1002 goals", by["1002"].Goals, 1},
		{"1002 assists", by["1002"].Assists, 1},
		{"1003 assists", by["1003"].Assists, 1},
		{"1003 minutes", by["1003"].Minutes, 60},
		{"1013 goals", by["1013"].Goals, 1},
		{"1013 minutes", by["1013"].Minutes, 30},
		{"2003 penalties scored", by["2003"].PenaltiesScored, 1},
		{"2003 penalties missed", by["2003"].PenaltiesMissed, 1},
		{"1004 penalties missed", by["1004"].PenaltiesMissed, 1},
		{"1001 penalties saved", by["1001"].PenaltiesSaved, 1},
		{"1001 conceded", by["1001"].GoalsConceded, 1},
		{"2005 own goals", by["2005"].OwnGoals, 1},
		{"2001 conceded", by["2001"].GoalsConceded, 2},
		{"2001 minutes", by["2001"].Minutes, 80},
		{"2006 conceded", by["2006"].GoalsConceded, 1},
		{"2006 minutes", by["2006"].Minutes, 10},
		{"2002 yellows", by["2002"].YellowCards, 2},
		{"2002 sent off", by["2002"].SentOff, true},
		{"1005 sent off", by["1005"].SentOff, true},
		{"1005 minutes", by["1005"].Minutes, 88},
		{"1004 yellows (coach card ignored)", by["1004"].YellowCards, 0},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.player, c.got, c.want)
		}
	}

	rating := map[d.PlayerID]*d.Points{}
	for _, a := range f.Facts.Appearances {
		rating[a.Player] = a.Rating
	}
	if r := rating["1013"]; r == nil || *r != 724 {
		t.Errorf("1013 rating = %v", r)
	}
	if r := rating["2006"]; r == nil || *r != 600 {
		t.Errorf("2006 rating given as a string = %v", r)
	}
	if rating["2007"] != nil {
		t.Error("unused player should have no rating")
	}
}

func TestParseLiveFixtureIsNotFinal(t *testing.T) {
	f, err := ParseFixture(raw(t, "fixture_5001_live.json"))
	if err != nil {
		t.Fatal(err)
	}
	if f.Final {
		t.Error("in-play fixture reported as final")
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"not json":           `{`,
		"no participants":    `{"data":{"id":1,"participants":[]}}`,
		"bad kickoff":        `{"data":{"id":1,"starting_at":"yesterday"}}`,
		"event unknown team": `{"data":{"id":1,"participants":[{"id":1,"meta":{"location":"home"}},{"id":2,"meta":{"location":"away"}}],"events":[{"id":1,"type_id":14,"player_id":99}]}}`,
		"related unknown": `{"data":{"id":1,"participants":[{"id":1,"meta":{"location":"home"}},{"id":2,"meta":{"location":"away"}}],
			"lineups":[{"player_id":5,"team_id":1,"type_id":11}],"events":[{"id":1,"type_id":14,"player_id":5,"related_player_id":99}]}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseFixture(provider.Raw{Key: name, Body: []byte(body)}); !errors.Is(err, ErrParse) {
				t.Fatalf("err = %v, want ErrParse", err)
			}
		})
	}
	if _, err := ParseSeason(provider.Raw{Body: []byte(`{"data":{}}`)}); !errors.Is(err, ErrParse) {
		t.Errorf("season without ids: %v", err)
	}
	if _, err := ParseSeason(provider.Raw{Body: []byte(`[`)}); !errors.Is(err, ErrParse) {
		t.Errorf("season bad json: %v", err)
	}
	if _, err := ParseSchedule(provider.Raw{Body: []byte(`{"data":{}}`)}); !errors.Is(err, ErrParse) {
		t.Errorf("schedule wrong shape: %v", err)
	}
}

// Real Sportmonks data puts a coach's person ID in player_id (coach_id stays
// null), so a card for someone outside the line-ups is staff and is dropped.
// Any other event for an unknown player is still an error.
func TestParseIgnoresStaffCards(t *testing.T) {
	const head = `{"data":{"id":1,"participants":[{"id":1,"meta":{"location":"home"}},{"id":2,"meta":{"location":"away"}}],
		"lineups":[{"player_id":5,"team_id":1,"type_id":11}],"events":[`
	for _, typeID := range []int{eventYellow, eventRed, eventYellowRed} {
		body := head + fmt.Sprintf(`{"id":1,"type_id":%d,"player_id":99,"coach_id":null}]}}`, typeID)
		f, err := ParseFixture(provider.Raw{Key: "staff card", Body: []byte(body)})
		if err != nil {
			t.Fatalf("type %d: %v", typeID, err)
		}
		if len(f.Facts.Events) != 0 {
			t.Errorf("type %d: events = %v, want none", typeID, f.Facts.Events)
		}
	}
	body := head + `{"id":1,"type_id":17,"player_id":99}]}}`
	if _, err := ParseFixture(provider.Raw{Key: "unknown penalty taker", Body: []byte(body)}); !errors.Is(err, ErrParse) {
		t.Errorf("missed penalty by unknown player: err = %v, want ErrParse", err)
	}
}

func TestParseRating(t *testing.T) {
	cases := map[string]struct {
		want d.Points
		ok   bool
	}{
		`7.23`: {723, true}, `"6.5"`: {650, true}, `6.835`: {684, true}, `6.8351`: {684, true},
		`0`: {0, false}, `11`: {0, false}, `null`: {0, false}, `"x"`: {0, false}, `{}`: {0, false},
	}
	for in, c := range cases {
		got, ok := parseRating(json.RawMessage(in))
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseRating(%s) = %d, %v; want %d, %v", in, got, ok, c.want, c.ok)
		}
	}
}

func TestClientRequests(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "tok" || r.URL.Query().Has("api_token") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		paths = append(paths, r.URL.RequestURI())
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()

	c := New("tok", httpx.New(nil, nil))
	c.BaseURL = srv.URL
	c.Now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("x", 3600)) }
	ctx := context.Background()

	r, err := c.FetchFixture(ctx, "5001")
	if err != nil {
		t.Fatal(err)
	}
	if r.Key != "fixtures/5001" || r.Status != 200 || r.FetchedAt.Location() != time.UTC {
		t.Errorf("raw = %+v", r)
	}
	if _, err := c.FetchSeason(ctx, "777"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.FetchSchedule(ctx, "777"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"/fixtures/5001?include=participants%3Blineups.details%3Bevents%3Bstate",
		"/seasons/777?include=league.country",
		"/schedules/seasons/777",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("paths = %v", paths)
	}
	if c.Name() != "sportmonks" || c.ParserVersion() == "" {
		t.Error("name or parser version missing")
	}

	c.Token = "wrong"
	if _, err := c.FetchFixture(ctx, "1"); !httpx.IsStatus(err, 401) {
		t.Errorf("unauthorized: %v", err)
	}
}

func TestRetryAfterFromBody(t *testing.T) {
	cases := map[string]time.Duration{
		`{"retry_after": 12}`:                      12 * time.Second,
		`{"rate_limit":{"resets_in_seconds": 30}}`: 30 * time.Second,
		`not json`: 0,
	}
	for body, want := range cases {
		if got := retryAfter(nil, []byte(body)); got != want {
			t.Errorf("retryAfter(%s) = %v, want %v", body, got, want)
		}
	}
}

func TestClientParseMethodsDelegate(t *testing.T) {
	c := &Client{}
	if _, err := c.ParseSeason(raw(t, "season_777.json")); err != nil {
		t.Error(err)
	}
	if _, err := c.ParseSchedule(raw(t, "schedule_777.json")); err != nil {
		t.Error(err)
	}
	if _, err := c.ParseFixture(raw(t, "fixture_5001_ft.json")); err != nil {
		t.Error(err)
	}
}

// A card or goal later cancelled by video review never reaches the facts, so
// it cannot score, send a player off or make a goal conceded.
func TestParseDropsRescindedEvents(t *testing.T) {
	const head = `{"data":{"id":1,"participants":[{"id":1,"meta":{"location":"home"}},{"id":2,"meta":{"location":"away"}}],
		"lineups":[{"player_id":5,"team_id":1,"type_id":11},{"player_id":6,"team_id":1,"type_id":11},{"player_id":7,"team_id":2,"type_id":11}],"events":[`
	for _, typeID := range []int{eventGoal, eventOwnGoal, eventPenalty, eventMissedPenalty, eventYellow, eventRed, eventYellowRed} {
		body := head + fmt.Sprintf(`{"id":1,"type_id":%d,"player_id":5,"related_player_id":6,"team_id":1,"minute":80,"rescinded":true}]}}`, typeID)
		f, err := ParseFixture(provider.Raw{Key: "rescinded", Body: []byte(body)})
		if err != nil {
			t.Fatalf("type %d: %v", typeID, err)
		}
		if len(f.Facts.Events) != 0 {
			t.Errorf("type %d: events = %v, want none", typeID, f.Facts.Events)
		}
	}
}

// Real payloads put the way the goal was struck in sub_type_id (left foot 1521,
// right foot 1522, header 1694). It says nothing about the assist, so every
// assist stays open play.
func TestGoalSubTypesDoNotClassifyAssists(t *testing.T) {
	const head = `{"data":{"id":1,"participants":[{"id":1,"meta":{"location":"home"}},{"id":2,"meta":{"location":"away"}}],
		"lineups":[{"player_id":5,"team_id":1,"type_id":11},{"player_id":6,"team_id":1,"type_id":11}],"events":[`
	for _, sub := range []int{1521, 1522, 1694} {
		body := head + fmt.Sprintf(`{"id":1,"type_id":14,"sub_type_id":%d,"player_id":5,"related_player_id":6,"team_id":1,"minute":10}]}}`, sub)
		f, err := ParseFixture(provider.Raw{Key: "goal", Body: []byte(body)})
		if err != nil {
			t.Fatal(err)
		}
		if e := f.Facts.Events[0]; e.Kind != d.EventGoal || e.Related != "6" || e.AssistClass != d.AssistOpenPlay {
			t.Errorf("sub type %d: event = %+v, want a goal with an open-play assist", sub, e)
		}
	}
}

func TestParseManOfTheMatch(t *testing.T) {
	body := `{"data":{"id":1,"participants":[{"id":1,"meta":{"location":"home"}},{"id":2,"meta":{"location":"away"}}],
		"lineups":[
			{"player_id":5,"team_id":1,"type_id":11,"details":[{"type_id":1490,"data":{"value":1}}]},
			{"player_id":6,"team_id":1,"type_id":11,"details":[{"type_id":1490,"data":{"value":0}}]},
			{"player_id":7,"team_id":2,"type_id":11}],"events":[]}}`
	f, err := ParseFixture(provider.Raw{Key: "motm", Body: []byte(body)})
	if err != nil {
		t.Fatal(err)
	}
	got := map[d.PlayerID]bool{}
	for _, a := range f.Facts.Appearances {
		got[a.Player] = a.PlayerOfTheMatch
	}
	if !got["5"] || got["6"] || got["7"] {
		t.Errorf("player of the match = %v, want only player 5", got)
	}
}

func TestParseFixtureStatsAndPositions(t *testing.T) {
	f, err := ParseFixture(raw(t, "fixture_5001_ft.json"))
	if err != nil {
		t.Fatal(err)
	}
	by := map[d.PlayerID]d.Appearance{}
	for _, a := range f.Facts.Appearances {
		by[a.Player] = a
	}

	for id, want := range map[d.PlayerID]d.Position{
		"1001": d.PositionGoalkeeper, "1005": d.PositionDefender,
		"1004": d.PositionMidfielder, "1002": d.PositionForward,
	} {
		if got := by[id].Position; got != want {
			t.Errorf("player %s position = %q, want %q", id, got, want)
		}
	}

	// A stat the provider omits is 0 for a player with a statistics block.
	if v, ok := by["1002"].Stat(d.StatTacklesWon); !ok || v != 0 {
		t.Errorf("omitted stat = %d, %v; want 0, true", v, ok)
	}
	// A stat sent as 0 and one omitted look the same.
	if _, present := by["1002"].Stats[d.StatPenaltiesWon]; present {
		t.Error("a zero stat was stored")
	}
	if v, _ := by["1002"].Stat(d.StatShotsOnTarget); v != 3 {
		t.Errorf("shots on target = %d", v)
	}
	// A count sent as a string is read.
	if v, _ := by["2003"].Stat(d.StatKeyPasses); v != 1 {
		t.Errorf("key passes sent as a string = %d", v)
	}
	// Chances created is kept apart from key passes.
	if v, ok := by["1004"].Stat(d.StatKeyPasses); !ok || v != 0 {
		t.Errorf("key passes = %d, %v", v, ok)
	}
	if v, _ := by["1004"].Stat(d.StatChancesCreated); v != 1 {
		t.Errorf("chances created = %d", v)
	}
	// Event duplicates are never stats; the stat vocabulary has no minutes.
	if _, ok := by["1001"].Stats["minutes"]; ok {
		t.Error("minutes imported as a stat")
	}

	// No statistics block at all: everything is unknown.
	for _, id := range []d.PlayerID{"1012", "2007"} {
		if v, ok := by[id].Stat(d.StatPasses); ok || v != 0 || by[id].HasStats {
			t.Errorf("player %s without stats: HasStats=%v stat=%d,%v", id, by[id].HasStats, v, ok)
		}
	}
	// A block with only the rating and minutes still means the player took part.
	if !by["2006"].HasStats {
		t.Error("a reserve keeper with a rating-only block has no stats")
	}
}

func TestParseCount(t *testing.T) {
	for in, want := range map[string]struct {
		n  int64
		ok bool
	}{
		`7`: {7, true}, `7.0`: {7, true}, `"12"`: {12, true}, `0`: {0, true},
		`7.5`: {0, false}, `-1`: {0, false}, `"x"`: {0, false}, `true`: {0, false}, `null`: {0, false},
	} {
		if n, ok := parseCount(json.RawMessage(in)); n != want.n || ok != want.ok {
			t.Errorf("parseCount(%s) = %d, %v; want %d, %v", in, n, ok, want.n, want.ok)
		}
	}
}

func TestParseFixtureExceptions(t *testing.T) {
	base := raw(t, "fixture_5001_ft.json")
	for state, want := range map[string]d.Exception{
		"FT": d.ExceptionNone, "NS": d.ExceptionNone, "CANCELLED": d.ExceptionNone, "AWARDED": d.ExceptionNone,
		"POSTPONED": d.ExceptionPostponed, "SUSPENDED": d.ExceptionSuspended,
		"INTERRUPTED": d.ExceptionSuspended, "ABANDONED": d.ExceptionSuspended,
	} {
		r := base
		r.Body = bytes.Replace(base.Body, []byte(`"developer_name": "FT"`), []byte(`"developer_name": "`+state+`"`), 1)
		if bytes.Equal(r.Body, base.Body) && state != "FT" {
			t.Fatalf("%s: the state was not replaced", state)
		}
		f, err := ParseFixture(r)
		if err != nil {
			t.Fatal(err)
		}
		if f.Exception != want || f.Status != state {
			t.Errorf("%s: exception = %q, status = %q; want %q", state, f.Exception, f.Status, want)
		}
		if want != d.ExceptionNone && f.Final {
			t.Errorf("%s must not be final", state)
		}
	}
}

func TestParseSquad(t *testing.T) {
	r := raw(t, "squad_777_100.json")
	r.FetchedAt = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	sq, err := ParseSquad(r)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]provider.SquadMember{}
	for _, m := range sq.Members {
		by[m.PlayerRef] = m
	}
	if len(by) != 6 {
		t.Fatalf("members = %d", len(by))
	}
	gk := by["1001"]
	if gk.Name != "Home Keeper" || gk.Role != d.CatalogGoalkeeper || gk.BirthYear == nil || *gk.BirthYear != 1998 || !gk.Listed {
		t.Errorf("keeper = %+v", gk)
	}
	for ref, want := range map[string]d.CatalogRole{"1002": "A", "1003": "C", "1005": "D"} {
		if by[ref].Role != want {
			t.Errorf("%s role = %q, want %q", ref, by[ref].Role, want)
		}
	}
	if m := by["1004"]; m.Role != "" || m.BirthYear != nil || !m.Listed {
		t.Errorf("player without position and birth date = %+v (missing data must stay empty)", m)
	}
	if by["1099"].Listed {
		t.Errorf("player whose squad entry ended in January is still listed: %+v", by["1099"])
	}
}

func TestParseSquadRejectsBadEndDate(t *testing.T) {
	_, err := ParseSquad(provider.Raw{Key: "k", Body: []byte(`{"data":[{"player_id":1,"end":"soon"}]}`)})
	if !errors.Is(err, ErrParse) {
		t.Fatalf("err = %v", err)
	}
}
