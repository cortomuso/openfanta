// Package httpx is a small HTTP client for upstream data providers: per-request
// timeouts, client-side rate limiting, and retries with exponential backoff and
// jitter on network errors, 5xx and 429.
package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/time/rate"
)

// StatusError is returned for a final non-2xx response.
type StatusError struct {
	Status int
	Body   []byte
}

func (e *StatusError) Error() string {
	snippet := e.Body
	if len(snippet) > 200 {
		snippet = snippet[:200]
	}
	return fmt.Sprintf("upstream status %d: %s", e.Status, snippet)
}

// Client performs GET requests with rate limiting and retries.
type Client struct {
	HTTP       *http.Client
	Limiter    *rate.Limiter // nil disables client-side limiting
	MaxRetries int           // retries after the first attempt
	BaseDelay  time.Duration // first backoff; doubles each retry
	MaxDelay   time.Duration
	Timeout    time.Duration // per attempt
	Logger     *slog.Logger
	// RetryAfter extracts a server-requested delay from a 429 response. The
	// default reads the Retry-After header in seconds.
	RetryAfter func(h http.Header, body []byte) time.Duration
	// Sleep waits for d or until ctx ends; replaceable in tests.
	Sleep func(ctx context.Context, d time.Duration) error
}

// New returns a Client with production defaults.
func New(limiter *rate.Limiter, logger *slog.Logger) *Client {
	return &Client{
		HTTP:       &http.Client{},
		Limiter:    limiter,
		MaxRetries: 4,
		BaseDelay:  500 * time.Millisecond,
		MaxDelay:   30 * time.Second,
		Timeout:    20 * time.Second,
		Logger:     logger,
	}
}

// Response is a successful upstream response.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Get fetches url with the given headers. It returns a *StatusError for a
// final non-2xx status.
func (c *Client) Get(ctx context.Context, url string, header http.Header) (*Response, error) {
	var lastErr error
	for attempt := 0; ; attempt++ {
		if c.Limiter != nil {
			if err := c.Limiter.Wait(ctx); err != nil {
				return nil, err
			}
		}
		resp, err := c.once(ctx, url, header)
		if err == nil && resp.Status < 300 {
			return resp, nil
		}

		delay, retryable := c.backoff(attempt), false
		switch {
		case err != nil:
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr, retryable = err, true
		case resp.Status == http.StatusTooManyRequests:
			lastErr, retryable = &StatusError{resp.Status, resp.Body}, true
			if d := c.retryAfter(resp); d > 0 {
				delay = d
			}
		case resp.Status >= 500:
			lastErr, retryable = &StatusError{resp.Status, resp.Body}, true
		default:
			return nil, &StatusError{resp.Status, resp.Body}
		}
		if !retryable || attempt >= c.MaxRetries {
			return nil, fmt.Errorf("after %d attempts: %w", attempt+1, lastErr)
		}
		if c.Logger != nil {
			c.Logger.WarnContext(ctx, "upstream request failed, retrying",
				"attempt", attempt+1, "delay", delay, "error", lastErr)
		}
		if err := c.sleep(ctx, delay); err != nil {
			return nil, err
		}
	}
}

func (c *Client) once(ctx context.Context, url string, header http.Header) (*Response, error) {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	return &Response{Status: resp.StatusCode, Header: resp.Header, Body: body}, nil
}

// backoff returns the full-jitter delay for a retry after attempt.
func (c *Client) backoff(attempt int) time.Duration {
	d := c.BaseDelay << min(attempt, 20)
	if d <= 0 || d > c.MaxDelay {
		d = c.MaxDelay
	}
	return d/2 + rand.N(d/2+1)
}

func (c *Client) retryAfter(r *Response) time.Duration {
	if c.RetryAfter != nil {
		return min(c.RetryAfter(r.Header, r.Body), c.MaxDelay*4)
	}
	if s, err := strconv.Atoi(r.Header.Get("Retry-After")); err == nil && s > 0 {
		return min(time.Duration(s)*time.Second, c.MaxDelay*4)
	}
	return 0
}

func (c *Client) sleep(ctx context.Context, d time.Duration) error {
	if c.Sleep != nil {
		return c.Sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// IsStatus reports whether err is a *StatusError with the given status.
func IsStatus(err error, status int) bool {
	var se *StatusError
	return errors.As(err, &se) && se.Status == status
}
