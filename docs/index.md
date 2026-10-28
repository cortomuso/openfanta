---
title: OpenFanta
description: An open fantacalcio data framework.
template: splash
hero:
  tagline: Raw football data in, auditable voti and fantavoti out, over a read-only REST API.
  actions:
    - text: Get started
      link: /openfanta/getting-started/
      icon: right-arrow
    - text: API reference
      link: /openfanta/api/
      variant: minimal
---

OpenFanta ingests raw football data from a third-party provider, computes **voti** and
**fantavoti** from league settings defined as data, and serves them over a read-only HTTP API that any
client can consume.

```
provider (HTTP, cache, rate limit) → raw_payloads → parser → MatchFacts
    → timeline → engine (+ settings, + voti) → versioned runs → REST API
```

The first version covers the Classic rules, one provider (Sportmonks), Postgres storage with
auditable recomputation, and a REST API. Leagues are on the roadmap; see the
[design note](design.md).
