package openstat

import (
	"reflect"
	"strings"
	"testing"

	d "github.com/openfanta/openfanta/internal/domain"
)

type stats = map[d.StatKey]int64

func app(id string, team d.TeamID, role d.Role, pos d.Position, starter bool, s stats) d.Appearance {
	return d.Appearance{Player: d.PlayerID(id), Team: team, Role: role, Position: pos, Starter: starter, HasStats: true, Stats: s}
}

func goal(seq, minute int, team d.TeamID, player string) d.Event {
	return d.Event{Seq: seq, Minute: minute, Kind: d.EventGoal, Team: team, Player: d.PlayerID(player)}
}

// match builds H vs A with the given appearances and events.
func match(as []d.Appearance, es ...d.Event) d.MatchFacts {
	return d.MatchFacts{HomeTeam: "H", AwayTeam: "A", Length: 90, Appearances: as, Events: es}
}

func rate(t *testing.T, f d.MatchFacts, id string) Rating {
	t.Helper()
	got, err := Default().Rate(f)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := got[d.PlayerID(id)]
	if !ok {
		t.Fatalf("player %s has no rating", id)
	}
	return r
}

func TestForwardWhoScoresInAWinIsWellRated(t *testing.T) {
	f := match([]d.Appearance{
		app("f", "H", d.RoleOutfield, d.PositionForward, true, stats{d.StatShotsOnTarget: 2, d.StatPasses: 20, d.StatAccuratePasses: 16}),
		app("gk", "A", d.RoleGoalkeeper, d.PositionGoalkeeper, true, stats{d.StatSaves: 3}),
	}, goal(1, 30, "H", "f"))
	if r := rate(t, f, "f"); r.Voto < 650 {
		t.Errorf("forward voto = %d, want >= 650 (%+v)", r.Voto, r)
	}
}

func TestKeeperWhoConcedesFourIsPoorlyRated(t *testing.T) {
	f := match([]d.Appearance{
		app("gk", "H", d.RoleGoalkeeper, d.PositionGoalkeeper, true, stats{d.StatSaves: 3, d.StatSavesInsideBox: 2}),
		app("f", "A", d.RoleOutfield, d.PositionForward, true, nil),
	}, goal(1, 10, "A", "f"), goal(2, 30, "A", "f"), goal(3, 50, "A", "f"), goal(4, 70, "A", "f"))
	if r := rate(t, f, "gk"); r.Voto > 550 {
		t.Errorf("keeper voto = %d, want <= 550 (%+v)", r.Voto, r)
	}
}

func TestCentreBackWithACleanSheetWinSitsNearTheBase(t *testing.T) {
	f := match([]d.Appearance{
		app("cb", "H", d.RoleOutfield, d.PositionDefender, true, stats{d.StatClearances: 1}),
		app("f", "H", d.RoleOutfield, d.PositionForward, true, nil),
		app("gk", "A", d.RoleGoalkeeper, d.PositionGoalkeeper, true, nil),
	}, goal(1, 20, "H", "f"))
	if r := rate(t, f, "cb"); r.Voto < 600 || r.Voto > 650 {
		t.Errorf("centre back voto = %d, want 6.00 to 6.50 (%+v)", r.Voto, r)
	}
}

func TestShortAppearancesAreShrunkTowardTheBase(t *testing.T) {
	same := stats{d.StatKeyPasses: 1}
	f := match([]d.Appearance{
		app("starter", "H", d.RoleOutfield, d.PositionMidfielder, true, same),
		app("off", "H", d.RoleOutfield, d.PositionMidfielder, true, nil),
		app("cameo", "H", d.RoleOutfield, d.PositionMidfielder, false, same),
	}, d.Event{Seq: 1, Minute: 80, Kind: d.EventSubstitution, Team: "H", Player: "off", Related: "cameo"})
	full, cameo := rate(t, f, "starter"), rate(t, f, "cameo")
	if full.Delta != cameo.Delta {
		t.Fatalf("unscaled deltas differ: %d vs %d", full.Delta, cameo.Delta)
	}
	if cameo.Scaled <= 0 || cameo.Scaled >= full.Scaled {
		t.Errorf("scaled: cameo %d, full match %d; the cameo should move less", cameo.Scaled, full.Scaled)
	}
	// 15 points over 10 minutes: 15*10/(10+15) = 6; over 90: 15*90/105 = 12.86 -> 13.
	if cameo.Scaled != 6 || full.Scaled != 13 {
		t.Errorf("scaled = %d and %d, want 6 and 13", cameo.Scaled, full.Scaled)
	}
}

func TestAnOmittedStatIsZeroAndNoBlockIsUnknown(t *testing.T) {
	omitted := app("a", "H", d.RoleOutfield, d.PositionMidfielder, true, nil)
	explicit := app("b", "H", d.RoleOutfield, d.PositionMidfielder, true, stats{d.StatInterceptions: 0, d.StatTacklesWon: 0})
	none := app("c", "H", d.RoleOutfield, d.PositionMidfielder, true, nil)
	none.HasStats = false
	f := match([]d.Appearance{omitted, explicit, none})

	got, err := Default().Rate(f)
	if err != nil {
		t.Fatal(err)
	}
	if got["a"].Voto != got["b"].Voto || got["a"].Voto != 600 {
		t.Errorf("omitted = %d, explicit zero = %d; both should be the base 600", got["a"].Voto, got["b"].Voto)
	}
	if _, ok := got["c"]; ok {
		t.Error("a player without a statistics block was rated")
	}
}

func TestUnusedBenchPlayerIsNotRated(t *testing.T) {
	f := match([]d.Appearance{
		app("p", "H", d.RoleOutfield, d.PositionMidfielder, true, nil),
		app("bench", "H", d.RoleOutfield, d.PositionMidfielder, false, nil),
	})
	got, _ := Default().Rate(f)
	if _, ok := got["bench"]; ok {
		t.Error("a player who never took the pitch was rated")
	}
}

func TestUnknownPositionUsesMidfielderWeights(t *testing.T) {
	s := stats{d.StatInterceptions: 2, d.StatAerialsWon: 2} // defenders weigh these more than midfielders
	f := match([]d.Appearance{
		app("unknown", "H", d.RoleOutfield, "", true, s),
		app("mid", "H", d.RoleOutfield, d.PositionMidfielder, true, s),
		app("def", "H", d.RoleOutfield, d.PositionDefender, true, s),
		app("keeper", "H", d.RoleGoalkeeper, "", true, stats{d.StatSaves: 2}),
	})
	if u, m, df := rate(t, f, "unknown"), rate(t, f, "mid"), rate(t, f, "def"); u.Voto != m.Voto || u.Voto >= df.Voto {
		t.Errorf("unknown %d, mid %d, def %d", u.Voto, m.Voto, df.Voto)
	}
	// A keeper without a position still gets the keeper column: two saves
	// (15 each) and the clean sheet (30).
	if r := rate(t, f, "keeper"); r.Delta != 60 {
		t.Errorf("keeper delta = %d, want 60 (%+v)", r.Delta, r.Items)
	}
}

func TestKeyPassesFallBackToChancesCreatedButAreNeverSummed(t *testing.T) {
	both := app("both", "H", d.RoleOutfield, d.PositionMidfielder, true, stats{d.StatKeyPasses: 2, d.StatChancesCreated: 2})
	only := app("only", "H", d.RoleOutfield, d.PositionMidfielder, true, stats{d.StatChancesCreated: 2})
	f := match([]d.Appearance{both, only})
	if b, o := rate(t, f, "both"), rate(t, f, "only"); b.Delta != 30 || o.Delta != 30 {
		t.Errorf("deltas = %d and %d, want 30 each", b.Delta, o.Delta)
	}
}

func TestDerivedCountsAndEventTerms(t *testing.T) {
	f := match([]d.Appearance{
		app("m", "H", d.RoleOutfield, d.PositionMidfielder, true, stats{
			d.StatPasses: 30, d.StatAccuratePasses: 25, // 5 inaccurate
			d.StatDribbles: 4, d.StatDribblesWon: 1, // 3 failed
			d.StatDuels: 10, d.StatDuelsWon: 4, // 6 lost
		}),
		app("x", "A", d.RoleOutfield, d.PositionForward, true, nil),
	}, d.Event{Seq: 1, Minute: 50, Kind: d.EventYellowCard, Team: "H", Player: "m"})
	r := rate(t, f, "m")
	got := map[string]int64{}
	for _, it := range r.Items {
		got[it.Term] = it.Count
	}
	want := map[string]int64{"accurate_pass": 25, "inaccurate_pass": 5, "dribble_won": 1, "failed_dribble": 3, "duel_won": 4, "duel_lost": 6, "yellow": 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
	var sum d.Points
	for _, it := range r.Items {
		sum += it.Points
	}
	if sum != r.Delta {
		t.Errorf("items sum to %d, delta is %d", sum, r.Delta)
	}
}

func TestVotoIsClamped(t *testing.T) {
	huge := stats{d.StatTacklesWon: 100, d.StatInterceptions: 100}
	awful := stats{d.StatErrorsToGoal: 50}
	f := match([]d.Appearance{
		app("hi", "H", d.RoleOutfield, d.PositionDefender, true, huge),
		app("lo", "H", d.RoleOutfield, d.PositionDefender, true, awful),
	})
	if hi := rate(t, f, "hi"); hi.Voto != 1000 {
		t.Errorf("max: %d", hi.Voto)
	}
	if lo := rate(t, f, "lo"); lo.Voto != 300 {
		t.Errorf("min: %d", lo.Voto)
	}
}

func TestDeterministic(t *testing.T) {
	f := match([]d.Appearance{
		app("a", "H", d.RoleOutfield, d.PositionForward, true, stats{d.StatShotsOnTarget: 3, d.StatKeyPasses: 2, d.StatFouls: 1}),
		app("b", "A", d.RoleGoalkeeper, d.PositionGoalkeeper, true, stats{d.StatSaves: 5}),
	}, goal(1, 10, "H", "a"))
	first, _ := Default().Rate(f)
	for i := 0; i < 20; i++ {
		again, _ := Default().Rate(f)
		if !reflect.DeepEqual(first, again) {
			t.Fatal("ratings differ between runs")
		}
	}
}

func TestScoreCreditsOwnGoalsToTheOtherSide(t *testing.T) {
	f := match(nil,
		goal(1, 5, "H", "x"),
		d.Event{Seq: 2, Minute: 20, Kind: d.EventOwnGoal, Team: "H", Player: "y"},
		d.Event{Seq: 3, Minute: 30, Kind: d.EventPenaltyGoal, Team: "A", Player: "z"},
	)
	if h, a := Score(f); h != 1 || a != 2 {
		t.Errorf("score = %d-%d, want 1-2", h, a)
	}
}

func TestCurve(t *testing.T) {
	m := New(Params{Curve: [][2]int64{{-100, -50}, {0, 0}, {100, 200}}})
	for _, c := range []struct{ in, want int64 }{
		{-500, -50}, {-100, -50}, {-50, -25}, {0, 0}, {25, 50}, {99, 198}, {100, 200}, {900, 200},
	} {
		if got := m.curve(c.in); got != c.want {
			t.Errorf("curve(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestDivRound(t *testing.T) {
	for _, c := range []struct{ a, b, want int64 }{
		{5, 2, 3}, {-5, 2, -3}, {4, 3, 1}, {-4, 3, -1}, {3, 2, 2}, {0, 7, 0}, {1350, 105, 13},
	} {
		if got := divRound(c.a, c.b); got != c.want {
			t.Errorf("%d/%d = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestParamsValidation(t *testing.T) {
	good, err := LoadParams(paramsV1)
	if err != nil || good.Version != "v1" {
		t.Fatalf("embedded params: %+v, %v", good, err)
	}
	for name, edit := range map[string][2]string{
		"unknown term":        {`"goal":`, `"gaol":`},
		"unknown position":    {`"def": 140`, `"wing": 140`},
		"unknown field":       {`"version": "v1"`, `"version": "v1", "extra": 1`},
		"base outside bounds": {`"base": 600`, `"base": 2000`},
		"flat curve":          {`[[-10000, -10000], [10000, 10000]]`, `[[0, 0], [0, 5]]`},
		"no version":          {`"version": "v1"`, `"version": ""`},
		"negative cap":        {`"cap": 30`, `"cap": -30`},
	} {
		doc := strings.Replace(string(paramsV1), edit[0], edit[1], 1)
		if doc == string(paramsV1) {
			t.Fatalf("%s: edit did not apply", name)
		}
		if _, err := LoadParams([]byte(doc)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
