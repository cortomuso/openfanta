package ingest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/openfanta/openfanta/internal/provider"
	"github.com/openfanta/openfanta/internal/store"
	"github.com/openfanta/openfanta/internal/store/db"
)

// squadTTL is how long a team's squad is reused; squads change in transfer windows.
const squadTTL = 24 * time.Hour

// SquadReport is the outcome of ingesting the squads of a season.
type SquadReport struct {
	Teams   int
	Players int
	// Failed lists the team refs whose squad could not be fetched or stored.
	Failed map[string]error
}

// IngestSquads fetches the squad of every team that plays in the season and
// stores each player's club, role, birth year and listed status. Teams come
// from the fixtures already ingested, so ingest a matchday first. A failing
// team is reported and does not stop the others.
func (s *Service) IngestSquads(ctx context.Context, seasonRef string, opts Options) (SquadReport, error) {
	seasonRaw, _, err := s.fetch(ctx, "season "+seasonRef, seasonTTL, opts.Refresh, func() (provider.Raw, error) {
		return s.Provider.FetchSeason(ctx, seasonRef)
	}, "season/"+seasonRef)
	if err != nil {
		return SquadReport{}, err
	}
	season, err := s.Provider.ParseSeason(toProviderRaw(seasonRaw))
	if err != nil {
		return SquadReport{}, err
	}
	var seasonID db.SeasonTeamRefsParams
	var teams []db.SeasonTeamRefsRow
	err = s.Store.Tx(ctx, func(q *db.Queries) error {
		id, err := s.upsertSeason(ctx, q, season)
		if err != nil {
			return err
		}
		seasonID = db.SeasonTeamRefsParams{SeasonID: id, Provider: s.Provider.Name()}
		teams, err = q.SeasonTeamRefs(ctx, seasonID)
		return err
	})
	if err != nil {
		return SquadReport{}, fmt.Errorf("store season: %w", err)
	}
	if len(teams) == 0 {
		return SquadReport{}, errors.New("season has no ingested fixtures, so no teams: ingest a matchday first")
	}

	rep := SquadReport{Failed: map[string]error{}}
	for _, t := range teams {
		if ctx.Err() != nil {
			return rep, ctx.Err()
		}
		n, err := s.ingestSquad(ctx, seasonRef, seasonID, t, opts)
		if err != nil {
			s.Logger.ErrorContext(ctx, "squad failed", "team_ref", t.ExternalID, "error", err)
			rep.Failed[t.ExternalID] = err
			continue
		}
		rep.Teams++
		rep.Players += n
	}
	return rep, nil
}

func (s *Service) ingestSquad(ctx context.Context, seasonRef string, season db.SeasonTeamRefsParams, team db.SeasonTeamRefsRow, opts Options) (int, error) {
	raw, _, err := s.fetch(ctx, "squad "+seasonRef+"/"+team.ExternalID, squadTTL, opts.Refresh, func() (provider.Raw, error) {
		return s.Provider.FetchSquad(ctx, seasonRef, team.ExternalID)
	}, "squad/"+seasonRef+"/"+team.ExternalID)
	if err != nil {
		return 0, err
	}
	squad, err := s.Provider.ParseSquad(toProviderRaw(raw))
	if err != nil {
		return 0, err
	}
	p := s.Provider.Name()
	err = s.Store.Tx(ctx, func(q *db.Queries) error {
		for _, m := range squad.Members {
			id, err := store.Resolve(ctx, q, p, "player", m.PlayerRef)
			if err != nil {
				return err
			}
			name := m.Name
			if name == "" {
				name = "Player " + m.PlayerRef // provider named nobody; a later fixture overwrites it
			}
			var birth *int16
			if m.BirthYear != nil {
				y := int16(*m.BirthYear)
				birth = &y
			}
			if err := q.UpsertPlayerProfile(ctx, db.UpsertPlayerProfileParams{ID: id, Name: name, BirthYear: birth}); err != nil {
				return err
			}
			var role *string
			if m.Role.Valid() {
				r := string(m.Role)
				role = &r
			}
			if err := q.UpsertPlayerSeason(ctx, db.UpsertPlayerSeasonParams{
				SeasonID: season.SeasonID, PlayerID: id, TeamID: team.ID, Role: role, Listed: m.Listed,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("store squad: %w", err)
	}
	return len(squad.Members), nil
}
