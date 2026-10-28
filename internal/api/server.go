package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/engine"
	"github.com/openfanta/openfanta/internal/settings"
	"github.com/openfanta/openfanta/internal/store/db"
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

// ListSeasons implements StrictServerInterface.
func (s *Server) ListSeasons(ctx context.Context, _ ListSeasonsRequestObject) (ListSeasonsResponseObject, error) {
	rows, err := s.q.ListSeasons(ctx)
	if err != nil {
		return nil, err
	}
	out := ListSeasons200JSONResponse{Seasons: make([]Season, 0, len(rows))}
	for _, r := range rows {
		out.Seasons = append(out.Seasons, season(db.GetSeasonRow(r)))
	}
	return out, nil
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
			Id: r.ID.String(), Matchday: n, Kickoff: r.Kickoff, Status: r.Status,
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
	list, err := scoreList(rs, vs, rows)
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
	list, err := scoreList(rs, vs, rows)
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
	list, err := scoreList(rs, vs, rows)
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
			Id: f.ID.String(), Matchday: int(f.Matchday), Kickoff: f.Kickoff, Status: f.Status,
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
	return GetPlayer200JSONResponse{Id: p.ID.String(), Name: p.Name}, nil
}

func scoreList(rs, vs string, rows []db.CurrentScore) (ScoreList, error) {
	out := ScoreList{SettingsSha256: rs, VotoSource: vs, Scores: make([]Score, 0, len(rows))}
	for _, r := range rows {
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
