// Package sportmonks implements provider.Provider for the Sportmonks Football API v3.
//
// Requests authenticate with the Authorization header (never a query string, so
// tokens stay out of logs) and go through httpx for rate limiting and retries.
package sportmonks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/openfanta/openfanta/internal/provider"
	"github.com/openfanta/openfanta/internal/provider/httpx"
)

// Name is the provider name used in storage.
const Name = "sportmonks"

// DefaultBaseURL is the Sportmonks Football API v3 root.
const DefaultBaseURL = "https://api.sportmonks.com/v3/football"

// Includes requested for a fixture: everything the parser reads.
const fixtureIncludes = "participants;lineups.details;events;state"

// Client talks to Sportmonks.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *httpx.Client
	Now     func() time.Time
}

var _ provider.Provider = (*Client)(nil)

// New returns a client. Sportmonks reports 429 back-off in the response body,
// so the HTTP client's RetryAfter is wired to read it.
func New(token string, http *httpx.Client) *Client {
	http.RetryAfter = retryAfter
	return &Client{BaseURL: DefaultBaseURL, Token: token, HTTP: http, Now: time.Now}
}

// Name implements provider.Provider.
func (c *Client) Name() string { return Name }

// ParserVersion implements provider.Provider.
func (c *Client) ParserVersion() string { return parserVersion }

// FetchSeason implements provider.Provider.
func (c *Client) FetchSeason(ctx context.Context, seasonRef string) (provider.Raw, error) {
	return c.get(ctx, "seasons/"+url.PathEscape(seasonRef), url.Values{"include": {"league.country"}})
}

// FetchSchedule implements provider.Provider.
func (c *Client) FetchSchedule(ctx context.Context, seasonRef string) (provider.Raw, error) {
	return c.get(ctx, "schedules/seasons/"+url.PathEscape(seasonRef), nil)
}

// FetchFixture implements provider.Provider.
func (c *Client) FetchFixture(ctx context.Context, fixtureRef string) (provider.Raw, error) {
	return c.get(ctx, "fixtures/"+url.PathEscape(fixtureRef), url.Values{"include": {fixtureIncludes}})
}

func (c *Client) get(ctx context.Context, path string, query url.Values) (provider.Raw, error) {
	u := c.BaseURL + "/" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	resp, err := c.HTTP.Get(ctx, u, http.Header{"Authorization": {c.Token}, "Accept": {"application/json"}})
	if err != nil {
		return provider.Raw{}, fmt.Errorf("sportmonks %s: %w", path, err)
	}
	return provider.Raw{Key: path, Status: resp.Status, Body: resp.Body, FetchedAt: c.Now().UTC()}, nil
}

// retryAfter reads the delay Sportmonks reports in a 429 body.
func retryAfter(_ http.Header, body []byte) time.Duration {
	var b struct {
		RetryAfter int `json:"retry_after"`
		RateLimit  struct {
			ResetsInSeconds int `json:"resets_in_seconds"`
		} `json:"rate_limit"`
	}
	if json.Unmarshal(body, &b) != nil {
		return 0
	}
	if b.RetryAfter > 0 {
		return time.Duration(b.RetryAfter) * time.Second
	}
	return time.Duration(b.RateLimit.ResetsInSeconds) * time.Second
}
