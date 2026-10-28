package ingest

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/openfanta/openfanta/internal/provider/httpx"
	"github.com/openfanta/openfanta/internal/provider/sportmonks"
	"github.com/openfanta/openfanta/internal/provider/sportmonks/smtest"
	"github.com/openfanta/openfanta/internal/settings"
	"github.com/openfanta/openfanta/internal/store"
	"github.com/openfanta/openfanta/internal/store/db"
	"github.com/openfanta/openfanta/internal/store/storetest"
	"github.com/openfanta/openfanta/internal/voto"
)

var ctx = context.Background()

func newService(t *testing.T, st *store.Store, srv *httptest.Server, st2 *settings.Settings) *Service {
	t.Helper()
	hc := httpx.New(nil, nil)
	hc.MaxRetries = 0
	sm := sportmonks.New("token", hc)
	sm.BaseURL = srv.URL
	return &Service{
		Store: st, Provider: sm, Settings: st2, Voto: voto.NewProviderRating(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: time.Now,
	}
}

type scoreView struct {
	voto, fanta string
	sv          bool
}

func scores(t *testing.T, st *store.Store, fixture uuid.UUID) map[string]scoreView {
	t.Helper()
	return scoresUnder(t, st, fixture, settings.Classic())
}

func scoresUnder(t *testing.T, st *store.Store, fixture uuid.UUID, set *settings.Settings) map[string]scoreView {
	t.Helper()
	rows, err := st.Queries().FixtureScores(ctx, db.FixtureScoresParams{FixtureID: fixture, SettingsSha256: set.SHA256, VotoSource: "provider-rating"})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]scoreView{}
	for _, r := range rows {
		sv := scoreView{voto: "-", fanta: "-", sv: r.SenzaVoto}
		if r.Voto != nil {
			sv.voto = pointsString(*r.Voto)
		}
		if r.Fantavoto != nil {
			sv.fanta = pointsString(*r.Fantavoto)
		}
		out[r.PlayerName] = sv
	}
	return out
}

func TestIngestMatchdayEndToEnd(t *testing.T) {
	st, _ := storetest.New(t)
	up, srv := smtest.New(t)
	svc := newService(t, st, srv, nil)

	rep, err := svc.IngestMatchday(ctx, "777", 1, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Fixtures) != 2 || rep.Failed() != 1 {
		t.Fatalf("report = %+v", rep)
	}
	var ok FixtureReport
	for _, f := range rep.Fixtures {
		if f.Ref == "5001" {
			ok = f
		} else if f.Err == nil || !httpx.IsStatus(f.Err, 404) {
			t.Errorf("fixture %s should fail with 404, got %v", f.Ref, f.Err)
		}
	}
	if ok.Err != nil || !ok.Final || ok.RunsCreated != 1 || ok.FromCache {
		t.Fatalf("fixture 5001 = %+v", ok)
	}

	// Classic with unrounded provider-rating voti, computed by hand from the fixture:
	got := scores(t, st, ok.ID)
	want := map[string]scoreView{
		"Home Keeper":         {"6.90", "8.90", false},  // penalty saved +3, conceded 1 −1
		"Home Striker":        {"8.10", "12.10", false}, // goal +3, assist +1
		"Home Winger":         {"7.00", "8.00", false},  // assist +1
		"Home Midfielder":     {"5.60", "2.60", false},  // penalty missed −3
		"Home Defender":       {"5.40", "4.40", false},  // red −1
		"Home Super Sub":      {"7.24", "10.24", false}, // goal +3
		"Away Keeper":         {"6.30", "4.30", false},  // conceded 2
		"Away Defender":       {"5.00", "4.00", false},  // second yellow: red only
		"Away Striker":        {"6.25", "6.25", false},  // penalty +3, penalty saved −3
		"Away Winger":         {"6.00", "6.00", false},
		"Away Centre Back":    {"5.50", "3.50", false}, // own goal −2
		"Away Reserve Keeper": {"-", "-", true},        // 10 minutes < 15: s.v.
	}
	if len(got) != len(want) {
		t.Errorf("got %d scored players, want %d: %v", len(got), len(want), got)
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s = %+v, want %+v", name, got[name], w)
		}
	}

	// Second run: the finished fixture comes from the cache, nothing new is computed.
	rep, err = svc.IngestMatchday(ctx, "777", 1, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range rep.Fixtures {
		if f.Ref == "5001" && (!f.FromCache || f.RunsCreated != 0) {
			t.Errorf("re-ingest = %+v", f)
		}
	}
	if n := up.Count("/fixtures/5001"); n != 1 {
		t.Errorf("finished fixture fetched %d times, want 1", n)
	}
	if n := up.Count("/seasons/777") + up.Count("/schedules/seasons/777"); n != 2 {
		t.Errorf("season and schedule fetched %d times, want 2 (cached)", n)
	}
}

func TestCorrectedUpstreamEventCreatesAuditableRun(t *testing.T) {
	st, _ := storetest.New(t)
	up, srv := smtest.New(t)
	svc := newService(t, st, srv, nil)
	rep, err := svc.IngestMatchday(ctx, "777", 1, Options{})
	if err != nil {
		t.Fatal(err)
	}
	id := rep.Fixtures[0].ID
	if rep.Fixtures[0].Ref != "5001" {
		id = rep.Fixtures[1].ID
	}

	// Upstream corrects the first goal: no assist after all.
	b := []byte(smtest.Fixture(t, "fixture_5001_ft.json"))
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	changed := false
	for _, e := range doc["data"].(map[string]any)["events"].([]any) {
		ev := e.(map[string]any)
		if ev["id"] == float64(9001) {
			ev["related_player_id"] = nil
			changed = true
		}
	}
	if !changed {
		t.Fatal("event 9001 not found in the test fixture")
	}
	out, _ := json.Marshal(doc)
	corrected := string(out)
	up.Set("/fixtures/5001", corrected)

	rep, err = svc.IngestMatchday(ctx, "777", 1, Options{Refresh: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range rep.Fixtures {
		if f.Ref == "5001" && f.RunsCreated != 1 {
			t.Fatalf("corrected fixture = %+v", f)
		}
	}
	if got := scores(t, st, id)["Home Winger"]; got.fanta != "7.00" {
		t.Errorf("Home Winger after correction = %+v, want fantavoto 7.00", got)
	}
	runs, err := st.Queries().FixtureRuns(ctx, id)
	if err != nil || len(runs) != 2 {
		t.Fatalf("runs = %d, %v", len(runs), err)
	}
	if runs[1].SupersedesRunID == nil || *runs[1].SupersedesRunID != runs[0].ID || !runs[1].IsCurrent {
		t.Errorf("history = %+v", runs)
	}
}

func TestLiveFixtureIsNotScoredUntilFinal(t *testing.T) {
	st, _ := storetest.New(t)
	up, srv := smtest.New(t)
	ft := smtest.Fixture(t, "fixture_5001_ft.json")
	up.Set("/fixtures/5001", smtest.Fixture(t, "fixture_5001_live.json"))
	svc := newService(t, st, srv, nil)

	rep, _ := svc.IngestMatchday(ctx, "777", 1, Options{})
	for _, f := range rep.Fixtures {
		if f.Ref == "5001" && (f.Final || f.RunsCreated != 0 || f.Err != nil) {
			t.Fatalf("live fixture = %+v", f)
		}
	}
	up.Set("/fixtures/5001", ft)
	rep, _ = svc.IngestMatchday(ctx, "777", 1, Options{})
	for _, f := range rep.Fixtures {
		if f.Ref == "5001" && (!f.Final || f.RunsCreated != 1 || f.FromCache) {
			t.Fatalf("finished fixture = %+v (live payloads must not be cached as final)", f)
		}
	}
}

func TestRecomputeWithNewRuleSetVersion(t *testing.T) {
	st, _ := storetest.New(t)
	_, srv := smtest.New(t)
	svc := newService(t, st, srv, nil)
	if _, err := svc.IngestMatchday(ctx, "777", 1, Options{}); err != nil {
		t.Fatal(err)
	}

	// Nothing changed: recompute and reparse create no runs.
	for _, reparse := range []bool{false, true} {
		rep, err := svc.Recompute(ctx, RecomputeOptions{Reparse: reparse})
		if err != nil || len(rep.Fixtures) != 1 || rep.Fixtures[0].RunsCreated != 0 || rep.Failed() != 0 {
			t.Fatalf("reparse=%v: %+v, %v", reparse, rep, err)
		}
	}

	// Scoring under different settings (a bigger goal bonus) adds a run under
	// the new hash and leaves the Classic run current.
	bumped, err := settings.Parse([]byte(strings.Replace(string(settings.Classic().Canonical), `"goal":300`, `"goal":350`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	svc = newService(t, st, srv, bumped)
	rep, err := svc.Recompute(ctx, RecomputeOptions{Reason: "goal 3.5"})
	if err != nil || rep.Fixtures[0].RunsCreated != 1 {
		t.Fatalf("recompute = %+v, %v", rep, err)
	}
	id := rep.Fixtures[0].ID
	if got := scoresUnder(t, st, id, bumped)["Home Striker"]; got.fanta != "12.60" {
		t.Errorf("Home Striker under bumped settings = %+v", got)
	}
	if got := scores(t, st, id)["Home Striker"]; got.fanta == "12.60" {
		t.Errorf("Classic run changed: %+v", got)
	}
	runs, _ := st.Queries().FixtureRuns(ctx, id)
	if len(runs) != 2 || runs[1].SettingsSha256 != bumped.SHA256 || runs[1].Reason != "goal 3.5" || !runs[0].IsCurrent || !runs[1].IsCurrent {
		t.Errorf("runs = %+v", runs)
	}
}

func TestUnknownMatchday(t *testing.T) {
	st, _ := storetest.New(t)
	_, srv := smtest.New(t)
	svc := newService(t, st, srv, nil)
	if _, err := svc.IngestMatchday(ctx, "777", 38, Options{}); err == nil {
		t.Fatal("expected an error for a matchday without fixtures")
	}
}
