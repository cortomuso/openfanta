# OpenFanta

OpenFanta is an open fantacalcio data framework. It ingests raw football data from a
third-party provider, computes **voti** and **fantavoti** from rule sets defined as data, and
serves them over a read-only HTTP API that any client can consume.

This first version covers the Fantavoti engine with **Classic** rules, one provider
(**Sportmonks**), Postgres storage with auditable recomputation, and a REST API.
Architecture and decisions: [docs/design.md](docs/design.md).

```
provider (HTTP, cache, rate limit) → raw_payloads → parser → MatchFacts
    → timeline → engine (+ rule set, + voti) → versioned runs → REST API
```

## Quick start

Requirements: [Nix](https://nixos.org) with flakes enabled. Docker is optional.

```sh
nix develop          # or: direnv allow. Pins Go, Postgres, sqlc, oapi-codegen, golangci-lint
make test            # unit tests; database tests are skipped
make test-all        # every test, against a throwaway local Postgres in .pgdata/
make lint
```

`make test-all` starts Postgres from the devshell on port 5433 (`make pg-stop` stops it).
Tests never touch the network: provider responses come from recorded fixtures in
`internal/provider/sportmonks/testdata/`.

## Running

Configuration comes from environment variables only. Copy `.env.example` to `.env` (gitignored)
and fill it in.

| Variable | Default | Meaning |
|---|---|---|
| `OPENFANTA_DATABASE_URL` | required | Postgres connection URL |
| `OPENFANTA_SPORTMONKS_TOKEN` | required for `ingest` | Sportmonks API token |
| `OPENFANTA_SPORTMONKS_REQUESTS_PER_HOUR` | `1800` | Client-side request budget, kept below the plan limit |
| `OPENFANTA_HTTP_ADDR` | `:8080` | API listen address |
| `OPENFANTA_LOG_LEVEL` / `OPENFANTA_LOG_FORMAT` | `info` / `json` | Structured logging (`text` for humans) |
| `OPENFANTA_RULESETS_DIR` | none | Extra rule set files, merged with the built-in ones |
| `OPENFANTA_SHUTDOWN_TIMEOUT` | `15s` | How long to drain in-flight requests on SIGTERM |
| `OPENFANTA_MIGRATE_ON_STARTUP` | `false` | Apply migrations before `serve`/`ingest` |

### With Docker

```sh
docker compose up -d                       # Postgres, migrations, API on 127.0.0.1:8080
docker compose run --rm api ingest --season <season-id> --matchday 1
curl localhost:8080/v1/seasons
```

### Without Docker

```sh
make pg-start
export OPENFANTA_DATABASE_URL=$(scripts/dev-postgres.sh url)
export OPENFANTA_SPORTMONKS_TOKEN=...      # or put both in .env and `set -a; . ./.env`
make build
./bin/openfanta migrate
./bin/openfanta ingest --season <season-id> --matchday 1
./bin/openfanta serve
```

`<season-id>` is the provider's season ID (for Sportmonks, the ID of Serie A 2026/2027).

### Commands

| Command | What it does |
|---|---|
| `openfanta migrate` | Applies database migrations |
| `openfanta ingest --season S --matchday N [--refresh]` | Fetches a matchday, stores raw payloads, computes fantavoti for finished fixtures. Finished fixtures are cached forever; `--refresh` re-fetches them to pick up upstream corrections |
| `openfanta recompute [--reparse] [--season-id UUID] [--reason TEXT]` | Recomputes stored fixtures under the current rule sets with no network access. `--reparse` re-parses stored raw payloads first (after a parser fix) |
| `openfanta rulesets` | Lists loaded rule sets and their content hashes |
| `openfanta serve` | Serves the API |

A recompute whose inputs did not change creates nothing. When something does change (a corrected
upstream event, a new rule set version, a parser fix), a new **run** supersedes the old one, and
both stay queryable at `GET /v1/fixtures/{id}/runs`.

## API

The spec is [`api/openapi.yaml`](api/openapi.yaml), and the server is generated from it.
Main endpoints:

```
GET /v1/seasons
GET /v1/seasons/{seasonId}/matchdays
GET /v1/seasons/{seasonId}/matchdays/{n}/scores?ruleset=classic&voto_source=provider-rating
GET /v1/seasons/{seasonId}/players/{playerId}/scores
GET /v1/fixtures/{fixtureId}/scores
GET /v1/fixtures/{fixtureId}/runs
GET /v1/rulesets            GET /v1/rulesets/{id}/versions/{version}
GET /healthz                GET /readyz
```

Scores are decimal strings with two digits (`"6.38"`), never floats. `voto` is the provider
rating unrounded; each score also carries the player's match `stats` (goals, assists, cards,
goals conceded, ...). Errors are RFC 9457
`application/problem+json`. Responses carry an `ETag` for conditional requests.

## Changing bonus and malus values

The values are data, so changing them needs no code change. See [docs/rulesets.md](docs/rulesets.md) for every key.

1. Copy `rulesets/classic.yaml`, change the values, and **bump `version`**. A published version
   is immutable, and the server refuses to start if its content changed.
2. Put the file in `rulesets/` (built into the binary) or in `OPENFANTA_RULESETS_DIR`.
3. Run `openfanta recompute --reason "classic 1.1.0"`.

## Adding a data provider

A provider is a Go package that implements `provider.Provider`
([internal/provider/provider.go](internal/provider/provider.go)):

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

## Project layout

```
cmd/openfanta/        CLI: serve, ingest, recompute, migrate, rulesets
internal/domain/      Points (integer hundredths), MatchFacts, PlayerLine
internal/rules/       rule set loading and validation
internal/timeline/    pure: minutes, goals conceded on the pitch, per-player lines
internal/engine/      pure: voto + bonus/malus → fantavoto, with breakdown
internal/voto/        voto sources (provider-rating)
internal/provider/    provider contract, resilient HTTP client, Sportmonks
internal/ingest/      fetch → store → parse → compute → record
internal/store/       Postgres: migrations, sqlc queries
internal/api/         generated server + handlers
api/openapi.yaml      API contract
rulesets/             built-in rule sets and their JSON Schema
```

Generated code (`internal/store/db`, `internal/api/gen.go`) is committed. After changing SQL or
the spec, run `make gen`.

## Roadmap: leagues

The server will host leagues and compute every league's fantavoti from its own settings, so all
members get identical results whatever client they use. The design (settings format, audit log,
matchday lifecycle, auth) is in [docs/design.md](docs/design.md) §8–10, backed by the research in
[docs/research/league-settings/](docs/research/league-settings/report.md).

## Status and open questions

See section 7 of [docs/design.md](docs/design.md). In short: the Sportmonks test fixtures are
synthetic (see their README) until a real token is available, and voti currently come from the
provider's player rating.

## License

[AGPL-3.0](LICENSE).
