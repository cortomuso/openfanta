package ingest

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/openfanta/openfanta/internal/provider"
	"github.com/openfanta/openfanta/internal/store"
	"github.com/openfanta/openfanta/internal/store/db"
)

// squadTTL is how long a team's squad is reused; squads change in transfer windows.
const squadTTL = 24 * time.Hour

// SquadReport is the outcome of ingesting the squads of a season.
type SquadReport struct {
	Teams   int // teams whose season squad was read
	Players int // players stored
	// Failed lists the team refs whose squad could not be fetched or parsed.
	Failed map[string]error
}

// squadEntry is one team's season squad row of a player.
type squadEntry struct {
	team db.SeasonTeamRefsRow
	m    provider.SquadMember
}

// IngestSquads stores each player's club, role, birth year and listed status for a season.
// Teams come from the fixtures already ingested, so ingest a matchday first.
//
// A player who moved during the season is in the season squads of two teams, and those squads
// carry no dates. For the running season the teams' current squads decide: the player's club is
// the team that has them now, and a player no current squad has is no longer listed (the club
// is then the first team, by ref, whose season squad had them). For a finished season every
// squad is as it was, so everyone is listed. A failing team is reported; if a current squad
// fails nothing is stored, because listed status needs all of them.
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
	var seasonID uuid.UUID
	var teams []db.SeasonTeamRefsRow
	err = s.Store.Tx(ctx, func(q *db.Queries) error {
		var err error
		if seasonID, err = s.upsertSeason(ctx, q, season); err != nil {
			return err
		}
		teams, err = q.SeasonTeamRefs(ctx, db.SeasonTeamRefsParams{SeasonID: seasonID, Provider: s.Provider.Name()})
		return err
	})
	if err != nil {
		return SquadReport{}, fmt.Errorf("store season: %w", err)
	}
	if len(teams) == 0 {
		return SquadReport{}, errors.New("season has no ingested fixtures, so no teams: ingest a matchday first")
	}

	rep := SquadReport{Failed: map[string]error{}}
	byPlayer := map[string][]squadEntry{}
	for _, t := range teams {
		if ctx.Err() != nil {
			return rep, ctx.Err()
		}
		squad, err := s.seasonSquad(ctx, seasonRef, t.ExternalID, opts)
		if err != nil {
			s.Logger.ErrorContext(ctx, "squad failed", "team_ref", t.ExternalID, "error", err)
			rep.Failed[t.ExternalID] = err
			continue
		}
		rep.Teams++
		for _, m := range squad.Members {
			byPlayer[m.PlayerRef] = append(byPlayer[m.PlayerRef], squadEntry{team: t, m: m})
		}
	}

	var current map[string]string // player ref -> team ref, for the running season only
	if season.Current {
		failedBefore := len(rep.Failed)
		if current, err = s.currentTeams(ctx, teams, opts, rep.Failed); err != nil {
			return rep, err
		}
		if len(rep.Failed) > failedBefore {
			return rep, errors.New("a current squad could not be read, so nothing was stored: listed status needs every team")
		}
	}

	refs := make([]string, 0, len(byPlayer))
	for ref := range byPlayer {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	err = s.Store.Tx(ctx, func(q *db.Queries) error {
		for _, ref := range refs {
			if err := s.storeSquadPlayer(ctx, q, seasonID, ref, byPlayer[ref], current, season.Current); err != nil {
				return fmt.Errorf("player %s: %w", ref, err)
			}
		}
		return nil
	})
	if err != nil {
		return rep, fmt.Errorf("store squads: %w", err)
	}
	rep.Players = len(refs)
	return rep, nil
}

func (s *Service) seasonSquad(ctx context.Context, seasonRef, teamRef string, opts Options) (provider.Squad, error) {
	raw, _, err := s.fetch(ctx, "squad "+seasonRef+"/"+teamRef, squadTTL, opts.Refresh, func() (provider.Raw, error) {
		return s.Provider.FetchSquad(ctx, seasonRef, teamRef)
	}, "squad/"+seasonRef+"/"+teamRef)
	if err != nil {
		return provider.Squad{}, err
	}
	return s.Provider.ParseSquad(toProviderRaw(raw))
}

// currentTeams maps each player to the team whose current squad has them. Failures are recorded
// in failed under "current/<team ref>". A player in two current squads belongs to the first team
// by ref.
func (s *Service) currentTeams(ctx context.Context, teams []db.SeasonTeamRefsRow, opts Options, failed map[string]error) (map[string]string, error) {
	today := s.Now().UTC().Truncate(24 * time.Hour)
	out := map[string]string{}
	for _, t := range teams {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		raw, _, err := s.fetch(ctx, "current squad "+t.ExternalID, squadTTL, opts.Refresh, func() (provider.Raw, error) {
			return s.Provider.FetchCurrentSquad(ctx, t.ExternalID)
		}, "current-squad/"+t.ExternalID)
		var squad provider.CurrentSquad
		if err == nil {
			squad, err = s.Provider.ParseCurrentSquad(toProviderRaw(raw))
		}
		if err != nil {
			s.Logger.ErrorContext(ctx, "current squad failed", "team_ref", t.ExternalID, "error", err)
			failed["current/"+t.ExternalID] = err
			continue
		}
		for _, m := range squad.Members {
			if m.Until != nil && m.Until.Before(today) {
				continue // contract over
			}
			if _, taken := out[m.PlayerRef]; !taken {
				out[m.PlayerRef] = t.ExternalID
			}
		}
	}
	return out, nil
}

func (s *Service) storeSquadPlayer(ctx context.Context, q *db.Queries, seasonID uuid.UUID, ref string, entries []squadEntry, current map[string]string, runningSeason bool) error {
	chosen, listed := entries[0], true
	if runningSeason {
		listed = false
		for _, e := range entries {
			if current[ref] == e.team.ExternalID {
				chosen, listed = e, true
			}
		}
	}
	id, err := store.Resolve(ctx, q, s.Provider.Name(), "player", ref)
	if err != nil {
		return err
	}
	name := chosen.m.Name
	if name == "" {
		name = "Player " + ref // provider named nobody; a later fixture overwrites it
	}
	var birth *int16
	if chosen.m.BirthYear != nil {
		y := int16(*chosen.m.BirthYear)
		birth = &y
	}
	if err := q.UpsertPlayerProfile(ctx, db.UpsertPlayerProfileParams{ID: id, Name: name, BirthYear: birth}); err != nil {
		return err
	}
	var role *string
	if chosen.m.Role.Valid() {
		r := string(chosen.m.Role)
		role = &r
	}
	return q.UpsertPlayerSeason(ctx, db.UpsertPlayerSeasonParams{
		SeasonID: seasonID, PlayerID: id, TeamID: chosen.team.ID, Role: role, Listed: listed,
	})
}
