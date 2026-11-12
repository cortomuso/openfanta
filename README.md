<p align="center">
  <img src="docs/assets/logo.png" alt="OpenFanta logo" width="96">
</p>

<h1 align="center">OpenFanta</h1>

<p align="center">
  <b>Open, auditable fantacalcio.</b><br>
  Raw football data in, voti and fantavoti out, over a read-only REST API.
</p>

<p align="center">
  <a href="https://github.com/cortomuso/openfanta/actions/workflows/docs.yml"><img alt="docs" src="https://github.com/cortomuso/openfanta/actions/workflows/docs.yml/badge.svg"></a>
  <a href="LICENSE"><img alt="License: AGPL-3.0" src="https://img.shields.io/badge/license-AGPL--3.0-blue"></a>
  <img alt="Go" src="https://img.shields.io/badge/built%20with-Go-00ADD8">
</p>

<p align="center">
  <a href="https://cortomuso.github.io/openfanta/">Documentation</a> ·
  <a href="https://cortomuso.github.io/openfanta/api/">API reference</a> ·
  <a href="docs/design.md">Design note</a> ·
  <a href="#roadmap">Roadmap</a>
</p>

## What is OpenFanta?

OpenFanta ingests raw football data from a third-party provider, computes **voti** and
**fantavoti** from league settings defined as data, and serves the results over a read-only HTTP
API that any client can consume. Serie A first, Classic rules first.

```
provider → raw payloads → MatchFacts → engine (+ settings, + voti) → versioned runs → REST API
```

## Why?

Fantacalcio runs on scores nobody can check. Voti come from closed sources, the rules live inside
each app, and two apps can give the same league two different results. Every new client has to
rebuild the scoring from scratch.

OpenFanta makes the scoring a shared, open layer:

- **Rules are data, not code.** A league's bonuses and maluses are a settings document anyone can read and diff.
- **Same input, same result.** Scoring is deterministic integer arithmetic, so every client and every member sees identical numbers.
- **Nothing is overwritten.** Raw payloads are append-only and every computation is a versioned, hashed run, so a number can always be traced back to its inputs.

## Highlights

- **Pluggable voto sources**: the provider's rating, OpenFanta's own model built from match events and stats, or the average of both.
- **Provider-agnostic core**: a pure engine over normalized match facts. Sportmonks is the first provider, [adding another](docs/guides/adding-a-provider.md) is a contained job.
- **Auditable recomputation**: change a setting or a model and recompute without losing the old results.
- **Spec-first API**: a versioned `/v1` REST API described by an [OpenAPI document](api/openapi.yaml).
- **One static binary**: `serve`, `ingest`, `recompute` and `migrate`, backed by Postgres.

## Try it

```sh
docker compose up -d
docker compose run --rm api ingest --season <season-id> --matchday 1   # needs a Sportmonks token
curl localhost:8080/v1/seasons
```

Setup, configuration and the CLI: [Getting started](https://cortomuso.github.io/openfanta/getting-started/).

## Documentation

Everything lives on the **[documentation site](https://cortomuso.github.io/openfanta/)**. Good places to start:

- [Getting started](docs/getting-started.md): run it, configure it, use the CLI
- [API overview](docs/api-overview.md) and the [OpenAPI spec](api/openapi.yaml)
- [League settings](docs/settings.md) and [changing the defaults](docs/guides/changing-default-settings.md)
- [Voto sources](docs/guides/voto-sources.md): how voti are produced
- [Design note](docs/design.md): architecture, decisions, open questions
- [Project layout](docs/project-layout.md)

## Roadmap

**Today:** the Fantavoti engine with Classic rules, Sportmonks ingestion, auditable Postgres
storage and the read-only API.

**Next: leagues.** The server will host leagues and compute every league's fantavoti from its own
settings, so all members get identical results whatever client they use. Settings format, audit log,
matchday lifecycle and auth are designed in [design §8–10](docs/design.md), backed by the
[league settings research](docs/research/league-settings/report.md).

Open questions, including provider data-redistribution terms, are tracked in the
[design note](docs/design.md).

## Contributing

Dev environment (Nix) and test commands are in [Getting started](docs/getting-started.md).

## License

[AGPL-3.0](LICENSE)
