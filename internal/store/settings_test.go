package store_test

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/openfanta/openfanta/internal/settings"
	"github.com/openfanta/openfanta/internal/store"
	"github.com/openfanta/openfanta/internal/store/db"
	"github.com/openfanta/openfanta/internal/store/storetest"
)

type seeded struct{ league, season uuid.UUID }

// seedLeague creates a season with matchdays 1..3 and a league playing it.
func seedLeague(t *testing.T, s *store.Store) seeded {
	t.Helper()
	ids := make([]uuid.UUID, 2)
	for i := range ids {
		ids[i], _ = uuid.NewV7()
	}
	err := s.Tx(ctx, func(q *db.Queries) error {
		if err := q.UpsertCompetition(ctx, db.UpsertCompetitionParams{ID: ids[0], Name: "Serie A", Country: "IT"}); err != nil {
			return err
		}
		if err := q.UpsertSeason(ctx, db.UpsertSeasonParams{ID: ids[1], CompetitionID: ids[0], Name: "2026/2027"}); err != nil {
			return err
		}
		for n := int32(1); n <= 3; n++ {
			id, _ := uuid.NewV7()
			if _, err := q.UpsertMatchday(ctx, db.UpsertMatchdayParams{ID: id, SeasonID: ids[1], Number: n}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	league, err := s.CreateLeague(ctx, ids[1], "Lega del Bar")
	if err != nil {
		t.Fatal(err)
	}
	return seeded{league, ids[1]}
}

// classicWith returns the Classic document with its goal value replaced.
func classicWith(t *testing.T, goal string) []byte {
	t.Helper()
	raw := string(settings.Classic().Canonical)
	out := strings.Replace(raw, `"goal":300`, `"goal":`+goal, 1)
	if out == raw && goal != "300" {
		t.Fatal("goal value not found in Classic")
	}
	return []byte(out)
}

func submit(league uuid.UUID, doc []byte, from int) store.SettingsSubmission {
	return store.SettingsSubmission{League: league, Document: doc, EffectiveFromMatchday: from, Author: "admin", Reason: "test"}
}

func TestSettingsHistoryIsHashChained(t *testing.T) {
	s, _ := storetest.New(t)
	league := seedLeague(t, s).league

	v1, err := s.AppendSettings(ctx, submit(league, settings.Classic().Canonical, 1))
	if err != nil {
		t.Fatal(err)
	}
	if v1.Seq != 1 || v1.PrevSha256 != nil || v1.Sha256 != settings.Classic().SHA256 {
		t.Errorf("first version: %+v", v1)
	}
	if v1.CanonicalJson != string(settings.Classic().Canonical) {
		t.Error("stored JSON is not the canonical submission")
	}
	if v1.Schema != "openfanta.settings/1.0" || v1.Engine != "2026.10" {
		t.Errorf("schema/engine: %q %q", v1.Schema, v1.Engine)
	}

	v2, err := s.AppendSettings(ctx, submit(league, classicWith(t, "350"), 2))
	if err != nil {
		t.Fatal(err)
	}
	if v2.Seq != 2 || v2.PrevSha256 == nil || *v2.PrevSha256 != v1.Sha256 {
		t.Errorf("second version does not link to the first: %+v", v2)
	}

	hist, err := s.SettingsHistory(ctx, league)
	if err != nil || len(hist) != 2 {
		t.Fatalf("history: %v, %v", hist, err)
	}
	if err := s.VerifyChain(ctx, league); err != nil {
		t.Errorf("VerifyChain: %v", err)
	}

	diff, err := s.SettingsDiff(ctx, league, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff) != 1 || diff[0].Path != "/player/events/goal" || string(diff[0].Old) != "300" || string(diff[0].New) != "350" {
		t.Errorf("diff: %+v", diff)
	}
	first, err := s.SettingsDiff(ctx, league, 1)
	if err != nil || len(first) == 0 || first[0].Op != settings.OpAdd {
		t.Errorf("first diff: %+v, %v", first, err)
	}
	if _, err := s.SettingsDiff(ctx, league, 9); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing version: %v", err)
	}
}

func TestSettingsRejections(t *testing.T) {
	s, _ := storetest.New(t)
	league := seedLeague(t, s).league
	good := settings.Classic().Canonical
	if _, err := s.AppendSettings(ctx, submit(league, good, 1)); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		in   store.SettingsSubmission
		want error
	}{
		{"unchanged", submit(league, good, 2), store.ErrUnchanged},
		{"invalid document", submit(league, []byte(`{"bogus":1}`), 2), settings.ErrInvalid},
		{"no author", store.SettingsSubmission{League: league, Document: classicWith(t, "400"), EffectiveFromMatchday: 2, Reason: "x"}, store.ErrInvalidSubmission},
		{"no reason", store.SettingsSubmission{League: league, Document: classicWith(t, "400"), EffectiveFromMatchday: 2, Author: "a"}, store.ErrInvalidSubmission},
		{"matchday zero", submit(league, classicWith(t, "400"), 0), store.ErrInvalidSubmission},
		{"unknown league", submit(uuid.New(), classicWith(t, "400"), 2), store.ErrNotFound},
	}
	for _, c := range cases {
		if _, err := s.AppendSettings(ctx, c.in); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
	if hist, _ := s.SettingsHistory(ctx, league); len(hist) != 1 {
		t.Errorf("rejected submissions were stored: %d versions", len(hist))
	}
}

func TestSettingsCannotTakeEffectFromClosedMatchday(t *testing.T) {
	s, _ := storetest.New(t)
	sd := seedLeague(t, s)
	if _, err := s.AppendSettings(ctx, submit(sd.league, settings.Classic().Canonical, 1)); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.CloseMatchday(ctx, sd.season, 2, time.Now()); err != nil || !ok {
		t.Fatalf("close matchday 2: %v, %v", ok, err)
	}
	if ok, _ := s.CloseMatchday(ctx, sd.season, 2, time.Now()); ok {
		t.Error("closing twice reported a change")
	}

	doc := classicWith(t, "350")
	for _, from := range []int{1, 2} { // closed, and before a closed matchday
		if _, err := s.AppendSettings(ctx, submit(sd.league, doc, from)); !errors.Is(err, store.ErrMatchdayClosed) {
			t.Errorf("from %d: got %v, want ErrMatchdayClosed", from, err)
		}
	}
	if _, err := s.AppendSettings(ctx, submit(sd.league, doc, 3)); err != nil {
		t.Errorf("from open matchday 3: %v", err)
	}
}

func TestSettingsConcurrentAppendsHaveNoGaps(t *testing.T) {
	s, _ := storetest.New(t)
	sd := seedLeague(t, s)
	const n = 8
	docs := make([][]byte, n)
	for i := range docs {
		docs[i] = classicWith(t, fmt.Sprint(400+i))
	}
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for _, doc := range docs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.AppendSettings(ctx, submit(sd.league, doc, 2))
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	hist, err := s.SettingsHistory(ctx, sd.league)
	if err != nil || len(hist) != n {
		t.Fatalf("history: %d versions, %v", len(hist), err)
	}
	if err := s.VerifyChain(ctx, sd.league); err != nil {
		t.Error(err)
	}
}

func rawConn(t *testing.T, dsn string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close(ctx) })
	return c
}

func TestSettingsHistoryIsAppendOnlyInDatabase(t *testing.T) {
	s, dsn := storetest.New(t)
	league := seedLeague(t, s).league
	if _, err := s.AppendSettings(ctx, submit(league, settings.Classic().Canonical, 1)); err != nil {
		t.Fatal(err)
	}
	conn := rawConn(t, dsn)
	for _, stmt := range []string{
		`UPDATE settings_versions SET reason = 'edited'`,
		`DELETE FROM settings_versions`,
	} {
		if _, err := conn.Exec(ctx, stmt); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("%s: got %v, want append-only error", stmt, err)
		}
	}
}

func TestVerifyChainDetectsTampering(t *testing.T) {
	cases := []struct{ name, stmt, want string }{
		{"edited document",
			`UPDATE settings_versions SET canonical_json = replace(canonical_json, '"goal":350', '"goal":999') WHERE seq = 2`,
			"version 2: hash does not match"},
		{"rewritten link",
			`UPDATE settings_versions SET prev_sha256 = repeat('0', 64) WHERE seq = 3`,
			"version 3: does not link"},
		{"removed version",
			`DELETE FROM settings_versions WHERE seq = 2`,
			"version 3: sequence has a gap"},
		{"re-hashed non-canonical document",
			// An attacker who also fixes the hash still leaves a non-canonical document.
			`UPDATE settings_versions SET canonical_json = ' ' || canonical_json,
			     sha256 = encode(sha256((' ' || canonical_json)::bytea), 'hex') WHERE seq = 1`,
			"version 1: stored document is not canonical"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, dsn := storetest.New(t)
			league := seedLeague(t, s).league
			for i, doc := range [][]byte{settings.Classic().Canonical, classicWith(t, "350"), classicWith(t, "400")} {
				if _, err := s.AppendSettings(ctx, submit(league, doc, i+1)); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.VerifyChain(ctx, league); err != nil {
				t.Fatalf("untampered chain: %v", err)
			}
			conn := rawConn(t, dsn)
			// A database owner can bypass the trigger; the chain must still notice.
			for _, stmt := range []string{`ALTER TABLE settings_versions DISABLE TRIGGER settings_versions_append_only`, c.stmt} {
				if _, err := conn.Exec(ctx, stmt); err != nil {
					t.Fatalf("%s: %v", stmt, err)
				}
			}
			err := s.VerifyChain(ctx, league)
			if !errors.Is(err, store.ErrChainBroken) || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want ErrChainBroken containing %q", err, c.want)
			}
		})
	}
}
