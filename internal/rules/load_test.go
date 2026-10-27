package rules

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/openfanta/openfanta/internal/domain"
)

func TestBuiltinClassic(t *testing.T) {
	reg, err := LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	rs, ok := reg.Get("classic", "1.0.0")
	if !ok {
		t.Fatal("classic@1.0.0 not found")
	}
	want := Scores{
		Goal: 300, PenaltyScored: 300, PenaltyMissed: -300, PenaltySaved: 300, OwnGoal: -200,
		Assist: 100, YellowCard: -50, RedCard: -100, GKGoalConceded: -100, GKCleanSheet: 100,
	}
	if rs.Scores != want {
		t.Errorf("scores = %+v, want %+v", rs.Scores, want)
	}
	if !rs.Options.RedReplacesYellow || rs.SenzaVoto.MinMinutes != 15 {
		t.Errorf("options = %+v, senza voto = %+v", rs.Options, rs.SenzaVoto)
	}
	if got := rs.SenzaVoto.VotoUfficio[0]; got.When != RedCard || got.Voto != 400 {
		t.Errorf("first voto ufficio = %+v", got)
	}
	if rs.Key() != "classic@1.0.0" {
		t.Errorf("key = %s", rs.Key())
	}
}

const valid = `
id: test
version: 1.2.3
name: Test
scores: {goal: 300, penalty_scored: 300, penalty_missed: -300, penalty_saved: 300, own_goal: -200,
  assist: 100, yellow_card: -50, red_card: -100, gk_goal_conceded: -100, gk_clean_sheet: 100}
options: {red_replaces_yellow: true, gk_clean_sheet_min_minutes: 0}
senza_voto: {min_minutes: 0, voto_ufficio: []}
`

func TestParseInvalid(t *testing.T) {
	cases := map[string]string{
		"not yaml":         "id: [",
		"missing scores":   strings.Replace(valid, "scores:", "scoresx:", 1),
		"unknown key":      valid + "extra: 1\n",
		"float points":     strings.Replace(valid, "goal: 300", "goal: 3.5", 1),
		"string points":    strings.Replace(valid, "goal: 300", `goal: "3"`, 1),
		"bad version":      strings.Replace(valid, "1.2.3", "v1", 1),
		"bad id":           strings.Replace(valid, "id: test", "id: Test!", 1),
		"unknown event":    strings.Replace(valid, "voto_ufficio: []", "voto_ufficio: [{when: hat_trick, voto: 600}]", 1),
		"voto too large":   strings.Replace(valid, "voto_ufficio: []", "voto_ufficio: [{when: goal, voto: 1001}]", 1),
		"negative minutes": strings.Replace(valid, "min_minutes: 0", "min_minutes: -1", 1),
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(src)); !errors.Is(err, ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestParseJSONIsAccepted(t *testing.T) {
	src := `{"id":"j","version":"0.1.0","name":"J","scores":{"goal":1,"penalty_scored":1,"penalty_missed":1,
"penalty_saved":1,"own_goal":1,"assist":1,"yellow_card":1,"red_card":1,"gk_goal_conceded":1,"gk_clean_sheet":1},
"options":{"red_replaces_yellow":false,"gk_clean_sheet_min_minutes":60},"senza_voto":{"min_minutes":10,"voto_ufficio":[]}}`
	rs, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if rs.Options.GKCleanSheetMinMinutes != 60 || rs.Scores.Goal != domain.Points(1) {
		t.Errorf("parsed = %+v", rs)
	}
}

func TestHashIgnoresFormattingButNotValues(t *testing.T) {
	a, _ := Parse([]byte(valid))
	b, _ := Parse([]byte("# a comment\n" + strings.ReplaceAll(valid, ", ", ",\n  ")))
	c, _ := Parse([]byte(strings.Replace(valid, "assist: 100", "assist: 50", 1)))
	if a.Hash() != b.Hash() {
		t.Error("hash changed with formatting only")
	}
	if a.Hash() == c.Hash() {
		t.Error("hash did not change with a value change")
	}
	if len(a.Hash()) != 64 {
		t.Errorf("hash length = %d", len(a.Hash()))
	}
}

func TestScoresValue(t *testing.T) {
	rs, _ := Parse([]byte(valid))
	for _, c := range AllScoreCodes {
		if _, ok := rs.Scores.Value(c); !ok {
			t.Errorf("Value(%s) not found", c)
		}
	}
	if _, ok := rs.Scores.Value("nope"); ok {
		t.Error("unknown code reported as found")
	}
}

func TestRegistry(t *testing.T) {
	v := func(ver string) *fstest.MapFile {
		return &fstest.MapFile{Data: []byte(strings.Replace(valid, "1.2.3", ver, 1))}
	}
	fsys := fstest.MapFS{
		"a.yaml": v("1.2.3"), "b.yml": v("1.10.0"), "c.yaml": v("1.9.9"),
		"README.md": {Data: []byte("ignored")}, "sub/x.yaml": v("9.9.9"),
	}
	reg, err := LoadFS(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if latest, _ := reg.Latest("test"); latest.Version != "1.10.0" {
		t.Errorf("latest = %s, want 1.10.0", latest.Version)
	}
	if _, ok := reg.Latest("missing"); ok {
		t.Error("latest of missing id found")
	}
	var versions []string
	for _, rs := range reg.All() {
		versions = append(versions, rs.Version)
	}
	if got := strings.Join(versions, ","); got != "1.2.3,1.9.9,1.10.0" {
		t.Errorf("All() order = %s", got)
	}
	if _, ok := reg.Get("test", "1.9.9"); !ok {
		t.Error("Get 1.9.9 failed")
	}
}

func TestRegistryErrors(t *testing.T) {
	dup := fstest.MapFS{"a.yaml": {Data: []byte(valid)}, "b.yaml": {Data: []byte(valid)}}
	if _, err := LoadFS(dup); !errors.Is(err, ErrInvalid) {
		t.Errorf("duplicate: err = %v", err)
	}
	bad := fstest.MapFS{"a.yaml": {Data: []byte("id: x")}}
	if _, err := LoadFS(bad); !errors.Is(err, ErrInvalid) {
		t.Errorf("bad file: err = %v", err)
	}
}

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		sign int
	}{
		{"1.0.0", "1.0.0", 0}, {"1.0.0", "1.0.1", -1}, {"2.0.0", "10.0.0", -1}, {"1.2.0", "1.1.9", 1},
	}
	for _, c := range cases {
		got := compareSemver(c.a, c.b)
		if (got < 0 && c.sign >= 0) || (got > 0 && c.sign <= 0) || (got == 0 && c.sign != 0) {
			t.Errorf("compareSemver(%s, %s) = %d, want sign %d", c.a, c.b, got, c.sign)
		}
	}
}

func TestRegistryAllOrdersByID(t *testing.T) {
	other := strings.Replace(valid, "id: test", "id: alpha", 1)
	reg, err := LoadFS(fstest.MapFS{"a.yaml": {Data: []byte(valid)}, "b.yaml": {Data: []byte(other)}})
	if err != nil {
		t.Fatal(err)
	}
	if all := reg.All(); all[0].ID != "alpha" || all[1].ID != "test" {
		t.Errorf("All() = %s, %s", all[0].ID, all[1].ID)
	}
}

func TestMergeAndLatestOfEach(t *testing.T) {
	a, _ := LoadFS(fstest.MapFS{"a.yaml": {Data: []byte(valid)}})
	b, _ := LoadFS(fstest.MapFS{
		"b.yaml": {Data: []byte(strings.Replace(valid, "1.2.3", "2.0.0", 1))},
		"c.yaml": {Data: []byte(strings.Replace(valid, "id: test", "id: alpha", 1))},
	})
	m, err := Merge(a, b)
	if err != nil {
		t.Fatal(err)
	}
	latest := m.LatestOfEach()
	if len(latest) != 2 || latest[0].Key() != "alpha@1.2.3" || latest[1].Key() != "test@2.0.0" {
		t.Errorf("latest = %v, %v", latest[0].Key(), latest[1].Key())
	}
	if _, err := Merge(a, a); !errors.Is(err, ErrInvalid) {
		t.Errorf("merge duplicate: %v", err)
	}
}
