---
title: Project layout
description: Where things live in the repository.
---

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
