// Package ingest orchestrates the pipeline: fetch raw payloads (cached in the
// store), parse them, map provider IDs to OpenFanta IDs, store normalized
// facts, and compute and record versioned results.
package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/engine"
	"github.com/openfanta/openfanta/internal/provider"
	"github.com/openfanta/openfanta/internal/rules"
	"github.com/openfanta/openfanta/internal/store"
	"github.com/openfanta/openfanta/internal/store/db"
	"github.com/openfanta/openfanta/internal/timeline"
	"github.com/openfanta/openfanta/internal/voto"
)

// Cache lifetimes for payloads that can still change. Finished fixtures are
// cached forever and only re-fetched on request.
const (
	seasonTTL   = 24 * time.Hour
	scheduleTTL = 6 * time.Hour
)

// Service runs ingestion and recomputation.
type Service struct {
	Store    *store.Store
	Provider provider.Provider
	Rules    *rules.Registry
	Voto     voto.Source
	Logger   *slog.Logger
	Now      func() time.Time
}

// Options tune an ingestion.
type Options struct {
	// Refresh re-fetches every payload, including finished fixtures, to pick up
	// upstream corrections.
	Refresh bool
}

// Report summarizes an ingestion or recomputation.
type Report struct {
	Fixtures []FixtureReport
}

// FixtureReport is the outcome for one fixture.
type FixtureReport struct {
	Ref         string // provider ref; empty for recomputation
	ID          uuid.UUID
	Status      string
	Final       bool
	FromCache   bool
	RunsCreated int
	Err         error
}

// Failed reports how many fixtures failed.
func (r Report) Failed() int {
	n := 0
	for _, f := range r.Fixtures {
		if f.Err != nil {
			n++
		}
	}
	return n
}

func (s *Service) normalizer() string { return s.Provider.Name() + "/" + s.Provider.ParserVersion() }

// IngestMatchday ingests every fixture of one matchday of a provider season.
// A failing fixture is reported and does not stop the others.
func (s *Service) IngestMatchday(ctx context.Context, seasonRef string, matchday int, opts Options) (Report, error) {
	seasonRaw, _, err := s.fetch(ctx, "season "+seasonRef, seasonTTL, opts.Refresh, func() (provider.Raw, error) {
		return s.Provider.FetchSeason(ctx, seasonRef)
	}, "season/"+seasonRef)
	if err != nil {
		return Report{}, err
	}
	season, err := s.Provider.ParseSeason(toProviderRaw(seasonRaw))
	if err != nil {
		return Report{}, err
	}
	schedRaw, _, err := s.fetch(ctx, "schedule "+seasonRef, scheduleTTL, opts.Refresh, func() (provider.Raw, error) {
		return s.Provider.FetchSchedule(ctx, seasonRef)
	}, "schedule/"+seasonRef)
	if err != nil {
		return Report{}, err
	}
	schedule, err := s.Provider.ParseSchedule(toProviderRaw(schedRaw))
	if err != nil {
		return Report{}, err
	}
	refs := schedule.Matchdays[matchday]
	if len(refs) == 0 {
		return Report{}, fmt.Errorf("season %s has no fixtures for matchday %d", seasonRef, matchday)
	}

	var matchdayID uuid.UUID
	err = s.Store.Tx(ctx, func(q *db.Queries) error {
		var err error
		matchdayID, err = s.upsertMatchday(ctx, q, season, matchday)
		return err
	})
	if err != nil {
		return Report{}, fmt.Errorf("store season: %w", err)
	}

	var rep Report
	for _, ref := range refs {
		if ctx.Err() != nil {
			return rep, ctx.Err()
		}
		fr := s.ingestFixture(ctx, matchdayID, ref, opts)
		if fr.Err != nil {
			s.Logger.ErrorContext(ctx, "fixture failed", "fixture_ref", ref, "error", fr.Err)
		} else {
			s.Logger.InfoContext(ctx, "fixture ingested", "fixture_ref", ref, "fixture_id", fr.ID,
				"status", fr.Status, "cached", fr.FromCache, "runs_created", fr.RunsCreated)
		}
		rep.Fixtures = append(rep.Fixtures, fr)
	}
	return rep, nil
}

// fetch returns the cached payload for key if it is fresher than ttl, else
// fetches and stores a new one. ttl < 0 means "fresh forever". Cache keys are
// owned by ingest ("fixture/<ref>"), so providers need not agree on a format.
func (s *Service) fetch(ctx context.Context, what string, ttl time.Duration, refresh bool,
	get func() (provider.Raw, error), key string,
) (store.RawPayload, bool, error) {
	if !refresh {
		cached, err := s.Store.LatestRaw(ctx, s.Provider.Name(), key)
		switch {
		case err == nil && (ttl < 0 || s.Now().Sub(cached.FetchedAt) < ttl):
			return cached, true, nil
		case err != nil && !errors.Is(err, store.ErrNotFound):
			return store.RawPayload{}, false, err
		}
	}
	r, err := get()
	if err != nil {
		return store.RawPayload{}, false, fmt.Errorf("fetch %s: %w", what, err)
	}
	p := store.RawPayload{Provider: s.Provider.Name(), RequestKey: key, Status: r.Status, Body: r.Body, FetchedAt: r.FetchedAt}
	if p.ID, _, err = s.Store.SaveRaw(ctx, p); err != nil {
		return store.RawPayload{}, false, err
	}
	return p, false, nil
}

func (s *Service) ingestFixture(ctx context.Context, matchdayID uuid.UUID, ref string, opts Options) FixtureReport {
	fr := FixtureReport{Ref: ref}
	key := "fixture/" + ref

	raw, cached, err := s.cachedFinalFixture(ctx, key, opts.Refresh)
	if err != nil {
		fr.Err = err
		return fr
	}
	if raw == nil {
		r, _, err := s.fetch(ctx, "fixture "+ref, 0, true, func() (provider.Raw, error) {
			return s.Provider.FetchFixture(ctx, ref)
		}, key)
		if err != nil {
			fr.Err = err
			return fr
		}
		raw = &r
	}
	fr.FromCache = cached

	fx, err := s.Provider.ParseFixture(toProviderRaw(*raw))
	if err != nil {
		fr.Err = err
		return fr
	}
	fr.Status, fr.Final = fx.Status, fx.Final

	var facts domain.MatchFacts
	err = s.Store.Tx(ctx, func(q *db.Queries) error {
		var err error
		fr.ID, facts, err = s.upsertFixture(ctx, q, matchdayID, fx)
		return err
	})
	if err != nil {
		fr.Err = fmt.Errorf("store fixture: %w", err)
		return fr
	}
	if !fx.Final {
		return fr // nothing to score yet
	}
	stored, err := s.Store.SaveFacts(ctx, fr.ID, facts, s.normalizer(), []int64{raw.ID})
	if err != nil {
		fr.Err = err
		return fr
	}
	fr.RunsCreated, fr.Err = s.compute(ctx, stored, "ingest")
	return fr
}

// cachedFinalFixture returns the stored payload for a finished fixture, or nil
// if it must be fetched.
func (s *Service) cachedFinalFixture(ctx context.Context, key string, refresh bool) (*store.RawPayload, bool, error) {
	if refresh {
		return nil, false, nil
	}
	cached, err := s.Store.LatestRaw(ctx, s.Provider.Name(), key)
	if errors.Is(err, store.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	fx, err := s.Provider.ParseFixture(toProviderRaw(cached))
	if err != nil || !fx.Final {
		return nil, false, nil
	}
	return &cached, true, nil
}

func toProviderRaw(p store.RawPayload) provider.Raw {
	return provider.Raw{Key: p.RequestKey, Status: p.Status, Body: p.Body, FetchedAt: p.FetchedAt}
}

// compute scores stored facts under the latest version of every rule set and
// records a run for each. It returns how many new runs were created.
func (s *Service) compute(ctx context.Context, f store.Facts, reason string) (int, error) {
	lines, err := timeline.Derive(f.Facts)
	if err != nil {
		return 0, err
	}
	votes, err := s.Voto.Votes(ctx, f.Facts)
	if err != nil {
		return 0, fmt.Errorf("voto source %s: %w", s.Voto.Name(), err)
	}
	created := 0
	for _, rs := range s.Rules.LatestOfEach() {
		scores := engine.Compute(lines, votes, rs)
		_, isNew, err := s.Store.RecordRun(ctx, store.RunInput{
			FixtureID: f.FixtureID, FactsID: f.ID, RuleSet: rs, VotoSource: s.Voto.Name(),
			InputHash: inputHash(f.Hash, rs, s.Voto.Name(), votes), Reason: reason, Scores: scores,
		})
		if err != nil {
			return created, fmt.Errorf("record %s run: %w", rs.Key(), err)
		}
		if isNew {
			created++
		}
	}
	return created, nil
}

// inputHash identifies everything that determines a run's result. Two runs
// with the same hash are guaranteed to have identical scores.
func inputHash(factsHash string, rs *rules.RuleSet, votoSource string, votes map[domain.PlayerID]domain.Points) string {
	players := make([]string, 0, len(votes))
	for p := range votes {
		players = append(players, string(p))
	}
	sort.Strings(players)
	pairs := make([][2]string, len(players))
	for i, p := range players {
		pairs[i] = [2]string{p, strconv.FormatInt(int64(votes[domain.PlayerID(p)]), 10)}
	}
	b, _ := json.Marshal(map[string]any{
		"facts": factsHash, "rule_set": rs.Key(), "rule_set_hash": rs.Hash(),
		"voto_source": votoSource, "votes": pairs, "engine": engine.Version,
	})
	return domain.SHA256Hex(b)
}

func (s *Service) upsertMatchday(ctx context.Context, q *db.Queries, season provider.Season, matchday int) (uuid.UUID, error) {
	p := s.Provider.Name()
	compID, err := store.Resolve(ctx, q, p, "competition", season.CompetitionRef)
	if err != nil {
		return uuid.Nil, err
	}
	if err := q.UpsertCompetition(ctx, db.UpsertCompetitionParams{ID: compID, Name: season.CompetitionName, Country: season.CompetitionCountry}); err != nil {
		return uuid.Nil, err
	}
	seasonID, err := store.Resolve(ctx, q, p, "season", season.Ref)
	if err != nil {
		return uuid.Nil, err
	}
	if err := q.UpsertSeason(ctx, db.UpsertSeasonParams{ID: seasonID, CompetitionID: compID, Name: season.Name}); err != nil {
		return uuid.Nil, err
	}
	fresh, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, err
	}
	return q.UpsertMatchday(ctx, db.UpsertMatchdayParams{ID: fresh, SeasonID: seasonID, Number: int32(matchday)})
}

// upsertFixture stores teams, players and the fixture, and returns the facts
// rewritten with OpenFanta IDs.
func (s *Service) upsertFixture(ctx context.Context, q *db.Queries, matchdayID uuid.UUID, fx provider.Fixture) (uuid.UUID, domain.MatchFacts, error) {
	p := s.Provider.Name()
	teamIDs := map[domain.TeamID]domain.TeamID{}
	for _, t := range []provider.Team{fx.Home, fx.Away} {
		id, err := store.Resolve(ctx, q, p, "team", t.Ref)
		if err != nil {
			return uuid.Nil, domain.MatchFacts{}, err
		}
		if err := q.UpsertTeam(ctx, db.UpsertTeamParams{ID: id, Name: t.Name}); err != nil {
			return uuid.Nil, domain.MatchFacts{}, err
		}
		teamIDs[domain.TeamID(t.Ref)] = domain.TeamID(id.String())
	}

	refs := make([]string, 0, len(fx.Players))
	for ref := range fx.Players {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	playerIDs := map[domain.PlayerID]domain.PlayerID{}
	for _, ref := range refs {
		id, err := store.Resolve(ctx, q, p, "player", ref)
		if err != nil {
			return uuid.Nil, domain.MatchFacts{}, err
		}
		if err := q.UpsertPlayer(ctx, db.UpsertPlayerParams{ID: id, Name: fx.Players[ref]}); err != nil {
			return uuid.Nil, domain.MatchFacts{}, err
		}
		playerIDs[domain.PlayerID(ref)] = domain.PlayerID(id.String())
	}

	fixtureID, err := store.Resolve(ctx, q, p, "fixture", fx.Ref)
	if err != nil {
		return uuid.Nil, domain.MatchFacts{}, err
	}
	home, _ := uuid.Parse(string(teamIDs[domain.TeamID(fx.Home.Ref)]))
	away, _ := uuid.Parse(string(teamIDs[domain.TeamID(fx.Away.Ref)]))
	if err := q.UpsertFixture(ctx, db.UpsertFixtureParams{
		ID: fixtureID, MatchdayID: matchdayID, HomeTeamID: home, AwayTeamID: away, Kickoff: fx.Kickoff, Status: fx.Status,
	}); err != nil {
		return uuid.Nil, domain.MatchFacts{}, err
	}
	facts, err := remap(fx.Facts, teamIDs, playerIDs)
	return fixtureID, facts, err
}

// remap replaces provider refs in facts with OpenFanta IDs.
func remap(f domain.MatchFacts, teams map[domain.TeamID]domain.TeamID, players map[domain.PlayerID]domain.PlayerID) (domain.MatchFacts, error) {
	team := func(t domain.TeamID) (domain.TeamID, error) {
		if id, ok := teams[t]; ok {
			return id, nil
		}
		return "", fmt.Errorf("unmapped team %q", t)
	}
	player := func(p domain.PlayerID) (domain.PlayerID, error) {
		if p == "" {
			return "", nil
		}
		if id, ok := players[p]; ok {
			return id, nil
		}
		return "", fmt.Errorf("unmapped player %q", p)
	}
	out := domain.MatchFacts{Length: f.Length}
	var err error
	if out.HomeTeam, err = team(f.HomeTeam); err != nil {
		return out, err
	}
	if out.AwayTeam, err = team(f.AwayTeam); err != nil {
		return out, err
	}
	for _, a := range f.Appearances {
		if a.Player, err = player(a.Player); err != nil {
			return out, err
		}
		if a.Team, err = team(a.Team); err != nil {
			return out, err
		}
		out.Appearances = append(out.Appearances, a)
	}
	for _, e := range f.Events {
		if e.Team, err = team(e.Team); err != nil {
			return out, err
		}
		if e.Player, err = player(e.Player); err != nil {
			return out, err
		}
		if e.Related, err = player(e.Related); err != nil {
			return out, err
		}
		out.Events = append(out.Events, e)
	}
	return out, nil
}
