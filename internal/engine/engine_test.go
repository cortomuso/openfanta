package engine

import (
	"math/rand/v2"
	"reflect"
	"testing"

	d "github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/rules"
)

func classic(t *testing.T) *rules.RuleSet {
	t.Helper()
	reg, err := rules.LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	rs, ok := reg.Get("classic", "1.0.0")
	if !ok {
		t.Fatal("classic@1.0.0 missing")
	}
	return rs
}

func pts(p d.Points) *d.Points { return &p }

func outfield(id string, mod func(*d.PlayerLine)) d.PlayerLine {
	l := d.PlayerLine{Player: d.PlayerID(id), Team: "H", Role: d.RoleOutfield, Minutes: 90}
	if mod != nil {
		mod(&l)
	}
	return l
}

func keeper(id string, mod func(*d.PlayerLine)) d.PlayerLine {
	l := outfield(id, mod)
	l.Role = d.RoleGoalkeeper
	return l
}

func item(code rules.Code, count int, points d.Points) BreakdownItem {
	return BreakdownItem{Code: code, Count: count, Points: points}
}

func TestClassic(t *testing.T) {
	cases := []struct {
		name      string
		line      d.PlayerLine
		voto      *d.Points // nil: the voto source gave no voto
		wantVoto  *d.Points
		wantFanta *d.Points
		wantSV    bool
		ufficio   bool
		breakdown []BreakdownItem
	}{
		{name: "plain", line: outfield("p", nil), voto: pts(600), wantVoto: pts(600), wantFanta: pts(600)},
		{
			name: "brace with assist", line: outfield("p", func(l *d.PlayerLine) { l.Goals, l.Assists = 2, 1 }),
			voto: pts(750), wantVoto: pts(750), wantFanta: pts(1450),
			breakdown: []BreakdownItem{item(rules.Goal, 2, 600), item(rules.Assist, 1, 100)},
		},
		{
			name: "penalty scored", line: outfield("p", func(l *d.PlayerLine) { l.PenaltiesScored = 1 }),
			voto: pts(650), wantVoto: pts(650), wantFanta: pts(950),
			breakdown: []BreakdownItem{item(rules.PenaltyScored, 1, 300)},
		},
		{
			name: "penalty missed", line: outfield("p", func(l *d.PlayerLine) { l.PenaltiesMissed = 1 }),
			voto: pts(500), wantVoto: pts(500), wantFanta: pts(200),
			breakdown: []BreakdownItem{item(rules.PenaltyMissed, 1, -300)},
		},
		{
			name: "own goal", line: outfield("p", func(l *d.PlayerLine) { l.OwnGoals = 1 }),
			voto: pts(550), wantVoto: pts(550), wantFanta: pts(350),
			breakdown: []BreakdownItem{item(rules.OwnGoal, 1, -200)},
		},
		{
			name: "yellow", line: outfield("p", func(l *d.PlayerLine) { l.YellowCards = 1 }),
			voto: pts(600), wantVoto: pts(600), wantFanta: pts(550),
			breakdown: []BreakdownItem{item(rules.YellowCard, 1, -50)},
		},
		{
			name: "second yellow counts only as red", line: outfield("p", func(l *d.PlayerLine) { l.YellowCards, l.SentOff, l.Minutes = 2, true, 60 }),
			voto: pts(500), wantVoto: pts(500), wantFanta: pts(400),
			breakdown: []BreakdownItem{item(rules.RedCard, 1, -100)},
		},
		{
			name: "straight red", line: outfield("p", func(l *d.PlayerLine) { l.SentOff, l.Minutes = true, 30 }),
			voto: pts(450), wantVoto: pts(450), wantFanta: pts(350),
			breakdown: []BreakdownItem{item(rules.RedCard, 1, -100)},
		},
		{
			name: "outfield conceded goals are ignored", line: outfield("p", func(l *d.PlayerLine) { l.GoalsConceded = 3 }),
			voto: pts(550), wantVoto: pts(550), wantFanta: pts(550),
		},
		{
			name: "keeper clean sheet", line: keeper("gk", nil),
			voto: pts(650), wantVoto: pts(650), wantFanta: pts(750),
			breakdown: []BreakdownItem{item(rules.GKCleanSheet, 1, 100)},
		},
		{
			name: "keeper concedes two", line: keeper("gk", func(l *d.PlayerLine) { l.GoalsConceded = 2 }),
			voto: pts(550), wantVoto: pts(550), wantFanta: pts(350),
			breakdown: []BreakdownItem{item(rules.GKGoalConceded, 2, -200)},
		},
		{
			name: "keeper saves a penalty and keeps a clean sheet", line: keeper("gk", func(l *d.PlayerLine) { l.PenaltiesSaved = 1 }),
			voto: pts(700), wantVoto: pts(700), wantFanta: pts(1100),
			breakdown: []BreakdownItem{item(rules.PenaltySaved, 1, 300), item(rules.GKCleanSheet, 1, 100)},
		},
		{
			name: "keeper subbed on late keeps a clean sheet", line: keeper("gk", func(l *d.PlayerLine) { l.Minutes = 20 }),
			voto: pts(600), wantVoto: pts(600), wantFanta: pts(700),
			breakdown: []BreakdownItem{item(rules.GKCleanSheet, 1, 100)},
		},
		{
			name: "s.v. without events", line: outfield("p", func(l *d.PlayerLine) { l.Minutes = 10 }),
			wantSV: true,
		},
		{
			name: "under min minutes is s.v. even with a voto", line: outfield("p", func(l *d.PlayerLine) { l.Minutes = 14 }),
			voto: pts(600), wantSV: true,
		},
		{
			name: "exactly min minutes keeps the voto", line: outfield("p", func(l *d.PlayerLine) { l.Minutes = 15 }),
			voto: pts(600), wantVoto: pts(600), wantFanta: pts(600),
		},
		{
			name: "s.v. with a goal gets voto d'ufficio 6", line: outfield("p", func(l *d.PlayerLine) { l.Minutes, l.Goals = 5, 1 }),
			wantVoto: pts(600), wantFanta: pts(900), ufficio: true,
			breakdown: []BreakdownItem{item(rules.Goal, 1, 300)},
		},
		{
			name: "s.v. with a red card gets voto d'ufficio 4", line: outfield("p", func(l *d.PlayerLine) { l.Minutes, l.SentOff = 3, true }),
			wantVoto: pts(400), wantFanta: pts(300), ufficio: true,
			breakdown: []BreakdownItem{item(rules.RedCard, 1, -100)},
		},
		{
			name: "s.v. red card wins over goal (first rule)", line: outfield("p", func(l *d.PlayerLine) { l.Minutes, l.Goals, l.SentOff = 8, 1, true }),
			wantVoto: pts(400), wantFanta: pts(600), ufficio: true,
			breakdown: []BreakdownItem{item(rules.Goal, 1, 300), item(rules.RedCard, 1, -100)},
		},
		{
			name: "s.v. with only an assist stays s.v.", line: outfield("p", func(l *d.PlayerLine) { l.Minutes, l.Assists = 5, 1 }),
			wantSV: true,
		},
		{
			name: "zero minutes stays s.v.", line: outfield("p", func(l *d.PlayerLine) { l.Minutes = 0 }),
			voto: pts(600), wantSV: true,
		},
		{
			name: "keeper without a voto", line: keeper("gk", func(l *d.PlayerLine) { l.GoalsConceded = 1 }),
			wantSV: true,
		},
	}
	rs := classic(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			votes := map[d.PlayerID]d.Points{}
			if c.voto != nil {
				votes[c.line.Player] = *c.voto
			}
			got := Compute([]d.PlayerLine{c.line}, votes, rs)
			if len(got) != 1 {
				t.Fatalf("got %d scores", len(got))
			}
			s := got[0]
			if !reflect.DeepEqual(s.Voto, c.wantVoto) || !reflect.DeepEqual(s.Fantavoto, c.wantFanta) {
				t.Errorf("voto, fantavoto = %v, %v; want %v, %v", deref(s.Voto), deref(s.Fantavoto), deref(c.wantVoto), deref(c.wantFanta))
			}
			if s.SenzaVoto != c.wantSV || s.VotoUfficio != c.ufficio {
				t.Errorf("sv, ufficio = %v, %v; want %v, %v", s.SenzaVoto, s.VotoUfficio, c.wantSV, c.ufficio)
			}
			if len(s.Breakdown) != 0 || len(c.breakdown) != 0 {
				if !reflect.DeepEqual(s.Breakdown, c.breakdown) {
					t.Errorf("breakdown = %+v, want %+v", s.Breakdown, c.breakdown)
				}
			}
			if s.Line != c.line {
				t.Errorf("line not carried through: %+v", s.Line)
			}
		})
	}
}

func deref(p *d.Points) string {
	if p == nil {
		return "nil"
	}
	return p.String()
}

func TestFantavotoEqualsVotoPlusBreakdown(t *testing.T) {
	rs := classic(t)
	r := rand.New(rand.NewPCG(7, 7))
	for range 2000 {
		l := d.PlayerLine{
			Player: "p", Team: "H", Role: []d.Role{d.RoleGoalkeeper, d.RoleOutfield}[r.IntN(2)],
			Minutes: r.IntN(91), Goals: r.IntN(3), PenaltiesScored: r.IntN(2), PenaltiesMissed: r.IntN(2),
			PenaltiesSaved: r.IntN(2), OwnGoals: r.IntN(2), Assists: r.IntN(3), YellowCards: r.IntN(3),
			SentOff: r.IntN(5) == 0, GoalsConceded: r.IntN(4),
		}
		votes := map[d.PlayerID]d.Points{}
		if r.IntN(4) > 0 {
			votes["p"] = d.Points(300 + 50*r.IntN(15))
		}
		s := Compute([]d.PlayerLine{l}, votes, rs)[0]
		if s.Voto == nil {
			if s.Fantavoto != nil || !s.SenzaVoto || len(s.Breakdown) != 0 {
				t.Fatalf("s.v. score inconsistent: %+v", s)
			}
			continue
		}
		sum := *s.Voto
		for _, b := range s.Breakdown {
			if b.Count <= 0 {
				t.Fatalf("breakdown item with non-positive count: %+v", b)
			}
			v, _ := rs.Scores.Value(b.Code)
			if b.Points != v*d.Points(b.Count) {
				t.Fatalf("item points mismatch: %+v", b)
			}
			sum += b.Points
		}
		if sum != *s.Fantavoto {
			t.Fatalf("fantavoto %s != voto + breakdown %s for %+v", s.Fantavoto, sum, l)
		}
	}
}

func TestDeterministicAndOrderIndependent(t *testing.T) {
	rs := classic(t)
	lines := []d.PlayerLine{
		keeper("gk", func(l *d.PlayerLine) { l.GoalsConceded = 1 }),
		outfield("b", func(l *d.PlayerLine) { l.Goals = 1 }),
		outfield("a", func(l *d.PlayerLine) { l.YellowCards = 1 }),
		outfield("c", func(l *d.PlayerLine) { l.Minutes = 3 }),
	}
	votes := map[d.PlayerID]d.Points{"gk": 600, "a": 550, "b": 700, "c": 600}
	want := Compute(lines, votes, rs)
	if want[0].Line.Player != "a" || want[3].Line.Player != "gk" {
		t.Errorf("output not sorted by player id: %v, %v", want[0].Line.Player, want[3].Line.Player)
	}
	r := rand.New(rand.NewPCG(3, 4))
	for range 50 {
		shuffled := append([]d.PlayerLine(nil), lines...)
		r.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		if got := Compute(shuffled, votes, rs); !reflect.DeepEqual(got, want) {
			t.Fatal("output depends on input order")
		}
	}
}

func TestCustomRuleSetValuesAreRespected(t *testing.T) {
	rs := *classic(t)
	rs.Scores.Goal = 350
	rs.Options.RedReplacesYellow = false
	rs.Options.GKCleanSheetMinMinutes = 90
	rs.SenzaVoto.MinMinutes = 0
	rs.SenzaVoto.VotoUfficio = nil

	lines := []d.PlayerLine{
		outfield("scorer", func(l *d.PlayerLine) { l.Goals = 1 }),
		outfield("booked", func(l *d.PlayerLine) { l.YellowCards, l.SentOff = 1, true }),
		keeper("late", func(l *d.PlayerLine) { l.Minutes = 45 }),
		outfield("cameo", func(l *d.PlayerLine) { l.Minutes, l.Goals = 1, 1 }),
	}
	votes := map[d.PlayerID]d.Points{"scorer": 600, "booked": 500, "late": 600, "cameo": 600}
	got := map[d.PlayerID]Score{}
	for _, s := range Compute(lines, votes, &rs) {
		got[s.Line.Player] = s
	}
	if f := *got["scorer"].Fantavoto; f != 950 {
		t.Errorf("scorer fantavoto %s, want 9.5", f)
	}
	if f := *got["booked"].Fantavoto; f != 350 {
		t.Errorf("booked fantavoto %s, want 3.5 (yellow and red both count)", f)
	}
	if f := *got["late"].Fantavoto; f != 600 {
		t.Errorf("late keeper fantavoto %s, want 6 (no clean sheet under 90 minutes)", f)
	}
	if f := got["cameo"].Fantavoto; f == nil || *f != 950 {
		t.Errorf("cameo should keep voto with min_minutes 0, got %v", deref(f))
	}
}
