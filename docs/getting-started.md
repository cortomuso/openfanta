---
title: Getting started
description: Set up the dev environment, run OpenFanta with or without Docker, and use the CLI.
---

## Quick start

Requirements: [Nix](https://nixos.org) with flakes enabled. Docker is optional.

```sh
nix develop          # or: direnv allow. Pins Go, Postgres, sqlc, oapi-codegen, golangci-lint
make test            # unit tests; database tests are skipped
make test-all        # every test, against a throwaway local Postgres in .pgdata/
make lint
```

`make test-all` starts Postgres from the devshell on port 5433 (`make pg-stop` stops it).
Tests never touch the network: provider responses come from synthetic fixtures in
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

### Sportmonks subscription

Each operator brings their own Sportmonks account and token; OpenFanta ships none. Serie A is not
on the free plan: the cheapest plan that includes it is Starter, with Serie A as one of its
leagues. Player ratings (type 118), line-ups and events are included in every paid plan, so no
add-on is needed. Starter allows 2,000 requests per entity per hour; one matchday takes about a
dozen. Players with only a few minutes may get no rating, and are then *senza voto*.

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
