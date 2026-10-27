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


**Documentation: <https://cortomuso.github.io/openfanta/>** (sources in [`docs/`](docs/), built from [`website/`](website/)).

## Quick start

Requirements: [Nix](https://nixos.org) with flakes enabled. Docker is optional.

```sh
nix develop          # or: direnv allow. Pins Go, Postgres, sqlc, oapi-codegen, golangci-lint
make test            # unit tests; database tests are skipped
make test-all        # every test, against a throwaway local Postgres in .pgdata/
make lint
```

`make test-all` starts Postgres from the devshell on port 5433 (`make pg-stop` stops it).
Running the server, configuration and the CLI are covered in [Getting started](docs/getting-started.md).

## Documentation

| Topic | Where |
|---|---|
| Run, configure, CLI | [docs/getting-started.md](docs/getting-started.md) |
| API conventions and spec | [docs/api-overview.md](docs/api-overview.md), [`api/openapi.yaml`](api/openapi.yaml) |
| Rule set keys, changing values | [docs/rulesets.md](docs/rulesets.md), [docs/guides/changing-ruleset-values.md](docs/guides/changing-ruleset-values.md) |
| Adding a data provider | [docs/guides/adding-a-provider.md](docs/guides/adding-a-provider.md) |
| Project layout | [docs/project-layout.md](docs/project-layout.md) |
| Design, decisions, open questions | [docs/design.md](docs/design.md) |

Preview the site locally with `make docs-dev` (needs Node, provided by `nix develop`).

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
