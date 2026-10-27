package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func testClient(sleeps *[]time.Duration) *Client {
	c := New(nil, nil)
	c.BaseDelay = 100 * time.Millisecond
	c.MaxDelay = time.Second
	c.Sleep = func(_ context.Context, d time.Duration) error {
		*sleeps = append(*sleeps, d)
		return nil
	}
	return c
}

func server(t *testing.T, statuses ...int) (*httptest.Server, *atomic.Int32) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(calls.Add(1)) - 1
		status := statuses[min(n, len(statuses)-1)]
		if status == http.StatusTooManyRequests {
			w.Header().Set("Retry-After", "7")
		}
		if r.Header.Get("Authorization") != "secret" {
			status = http.StatusUnauthorized
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

var auth = http.Header{"Authorization": {"secret"}}

func TestSuccess(t *testing.T) {
	var sleeps []time.Duration
	srv, calls := server(t, 200)
	resp, err := testClient(&sleeps).Get(context.Background(), srv.URL, auth)
	if err != nil || string(resp.Body) != `{"ok":true}` || calls.Load() != 1 || len(sleeps) != 0 {
		t.Fatalf("resp=%v err=%v calls=%d sleeps=%v", resp, err, calls.Load(), sleeps)
	}
}

func TestRetriesServerErrorsWithBackoff(t *testing.T) {
	var sleeps []time.Duration
	srv, calls := server(t, 500, 502, 503, 200)
	if _, err := testClient(&sleeps).Get(context.Background(), srv.URL, auth); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 4 || len(sleeps) != 3 {
		t.Fatalf("calls=%d sleeps=%v", calls.Load(), sleeps)
	}
	for i, d := range sleeps {
		ceiling := 100 * time.Millisecond << i
		if d < ceiling/2 || d > ceiling {
			t.Errorf("sleep %d = %v, want within [%v, %v]", i, d, ceiling/2, ceiling)
		}
	}
}

func TestHonoursRetryAfterOn429(t *testing.T) {
	var sleeps []time.Duration
	srv, _ := server(t, 429, 200)
	if _, err := testClient(&sleeps).Get(context.Background(), srv.URL, auth); err != nil {
		t.Fatal(err)
	}
	if len(sleeps) != 1 || sleeps[0] != 4*time.Second { // 7s capped at 4 × MaxDelay
		t.Fatalf("sleeps = %v", sleeps)
	}
}

func TestCustomRetryAfter(t *testing.T) {
	var sleeps []time.Duration
	srv, _ := server(t, 429, 200)
	c := testClient(&sleeps)
	c.RetryAfter = func(http.Header, []byte) time.Duration { return 2 * time.Second }
	if _, err := c.Get(context.Background(), srv.URL, auth); err != nil {
		t.Fatal(err)
	}
	if sleeps[0] != 2*time.Second {
		t.Fatalf("sleeps = %v", sleeps)
	}
}

func TestGivesUpAfterMaxRetries(t *testing.T) {
	var sleeps []time.Duration
	srv, calls := server(t, 503)
	_, err := testClient(&sleeps).Get(context.Background(), srv.URL, auth)
	if !IsStatus(err, 503) || calls.Load() != 5 {
		t.Fatalf("err=%v calls=%d", err, calls.Load())
	}
}

func TestDoesNotRetryClientErrors(t *testing.T) {
	var sleeps []time.Duration
	srv, calls := server(t, 200)
	_, err := testClient(&sleeps).Get(context.Background(), srv.URL, nil) // no auth → 401
	var se *StatusError
	if !errors.As(err, &se) || se.Status != 401 || calls.Load() != 1 {
		t.Fatalf("err=%v calls=%d", err, calls.Load())
	}
	if se.Error() == "" {
		t.Error("empty error message")
	}
}

func TestTimeoutIsRetriedThenFails(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	var sleeps []time.Duration
	c := testClient(&sleeps)
	c.Timeout = 20 * time.Millisecond
	c.MaxRetries = 1
	if _, err := c.Get(context.Background(), srv.URL, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
	if len(sleeps) != 1 {
		t.Fatalf("sleeps = %v", sleeps)
	}
}

func TestContextCancelStops(t *testing.T) {
	srv, _ := server(t, 500)
	c := New(nil, nil)
	c.BaseDelay = time.Hour
	c.MaxDelay = time.Hour
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.Get(ctx, srv.URL, auth); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
}

func TestLimiterIsApplied(t *testing.T) {
	srv, _ := server(t, 200)
	var sleeps []time.Duration
	c := testClient(&sleeps)
	c.Limiter = rate.NewLimiter(rate.Every(time.Hour), 1)
	if _, err := c.Get(context.Background(), srv.URL, auth); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := c.Get(ctx, srv.URL, auth); err == nil {
		t.Fatal("second request should block on the limiter and fail with the context")
	}
}
