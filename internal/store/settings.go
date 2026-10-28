package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/settings"
	"github.com/openfanta/openfanta/internal/store/db"
)

var (
	// ErrMatchdayClosed is returned when a settings version would take effect
	// from a matchday that is already closed. Rule changes are never retroactive.
	ErrMatchdayClosed = errors.New("settings cannot take effect from a closed matchday")

	// ErrUnchanged is returned when a submission equals the current version.
	ErrUnchanged = errors.New("settings are identical to the current version")

	// ErrInvalidSubmission is returned when the author, reason or matchday of a
	// submission is missing or out of range.
	ErrInvalidSubmission = errors.New("invalid settings submission")

	// ErrChainBroken is returned by VerifyChain when history was altered.
	ErrChainBroken = errors.New("settings history is broken")
)

// SettingsSubmission is a new settings version for a league.
type SettingsSubmission struct {
	League uuid.UUID
	// Document is the submitted JSON. It is validated and canonicalized; the
	// canonical form is what gets stored.
	Document []byte
	// EffectiveFromMatchday is the first matchday the version applies to.
	EffectiveFromMatchday int
	Author                string
	Reason                string
}

// CreateLeague adds a league that plays the given season.
func (s *Store) CreateLeague(ctx context.Context, season uuid.UUID, name string) (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, err
	}
	err = s.q.InsertLeague(ctx, db.InsertLeagueParams{ID: id, SeasonID: season, Name: name, CreatedAt: s.now()})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create league: %w", err)
	}
	return id, nil
}

// CloseMatchday marks a matchday final. It reports whether it was open.
func (s *Store) CloseMatchday(ctx context.Context, season uuid.UUID, number int, at time.Time) (bool, error) {
	n, err := s.q.CloseMatchday(ctx, db.CloseMatchdayParams{SeasonID: season, Number: int32(number), ClosedAt: &at})
	return n > 0, err
}

// AppendSettings validates a submission and appends it to the league's
// history, linked to the previous version by hash. Concurrent submissions for
// one league are serialized, so seq has no gaps.
func (s *Store) AppendSettings(ctx context.Context, in SettingsSubmission) (db.SettingsVersion, error) {
	switch {
	case strings.TrimSpace(in.Author) == "":
		return db.SettingsVersion{}, fmt.Errorf("%w: author is required", ErrInvalidSubmission)
	case strings.TrimSpace(in.Reason) == "":
		return db.SettingsVersion{}, fmt.Errorf("%w: reason is required", ErrInvalidSubmission)
	case in.EffectiveFromMatchday < 1:
		return db.SettingsVersion{}, fmt.Errorf("%w: effective_from_matchday must be at least 1", ErrInvalidSubmission)
	}
	doc, err := settings.Parse(in.Document)
	if err != nil {
		return db.SettingsVersion{}, err
	}

	var out db.SettingsVersion
	err = s.Tx(ctx, func(q *db.Queries) error {
		season, err := q.LockLeague(ctx, in.League)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		closed, err := q.HasClosedMatchdayFrom(ctx, db.HasClosedMatchdayFromParams{
			SeasonID: season, Number: int32(in.EffectiveFromMatchday),
		})
		if err != nil {
			return err
		}
		if closed {
			return fmt.Errorf("%w: matchday %d or a later one is closed", ErrMatchdayClosed, in.EffectiveFromMatchday)
		}

		seq, prev := int32(1), (*string)(nil)
		head, err := q.SettingsHead(ctx, in.League)
		switch {
		case err == nil:
			if head.Sha256 == doc.SHA256 {
				return ErrUnchanged
			}
			seq, prev = head.Seq+1, &head.Sha256
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}

		out, err = q.InsertSettingsVersion(ctx, db.InsertSettingsVersionParams{
			LeagueID: in.League, Seq: seq, CanonicalJson: string(doc.Canonical), Sha256: doc.SHA256,
			PrevSha256: prev, Schema: doc.Doc.Schema, Engine: doc.Doc.Engine,
			EffectiveFromMatchday: int32(in.EffectiveFromMatchday),
			Author:                in.Author, Reason: in.Reason, CreatedAt: s.now(),
		})
		return err
	})
	if err != nil {
		return db.SettingsVersion{}, err
	}
	return out, nil
}

// SettingsHistory returns every version of a league's settings, oldest first.
func (s *Store) SettingsHistory(ctx context.Context, league uuid.UUID) ([]db.SettingsVersion, error) {
	return s.q.ListSettingsVersions(ctx, league)
}

// SettingsDiff returns what version seq changed relative to the version before
// it. For the first version, everything is an addition.
func (s *Store) SettingsDiff(ctx context.Context, league uuid.UUID, seq int) ([]settings.Change, error) {
	cur, err := s.q.GetSettingsVersion(ctx, db.GetSettingsVersionParams{LeagueID: league, Seq: int32(seq)})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	prev := "{}"
	if seq > 1 {
		p, err := s.q.GetSettingsVersion(ctx, db.GetSettingsVersionParams{LeagueID: league, Seq: int32(seq - 1)})
		if err != nil {
			return nil, err
		}
		prev = p.CanonicalJson
	}
	return settings.Diff([]byte(prev), []byte(cur.CanonicalJson))
}

// VerifyChain recomputes the hash of every stored version and checks that each
// one points at the hash of the version before it. It returns ErrChainBroken,
// naming the first bad version, if history was altered.
func (s *Store) VerifyChain(ctx context.Context, league uuid.UUID) error {
	versions, err := s.q.ListSettingsVersions(ctx, league)
	if err != nil {
		return err
	}
	prev := (*string)(nil)
	for i, v := range versions {
		bad := func(why string) error { return fmt.Errorf("%w: version %d: %s", ErrChainBroken, v.Seq, why) }
		switch {
		case int(v.Seq) != i+1:
			return bad("sequence has a gap")
		case domain.SHA256Hex([]byte(v.CanonicalJson)) != v.Sha256:
			return bad("hash does not match the stored document")
		case !equalHash(prev, v.PrevSha256):
			return bad("does not link to the previous version")
		}
		if canon, err := settings.Canonicalize([]byte(v.CanonicalJson)); err != nil || string(canon) != v.CanonicalJson {
			return bad("stored document is not canonical")
		}
		prev = &versions[i].Sha256
	}
	return nil
}

func equalHash(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
