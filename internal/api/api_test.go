package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"

	"github.com/openfanta/openfanta/internal/api"
	"github.com/openfanta/openfanta/internal/ingest"
	"github.com/openfanta/openfanta/internal/provider/httpx"
	"github.com/openfanta/openfanta/internal/provider/sportmonks"
	"github.com/openfanta/openfanta/internal/provider/sportmonks/smtest"
	"github.com/openfanta/openfanta/internal/rules"
	"github.com/openfanta/openfanta/internal/store"
	"github.com/openfanta/openfanta/internal/store/storetest"
	"github.com/openfanta/openfanta/internal/voto"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// fixture is an API backed by a database seeded with matchday 1 of the
// Sportmonks test season.
type fixture struct {
	t      *testing.T
	h      http.Handler
	router routers.Router
}

func setup(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	st, _ := storetest.New(t)
	_, srv := smtest.New(t)
	reg, err := rules.LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SyncRuleSets(ctx, reg.All()); err != nil {
		t.Fatal(err)
	}
	hc := httpx.New(nil, nil)
	hc.MaxRetries = 0
	sm := sportmonks.New("token", hc)
	sm.BaseURL = srv.URL
	svc := &ingest.Service{Store: st, Provider: sm, Rules: reg, Voto: voto.NewProviderRating(), Logger: quiet, Now: time.Now}
	if _, err := svc.IngestMatchday(ctx, "777", 1, ingest.Options{}); err != nil {
		t.Fatal(err)
	}

	spec, err := api.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	spec.Servers = nil
	router, err := legacy.NewRouter(spec)
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, h: api.NewHandler(api.NewServer(st.Queries(), st.Ping), quiet), router: router}
}

// get performs a request, checks the response against the OpenAPI spec, and
// decodes the body into out (if non-nil).
func (f *fixture) get(path string, wantStatus int, out any) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		f.t.Fatalf("GET %s = %d, want %d: %s", path, rec.Code, wantStatus, rec.Body)
	}
	f.validate(req, rec)
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			f.t.Fatalf("decode %s: %v", path, err)
		}
	}
	return rec
}

func (f *fixture) validate(req *http.Request, rec *httptest.ResponseRecorder) {
	f.t.Helper()
	route, params, err := f.router.FindRoute(req)
	if err != nil {
		if rec.Code == http.StatusNotFound && errors.Is(err, routers.ErrPathNotFound) {
			return
		}
		f.t.Fatalf("route %s: %v", req.URL, err)
	}
	in := &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route}
	err = openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
		RequestValidationInput: in, Status: rec.Code, Header: rec.Header(),
		Body: io.NopCloser(bytes.NewReader(rec.Body.Bytes())),
	})
	if err != nil {
		f.t.Errorf("GET %s: response does not match the OpenAPI spec: %v", req.URL, err)
	}
}

func TestBrowseAndScores(t *testing.T) {
	f := setup(t)

	var seasons struct{ Seasons []api.Season }
	f.get("/v1/seasons", 200, &seasons)
	if len(seasons.Seasons) != 1 || seasons.Seasons[0].Name != "2026/2027" || seasons.Seasons[0].Competition.Name != "Serie A" {
		t.Fatalf("seasons = %+v", seasons)
	}
	seasonID := seasons.Seasons[0].Id

	var mds struct{ Matchdays []api.Matchday }
	f.get("/v1/seasons/"+seasonID+"/matchdays", 200, &mds)
	if len(mds.Matchdays) != 1 || mds.Matchdays[0].Number != 1 || len(mds.Matchdays[0].Fixtures) != 1 {
		t.Fatalf("matchdays = %+v (fixture 5002 failed upstream and must be absent)", mds)
	}
	fixtureID := mds.Matchdays[0].Fixtures[0].Id

	var list api.ScoreList
	f.get("/v1/seasons/"+seasonID+"/matchdays/1/scores", 200, &list)
	if list.Ruleset != "classic" || list.VotoSource != "provider-rating" || len(list.Scores) != 12 {
		t.Fatalf("matchday scores: ruleset=%s source=%s n=%d", list.Ruleset, list.VotoSource, len(list.Scores))
	}
	by := map[string]api.Score{}
	for _, s := range list.Scores {
		by[s.Player.Name] = s
	}
	striker := by["Home Striker"]
	if striker.Voto == nil || *striker.Voto != "8" || striker.Fantavoto == nil || *striker.Fantavoto != "12" {
		t.Errorf("Home Striker = %+v", striker)
	}
	if len(striker.Breakdown) != 2 || striker.Breakdown[0] != (api.BreakdownItem{Code: "goal", Count: 1, Points: "3"}) {
		t.Errorf("Home Striker breakdown = %+v", striker.Breakdown)
	}
	if striker.Run.RulesetVersion != "1.0.0" || striker.Matchday != 1 || striker.FixtureId != fixtureID {
		t.Errorf("Home Striker run/fixture = %+v", striker)
	}
	if sv := by["Away Reserve Keeper"]; !sv.SenzaVoto || sv.Voto != nil || sv.Fantavoto != nil || len(sv.Breakdown) != 0 {
		t.Errorf("s.v. keeper = %+v", sv)
	}
	if mid := by["Home Midfielder"]; *mid.Fantavoto != "2.5" {
		t.Errorf("Home Midfielder fantavoto = %s", *mid.Fantavoto)
	}

	var fx api.ScoreList
	f.get("/v1/fixtures/"+fixtureID+"/scores", 200, &fx)
	if len(fx.Scores) != 12 {
		t.Errorf("fixture scores = %d", len(fx.Scores))
	}

	playerID := striker.Player.Id
	var player api.Ref
	f.get("/v1/players/"+playerID, 200, &player)
	if player.Name != "Home Striker" {
		t.Errorf("player = %+v", player)
	}
	var ps api.ScoreList
	f.get("/v1/seasons/"+seasonID+"/players/"+playerID+"/scores", 200, &ps)
	if len(ps.Scores) != 1 || *ps.Scores[0].Fantavoto != "12" {
		t.Errorf("player season scores = %+v", ps)
	}

	var runs struct {
		Fixture api.Fixture
		Runs    []api.Run
	}
	f.get("/v1/fixtures/"+fixtureID+"/runs", 200, &runs)
	if len(runs.Runs) != 1 || !runs.Runs[0].Current || runs.Runs[0].Supersedes != nil || runs.Fixture.Home.Name != "Home FC" {
		t.Errorf("runs = %+v", runs)
	}

	// Other rule set families or voto sources simply have no scores yet.
	var empty api.ScoreList
	f.get("/v1/seasons/"+seasonID+"/matchdays/1/scores?ruleset=mantra", 200, &empty)
	if empty.Ruleset != "mantra" || len(empty.Scores) != 0 {
		t.Errorf("unknown ruleset = %+v", empty)
	}
}

func TestRuleSets(t *testing.T) {
	f := setup(t)
	var list struct{ Rulesets []api.RuleSetSummary }
	f.get("/v1/rulesets", 200, &list)
	if len(list.Rulesets) != 1 || list.Rulesets[0].Id != "classic" || list.Rulesets[0].Name != "Classic" || len(list.Rulesets[0].ContentHash) != 64 {
		t.Fatalf("rulesets = %+v", list)
	}
	var rs api.RuleSet
	f.get("/v1/rulesets/classic/versions/1.0.0", 200, &rs)
	scores := rs.Body["scores"].(map[string]any)
	if scores["goal"] != float64(300) {
		t.Errorf("classic goal = %v", scores["goal"])
	}
	f.get("/v1/rulesets/classic/versions/9.9.9", 404, nil)
}

func TestErrors(t *testing.T) {
	f := setup(t)
	const missing = "00000000-0000-7000-8000-000000000000"
	for _, path := range []string{
		"/v1/seasons/" + missing + "/matchdays",
		"/v1/seasons/" + missing + "/matchdays/1/scores",
		"/v1/seasons/" + missing + "/players/" + missing + "/scores",
		"/v1/fixtures/" + missing + "/scores",
		"/v1/fixtures/" + missing + "/runs",
		"/v1/players/" + missing,
	} {
		rec := f.get(path, 404, nil)
		if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
			t.Errorf("%s content type = %s", path, ct)
		}
	}

	var seasons struct{ Seasons []api.Season }
	f.get("/v1/seasons", 200, &seasons)
	sid := seasons.Seasons[0].Id
	f.get("/v1/seasons/"+sid+"/matchdays/2/scores", 404, nil)
	f.get("/v1/seasons/"+sid+"/players/"+missing+"/scores", 404, nil)

	var p api.Problem
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/players/not-a-uuid", nil))
	if rec.Code != 400 || json.Unmarshal(rec.Body.Bytes(), &p) != nil || p.Status != 400 {
		t.Errorf("bad uuid: %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	f.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/seasons/"+sid+"/matchdays/one/scores", nil))
	if rec.Code != 400 {
		t.Errorf("bad matchday number: %d", rec.Code)
	}
}

func TestHealthAndETag(t *testing.T) {
	f := setup(t)
	f.get("/healthz", 200, nil)
	f.get("/readyz", 200, nil)

	rec := f.get("/v1/seasons", 200, nil)
	tag := rec.Header().Get("ETag")
	if tag == "" || rec.Header().Get("Cache-Control") != "public, max-age=60" {
		t.Fatalf("headers = %v", rec.Header())
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/seasons", nil)
	req.Header.Set("If-None-Match", tag)
	rec = httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Errorf("conditional GET = %d, body %q", rec.Code, rec.Body)
	}
}

func TestReadinessFailsWithoutDatabase(t *testing.T) {
	down := func(context.Context) error { return errors.New("down") }
	h := api.NewHandler(api.NewServer(nil, down), quiet)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("readyz = %d %v", rec.Code, rec.Header())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("healthz = %d %v", rec.Code, rec.Header())
	}
}

func TestStoreErrorsBecome500(t *testing.T) {
	ctx := context.Background()
	st, dsn := storetest.New(t)
	_ = dsn
	h := api.NewHandler(api.NewServer(st.Queries(), st.Ping), quiet)
	st.Close() // every query now fails
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/seasons", nil).WithContext(ctx))
	if rec.Code != http.StatusInternalServerError || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("closed store: %d %v %s", rec.Code, rec.Header(), rec.Body)
	}
	_ = store.ErrNotFound
}
