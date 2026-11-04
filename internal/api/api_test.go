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
	"strconv"
	"strings"
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
	"github.com/openfanta/openfanta/internal/settings"
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
	hc := httpx.New(nil, nil)
	hc.MaxRetries = 0
	sm := sportmonks.New("token", hc)
	sm.BaseURL = srv.URL
	svc := &ingest.Service{Store: st, Provider: sm, Sources: []voto.Source{voto.NewProviderRating()}, Logger: quiet, Now: time.Now}
	if _, err := svc.IngestMatchday(ctx, "777", 1, ingest.Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.IngestSquads(ctx, "777", ingest.Options{}); err != nil {
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

func TestListVotoSources(t *testing.T) {
	f := setup(t)
	var out api.VotoSourceList
	f.get("/v1/voto-sources", 200, &out)
	if out.Default != "provider-rating" || len(out.Sources) != 3 || out.Sources[0].Name != "provider-rating" || out.Sources[0].Version == "" {
		t.Fatalf("voto sources = %+v", out)
	}
}

func TestOverridingMatchdayExceptionsIsNotServedYet(t *testing.T) {
	f := setup(t)
	body := `{"overrides":[{"fixture_id":"0190a0a0-0000-7000-8000-000000000001","policy":"automatic_voto","voto":"6.00"}]}`
	req := httptest.NewRequest(http.MethodPut, "/v1/leagues/0190a0a0-0000-7000-8000-000000000002/matchdays/1/exceptions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("PUT = %d %s: %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
	}
	f.validate(req, rec)

	// A malformed override is a client error before it is a missing feature.
	bad := httptest.NewRequest(http.MethodPut, "/v1/leagues/0190a0a0-0000-7000-8000-000000000002/matchdays/1/exceptions", strings.NewReader(`{`))
	bad.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	f.h.ServeHTTP(rec, bad)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("malformed body = %d", rec.Code)
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
	if list.SettingsSha256 != settings.Classic().SHA256 || list.VotoSource != "provider-rating" || len(list.Scores) != 12 {
		t.Fatalf("matchday scores: settings=%s source=%s n=%d", list.SettingsSha256, list.VotoSource, len(list.Scores))
	}
	by := map[string]api.Score{}
	for _, s := range list.Scores {
		by[s.Player.Name] = s
	}
	striker := by["Home Striker"]
	if striker.Voto == nil || *striker.Voto != "8.10" || striker.Fantavoto == nil || *striker.Fantavoto != "12.10" {
		t.Errorf("Home Striker = %+v", striker)
	}
	if len(striker.Breakdown) != 2 || striker.Breakdown[0] != (api.BreakdownItem{Code: "goal", Count: 1, Points: "3.00"}) {
		t.Errorf("Home Striker breakdown = %+v", striker.Breakdown)
	}
	if striker.Run.SettingsSha256 != settings.Classic().SHA256 || striker.Matchday != 1 || striker.FixtureId != fixtureID {
		t.Errorf("Home Striker run/fixture = %+v", striker)
	}
	if sv := by["Away Reserve Keeper"]; !sv.SenzaVoto || sv.Voto != nil || sv.Fantavoto != nil || len(sv.Breakdown) != 0 {
		t.Errorf("s.v. keeper = %+v", sv)
	}
	if mid := by["Home Midfielder"]; *mid.Fantavoto != "2.60" {
		t.Errorf("Home Midfielder fantavoto = %s", *mid.Fantavoto)
	}
	if got := striker.Stats; got != (api.PlayerMatchStats{Goals: 1, Assists: 1, GoalsConceded: 1}) {
		t.Errorf("Home Striker stats = %+v", got)
	}
	if got := by["Home Keeper"].Stats; got != (api.PlayerMatchStats{PenaltiesSaved: 1, GoalsConceded: 1}) {
		t.Errorf("Home Keeper stats = %+v", got)
	}
	if got := by["Away Defender"].Stats; got != (api.PlayerMatchStats{YellowCards: 2, SentOff: true, GoalsConceded: 1}) {
		t.Errorf("Away Defender stats = %+v", got)
	}
	if got := by["Away Striker"].Stats; got != (api.PlayerMatchStats{PenaltiesScored: 1, PenaltiesMissed: 1, GoalsConceded: 3}) {
		t.Errorf("Away Striker stats = %+v", got)
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
	if len(ps.Scores) != 1 || *ps.Scores[0].Fantavoto != "12.10" {
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

	// Other voto sources simply have no scores yet.
	var empty api.ScoreList
	f.get("/v1/seasons/"+seasonID+"/matchdays/1/scores?voto_source=mantra", 200, &empty)
	if empty.VotoSource != "mantra" || len(empty.Scores) != 0 {
		t.Errorf("unknown voto source = %+v", empty)
	}
}

func TestDefaultSettings(t *testing.T) {
	f := setup(t)
	var got api.Settings
	f.get("/v1/settings/default", 200, &got)
	classic := settings.Classic()
	if got.Sha256 != classic.SHA256 {
		t.Errorf("sha256 = %s, want %s", got.Sha256, classic.SHA256)
	}
	// The body must hash back to the advertised sha256 once canonicalized.
	raw, err := json.Marshal(got.Body)
	if err != nil {
		t.Fatal(err)
	}
	canon, err := settings.Canonicalize(raw)
	if err != nil || string(canon) != string(classic.Canonical) {
		t.Errorf("body does not round-trip to the canonical document: %v", err)
	}
	events := got.Body["player"].(map[string]any)["events"].(map[string]any)
	if events["goal"] != float64(300) {
		t.Errorf("classic goal = %v", events["goal"])
	}
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

// walkScores follows next_cursor through a score list and returns the player ids in order.
func (f *fixture) walkScores(path string, limit int) []string {
	f.t.Helper()
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	var ids []string
	cursor := ""
	for i := 0; i < 100; i++ {
		url := path + sep + "limit=" + strconv.Itoa(limit)
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		var page api.ScoreList
		f.get(url, 200, &page)
		if len(page.Scores) > limit {
			f.t.Fatalf("%s: %d rows for limit %d", url, len(page.Scores), limit)
		}
		for _, s := range page.Scores {
			ids = append(ids, s.FixtureId+"/"+s.Player.Id)
		}
		if page.NextCursor == nil {
			return ids
		}
		cursor = *page.NextCursor
	}
	f.t.Fatalf("%s: paging does not end", path)
	return nil
}

func TestScoreListPagination(t *testing.T) {
	f := setup(t)
	var seasons struct{ Seasons []api.Season }
	f.get("/v1/seasons", 200, &seasons)
	var mds struct{ Matchdays []api.Matchday }
	f.get("/v1/seasons/"+seasons.Seasons[0].Id+"/matchdays", 200, &mds)
	path := "/v1/seasons/" + seasons.Seasons[0].Id + "/matchdays/1/scores"

	var all api.ScoreList
	f.get(path, 200, &all)
	if all.NextCursor != nil || len(all.Scores) < 3 {
		t.Fatalf("unpaged list: next=%v rows=%d", all.NextCursor, len(all.Scores))
	}
	want := make([]string, len(all.Scores))
	for i, s := range all.Scores {
		want[i] = s.FixtureId + "/" + s.Player.Id
	}
	// Page sizes below, at and above the list length, including an exact divisor.
	for _, limit := range []int{1, 2, len(want), len(want) + 1} {
		got := f.walkScores(path, limit)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("limit %d: pages differ from the whole list:\n got %v\nwant %v", limit, got, want)
		}
	}
}

func TestPaginationErrors(t *testing.T) {
	f := setup(t)
	for _, q := range []string{"limit=0", "limit=201", "limit=x", "cursor=garbage", "cursor=v1.bm9wZQ"} {
		var p api.Problem
		rec := f.get("/v1/seasons?"+q, 400, &p)
		if rec.Header().Get("Content-Type") != "application/problem+json" || p.Status != 400 {
			t.Errorf("%s: %s %+v", q, rec.Header().Get("Content-Type"), p)
		}
	}
}

func TestSeasonsPagination(t *testing.T) {
	f := setup(t)
	var page struct {
		Seasons    []api.Season
		NextCursor *string `json:"next_cursor"`
	}
	f.get("/v1/seasons?limit=1", 200, &page)
	if len(page.Seasons) != 1 {
		t.Fatalf("limit=1 returned %d seasons", len(page.Seasons))
	}
}

func TestPlayerProfile(t *testing.T) {
	f := setup(t)
	var seasons struct{ Seasons []api.Season }
	f.get("/v1/seasons", 200, &seasons)
	var list api.ScoreList
	f.get("/v1/seasons/"+seasons.Seasons[0].Id+"/matchdays/1/scores", 200, &list)
	byName := map[string]string{}
	for _, s := range list.Scores {
		byName[s.Player.Name] = s.Player.Id
	}
	get := func(name string) api.Player {
		t.Helper()
		var p api.Player
		f.get("/v1/players/"+byName[name], 200, &p)
		return p
	}

	k := get("Home Keeper")
	if k.Club == nil || k.Club.Name != "Home FC" || len(k.Roles) != 1 || k.Roles[0] != api.P ||
		k.BirthYear == nil || *k.BirthYear != 1998 || k.Status != api.Listed {
		t.Errorf("Home Keeper = %+v", k)
	}
	// Named by the squad with no position or birth date: nothing is invented.
	if m := get("Home Midfielder"); len(m.Roles) != 0 || m.BirthYear != nil || m.Status != api.Listed || m.Club == nil {
		t.Errorf("Home Midfielder = %+v", m)
	}
	// Played, but no squad names the player.
	if u := get("Away Defender"); u.Club != nil || u.Status != api.Unknown || u.Roles == nil || len(u.Roles) != 0 {
		t.Errorf("Away Defender = %+v", u)
	}
}
