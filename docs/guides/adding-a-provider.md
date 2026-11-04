---
title: Adding a data provider
description: Implement provider.Provider to ingest a new data source.
---

A provider is a Go package that implements `provider.Provider`
([internal/provider/provider.go](https://github.com/cortomuso/openfanta/blob/main/internal/provider/provider.go)):

```go
type Provider interface {
    Name() string            // stored with every raw payload, e.g. "sportmonks"
    ParserVersion() string   // bump when parsing the same payload can give different output

    FetchSeason(ctx, seasonRef) (Raw, error)    // I/O
    ParseSeason(Raw) (Season, error)            // pure
    FetchSchedule(ctx, seasonRef) (Raw, error)
    ParseSchedule(Raw) (Schedule, error)        // matchday number -> fixture refs
    FetchFixture(ctx, fixtureRef) (Raw, error)
    ParseFixture(Raw) (Fixture, error)          // the important one
    FetchSquad(ctx, seasonRef, teamRef) (Raw, error)
    ParseSquad(Raw) (Squad, error)              // a team's players: role, birth year, still listed
}
```

Steps:

1. Create `internal/provider/<name>/`. Use `httpx.Client` for requests: it handles timeouts,
   retries with backoff, `Retry-After` and client-side rate limiting. If the provider reports
   back-off somewhere other than the `Retry-After` header, set `RetryAfter`.
2. Keep Fetch and Parse separate. Fetch returns the response body verbatim. Parse must be
   pure, because stored payloads are re-parsed later without the network.
3. `ParseFixture` produces `domain.MatchFacts` using the provider's own IDs: lineups with roles and
   starters, optional ratings, and the events that matter (goals, penalties scored, missed or
   saved, own goals, substitutions, cards). Order events with `Seq`. Skip rescinded events and
   penalty shoot-outs. Set `Final` only once the match is over.
4. Add recorded responses under `testdata/` with a golden test, and check that
   `timeline.Derive` accepts the parsed facts. The Sportmonks package is a complete example.
5. Wire it in `cmd/openfanta` (today the Sportmonks client is created in `newService`).

Ingest owns caching, ID mapping (provider IDs never leak into the API) and versioning, so a
provider needs none of that.

## Squads and the player profile

`ParseSquad` feeds the player profile (`roles`, `club`, `birth_year`, `status`). Fill only what
the provider gives; leave the rest empty, because the API shows missing data as empty or `unknown`
and never guesses it.

- `Role` is the P/D/C/A catalog (`domain.CatalogRole`): map the provider's position to it.
  Sportmonks gives one position per player (24 goalkeeper, 25 defender, 26 midfielder, 27
  attacker), so every player has at most one role. Dual roles need another source.
- `Listed` is false once the player has left the team (Sportmonks: the squad entry has an `end`
  date in the past). That is what the API reports as `no_longer_listed`.
- `BirthYear` is the year of the provider's birth date.

Squads are stored per season, so they are also the club history. Run `openfanta squads --season S`
after a matchday of that season is ingested.
