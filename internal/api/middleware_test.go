package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecovererReturnsProblem(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := recoverer(log)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != 500 || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("panic: %d %v", rec.Code, rec.Header())
	}
}

func TestETagSkipsNonGetAndErrors(t *testing.T) {
	h := etag(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			w.WriteHeader(404)
		}
		_, _ = w.Write([]byte("x"))
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/x", nil))
	if rec.Header().Get("ETag") != "" {
		t.Error("POST got an ETag")
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rec.Code != 404 || rec.Header().Get("ETag") != "" || rec.Body.String() != "x" {
		t.Errorf("404: %d %v %q", rec.Code, rec.Header(), rec.Body)
	}
}

func TestMatches(t *testing.T) {
	cases := map[string]bool{`"a"`: true, `W/"a"`: true, `"b", "a"`: true, `*`: true, `"b"`: false, ``: false}
	for in, want := range cases {
		if got := matches(in, `"a"`); got != want {
			t.Errorf("matches(%q) = %v", in, got)
		}
	}
}
