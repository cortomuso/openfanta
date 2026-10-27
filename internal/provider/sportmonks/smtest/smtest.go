// Package smtest serves the Sportmonks test fixtures over HTTP, so tests can
// exercise the real client without touching the network.
package smtest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// Upstream is a fake Sportmonks API backed by the files in ../testdata.
type Upstream struct {
	mu       sync.Mutex
	files    map[string]string // request path -> body
	requests map[string]int
}

// Fixture reads a file from the Sportmonks testdata directory.
func Fixture(t testing.TB, name string) string {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// New starts a fake API serving season 777 (matchday 1: fixtures 5001 and
// 5002, where 5002 returns 404) and closes it on cleanup.
func New(t testing.TB) (*Upstream, *httptest.Server) {
	t.Helper()
	u := &Upstream{
		files: map[string]string{
			"/seasons/777":           Fixture(t, "season_777.json"),
			"/schedules/seasons/777": Fixture(t, "schedule_777.json"),
			"/fixtures/5001":         Fixture(t, "fixture_5001_ft.json"),
		},
		requests: map[string]int{},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.mu.Lock()
		defer u.mu.Unlock()
		u.requests[r.URL.Path]++
		body, ok := u.files[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"not found"}`)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return u, srv
}

// Set replaces the body served for path.
func (u *Upstream) Set(path, body string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.files[path] = body
}

// Count returns how many times path was requested.
func (u *Upstream) Count(path string) int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.requests[path]
}
