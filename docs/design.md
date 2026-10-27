---
title: "Design note"
description: "Architecture, decisions, data model, scoring semantics and the leagues design."
---

## 1. Architecture

```
            ┌──────────────┐  raw JSON   ┌──────────────┐
 Sportmonks ─▶ provider     ├────────────▶ raw_payloads  │ (append-only, sha256)
            │ (HTTP, cache, │            └──────┬───────┘
            │ rate limit)   │                   │
            └──────────────┘            normalizer (pure, per provider)
                                                ▼
                                         MatchFacts (provider-agnostic)
                                                │  + VotoSource (voti)
                                                │  + RuleSet (data)
                                                ▼
                                       engine.Compute (pure)
                                                ▼
                                computation_runs + player_scores (versioned)
                                                ▼
                                    REST API /v1 (read-only, OpenAPI 3.1)
```
- **Pure core**: `timeline`, `engine`, `rules` and the normalizers do no I/O. They take values in and return values out, and each is tested by table tests and golden files.
- **Imperative shell**: the provider HTTP client, the store, the ingest orchestration and the API.
- **Single binary** with subcommands: `openfanta serve | ingest | recompute | migrate`.

## 2. Decisions

### 2.1 Language: **Go** (chosen)
| | Go | Rust | Python | TypeScript |
|---|---|---|---|---|
| Correctness for a deterministic engine | Good: static types, explicit errors, integer fixed point. No sum types. | Best: enums, exhaustive match, no null | Weak: depends on mypy and Decimal discipline | Medium: `number` is a float, so it needs a branded int type |
| Performance | More than enough | Best | Enough | Enough |
| HTTP / ingestion ecosystem | Strong stdlib, pgx, sqlc, oapi-codegen | Good (axum, sqlx), more ceremony | Excellent (FastAPI, httpx) | Excellent |
| Ease of testing | stdlib `testing`, fast, race detector | Good, slower compile | pytest, very easy | vitest, easy |
| Deployment footprint | Static binary, ~20 MB distroless image | Same, longer builds | Interpreter + deps, larger image | Node runtime, larger |
| Contributor friendliness | High, quick to read | Steeper | Highest among football-data hobbyists | High |
| Fit for a public protocol | Spec-first OpenAPI works in any language; Go doesn't constrain implementers | Same, plus a WASM engine for clients | Same | Shares types with web clients |

**What would change the choice:** if clients need to run the exact engine on their side (browser or mobile, bit-identical), use Rust compiled to WASM. If most contributors turn out to be data and analytics people, use Python.

### 2.2 Provider: **Sportmonks v3** (chosen)
| | Sportmonks | API-Football | football-data.org |
|---|---|---|---|
| Serie A | Paid plans (Starter €29/mo, any 5 leagues, 14-day trial). **The free plan only covers Danish and Scottish leagues** | Free tier (100 req/day, limited seasons) | Free (fixtures only) |
| Granularity | Events: goal 14, own goal 15, penalty 16, missed penalty 17 (sub_type 1509 = saved), sub 18, cards 19/20/21. Assist = `related_player_id` on the goal. Per-player lineup details: rating 118, minutes 119, penalties 47 (scored/missed/saved), goals conceded 88 | Similar per-player stats + rating | Scorers/cards only on €29 Deep Data, no penalty saves, no rating |
| Rate limits | 2,000–5,000 calls per entity per hour; `rate_limit.remaining` and `resets_in_seconds` in the body; 429 with `retry_after` | 10–900/min by plan | 10–120/min |
| Redistribution | Not stated publicly; custom terms at Enterprise. **Must be verified** | ToS forbids reselling raw data | Ask the vendor |

Mitigation: the API serves only **derived** values (voto, fantavoto, breakdown) and never raw payloads. Even so, the provider-rating-derived voto needs a ToS check before any public deployment (open question Q3).

### 2.3 Database: **Postgres 16+** (chosen)
JSONB for raw payloads and breakdowns, a single SQL dialect, and concurrent ingest and serve. Migrations use goose, embedded in the binary. Queries are generated with sqlc on pgx/v5. Dev and tests use Postgres from the Nix devshell or docker compose.

> **Implemented parts: the code is the source of truth.** The sections below give the reasoning
> and point to the code; they deliberately don't restate behaviour that lives there.

### 2.4 Rule sets (interim scorer)
Bonus/malus values are data, not code, in integer hundredths so results are deterministic. A
published `id@version` is immutable. This format is the **interim** Classic scorer and will be
replaced by the per-league settings in §9 (the engine is generalized, not removed).
Code: `rulesets/schema.json`, `rulesets/classic.yaml`, `internal/rules/`, `internal/engine/`.

### 2.5 Versioning and audit
Raw payloads are append-only, normalized facts and computation runs are immutable, and a run is
only created when its input hash changes; the current run pointer moves in the same transaction.
Code: `internal/store/migrations/`, `store.RecordRun` in `internal/store/store.go`,
`inputHash` in `internal/ingest/ingest.go`.

### 2.6 Voto source
The engine never fetches voti; the shell resolves them and passes them in as data, so sources can
be swapped (a CSV/manual source later) without touching the engine. Voto is the provider rating
unrounded at two decimals; rounding is a league setting (§9).
Code: `internal/voto/voto.go`, `parseRating` in `internal/provider/sportmonks/parse.go`.

### 2.7 API style: REST + OpenAPI, spec-first
`api/openapi.yaml` is the contract and the server is generated from it. It is OpenAPI **3.0.3**,
not 3.1, because oapi-codegen and kin-openapi only fully support 3.0. Conventions chosen so the
API can be opened up later: versioned `/v1`, opaque UUIDv7 IDs (provider IDs never leak), decimals
as two-digit strings (no float drift for other implementations), RFC 9457 problem+json, ETags.
Cursor pagination will come once a list can grow unbounded.

## 3. Data model
Code: `internal/store/migrations/` (schema) and `internal/store/queries/` (access paths).
All scores are `integer` hundredths.

## 4. Scoring semantics
Code: `internal/timeline/` (per-player facts: minutes, goals conceded on the pitch, cards) and
`internal/engine/` (interim Classic fantavoto with breakdown). Their tests are the executable spec.

## 5. Package layout
See the "Project layout" section of the README and the package doc comments.
Provider contract: `internal/provider/provider.go`.

## 6. Operational
Configuration: `internal/config/config.go` (env only, see `.env.example`). Server timeouts and
graceful shutdown: `serve` in `cmd/openfanta/main.go`. Upstream retries, backoff and rate limiting:
`internal/provider/httpx/client.go`. Caching policy: `internal/ingest/ingest.go`.

## 7. Open questions
1. **Sportmonks plan:** Serie A isn't on the free plan. Will you take the Starter plan or the 14-day trial? A token is needed to record fixtures and for the DoD real-matchday run.
2. **Player rating availability:** confirm that rating (118) is included in the chosen plan. If not, voto falls back to s.v. for everyone, and we need a different VotoSource.
3. **Redistribution:** read the Sportmonks ToS (or ask them) before any public deployment of provider-rating-based voti.
4. **Classic defaults (interim scorer only):** confirm the s.v. policy (voto d'ufficio 6 for a goal, 4 for a red card), the clean-sheet minimum minutes (proposed: any minutes, with 0 conceded while on pitch), and the penalty-scored value (proposed 300, same as goal).
5. ~~**Licence**~~: decided, **AGPL-3.0** (see `LICENSE`).
6. **Target season** for the real-matchday demo (proposed: current Serie A 2026/27).


## 8. Leagues (design, not built yet)

**Principle: the server is the source of truth for every league.** Clients author settings, line-ups and market actions. The server validates them, computes fantavoti and team results, and serves the same deterministic outcome to every member, whatever client they use.

- **Entities:** league (name, season, invite code), members with roles (owner / admin / member), one fantasy team per member, rosters, matchday line-ups, competitions (league, cups).
- **Line-ups** lock at the first kickoff of each matchday. The server enforces this from fixture kickoff times.
- **Matchday lifecycle:** open → closed. On close, every input (ratings, events, line-ups, the settings version in force) is frozen into a hashed snapshot. Provider corrections after close are ignored.
- **Rule changes are never retroactive.** A new settings version applies only from an open matchday.
- **Closed-matchday corrections:** the owner or an admin can manually fix a closed matchday (a player's voto or event, a team score, a result). Each fix is an append-only, attributed entry (old → new, reason, time), visible to every member. It is applied as a layer on top of the frozen snapshot, and triggers a visible recompute of that matchday and the standings after it.
- **Auth:** OpenID Connect. Users sign in with an external identity provider; the server verifies bearer tokens and keys users by issuer + subject. No passwords are stored. League roles (authorization) are OpenFanta's own.
- **Privacy:** store only the identity reference and a display name. On account deletion, pseudonymize the user ("deleted user", opaque ID), so the audit log stays intact.
- **Write API conventions:** `ETag` + `If-Match` for optimistic concurrency (a stale save gets `412`); an `Idempotency-Key` header so retried actions (a line-up submission, a trade acceptance) are not applied twice; `application/problem+json` errors.

## 9. League settings format (design, not built yet)

Research behind this section: [league settings research](research/league-settings/report.md).

**Finding:** fantacalcio rules vary widely but shallowly. Almost every rule is one of a few shapes with parameters: per-event values with per-role overrides, band tables (modificatore difesa, captain), goal-threshold lists (fasce gol), zero-sum duels (modificatore centrocampo), score corrections, ordered tie-breakers. Substitution logic (Classic modes, Mantra Basic/Easy/Master) is algorithmic: it lives in engine code and leagues select it by name.

**Scoring pipeline.** Each stage has a catalog of typed modules:

| Stage | Depends on | Examples |
|---|---|---|
| 0. Real-match exceptions | the real fixture | postponed / suspended match policy |
| 1. Player fantavoto | the player (+ real match context) | bonus/malus by role, assist variants, gol decisivo, voto rounding, senza voto / voto d'ufficio |
| 2. Line-up resolution | own line-up | formations, bench, substitution algorithm, Mantra out-of-position malus, riserva d'ufficio |
| 3. Team modifiers | own line-up | modificatore difesa, captain, fair play |
| 4. Opponent and calendar | opponent, calendar | fattore campo, modificatore centrocampo, fasce gol, draw / superiority corrections |
| 5. Competition | whole league | head-to-head, total points, Formula 1, cups, tie-breakers |

Roster and market rules (quotas, budget, auction type) only validate actions; they never affect scores, and they version separately from scoring.

**Document rules:**
- `schema` versions the document shape: additive within a major version, new fields have defaults that keep the old meaning, and names are never reused.
- `engine` versions the semantics (tie-breaks, substitution algorithms, the expression environment) and is pinned per settings version, so an engine bug fix never silently changes old results. Adopting a new engine is an explicit, logged settings change.
- `requires` lists the features a document uses; a server that lacks one rejects the document instead of misreading it.
- Unknown keys inside scoring modules are rejected. Client UI data goes in `meta` / `x-*`, which the engine ignores and preserves verbatim. The server stores the canonical submitted JSON, not a re-marshalled struct.
- An optional **CEL** expression module is the escape hatch for custom rules: integer-only results, type-checked at submission, cost-limited, in a frozen environment pinned by `engine`.

**Determinism rules:** all values are integer hundredths; band checks compare sums (`sum ≥ from × n`) and never divide; bands use ascending lower bounds only; every "best N" selection names a tie-break; voto and fantavoto rounding (mode and step) is an explicit league setting.

**Versioning and audit:** settings are stored in append-only `settings_versions` (league, seq, canonical JSON, sha256, prev_sha256, schema, engine, effective_from_matchday, author, reason). They are hash-chained, so a client that caches the head hash can detect rewritten history. Results are immutable rows keyed by (settings hash, input snapshot hash, engine version). Every result carries an explain trace (which module added how many hundredths), and every recompute publishes a member-visible diff. Stage-1 fantavoti can be computed once and shared across leagues with identical player settings and inputs; the efficiency gain must be benchmarked before relying on it.

A full example document (a Classic league, and how Mantra fits the same structure) is in the research report.

**Still open:**
1. When does a matchday close: automatically (e.g. last fixture final + N hours), or by an admin action?
2. Do vice-admins get admin powers (settings changes, corrections), or a subset?
3. Ship the CEL module in v1, or after the module catalog stabilizes?
4. How long must old engine versions stay executable (forever, or N seasons)?
5. Ratings and events copied into the hashed snapshot (recommended), or referenced from the provider?
6. Is a hash chain enough tamper evidence, or are signed exports / a Merkle log needed for third-party-hosted instances?
7. Which OIDC providers to accept first, and are magic links needed too?

## 10. Decision log

- **2026-10-27: licence AGPL-3.0** (chosen by the owner).
- **2026-10-27: base voto at two-decimal precision.** `voto` is the provider rating unrounded (`6.38`). Rounding moved from the voto source into league settings.
- **2026-10-27: the server is the source of truth for leagues** and computes fantavoti from per-league settings (a brief proposal to compute fantavoti on clients was dropped, because members using different clients must get identical results).
- **2026-10-27: no retroactive rule changes**; closed matchdays change only through logged owner/admin corrections; all inputs freeze at matchday close.
- **2026-10-27: rounding is a league setting**; stage-1 results may be shared across leagues, pending a benchmark.
- **2026-10-27: settings format direction:** typed versioned modules plus a CEL escape hatch, hash-chained settings history (§9).
