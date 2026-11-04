---
title: API conventions
description: Endpoints, score formats, errors and caching for the OpenFanta REST API.
---

The full, generated reference is under **API reference**; this page covers the conventions.

The spec is [`api/openapi.yaml`](https://github.com/cortomuso/openfanta/blob/main/api/openapi.yaml), and the server is generated from it.
Main endpoints:

```
GET /v1/seasons
GET /v1/seasons/{seasonId}/matchdays
GET /v1/seasons/{seasonId}/matchdays/{n}/scores?voto_source=provider-rating
GET /v1/seasons/{seasonId}/players/{playerId}/scores
GET /v1/fixtures/{fixtureId}/scores
GET /v1/fixtures/{fixtureId}/runs
GET /v1/players/{playerId}   roles (P/D/C/A), club, birth_year, status
GET /v1/settings/default
GET /healthz                GET /readyz
```

Lists take `limit` (1 to 200) and an opaque `cursor`; while more rows follow the response has a
`next_cursor`, which you pass back as `cursor`. Without either parameter you get the whole list.
Never build or parse a cursor, and a bad `limit` or `cursor` is a `400` problem.

Scores are decimal strings with two digits (`"6.38"`), never floats. `voto` is the provider
rating unrounded; each score also carries the player's match `stats` (goals, assists, cards,
goals conceded, ...). Errors are RFC 9457
`application/problem+json`. Responses carry an `ETag` for conditional requests.
