package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	d "github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/engine"
	"github.com/openfanta/openfanta/internal/rules"
	"github.com/openfanta/openfanta/internal/store"
	"github.com/openfanta/openfanta/internal/store/db"
	"github.com/openfanta/openfanta/internal/store/storetest"
)

var ctx = context.Background()

func TestRawPayloadsAreAppendOnlyAndDeduplicated(t *testing.T) {
	s, _ := storetest.New(t)
	at := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	p := store.RawPayload{Provider: "fake", RequestKey: "fixtures/1", Status: 200, Body: []byte(`{"a":1}`), FetchedAt: at}

	id1, inserted, err := s.SaveRaw(ctx, p)
	if err != nil || !inserted {
		t.Fatalf("first save: %v inserted=%v", err, inserted)
	}
	p.FetchedAt = at.Add(time.Hour)
	id2, inserted, err := s.SaveRaw(ctx, p)
	if err != nil || inserted || id2 != id1 {
		t.Fatalf("identical save: id %d vs %d, inserted=%v, %v", id2, id1, inserted, err)
	}
	p.Body = []byte(`{"a":2}`)
	p.FetchedAt = at.Add(2 * time.Hour)
	id3, inserted, err := s.SaveRaw(ctx, p)
	if err != nil || !inserted || id3 == id1 {
		t.Fatalf("changed save: %v inserted=%v", err, inserted)
	}

	latest, err := s.LatestRaw(ctx, "fake", "fixtures/1")
	if err != nil || latest.ID != id3 || string(latest.Body) != `{"a": 2}` {
		t.Fatalf("latest = %+v (%s), %v", latest, latest.Body, err)
	}
	if _, err := s.LatestRaw(ctx, "fake", "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestResolveIsStable(t *testing.T) {
	s, _ := storetest.New(t)
	q := s.Queries()
	a, err := store.Resolve(ctx, q, "fake", "player", "42")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := store.Resolve(ctx, q, "fake", "player", "42")
	c, _ := store.Resolve(ctx, q, "fake", "team", "42")
	if a != b || a == c {
		t.Fatalf("a=%s b=%s c=%s", a, b, c)
	}
	if a.Version() != 7 {
		t.Errorf("want UUIDv7, got v%d", a.Version())
	}
}

func classic(t *testing.T) *rules.RuleSet {
	t.Helper()
	reg, err := rules.LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	rs, _ := reg.Get("classic", "1.0.0")
	return rs
}

func TestRuleSetImmutability(t *testing.T) {
	s, _ := storetest.New(t)
	rs := classic(t)
	if err := s.SyncRuleSets(ctx, []*rules.RuleSet{rs}); err != nil {
		t.Fatal(err)
	}
	if err := s.SyncRuleSets(ctx, []*rules.RuleSet{rs}); err != nil {
		t.Fatalf("re-sync of identical rule set: %v", err)
	}
	changed := *rs
	changed.Scores.Goal = 350
	if err := s.SyncRuleSets(ctx, []*rules.RuleSet{&changed}); !errors.Is(err, store.ErrRuleSetChanged) {
		t.Fatalf("changed rule set: err = %v", err)
	}
	changed.Version = "1.0.1"
	if err := s.SyncRuleSets(ctx, []*rules.RuleSet{&changed}); err != nil {
		t.Fatalf("bumped version: %v", err)
	}
}

// fixture inserts the minimum reference data for one fixture with one player.
func fixture(t *testing.T, s *store.Store) (fixtureID, playerID, teamID uuid.UUID) {
	t.Helper()
	ids := make([]uuid.UUID, 6)
	for i := range ids {
		ids[i] = uuid.Must(uuid.NewV7())
	}
	err := s.Tx(ctx, func(q *db.Queries) error {
		if err := q.UpsertCompetition(ctx, db.UpsertCompetitionParams{ID: ids[0], Name: "Serie A", Country: "Italy"}); err != nil {
			return err
		}
		if err := q.UpsertSeason(ctx, db.UpsertSeasonParams{ID: ids[1], CompetitionID: ids[0], Name: "2026/2027"}); err != nil {
			return err
		}
		md, err := q.UpsertMatchday(ctx, db.UpsertMatchdayParams{ID: ids[2], SeasonID: ids[1], Number: 1})
		if err != nil {
			return err
		}
		if err := q.UpsertTeam(ctx, db.UpsertTeamParams{ID: ids[3], Name: "Home"}); err != nil {
			return err
		}
		if err := q.UpsertPlayer(ctx, db.UpsertPlayerParams{ID: ids[4], Name: "Player"}); err != nil {
			return err
		}
		return q.UpsertFixture(ctx, db.UpsertFixtureParams{ID: ids[5], MatchdayID: md, HomeTeamID: ids[3], AwayTeamID: ids[3], Status: "FT"})
	})
	if err != nil {
		t.Fatal(err)
	}
	return ids[5], ids[4], ids[3]
}

func TestFactsRoundTripAndDedup(t *testing.T) {
	s, _ := storetest.New(t)
	fx, _, _ := fixture(t, s)
	facts := d.MatchFacts{HomeTeam: "h", AwayTeam: "a", Length: 90}
	f1, err := s.SaveFacts(ctx, fx, facts, "fake/1", []int64{})
	if err != nil {
		t.Fatal(err)
	}
	f2, _ := s.SaveFacts(ctx, fx, facts, "fake/1", []int64{})
	if f1.ID != f2.ID {
		t.Errorf("identical facts stored twice: %d, %d", f1.ID, f2.ID)
	}
	facts.Length = 120
	f3, _ := s.SaveFacts(ctx, fx, facts, "fake/1", []int64{})
	latest, err := s.LatestFacts(ctx, fx)
	if err != nil || latest.ID != f3.ID || latest.Facts.Length != 120 {
		t.Fatalf("latest = %+v, %v", latest, err)
	}
	if _, err := s.LatestFacts(ctx, uuid.New()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing facts: %v", err)
	}
}

func TestRecordRunVersioning(t *testing.T) {
	s, _ := storetest.New(t)
	rs := classic(t)
	if err := s.SyncRuleSets(ctx, []*rules.RuleSet{rs}); err != nil {
		t.Fatal(err)
	}
	fx, player, team := fixture(t, s)
	facts, _ := s.SaveFacts(ctx, fx, d.MatchFacts{Length: 90}, "fake/1", []int64{})
	voto, fanta := d.Points(600), d.Points(900)
	scores := []engine.Score{{
		Line: d.PlayerLine{Player: d.PlayerID(player.String()), Team: d.TeamID(team.String()), Role: d.RoleOutfield, Minutes: 90, Goals: 1},
		Voto: &voto, Fantavoto: &fanta,
		Breakdown: []engine.BreakdownItem{{Code: rules.Goal, Count: 1, Points: 300}},
	}}
	in := store.RunInput{FixtureID: fx, FactsID: facts.ID, RuleSet: rs, VotoSource: "fake", InputHash: "h1", Reason: "ingest", Scores: scores}

	run1, created, err := s.RecordRun(ctx, in)
	if err != nil || !created {
		t.Fatalf("first run: %v created=%v", err, created)
	}
	again, created, err := s.RecordRun(ctx, in)
	if err != nil || created || again != run1 {
		t.Fatalf("same input: run %s vs %s created=%v %v", again, run1, created, err)
	}
	in.InputHash, in.Reason = "h2", "corrected event"
	run2, created, err := s.RecordRun(ctx, in)
	if err != nil || !created || run2 == run1 {
		t.Fatalf("changed input: %v created=%v", err, created)
	}

	runs, err := s.Queries().FixtureRuns(ctx, fx)
	if err != nil || len(runs) != 2 {
		t.Fatalf("runs = %d, %v", len(runs), err)
	}
	if runs[1].ID != run2 || runs[1].SupersedesRunID == nil || *runs[1].SupersedesRunID != run1 || !runs[1].IsCurrent || runs[0].IsCurrent {
		t.Errorf("history = %+v", runs)
	}
}

func TestRecordRunConcurrentSameInput(t *testing.T) {
	s, _ := storetest.New(t)
	rs := classic(t)
	if err := s.SyncRuleSets(ctx, []*rules.RuleSet{rs}); err != nil {
		t.Fatal(err)
	}
	fx, _, _ := fixture(t, s)
	facts, _ := s.SaveFacts(ctx, fx, d.MatchFacts{Length: 90}, "fake/1", []int64{})
	in := store.RunInput{FixtureID: fx, FactsID: facts.ID, RuleSet: rs, VotoSource: "fake", InputHash: "same", Reason: "ingest"}

	var wg sync.WaitGroup
	var mu sync.Mutex
	created := 0
	for range 8 {
		wg.Go(func() {
			_, c, err := s.RecordRun(ctx, in)
			if err != nil {
				t.Error(err)
			}
			if c {
				mu.Lock()
				created++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if created != 1 {
		t.Fatalf("created %d runs for identical input, want 1", created)
	}
}

func TestRecordRunRejectsNonUUIDPlayers(t *testing.T) {
	s, _ := storetest.New(t)
	rs := classic(t)
	_ = s.SyncRuleSets(ctx, []*rules.RuleSet{rs})
	fx, _, team := fixture(t, s)
	facts, _ := s.SaveFacts(ctx, fx, d.MatchFacts{Length: 90}, "fake/1", []int64{})
	in := store.RunInput{FixtureID: fx, FactsID: facts.ID, RuleSet: rs, VotoSource: "fake", InputHash: "x",
		Scores: []engine.Score{{Line: d.PlayerLine{Player: "not-a-uuid", Team: d.TeamID(team.String())}}}}
	if _, _, err := s.RecordRun(ctx, in); err == nil {
		t.Fatal("expected an error for a non-UUID player")
	}
	if runs, _ := s.Queries().FixtureRuns(ctx, fx); len(runs) != 0 {
		t.Fatal("failed run must roll back entirely")
	}
}
