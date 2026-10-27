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
