package ingest

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/openfanta/openfanta/internal/store"
	"github.com/openfanta/openfanta/internal/store/db"
)

// RecomputeOptions select what to recompute.
type RecomputeOptions struct {
	SeasonID *uuid.UUID // nil: every fixture with facts
	// Reparse re-parses the stored raw payloads with the current parser before
	// computing, without any network access. Use it after a parser fix.
	Reparse bool
	Reason  string
}

// Recompute scores stored facts again under the current rule sets, engine and
// voto source. Runs whose inputs did not change are not duplicated.
func (s *Service) Recompute(ctx context.Context, opts RecomputeOptions) (Report, error) {
	ids, err := s.Store.Queries().FixturesWithFacts(ctx, opts.SeasonID)
	if err != nil {
		return Report{}, err
	}
	reason := opts.Reason
	if reason == "" {
		reason = "recompute"
	}
	var rep Report
	for _, id := range ids {
		if ctx.Err() != nil {
			return rep, ctx.Err()
		}
		fr := FixtureReport{ID: id, Final: true}
		facts, err := s.Store.LatestFacts(ctx, id)
		if err == nil && opts.Reparse {
			facts, err = s.reparse(ctx, facts)
		}
		if err == nil {
			fr.RunsCreated, err = s.compute(ctx, facts, reason)
		}
		fr.Err = err
		if err != nil {
			s.Logger.ErrorContext(ctx, "recompute failed", "fixture_id", id, "error", err)
		}
		rep.Fixtures = append(rep.Fixtures, fr)
	}
	return rep, nil
}

func (s *Service) reparse(ctx context.Context, f store.Facts) (store.Facts, error) {
	if !strings.HasPrefix(f.Normalizer, s.Provider.Name()+"/") || len(f.RawPayloadIDs) == 0 {
		return f, fmt.Errorf("facts %d come from %q, cannot reparse with %s", f.ID, f.Normalizer, s.Provider.Name())
	}
	raw, err := s.Store.RawByID(ctx, slices.Max(f.RawPayloadIDs))
	if err != nil {
		return f, err
	}
	fx, err := s.Provider.ParseFixture(toProviderRaw(raw))
	if err != nil {
		return f, err
	}
	var md uuid.UUID
	if md, err = s.Store.Queries().FixtureMatchday(ctx, f.FixtureID); err != nil {
		return f, err
	}
	var out store.Facts
	err = s.Store.Tx(ctx, func(q *db.Queries) error {
		_, facts, err := s.upsertFixture(ctx, q, md, fx)
		if err != nil {
			return err
		}
		out.Facts = facts
		return nil
	})
	if err != nil {
		return f, err
	}
	return s.Store.SaveFacts(ctx, f.FixtureID, out.Facts, s.normalizer(), []int64{raw.ID})
}
