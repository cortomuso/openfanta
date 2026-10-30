package timeline

import (
	"errors"
	"math/rand/v2"
	"reflect"
	"testing"

	d "github.com/openfanta/openfanta/internal/domain"
)

// match builds a 90-minute fixture H vs A with a starting XI of four per side
// (enough for the tests): hGK, h1, h2, h3 and aGK, a1, a2, a3; plus benches
// hGK2, h4 and aGK2, a4.
func match(events ...d.Event) d.MatchFacts {
	app := func(p, team string, gk, starter bool) d.Appearance {
		role := d.RoleOutfield
		if gk {
			role = d.RoleGoalkeeper
		}
		return d.Appearance{Player: d.PlayerID(p), Team: d.TeamID(team), Role: role, Starter: starter}
	}
	for i := range events {
		events[i].Seq = i + 1
	}
	return d.MatchFacts{
		HomeTeam: "H", AwayTeam: "A", Length: 90,
		Appearances: []d.Appearance{
			app("hGK", "H", true, true), app("h1", "H", false, true), app("h2", "H", false, true), app("h3", "H", false, true),
			app("hGK2", "H", true, false), app("h4", "H", false, false),
			app("aGK", "A", true, true), app("a1", "A", false, true), app("a2", "A", false, true), app("a3", "A", false, true),
			app("aGK2", "A", true, false), app("a4", "A", false, false),
		},
		Events: events,
	}
}

func ev(minute int, kind d.EventKind, team, player string, related ...string) d.Event {
	e := d.Event{Minute: minute, Kind: kind, Team: d.TeamID(team), Player: d.PlayerID(player)}
	if len(related) > 0 {
		e.Related = d.PlayerID(related[0])
	}
	return e
}

func byPlayer(t *testing.T, f d.MatchFacts) map[d.PlayerID]d.PlayerLine {
	t.Helper()
	lines, err := Derive(f)
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	m := map[d.PlayerID]d.PlayerLine{}
	for _, l := range lines {
		m[l.Player] = l
	}
	return m
}

func TestNoEvents(t *testing.T) {
	m := byPlayer(t, match())
	if len(m) != 8 {
		t.Fatalf("got %d players, want the 8 starters (unused bench excluded)", len(m))
	}
	want := d.PlayerLine{Player: "hGK", Team: "H", Role: d.RoleGoalkeeper, Minutes: 90}
	if m["hGK"] != want {
		t.Errorf("hGK = %+v", m["hGK"])
	}
	if _, ok := m["h4"]; ok {
		t.Error("unused substitute should not appear")
	}
}

func TestGoalsAndAssists(t *testing.T) {
	m := byPlayer(t, match(
		ev(10, d.EventGoal, "H", "h1", "h2"),
		ev(20, d.EventGoal, "H", "h1"),
		ev(30, d.EventPenaltyGoal, "A", "a1"),
		ev(40, d.EventPenaltyGoal, "A", "a1", "a2"),
	))
	if l := m["h1"]; l.Goals != 2 || l.Assists != 0 {
		t.Errorf("h1 = %+v", l)
	}
	if l := m["h2"]; l.Assists != 1 {
		t.Errorf("h2 = %+v", l)
	}
	if l := m["a1"]; l.PenaltiesScored != 2 || l.Goals != 0 {
		t.Errorf("a1 = %+v", l)
	}
	if l := m["a2"]; l.Assists != 1 {
		t.Errorf("a2 = %+v", l)
	}
	if m["hGK"].GoalsConceded != 2 || m["aGK"].GoalsConceded != 2 || m["h3"].GoalsConceded != 2 {
		t.Errorf("conceded: hGK=%d aGK=%d h3=%d", m["hGK"].GoalsConceded, m["aGK"].GoalsConceded, m["h3"].GoalsConceded)
	}
}

func TestOwnGoalCountsAgainstOwnTeam(t *testing.T) {
	m := byPlayer(t, match(ev(55, d.EventOwnGoal, "H", "h3")))
	if m["h3"].OwnGoals != 1 || m["h3"].Goals != 0 {
		t.Errorf("h3 = %+v", m["h3"])
	}
	if m["hGK"].GoalsConceded != 1 || m["aGK"].GoalsConceded != 0 {
		t.Errorf("hGK conceded %d, aGK conceded %d", m["hGK"].GoalsConceded, m["aGK"].GoalsConceded)
	}
}

func TestPenaltyMissedAndSaved(t *testing.T) {
	m := byPlayer(t, match(
		ev(10, d.EventPenaltyMissed, "H", "h1"),
		ev(20, d.EventPenaltySaved, "H", "h2"),          // saver inferred: aGK on pitch
		ev(30, d.EventPenaltySaved, "A", "a1", "hGK"),   // saver given explicitly
		ev(60, d.EventSubstitution, "A", "aGK", "aGK2"), // keeper change
		ev(70, d.EventPenaltySaved, "H", "h1"),          // now aGK2 is in goal
	))
	if m["h1"].PenaltiesMissed != 2 || m["h2"].PenaltiesMissed != 1 || m["a1"].PenaltiesMissed != 1 {
		t.Errorf("missed: h1=%d h2=%d a1=%d", m["h1"].PenaltiesMissed, m["h2"].PenaltiesMissed, m["a1"].PenaltiesMissed)
	}
	if m["aGK"].PenaltiesSaved != 1 || m["hGK"].PenaltiesSaved != 1 || m["aGK2"].PenaltiesSaved != 1 {
		t.Errorf("saved: aGK=%d hGK=%d aGK2=%d", m["aGK"].PenaltiesSaved, m["hGK"].PenaltiesSaved, m["aGK2"].PenaltiesSaved)
	}
	if m["aGK"].GoalsConceded+m["hGK"].GoalsConceded != 0 {
		t.Error("missed and saved penalties must not count as conceded")
	}
}

func TestSubstitutionMinutesAndConceded(t *testing.T) {
	m := byPlayer(t, match(
		ev(30, d.EventGoal, "A", "a1"),
		ev(60, d.EventSubstitution, "H", "hGK", "hGK2"),
		ev(60, d.EventGoal, "A", "a2"), // same minute, after the change by Seq
		ev(75, d.EventSubstitution, "H", "h1", "h4"),
	))
	if l := m["hGK"]; l.Minutes != 60 || l.GoalsConceded != 1 {
		t.Errorf("hGK = %+v", l)
	}
	if l := m["hGK2"]; l.Minutes != 30 || l.GoalsConceded != 1 {
		t.Errorf("hGK2 = %+v", l)
	}
	if l := m["h4"]; l.Minutes != 15 || l.GoalsConceded != 0 {
		t.Errorf("h4 = %+v", l)
	}
	if l := m["h1"]; l.Minutes != 75 || l.GoalsConceded != 2 {
		t.Errorf("h1 = %+v", l)
	}
}

func TestStoppageTimeIsClampedToLength(t *testing.T) {
	m := byPlayer(t, match(ev(94, d.EventSubstitution, "H", "h1", "h4")))
	if m["h1"].Minutes != 90 {
		t.Errorf("h1 minutes = %d, want 90", m["h1"].Minutes)
	}
	l, ok := m["h4"]
	if !ok || l.Minutes != 0 {
		t.Errorf("h4 should have played 0 minutes, got %+v (present %v)", l, ok)
	}
}

func TestCards(t *testing.T) {
	m := byPlayer(t, match(
		ev(10, d.EventYellowCard, "H", "h1"),
		ev(50, d.EventSecondYellow, "H", "h1"),
		ev(20, d.EventYellowCard, "A", "a1"),
		ev(70, d.EventRedCard, "A", "a2"),
		ev(80, d.EventGoal, "H", "h2"),
	))
	if l := m["h1"]; l.YellowCards != 2 || !l.SentOff || l.Minutes != 50 {
		t.Errorf("h1 = %+v", l)
	}
	if l := m["a1"]; l.YellowCards != 1 || l.SentOff || l.Minutes != 90 {
		t.Errorf("a1 = %+v", l)
	}
	if l := m["a2"]; l.YellowCards != 0 || !l.SentOff || l.Minutes != 70 || l.GoalsConceded != 0 {
		t.Errorf("a2 = %+v (sent off before the goal)", l)
	}
	if m["aGK"].GoalsConceded != 1 {
		t.Errorf("aGK conceded %d", m["aGK"].GoalsConceded)
	}
}

func TestGoalkeeperSentOffConcededSplit(t *testing.T) {
	m := byPlayer(t, match(
		ev(40, d.EventRedCard, "H", "hGK"),
		ev(41, d.EventSubstitution, "H", "h3", "hGK2"),
		ev(42, d.EventPenaltyGoal, "A", "a1"),
	))
	if l := m["hGK"]; l.GoalsConceded != 0 || l.Minutes != 40 || !l.SentOff {
		t.Errorf("hGK = %+v", l)
	}
	if l := m["hGK2"]; l.GoalsConceded != 1 || l.Minutes != 49 {
		t.Errorf("hGK2 = %+v", l)
	}
}

func TestEventOrderIsBySeqNotSliceOrder(t *testing.T) {
	f := match(
		ev(10, d.EventGoal, "H", "h1", "h2"),
		ev(60, d.EventSubstitution, "A", "aGK", "aGK2"),
		ev(61, d.EventGoal, "H", "h2"),
		ev(70, d.EventYellowCard, "A", "a1"),
		ev(80, d.EventSecondYellow, "A", "a1"),
	)
	want, err := Derive(f)
	if err != nil {
		t.Fatal(err)
	}
	r := rand.New(rand.NewPCG(1, 2))
	for range 50 {
		g := f
		g.Events = append([]d.Event(nil), f.Events...)
		g.Appearances = append([]d.Appearance(nil), f.Appearances...)
		r.Shuffle(len(g.Events), func(i, j int) { g.Events[i], g.Events[j] = g.Events[j], g.Events[i] })
		r.Shuffle(len(g.Appearances), func(i, j int) { g.Appearances[i], g.Appearances[j] = g.Appearances[j], g.Appearances[i] })
		got, err := Derive(g)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("shuffled input changed output: %v\n got %+v\nwant %+v", err, got, want)
		}
	}
}

func TestInvalidFacts(t *testing.T) {
	base := func() d.MatchFacts { return match(ev(10, d.EventGoal, "H", "h1")) }
	cases := map[string]func(*d.MatchFacts){
		"same teams":         func(f *d.MatchFacts) { f.AwayTeam = "H" },
		"zero length":        func(f *d.MatchFacts) { f.Length = 0 },
		"unknown event team": func(f *d.MatchFacts) { f.Events[0].Team = "X" },
		"unknown player":     func(f *d.MatchFacts) { f.Events[0].Player = "ghost" },
		"unknown related":    func(f *d.MatchFacts) { f.Events[0].Related = "ghost" },
		"player wrong team":  func(f *d.MatchFacts) { f.Events[0].Team = "A" },
		"duplicate seq": func(f *d.MatchFacts) {
			f.Events = append(f.Events, ev(20, d.EventGoal, "H", "h2"))
			f.Events[1].Seq = f.Events[0].Seq
		},
		"unknown kind":        func(f *d.MatchFacts) { f.Events[0].Kind = "bicycle_kick" },
		"negative minute":     func(f *d.MatchFacts) { f.Events[0].Minute = -1 },
		"duplicate player":    func(f *d.MatchFacts) { f.Appearances = append(f.Appearances, f.Appearances[0]) },
		"appearance bad team": func(f *d.MatchFacts) { f.Appearances[0].Team = "X" },
		"appearance bad role": func(f *d.MatchFacts) { f.Appearances[0].Role = "libero" },
		"sub without player in": func(f *d.MatchFacts) {
			f.Events[0] = d.Event{Seq: 1, Minute: 10, Kind: d.EventSubstitution, Team: "H", Player: "h1"}
		},
		"sub off player not on pitch": func(f *d.MatchFacts) {
			f.Events[0] = d.Event{Seq: 1, Minute: 10, Kind: d.EventSubstitution, Team: "H", Player: "h4", Related: "hGK2"}
		},
		"sub on player already on": func(f *d.MatchFacts) {
			f.Events[0] = d.Event{Seq: 1, Minute: 10, Kind: d.EventSubstitution, Team: "H", Player: "h1", Related: "h2"}
		},
		"event by player not on pitch": func(f *d.MatchFacts) { f.Events[0].Player = "h4" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := base()
			mutate(&f)
			if _, err := Derive(f); !errors.Is(err, ErrInvalidFacts) {
				t.Fatalf("err = %v, want ErrInvalidFacts", err)
			}
		})
	}
}

func TestPenaltySavedWithoutKeeperOnPitch(t *testing.T) {
	// The away side has no goalkeeper on the pitch after a red card and before a change.
	m := byPlayer(t, match(
		ev(40, d.EventRedCard, "A", "aGK"),
		ev(41, d.EventPenaltySaved, "H", "h1"),
	))
	if m["h1"].PenaltiesMissed != 1 {
		t.Errorf("h1 = %+v", m["h1"])
	}
	for _, l := range m {
		if l.PenaltiesSaved != 0 {
			t.Errorf("%s credited with a save without being in goal", l.Player)
		}
	}
}

func TestCardOnBenchPlayerIsAccepted(t *testing.T) {
	m := byPlayer(t, match(ev(70, d.EventYellowCard, "H", "h4")))
	if _, ok := m["h4"]; ok {
		t.Error("booked bench player did not play and must not appear")
	}
}

func TestRedCardAfterFinalWhistle(t *testing.T) {
	m := byPlayer(t, match(ev(94, d.EventRedCard, "H", "h1")))
	if l := m["h1"]; !l.SentOff || l.Minutes != 90 {
		t.Errorf("h1 = %+v, want sent off with the full 90 minutes", l)
	}
}

func TestRedCardOnBenchPlayerDoesNotMakeThemPlay(t *testing.T) {
	m := byPlayer(t, match(ev(70, d.EventRedCard, "H", "h4")))
	if _, ok := m["h4"]; ok {
		t.Error("a bench player sent off never played and must not appear")
	}
}

func TestCleanSheet(t *testing.T) {
	m := byPlayer(t, match(
		ev(60, d.EventSubstitution, "H", "hGK", "hGK2"),
		ev(70, d.EventGoal, "A", "a1"),
		ev(80, d.EventOwnGoal, "A", "a2"),
	))
	cases := map[d.PlayerID]bool{
		"hGK":  true,  // off before either goal
		"hGK2": false, // on for both goals, one of them an own goal
		"aGK":  false, // conceded to the own goal
		"h1":   false, // outfield players never keep one
	}
	for id, want := range cases {
		if got := m[id].CleanSheet(); got != want {
			t.Errorf("%s clean sheet = %v, want %v (%+v)", id, got, want, m[id])
		}
	}
	if !byPlayer(t, match())["aGK"].CleanSheet() {
		t.Error("goalless match: the starting keeper keeps a clean sheet")
	}
	if byPlayer(t, match())["aGK2"].CleanSheet() {
		t.Error("an unused substitute keeper did not play")
	}
}
