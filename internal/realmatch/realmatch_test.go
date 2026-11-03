package realmatch

import (
	"errors"
	"testing"
	"time"

	d "github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/settings"
)

func pts(p d.Points) *d.Points { return &p }

func TestDecide(t *testing.T) {
	wait := settings.RealMatch{}
	auto := settings.RealMatch{OnPostponed: settings.PolicyAutomaticVoto, AutomaticVoto: pts(600)}
	mixed := settings.RealMatch{OnPostponed: settings.PolicyAutomaticVoto, OnSuspended: settings.PolicyWait, AutomaticVoto: pts(550)}
	onlyValue := settings.RealMatch{AutomaticVoto: pts(600)}

	cases := []struct {
		name string
		rm   settings.RealMatch
		e    d.Exception
		ov   *Override
		want Outcome
	}{
		{"normal match is scored", wait, d.ExceptionNone, nil, Outcome{Action: ActionScore, Origin: OriginRegular}},
		{"default waits when postponed", wait, d.ExceptionPostponed, nil, Outcome{Action: ActionWait, Origin: OriginLeague, Exception: d.ExceptionPostponed}},
		{"default waits when suspended", wait, d.ExceptionSuspended, nil, Outcome{Action: ActionWait, Origin: OriginLeague, Exception: d.ExceptionSuspended}},
		{"automatic voto when postponed", auto, d.ExceptionPostponed, nil, Outcome{Action: ActionAutomatic, Voto: 600, Origin: OriginLeague, Exception: d.ExceptionPostponed}},
		{"suspended follows its own policy", mixed, d.ExceptionSuspended, nil, Outcome{Action: ActionWait, Origin: OriginLeague, Exception: d.ExceptionSuspended}},
		{"postponed follows its own policy", mixed, d.ExceptionPostponed, nil, Outcome{Action: ActionAutomatic, Voto: 550, Origin: OriginLeague, Exception: d.ExceptionPostponed}},
		{"a normal match ignores the league policy", auto, d.ExceptionNone, nil, Outcome{Action: ActionScore, Origin: OriginRegular}},
		{"override beats the league policy", auto, d.ExceptionPostponed, &Override{Policy: settings.PolicyWait},
			Outcome{Action: ActionWait, Origin: OriginOverride, Exception: d.ExceptionPostponed}},
		{"override can hand out the league voto", onlyValue, d.ExceptionPostponed, &Override{Policy: settings.PolicyAutomaticVoto},
			Outcome{Action: ActionAutomatic, Voto: 600, Origin: OriginOverride, Exception: d.ExceptionPostponed}},
		{"override can set its own voto", auto, d.ExceptionSuspended, &Override{Policy: settings.PolicyAutomaticVoto, Voto: pts(650)},
			Outcome{Action: ActionAutomatic, Voto: 650, Origin: OriginOverride, Exception: d.ExceptionSuspended}},
		{"override on a match played outside its window", wait, d.ExceptionNone, &Override{Policy: settings.PolicyAutomaticVoto, Voto: pts(600)},
			Outcome{Action: ActionAutomatic, Voto: 600, Origin: OriginOverride}},
	}
	for _, c := range cases {
		got, err := Decide(c.rm, c.e, c.ov)
		if err != nil || got != c.want {
			t.Errorf("%s: got %+v, %v; want %+v", c.name, got, err, c.want)
		}
	}
}

func TestDecideErrors(t *testing.T) {
	if _, err := Decide(settings.RealMatch{}, d.ExceptionPostponed, &Override{Policy: settings.PolicyAutomaticVoto}); !errors.Is(err, ErrNoAutomaticVoto) {
		t.Errorf("no voto anywhere: err = %v", err)
	}
	if _, err := Decide(settings.RealMatch{}, d.ExceptionPostponed, &Override{Policy: "shrug"}); err == nil {
		t.Error("unknown policy accepted")
	}
}

func TestPickVotoKeepsOneVotoPerPlayer(t *testing.T) {
	t0 := time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)
	later := t0.Add(48 * time.Hour)
	played := func(f string, at time.Time, v d.Points) Candidate { return Candidate{Fixture: f, Kickoff: at, Voto: v} }
	auto := func(f string, at time.Time) Candidate {
		return Candidate{Fixture: f, Kickoff: at, Voto: 600, Automatic: true}
	}

	cases := []struct {
		name  string
		cands []Candidate
		want  string
	}{
		{"a real voto beats an automatic one, even an earlier one", []Candidate{auto("b", later), played("a", t0, 700)}, "a"},
		{"transfer: the later match, the new club", []Candidate{played("a", t0, 700), played("b", later, 650)}, "b"},
		{"order of the input does not matter", []Candidate{played("b", later, 650), played("a", t0, 700)}, "b"},
		{"two automatic votos: the later match", []Candidate{auto("a", t0), auto("b", later)}, "b"},
		{"unknown kickoff loses to a known one", []Candidate{played("a", time.Time{}, 700), played("b", t0, 650)}, "b"},
		{"same kickoff: the higher fixture id", []Candidate{played("a", t0, 700), played("b", t0, 650)}, "b"},
		{"a single candidate", []Candidate{played("only", t0, 640)}, "only"},
	}
	for _, c := range cases {
		got, ok := PickVoto(c.cands)
		if !ok || got.Fixture != c.want {
			t.Errorf("%s: got %q (%v), want %q", c.name, got.Fixture, ok, c.want)
		}
	}
	if _, ok := PickVoto(nil); ok {
		t.Error("no candidates still picked one")
	}
}
