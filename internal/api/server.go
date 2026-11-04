package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/engine"
	"github.com/openfanta/openfanta/internal/settings"
	"github.com/openfanta/openfanta/internal/store/db"
	"github.com/openfanta/openfanta/internal/voto"
)

// DefaultVotoSource is the voto source used when none is requested.
const (
	DefaultVotoSource = "provider-rating"
)

// Server implements StrictServerInterface on top of the store's read queries.
type Server struct {
	q    *db.Queries
	ping func(context.Context) error
}

var _ StrictServerInterface = (*Server)(nil)

// NewServer returns a Server. ping backs the readiness probe.
func NewServer(q *db.Queries, ping func(context.Context) error) *Server {
	return &Server{q: q, ping: ping}
}

// NewHandler builds the complete HTTP handler: routes, error mapping and middleware.
func NewHandler(s *Server, log *slog.Logger) http.Handler {
	strict := NewStrictHandlerWithOptions(s, nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeProblem(w, http.StatusBadRequest, err.Error())
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			log.ErrorContext(r.Context(), "request failed", "path", r.URL.Path, "error", err)
			writeProblem(w, http.StatusInternalServerError, "internal error")
		},
	})
	h := HandlerWithOptions(strict, StdHTTPServerOptions{
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeProblem(w, http.StatusBadRequest, err.Error())
		},
	})
	return chain(h, recoverer(log), requestLogger(log), timeout(10*time.Second), etag)
}

func notFound(format string, args ...any) Problem {
	detail := fmt.Sprintf(format, args...)
	return Problem{Type: "about:blank", Title: http.StatusText(http.StatusNotFound), Status: http.StatusNotFound, Detail: &detail}
}

func badRequest(err error) Problem {
	detail := err.Error()
	return Problem{Type: "about:blank", Title: http.StatusText(http.StatusBadRequest), Status: http.StatusBadRequest, Detail: &detail}
}

func writeProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{Type: "about:blank", Title: http.StatusText(status), Status: status, Detail: &detail})
}

func or(p *string, def string) string {
	if p == nil || *p == "" {
		return def
	}
	return *p
}

// GetHealth implements StrictServerInterface.
func (s *Server) GetHealth(context.Context, GetHealthRequestObject) (GetHealthResponseObject, error) {
	return GetHealth200JSONResponse{Status: "ok"}, nil
}

// GetReady implements StrictServerInterface.
func (s *Server) GetReady(ctx context.Context, _ GetReadyRequestObject) (GetReadyResponseObject, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.ping(ctx); err != nil {
		detail := "database unavailable"
		return GetReady503ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse{
			Type: "about:blank", Title: http.StatusText(http.StatusServiceUnavailable), Status: http.StatusServiceUnavailable, Detail: &detail,
		}}, nil
	}
	return GetReady200JSONResponse{Status: "ok"}, nil
}

// GetDefaultSettings implements StrictServerInterface.
func (s *Server) GetDefaultSettings(_ context.Context, _ GetDefaultSettingsRequestObject) (GetDefaultSettingsResponseObject, error) {
	d := settings.Classic()
	var body map[string]any
	if err := json.Unmarshal(d.Canonical, &body); err != nil {
		return nil, err
	}
	return GetDefaultSettings200JSONResponse{Sha256: d.SHA256, Body: body}, nil
}

// PutMatchdayExceptions implements StrictServerInterface. Overrides are for
// league admins, and leagues have no authentication or lifecycle yet.
func (s *Server) PutMatchdayExceptions(_ context.Context, _ PutMatchdayExceptionsRequestObject) (PutMatchdayExceptionsResponseObject, error) {
	detail := "overriding the real-match policy needs league authentication and the matchday lifecycle, which are not built yet"
	return PutMatchdayExceptions501ApplicationProblemPlusJSONResponse(Problem{
		Type: "about:blank", Title: http.StatusText(http.StatusNotImplemented), Status: http.StatusNotImplemented, Detail: &detail,
	}), nil
}

// exception maps a stored exception to the API field; nil for a normal match.
func exception(e string) *FixtureException {
	if e == "" {
		return nil
	}
	v := FixtureException(e)
	return &v
}

// ListVotoSources implements StrictServerInterface.
func (s *Server) ListVotoSources(_ context.Context, _ ListVotoSourcesRequestObject) (ListVotoSourcesResponseObject, error) {
	out := ListVotoSources200JSONResponse{Default: DefaultVotoSource}
	for _, src := range voto.Default() {
		out.Sources = append(out.Sources, VotoSource{Name: src.Name(), Version: src.Version(), Description: src.Description()})
	}
	return out, nil
}

// ListSeasons implements StrictServerInterface.
func (s *Server) ListSeasons(ctx context.Context, req ListSeasonsRequestObject) (ListSeasonsResponseObject, error) {
	rows, err := s.q.ListSeasons(ctx)
	if err != nil {
		return nil, err
	}
	page, next, err := paginate(rows, func(r db.ListSeasonsRow) string { return r.ID.String() }, req.Params.Limit, req.Params.Cursor)
	if err != nil {
		return ListSeasons400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse(badRequest(err))}, nil
	}
	out := ListSeasons200JSONResponse{Seasons: make([]Season, 0, len(page)), NextCursor: nextCursor(next)}
	for _, r := range page {
		out.Seasons = append(out.Seasons, season(db.GetSeasonRow(r)))
	}
	return out, nil
}

// nextCursor maps the "" of a last page to an absent field.
func nextCursor(c string) *NextCursor {
	if c == "" {
		return nil
	}
	return &c
}

func season(r db.GetSeasonRow) Season {
	s := Season{Id: r.ID.String(), Name: r.Name}
	s.Competition.Id, s.Competition.Name, s.Competition.Country = r.CompetitionID.String(), r.CompetitionName, r.CompetitionCountry
	return s
}

// ListMatchdays implements StrictServerInterface.
func (s *Server) ListMatchdays(ctx context.Context, req ListMatchdaysRequestObject) (ListMatchdaysResponseObject, error) {
	se, err := s.q.GetSeason(ctx, req.SeasonId)
	if errors.Is(err, pgx.ErrNoRows) {
		return ListMatchdays404ApplicationProblemPlusJSONResponse(notFound("season %s not found", req.SeasonId)), nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.q.SeasonFixtures(ctx, req.SeasonId)
	if err != nil {
		return nil, err
	}
	out := ListMatchdays200JSONResponse{Season: season(se), Matchdays: []Matchday{}}
	for _, r := range rows {
		n := int(r.Matchday)
		if len(out.Matchdays) == 0 || out.Matchdays[len(out.Matchdays)-1].Number != n {
			out.Matchdays = append(out.Matchdays, Matchday{Number: n, Fixtures: []Fixture{}})
		}
		md := &out.Matchdays[len(out.Matchdays)-1]
		md.Fixtures = append(md.Fixtures, Fixture{
			Id: r.ID.String(), Matchday: n, Kickoff: r.Kickoff, Status: r.Status, Exception: exception(r.Exception),
			Home: Ref{Id: r.HomeID.String(), Name: r.HomeName}, Away: Ref{Id: r.AwayID.String(), Name: r.AwayName},
		})
	}
	return out, nil
}

// GetMatchdayScores implements StrictServerInterface.
func (s *Server) GetMatchdayScores(ctx context.Context, req GetMatchdayScoresRequestObject) (GetMatchdayScoresResponseObject, error) {
	rs, vs := settings.Classic().SHA256, or(req.Params.VotoSource, DefaultVotoSource)
	exists, err := s.q.MatchdayExists(ctx, db.MatchdayExistsParams{SeasonID: req.SeasonId, Number: int32(req.Number)})
	if err != nil {
		return nil, err
	}
	if !exists {
		return GetMatchdayScores404ApplicationProblemPlusJSONResponse(notFound("matchday %d of season %s not found", req.Number, req.SeasonId)), nil
	}
	rows, err := s.q.MatchdayScores(ctx, db.MatchdayScoresParams{SeasonID: req.SeasonId, Matchday: int32(req.Number), SettingsSha256: rs, VotoSource: vs})
	if err != nil {
		return nil, err
	}
	list, err := scoreList(rs, vs, rows, req.Params.Limit, req.Params.Cursor)
	if bp := (badPageError{}); errors.As(err, &bp) {
		return GetMatchdayScores400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse(badRequest(bp.error))}, nil
	}
	return GetMatchdayScores200JSONResponse(list), err
}

// GetPlayerSeasonScores implements StrictServerInterface.
func (s *Server) GetPlayerSeasonScores(ctx context.Context, req GetPlayerSeasonScoresRequestObject) (GetPlayerSeasonScoresResponseObject, error) {
	rs, vs := settings.Classic().SHA256, or(req.Params.VotoSource, DefaultVotoSource)
	if _, err := s.q.GetSeason(ctx, req.SeasonId); errors.Is(err, pgx.ErrNoRows) {
		return GetPlayerSeasonScores404ApplicationProblemPlusJSONResponse(notFound("season %s not found", req.SeasonId)), nil
	} else if err != nil {
		return nil, err
	}
	if _, err := s.q.GetPlayer(ctx, req.PlayerId); errors.Is(err, pgx.ErrNoRows) {
		return GetPlayerSeasonScores404ApplicationProblemPlusJSONResponse(notFound("player %s not found", req.PlayerId)), nil
	} else if err != nil {
		return nil, err
	}
	rows, err := s.q.PlayerSeasonScores(ctx, db.PlayerSeasonScoresParams{SeasonID: req.SeasonId, PlayerID: req.PlayerId, SettingsSha256: rs, VotoSource: vs})
	if err != nil {
		return nil, err
	}
	list, err := scoreList(rs, vs, rows, req.Params.Limit, req.Params.Cursor)
	if bp := (badPageError{}); errors.As(err, &bp) {
		return GetPlayerSeasonScores400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse(badRequest(bp.error))}, nil
	}
	return GetPlayerSeasonScores200JSONResponse(list), err
}

// GetFixtureScores implements StrictServerInterface.
func (s *Server) GetFixtureScores(ctx context.Context, req GetFixtureScoresRequestObject) (GetFixtureScoresResponseObject, error) {
	rs, vs := settings.Classic().SHA256, or(req.Params.VotoSource, DefaultVotoSource)
	if _, err := s.q.GetFixture(ctx, req.FixtureId); errors.Is(err, pgx.ErrNoRows) {
		return GetFixtureScores404ApplicationProblemPlusJSONResponse(notFound("fixture %s not found", req.FixtureId)), nil
	} else if err != nil {
		return nil, err
	}
	rows, err := s.q.FixtureScores(ctx, db.FixtureScoresParams{FixtureID: req.FixtureId, SettingsSha256: rs, VotoSource: vs})
	if err != nil {
		return nil, err
	}
	list, err := scoreList(rs, vs, rows, req.Params.Limit, req.Params.Cursor)
	if bp := (badPageError{}); errors.As(err, &bp) {
		return GetFixtureScores400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse(badRequest(bp.error))}, nil
	}
	return GetFixtureScores200JSONResponse(list), err
}

// GetFixtureRuns implements StrictServerInterface.
func (s *Server) GetFixtureRuns(ctx context.Context, req GetFixtureRunsRequestObject) (GetFixtureRunsResponseObject, error) {
	f, err := s.q.GetFixture(ctx, req.FixtureId)
	if errors.Is(err, pgx.ErrNoRows) {
		return GetFixtureRuns404ApplicationProblemPlusJSONResponse(notFound("fixture %s not found", req.FixtureId)), nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.q.FixtureRuns(ctx, req.FixtureId)
	if err != nil {
		return nil, err
	}
	out := GetFixtureRuns200JSONResponse{
		Fixture: Fixture{
			Id: f.ID.String(), Matchday: int(f.Matchday), Kickoff: f.Kickoff, Status: f.Status, Exception: exception(f.Exception),
			Home: Ref{Id: f.HomeID.String(), Name: f.HomeName}, Away: Ref{Id: f.AwayID.String(), Name: f.AwayName},
		},
		Runs: make([]Run, 0, len(rows)),
	}
	for _, r := range rows {
		run := Run{
			Id: r.ID.String(), SettingsSha256: r.SettingsSha256, VotoSource: r.VotoSource,
			EngineVersion: r.EngineVersion, InputHash: r.InputHash, Reason: r.Reason, CreatedAt: r.CreatedAt, Current: r.IsCurrent,
		}
		if r.SupersedesRunID != nil {
			id := r.SupersedesRunID.String()
			run.Supersedes = &id
		}
		out.Runs = append(out.Runs, run)
	}
	return out, nil
}

// GetPlayer implements StrictServerInterface.
func (s *Server) GetPlayer(ctx context.Context, req GetPlayerRequestObject) (GetPlayerResponseObject, error) {
	p, err := s.q.GetPlayer(ctx, req.PlayerId)
	if errors.Is(err, pgx.ErrNoRows) {
		return GetPlayer404ApplicationProblemPlusJSONResponse(notFound("player %s not found", req.PlayerId)), nil
	}
	if err != nil {
		return nil, err
	}
	history, err := s.q.PlayerClubHistory(ctx, req.PlayerId)
	if err != nil {
		return nil, err
	}
	return GetPlayer200JSONResponse(player(p, history)), nil
}

// squadStatus maps a squad entry's listed flag to the API status.
func squadStatus(listed bool) PlayerStatus {
	if listed {
		return Listed
	}
	return NoLongerListed
}

func squadRoles(role *string) []PlayerRole {
	if role == nil {
		return []PlayerRole{}
	}
	return []PlayerRole{PlayerRole(*role)}
}

// player maps the stored profile to the API. Roles, club and status are those of the latest
// season in history (latest first) and stay empty or unknown when no squad named the player:
// nothing is guessed from match data.
func player(p db.Player, history []db.PlayerClubHistoryRow) Player {
	out := Player{Id: p.ID.String(), Name: p.Name, Roles: []PlayerRole{}, Status: Unknown, ClubHistory: make([]ClubSeason, 0, len(history))}
	if p.BirthYear != nil {
		y := int(*p.BirthYear)
		out.BirthYear = &y
	}
	for _, h := range history {
		cs := ClubSeason{Club: Ref{Id: h.ClubID.String(), Name: h.ClubName}, Roles: squadRoles(h.Role), Status: squadStatus(h.Listed)}
		cs.Season.Id, cs.Season.Name = h.SeasonID.String(), h.SeasonName
		out.ClubHistory = append(out.ClubHistory, cs)
	}
	if len(history) > 0 {
		out.Club = &out.ClubHistory[0].Club
		out.Roles = out.ClubHistory[0].Roles
		out.Status = out.ClubHistory[0].Status
	}
	return out
}

// ListPlayers implements StrictServerInterface.
func (s *Server) ListPlayers(ctx context.Context, req ListPlayersRequestObject) (ListPlayersResponseObject, error) {
	bad := func(err error) (ListPlayersResponseObject, error) {
		return ListPlayers400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse(badRequest(err))}, nil
	}
	pr := req.Params
	limit := DefaultPageSize
	if pr.Limit != nil {
		limit = *pr.Limit
	}
	if limit < 1 || limit > MaxLimit {
		return bad(fmt.Errorf("limit must be between 1 and %d", MaxLimit))
	}
	if pr.BirthYearFrom != nil && pr.BirthYearTo != nil && *pr.BirthYearFrom > *pr.BirthYearTo {
		return bad(errors.New("birth_year_from is after birth_year_to"))
	}
	sortBy := "name"
	if pr.Sort != nil {
		sortBy = string(*pr.Sort)
	}
	if sortBy != "name" && sortBy != "avg_voto" && sortBy != "avg_fantavoto" {
		return bad(fmt.Errorf("sort %q is not one of name, avg_voto, avg_fantavoto", sortBy))
	}
	if pr.Include != nil && *pr.Include != "stats" {
		return bad(fmt.Errorf("include %q is not stats", *pr.Include))
	}
	arg := db.ListPlayersParams{
		RowLimit: int32(limit) + 1, Sort: sortBy,
		SettingsSha256: settings.Classic().SHA256, VotoSource: or(pr.VotoSource, DefaultVotoSource),
	}
	if pr.Q != nil && *pr.Q != "" {
		p := likePattern(*pr.Q)
		arg.NamePattern = &p
	}
	if pr.Role != nil {
		if !domain.CatalogRole(*pr.Role).Valid() {
			return bad(fmt.Errorf("role %q is not one of P, D, C, A", *pr.Role))
		}
		r := string(*pr.Role)
		arg.Role = &r
	}
	arg.ClubID, arg.SeasonID = pr.Club, pr.Season
	if pr.BirthYearFrom != nil {
		v := int32(*pr.BirthYearFrom)
		arg.BirthFrom = &v
	}
	if pr.BirthYearTo != nil {
		v := int32(*pr.BirthYearTo)
		arg.BirthTo = &v
	}
	if pr.Cursor != nil && *pr.Cursor != "" {
		sortText, name, idText, err := decodeKeyset(*pr.Cursor)
		id, perr := uuid.Parse(idText)
		if err != nil || perr != nil {
			return bad(errBadCursor)
		}
		arg.AfterName, arg.AfterID = &name, &id
		if sortText != "" {
			v, err := strconv.ParseInt(sortText, 10, 32)
			if err != nil {
				return bad(errBadCursor)
			}
			v32 := int32(v)
			arg.AfterSort = &v32
		}
	}
	rows, err := s.q.ListPlayers(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := ListPlayers200JSONResponse{Players: make([]PlayerSummary, 0, len(rows))}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1]
		var sortValue *int32
		switch sortBy {
		case "avg_voto":
			sortValue = last.AvgVoto
		case "avg_fantavoto":
			sortValue = last.AvgFantavoto
		}
		sv := ""
		if sortValue != nil {
			sv = strconv.Itoa(int(*sortValue))
		}
		c := encodeKeyset(sv, last.Name, last.ID.String())
		out.NextCursor = &c
	}
	for _, r := range rows {
		ps := PlayerSummary{Id: r.ID.String(), Name: r.Name, Roles: squadRoles(r.Role), Status: Unknown}
		if r.BirthYear != nil {
			y := int(*r.BirthYear)
			ps.BirthYear = &y
		}
		if r.ClubID != nil {
			ps.Club = &Ref{Id: r.ClubID.String(), Name: *r.ClubName}
		}
		if r.Listed != nil {
			ps.Status = squadStatus(*r.Listed)
		}
		if pr.Include != nil && r.SeasonID != nil {
			st := seasonStats(*r.SeasonID, listStats(r))
			ps.Stats = &st
		}
		out.Players = append(out.Players, ps)
	}
	return out, nil
}

// listStats reads the season aggregates of a list row; a player with no scores has none, which
// shows as zeros and null averages.
func listStats(r db.ListPlayersRow) db.PlayerSeasonStat {
	n := func(v *int32) int32 {
		if v == nil {
			return 0
		}
		return *v
	}
	return db.PlayerSeasonStat{
		Appearances: n(r.Appearances), Minutes: n(r.Minutes), GamesWithVoto: n(r.GamesWithVoto),
		AvgVoto: r.AvgVoto, AvgFantavoto: r.AvgFantavoto,
		Goals: n(r.Goals), PenaltiesScored: n(r.PenaltiesScored), PenaltiesMissed: n(r.PenaltiesMissed),
		PenaltiesSaved: n(r.PenaltiesSaved), OwnGoals: n(r.OwnGoals), Assists: n(r.Assists),
		AssistsSetPiece: n(r.AssistsSetPiece), AssistsContribution: n(r.AssistsContribution),
		YellowCards: n(r.YellowCards), SentOff: n(r.SentOff), GoalsConceded: n(r.GoalsConceded),
		CleanSheets: n(r.CleanSheets), DecisiveGoalsWin: n(r.DecisiveGoalsWin),
		DecisiveGoalsDraw: n(r.DecisiveGoalsDraw), PlayerOfTheMatch: n(r.PlayerOfTheMatch),
	}
}

func seasonStats(seasonID uuid.UUID, r db.PlayerSeasonStat) SeasonStats {
	out := SeasonStats{
		Appearances: int(r.Appearances), Minutes: int(r.Minutes), GamesWithVoto: int(r.GamesWithVoto),
		AvgVoto: decimal(r.AvgVoto), AvgFantavoto: decimal(r.AvgFantavoto),
		Totals: StatTotals{
			Goals: int(r.Goals), PenaltiesScored: int(r.PenaltiesScored), PenaltiesMissed: int(r.PenaltiesMissed),
			PenaltiesSaved: int(r.PenaltiesSaved), OwnGoals: int(r.OwnGoals), Assists: int(r.Assists),
			AssistsSetPiece: int(r.AssistsSetPiece), AssistsContribution: int(r.AssistsContribution),
			YellowCards: int(r.YellowCards), SentOff: int(r.SentOff), GoalsConceded: int(r.GoalsConceded),
			CleanSheets: int(r.CleanSheets), DecisiveGoalsWin: int(r.DecisiveGoalsWin),
			DecisiveGoalsDraw: int(r.DecisiveGoalsDraw), PlayerOfTheMatch: int(r.PlayerOfTheMatch),
		},
	}
	out.Season.Id = seasonID.String()
	return out
}

// GetPlayerSeasonStats implements StrictServerInterface.
func (s *Server) GetPlayerSeasonStats(ctx context.Context, req GetPlayerSeasonStatsRequestObject) (GetPlayerSeasonStatsResponseObject, error) {
	rs, vs := settings.Classic().SHA256, or(req.Params.VotoSource, DefaultVotoSource)
	if _, err := s.q.GetSeason(ctx, req.SeasonId); errors.Is(err, pgx.ErrNoRows) {
		return GetPlayerSeasonStats404ApplicationProblemPlusJSONResponse(notFound("season %s not found", req.SeasonId)), nil
	} else if err != nil {
		return nil, err
	}
	p, err := s.q.GetPlayer(ctx, req.PlayerId)
	if errors.Is(err, pgx.ErrNoRows) {
		return GetPlayerSeasonStats404ApplicationProblemPlusJSONResponse(notFound("player %s not found", req.PlayerId)), nil
	}
	if err != nil {
		return nil, err
	}
	row, err := s.q.GetPlayerSeasonStats(ctx, db.GetPlayerSeasonStatsParams{PlayerID: req.PlayerId, SeasonID: req.SeasonId, SettingsSha256: rs, VotoSource: vs})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return GetPlayerSeasonStats200JSONResponse{
		Player: Ref{Id: p.ID.String(), Name: p.Name}, SettingsSha256: rs, VotoSource: vs,
		Stats: seasonStats(req.SeasonId, row), // no row: zeros and null averages
	}, nil
}

// ListClubs implements StrictServerInterface.
func (s *Server) ListClubs(ctx context.Context, req ListClubsRequestObject) (ListClubsResponseObject, error) {
	rows, err := s.q.ListClubs(ctx)
	if err != nil {
		return nil, err
	}
	page, next, err := paginate(rows, func(r db.Team) string { return r.ID.String() }, req.Params.Limit, req.Params.Cursor)
	if err != nil {
		return ListClubs400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse(badRequest(err))}, nil
	}
	out := ListClubs200JSONResponse{Clubs: make([]Ref, 0, len(page)), NextCursor: nextCursor(next)}
	for _, r := range page {
		out.Clubs = append(out.Clubs, Ref{Id: r.ID.String(), Name: r.Name})
	}
	return out, nil
}

// ListClubPlayers implements StrictServerInterface.
func (s *Server) ListClubPlayers(ctx context.Context, req ListClubPlayersRequestObject) (ListClubPlayersResponseObject, error) {
	club, err := s.q.GetClub(ctx, req.ClubId)
	if errors.Is(err, pgx.ErrNoRows) {
		return ListClubPlayers404ApplicationProblemPlusJSONResponse(notFound("club %s not found", req.ClubId)), nil
	}
	if err != nil {
		return nil, err
	}
	var se db.GetSeasonRow
	if req.Params.Season != nil {
		se, err = s.q.GetSeason(ctx, *req.Params.Season)
		if errors.Is(err, pgx.ErrNoRows) {
			return ListClubPlayers404ApplicationProblemPlusJSONResponse(notFound("season %s not found", *req.Params.Season)), nil
		}
	} else {
		var latest db.ClubLatestSeasonRow
		latest, err = s.q.ClubLatestSeason(ctx, req.ClubId)
		se = db.GetSeasonRow(latest)
	}
	out := ListClubPlayers200JSONResponse{Club: Ref{Id: club.ID.String(), Name: club.Name}, Players: []SquadPlayer{}}
	switch {
	case errors.Is(err, pgx.ErrNoRows): // a club without a squad has no season to show
		return out, nil
	case err != nil:
		return nil, err
	}
	sv := season(se)
	out.Season = &sv
	rows, err := s.q.ClubSquad(ctx, db.ClubSquadParams{TeamID: req.ClubId, SeasonID: se.ID})
	if err != nil {
		return nil, err
	}
	page, next, err := paginate(rows, func(r db.ClubSquadRow) string { return r.PlayerID.String() }, req.Params.Limit, req.Params.Cursor)
	if err != nil {
		return ListClubPlayers400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse(badRequest(err))}, nil
	}
	out.NextCursor = nextCursor(next)
	for _, r := range page {
		sp := SquadPlayer{Player: Ref{Id: r.PlayerID.String(), Name: r.PlayerName}, Roles: squadRoles(r.Role), Status: squadStatus(r.Listed)}
		if r.BirthYear != nil {
			y := int(*r.BirthYear)
			sp.BirthYear = &y
		}
		out.Players = append(out.Players, sp)
	}
	return out, nil
}

// badPageError carries a limit or cursor error so handlers answer 400 instead of 500.
type badPageError struct{ error }

func (e badPageError) Unwrap() error { return e.error }

func scoreList(rs, vs string, rows []db.CurrentScore, limit *int, cursor *string) (ScoreList, error) {
	page, next, err := paginate(rows, func(r db.CurrentScore) string { return r.FixtureID.String() + "/" + r.PlayerID.String() }, limit, cursor)
	if err != nil {
		return ScoreList{}, badPageError{err}
	}
	out := ScoreList{SettingsSha256: rs, VotoSource: vs, Scores: make([]Score, 0, len(page)), NextCursor: nextCursor(next)}
	for _, r := range page {
		sc, err := score(r)
		if err != nil {
			return ScoreList{}, err
		}
		out.Scores = append(out.Scores, sc)
	}
	return out, nil
}

func score(r db.CurrentScore) (Score, error) {
	var items []engine.BreakdownItem
	if err := json.Unmarshal(r.Breakdown, &items); err != nil {
		return Score{}, fmt.Errorf("breakdown of run %s: %w", r.RunID, err)
	}
	breakdown := make([]BreakdownItem, 0, len(items))
	for _, it := range items {
		breakdown = append(breakdown, BreakdownItem{Code: string(it.Code), Count: it.Count, Points: it.Points.Fixed2()})
	}
	return Score{
		Player: Ref{Id: r.PlayerID.String(), Name: r.PlayerName}, Team: Ref{Id: r.TeamID.String(), Name: r.TeamName},
		FixtureId: r.FixtureID.String(), Matchday: int(r.Matchday), Role: ScoreRole(r.Role), Minutes: int(r.Minutes),
		Voto: decimal(r.Voto), Fantavoto: decimal(r.Fantavoto), SenzaVoto: r.SenzaVoto, VotoUfficio: r.VotoUfficio,
		Stats: PlayerMatchStats{
			Goals: int(r.Goals), PenaltiesScored: int(r.PenaltiesScored), PenaltiesMissed: int(r.PenaltiesMissed),
			PenaltiesSaved: int(r.PenaltiesSaved), OwnGoals: int(r.OwnGoals), Assists: int(r.Assists),
			YellowCards: int(r.YellowCards), SentOff: r.SentOff, GoalsConceded: int(r.GoalsConceded),
			CleanSheet:      r.CleanSheet,
			AssistsSetPiece: int(r.AssistsSetPiece), AssistsContribution: int(r.AssistsContribution),
			DecisiveGoalsWin: int(r.DecisiveGoalsWin), DecisiveGoalsDraw: int(r.DecisiveGoalsDraw),
			PlayerOfTheMatch: r.PlayerOfTheMatch,
		},
		Breakdown: breakdown,
		Run:       RunRef{Id: r.RunID.String(), SettingsSha256: r.SettingsSha256, VotoSource: r.VotoSource, ComputedAt: r.ComputedAt},
	}, nil
}

func decimal(v *int32) *Decimal {
	if v == nil {
		return nil
	}
	s := domain.Points(*v).Fixed2()
	return &s
}
