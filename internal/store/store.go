// Package store persists raw payloads, normalized facts, settings histories and
// versioned computation results in Postgres.
package store

//go:generate sqlc generate

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/engine"
	"github.com/openfanta/openfanta/internal/settings"
	"github.com/openfanta/openfanta/internal/store/db"
)

//go:embed migrations/*.sql
var migrations embed.FS

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

// Store wraps a pgx pool with typed operations.
type Store struct {
	pool *pgxpool.Pool
	q    *db.Queries
	now  func() time.Time
}

// Open connects to Postgres. Call Close when done.
func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return &Store{pool: pool, q: db.New(pool), now: func() time.Time { return time.Now().UTC() }}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Ping checks connectivity, for readiness probes.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Queries exposes generated read queries.
func (s *Store) Queries() *db.Queries { return s.q }

// Migrate applies all pending migrations.
func Migrate(ctx context.Context, url string) error {
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return err
	}
	sqlDB := stdlib.OpenDB(*cfg)
	defer func() { _ = sqlDB.Close() }()
	return migrate(ctx, sqlDB)
}

func migrate(ctx context.Context, sqlDB *sql.DB) error {
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, mustSub(migrations, "migrations"))
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}

// Tx runs fn in a transaction, committing if it returns nil.
func (s *Store) Tx(ctx context.Context, fn func(q *db.Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.q.WithTx(tx)) })
}

// RawPayload is one upstream response as received.
type RawPayload struct {
	ID         int64
	Provider   string
	RequestKey string
	Status     int
	Body       []byte
	FetchedAt  time.Time
}

// SaveRaw stores p unless an identical body is already stored for the same
// request, in which case it only bumps last_seen_at. It returns the row ID and
// whether a new row was inserted.
func (s *Store) SaveRaw(ctx context.Context, p RawPayload) (int64, bool, error) {
	row, err := s.q.InsertRawPayload(ctx, db.InsertRawPayloadParams{
		Provider: p.Provider, RequestKey: p.RequestKey, Sha256: domain.SHA256Hex(p.Body),
		HttpStatus: int32(p.Status), Body: p.Body, FetchedAt: p.FetchedAt.UTC(),
	})
	if err != nil {
		return 0, false, fmt.Errorf("save raw payload: %w", err)
	}
	return row.ID, row.Inserted, nil
}

// LatestRaw returns the most recently seen payload for a request.
func (s *Store) LatestRaw(ctx context.Context, provider, requestKey string) (RawPayload, error) {
	r, err := s.q.LatestRawPayload(ctx, db.LatestRawPayloadParams{Provider: provider, RequestKey: requestKey})
	if errors.Is(err, pgx.ErrNoRows) {
		return RawPayload{}, ErrNotFound
	}
	if err != nil {
		return RawPayload{}, err
	}
	return RawPayload{
		ID: r.ID, Provider: r.Provider, RequestKey: r.RequestKey, Status: int(r.HttpStatus),
		Body: r.Body, FetchedAt: r.LastSeenAt,
	}, nil
}

// RawByID returns a stored payload by row ID.
func (s *Store) RawByID(ctx context.Context, id int64) (RawPayload, error) {
	r, err := s.q.GetRawPayload(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return RawPayload{}, ErrNotFound
	}
	if err != nil {
		return RawPayload{}, err
	}
	return RawPayload{
		ID: r.ID, Provider: r.Provider, RequestKey: r.RequestKey, Status: int(r.HttpStatus),
		Body: r.Body, FetchedAt: r.LastSeenAt,
	}, nil
}

// Resolve returns the OpenFanta ID for a provider entity, creating one if new.
func Resolve(ctx context.Context, q *db.Queries, provider, entity, externalID string) (uuid.UUID, error) {
	id, err := q.GetRef(ctx, db.GetRefParams{Provider: provider, Entity: entity, ExternalID: externalID})
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	fresh, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, err
	}
	return q.InsertRef(ctx, db.InsertRefParams{Provider: provider, Entity: entity, ExternalID: externalID, EntityID: fresh})
}

// Facts is a stored fixture_facts row.
type Facts struct {
	ID            int64
	FixtureID     uuid.UUID
	Facts         domain.MatchFacts
	Hash          string
	Normalizer    string
	RawPayloadIDs []int64
}

// SaveFacts stores normalized facts for a fixture, deduplicated by content.
func (s *Store) SaveFacts(ctx context.Context, fixtureID uuid.UUID, facts domain.MatchFacts, normalizer string, rawIDs []int64) (Facts, error) {
	body, err := json.Marshal(facts)
	if err != nil {
		return Facts{}, err
	}
	hash := domain.SHA256Hex(body)
	id, err := s.q.InsertFixtureFacts(ctx, db.InsertFixtureFactsParams{
		FixtureID: fixtureID, Facts: body, FactsHash: hash, Normalizer: normalizer,
		RawPayloadIds: rawIDs, CreatedAt: s.now(),
	})
	if err != nil {
		return Facts{}, fmt.Errorf("save facts: %w", err)
	}
	return Facts{ID: id, FixtureID: fixtureID, Facts: facts, Hash: hash, Normalizer: normalizer, RawPayloadIDs: rawIDs}, nil
}

// LatestFacts returns the most recently stored facts for a fixture.
func (s *Store) LatestFacts(ctx context.Context, fixtureID uuid.UUID) (Facts, error) {
	r, err := s.q.LatestFixtureFacts(ctx, fixtureID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Facts{}, ErrNotFound
	}
	if err != nil {
		return Facts{}, err
	}
	var f domain.MatchFacts
	if err := json.Unmarshal(r.Facts, &f); err != nil {
		return Facts{}, fmt.Errorf("decode facts %d: %w", r.ID, err)
	}
	return Facts{
		ID: r.ID, FixtureID: r.FixtureID, Facts: f, Hash: r.FactsHash, Normalizer: r.Normalizer, RawPayloadIDs: r.RawPayloadIds,
	}, nil
}

// RunInput describes one computation to record.
type RunInput struct {
	FixtureID  uuid.UUID
	FactsID    int64
	Settings   *settings.Settings
	VotoSource string
	InputHash  string // identifies all inputs; equal hashes mean an identical result
	Reason     string
	Scores     []engine.Score
}

// RecordRun stores a computation and makes it current, unless the current run
// for the same fixture, settings and voto source has the same input
// hash. It returns the current run ID and whether a new run was created.
func (s *Store) RecordRun(ctx context.Context, in RunInput) (uuid.UUID, bool, error) {
	var runID uuid.UUID
	var created bool
	err := s.Tx(ctx, func(q *db.Queries) error {
		key := in.FixtureID.String() + "|" + in.Settings.SHA256 + "|" + in.VotoSource
		if err := q.LockComputation(ctx, key); err != nil {
			return err
		}
		var supersedes *uuid.UUID
		cur, err := q.CurrentRun(ctx, db.CurrentRunParams{FixtureID: in.FixtureID, SettingsSha256: in.Settings.SHA256, VotoSource: in.VotoSource})
		switch {
		case err == nil && cur.InputHash == in.InputHash:
			runID = cur.ID
			return nil
		case err == nil:
			supersedes = &cur.ID
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}

		if runID, err = uuid.NewV7(); err != nil {
			return err
		}
		if err := q.InsertRun(ctx, db.InsertRunParams{
			ID: runID, FixtureID: in.FixtureID, FixtureFactsID: in.FactsID,
			SettingsSha256: in.Settings.SHA256, VotoSource: in.VotoSource,
			EngineVersion: engine.Version, InputHash: in.InputHash, SupersedesRunID: supersedes,
			Reason: in.Reason, CreatedAt: s.now(),
		}); err != nil {
			return fmt.Errorf("insert run: %w", err)
		}
		rows, err := scoreRows(runID, in.Scores)
		if err != nil {
			return err
		}
		if _, err := q.InsertPlayerScores(ctx, rows); err != nil {
			return fmt.Errorf("insert scores: %w", err)
		}
		created = true
		return q.SetCurrentRun(ctx, db.SetCurrentRunParams{
			FixtureID: in.FixtureID, SettingsSha256: in.Settings.SHA256, VotoSource: in.VotoSource, RunID: runID,
		})
	})
	return runID, created, err
}

func scoreRows(runID uuid.UUID, scores []engine.Score) ([]db.InsertPlayerScoresParams, error) {
	rows := make([]db.InsertPlayerScoresParams, 0, len(scores))
	for _, sc := range scores {
		player, err := uuid.Parse(string(sc.Line.Player))
		if err != nil {
			return nil, fmt.Errorf("player id %q: %w", sc.Line.Player, err)
		}
		team, err := uuid.Parse(string(sc.Line.Team))
		if err != nil {
			return nil, fmt.Errorf("team id %q: %w", sc.Line.Team, err)
		}
		breakdown := sc.Breakdown
		if breakdown == nil {
			breakdown = []engine.BreakdownItem{}
		}
		b, _ := json.Marshal(breakdown)
		rows = append(rows, db.InsertPlayerScoresParams{
			RunID: runID, PlayerID: player, TeamID: team, Role: string(sc.Line.Role),
			Minutes: int32(sc.Line.Minutes), Voto: toInt32(sc.Voto), Fantavoto: toInt32(sc.Fantavoto),
			SenzaVoto: sc.SenzaVoto, VotoUfficio: sc.VotoUfficio, Breakdown: b,
			Goals: int32(sc.Line.Goals), PenaltiesScored: int32(sc.Line.PenaltiesScored),
			PenaltiesMissed: int32(sc.Line.PenaltiesMissed), PenaltiesSaved: int32(sc.Line.PenaltiesSaved),
			OwnGoals: int32(sc.Line.OwnGoals), Assists: int32(sc.Line.Assists),
			YellowCards: int32(sc.Line.YellowCards), SentOff: sc.Line.SentOff, GoalsConceded: int32(sc.Line.GoalsConceded),
		})
	}
	return rows, nil
}

func toInt32(p *domain.Points) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p)
	return &v
}
