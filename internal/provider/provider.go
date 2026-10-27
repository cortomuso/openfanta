// Package provider defines the contract every upstream football data provider
// implements. Fetching (I/O) and parsing (pure) are separate methods, so raw
// payloads can be stored, cached and re-parsed without touching the network.
package provider

import (
	"context"
	"time"

	"github.com/openfanta/openfanta/internal/domain"
)

// Raw is one upstream response, stored verbatim before parsing.
type Raw struct {
	Key       string // request path, for diagnostics only; storage uses its own keys
	Status    int
	Body      []byte
	FetchedAt time.Time
}

// Provider fetches and parses data from one upstream API.
//
// All refs are the provider's own identifiers, as strings. Parse methods must
// be pure: same Raw in, same result out.
type Provider interface {
	// Name identifies the provider in storage, e.g. "sportmonks".
	Name() string
	// ParserVersion changes whenever parsing can produce different output from
	// the same raw payload, so normalized facts can be told apart.
	ParserVersion() string

	FetchSeason(ctx context.Context, seasonRef string) (Raw, error)
	ParseSeason(r Raw) (Season, error)

	// FetchSchedule fetches the season's matchdays and their fixtures.
	FetchSchedule(ctx context.Context, seasonRef string) (Raw, error)
	ParseSchedule(r Raw) (Schedule, error)

	FetchFixture(ctx context.Context, fixtureRef string) (Raw, error)
	ParseFixture(r Raw) (Fixture, error)
}

// Season describes a season of a competition.
type Season struct {
	Ref                string
	Name               string // e.g. "2026/2027"
	CompetitionRef     string
	CompetitionName    string
	CompetitionCountry string
}

// Schedule maps matchday numbers to fixture refs.
type Schedule struct {
	Matchdays map[int][]string
}

// Team is a team as named by the provider.
type Team struct {
	Ref  string
	Name string
}

// Fixture is a parsed fixture. Facts uses provider refs as player and team IDs.
type Fixture struct {
	Ref     string
	Final   bool // finished: no further changes expected except corrections
	Status  string
	Kickoff *time.Time
	Home    Team
	Away    Team
	Players map[string]string // player ref -> name
	Facts   domain.MatchFacts
}
