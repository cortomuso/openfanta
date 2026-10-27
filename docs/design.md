# OpenFanta design note

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

### 2.4 Rule sets as data
- File: `rulesets/classic.yaml`, validated against `rulesets/schema.json`. Each file has an id (`classic`) and a semver `version`.
- **All values are integer hundredths** (`goal: 300` = +3.00), to keep results deterministic with no floats anywhere.
- Classic v1.0.0 keys: `goal`, `penalty_scored`, `penalty_missed`, `penalty_saved`, `own_goal`, `assist`, `yellow_card`, `red_card`, `red_replaces_yellow`, `gk_goal_conceded`, `gk_clean_sheet`, `gk_clean_sheet_min_minutes`, plus a `senza_voto` policy: `min_minutes`, and the voto d'ufficio given when an s.v. player has an event (e.g. goal → 600, red card → 400).
- On startup each rule set is loaded and stored in `rule_sets` with a content hash. A published `(id, version)` is **immutable**: changing a value means bumping the version. Adding a league rule set means adding a YAML file, with no code change, as long as it only uses existing keys. New *kinds* of bonus need an engine change plus a schema bump.
- `docs/rulesets.md` documents every key.

### 2.5 Versioning and audit
- `raw_payloads` is append-only, keyed by `(provider, request_key, sha256)`. A corrected upstream response becomes a new row.
- `fixture_facts`: the normalized MatchFacts, plus `facts_hash`, `normalizer_version` and the raw payload ids it came from.
- `computation_runs`: `(fixture, fixture_facts_id, rule_set_id, voto_source, engine_version, input_hash, supersedes_run_id, reason, created_at)`, immutable.
- `player_scores` belong to a run. `current_runs(fixture, rule_set, voto_source) → run_id` is moved in the same transaction that inserts the run.
- Recompute is idempotent: if `input_hash` hasn't changed, no new run is created. The history is visible at `GET /v1/fixtures/{id}/runs`.

### 2.6 VotoSource
`type VotoSource interface { Votes(ctx, facts MatchFacts) (map[PlayerID]Voto, error) }`. The engine never calls it. The shell resolves votes and passes them in as data. v1 implements `provider-rating`: Sportmonks rating 118 **unrounded, at two-decimal precision** (`6.38`, stored as 638 hundredths). The parser rounds any extra provider digits half away from zero; nothing downstream rounds voti. Rounding to halves or whole points is a league setting (see §9). Players without a rating have no voto; whether they are s.v. is decided by the rule set (`senza_voto`). A CSV import source can be added later without touching the engine.

### 2.7 API style: REST + OpenAPI, spec-first
`api/openapi.yaml` is the source of truth. It is written as OpenAPI **3.0.3**, not 3.1, because
the Go generator (oapi-codegen) and validator (kin-openapi) only fully support 3.0. Moving to 3.1
is mechanical once tooling catches up. Go server stubs are generated from it with oapi-codegen (strict server, stdlib router). Conventions are chosen so the API can open up later:
- versioned `/v1`
- opaque string IDs (UUIDv7, never provider ids; provider refs live in a mapping table)
- **decimals as strings with exactly two digits** (`"6.38"`, `"7.00"`) so third-party implementations don't hit float drift
- RFC 9457 `application/problem+json` errors
- cursor pagination, once a list can grow unbounded (today every list is bounded: one matchday,
  one season of one player)
- `ETag` / `Cache-Control`

Endpoints:
```
GET /healthz, /readyz
GET /v1/rulesets                      GET /v1/rulesets/{id}/versions/{version}
GET /v1/seasons                       GET /v1/seasons/{seasonId}/matchdays
GET /v1/seasons/{seasonId}/matchdays/{n}/scores?ruleset=classic
GET /v1/seasons/{seasonId}/players/{playerId}/scores?ruleset=classic&voto_source=provider-rating
GET /v1/fixtures/{fixtureId}/scores   GET /v1/fixtures/{fixtureId}/runs
GET /v1/players/{playerId}
```
Score object: `{player, team, fixture, matchday, role, minutes, voto|null, fantavoto|null, sv, breakdown:[{code,count,points}], run:{id, ruleset, version, voto_source, computed_at}}`.

## 3. Data model (Postgres)
`competitions, seasons, matchdays, teams, players, fixtures` (UUIDv7 PKs) · `provider_refs(entity, entity_id, provider, external_id)` UNIQUE · `raw_payloads` · `fixture_facts` · `rule_sets(id, version, content_hash, body jsonb)` · `computation_runs` · `player_scores(run_id, player_id, team_id, role, minutes, voto, fantavoto, sv, breakdown jsonb)` · `current_runs`. Voti and fantavoti are stored as `integer` hundredths.

## 4. Engine semantics (Classic)
- Input per player: role (GK or outfield, from the lineup position), minutes, goals (open play), penalties scored/missed/saved, own goals, assists, yellow, red (a second yellow counts as red), goals conceded while on the pitch, and clean sheet.
- Goals conceded and clean sheet are derived in the pure `timeline` package from events and substitution and red-card exit minutes. They are cross-checked against Sportmonks stat 88 in tests.
- `fantavoto = voto + Σ(count × value)`, or s.v. per the policy.
- Engine output includes a full breakdown per player, so every number can be audited.

## 5. Package layout
```
cmd/openfanta/            main, subcommands
internal/domain/          IDs, Points (int hundredths), MatchFacts
internal/rules/           RuleSet types, YAML load, validate, hash
internal/timeline/        pure: on-pitch intervals, goals conceded, clean sheet
internal/engine/          pure: Compute(facts, ruleset, votes) → []PlayerScore
internal/voto/            VotoSource + providerrating
internal/provider/        Provider interface, resilient HTTP client (retry/backoff, rate limit, cache)
internal/provider/sportmonks/  client, normalizer, testdata/*.json (recorded)
internal/store/           pgx + sqlc, migrations/
internal/ingest/          orchestration (fetch → store raw → normalize → compute → persist)
internal/api/             generated + handlers
internal/config/          env parsing
api/openapi.yaml  rulesets/  docs/  flake.nix  Dockerfile  compose.yaml  Makefile
```
Provider contract:
```go
type Provider interface {
    Name() string
    ListFixtures(ctx context.Context, season SeasonRef, matchday int) ([]RawPayload, error)
    FetchFixture(ctx context.Context, ref FixtureRef) (RawPayload, error)
}
type Normalizer interface { Normalize(p RawPayload) (domain.MatchFacts, error) } // pure
```

## 6. Operational
- Configuration via env (`OPENFANTA_DATABASE_URL`, `OPENFANTA_SPORTMONKS_TOKEN`, `OPENFANTA_HTTP_ADDR`, `OPENFANTA_LOG_LEVEL`). A `.env.example` is committed; `.env` is gitignored.
- `log/slog` JSON logging.
- Graceful shutdown on SIGINT/SIGTERM.
- Server timeouts: read-header 5s, write 15s, idle 60s. Upstream timeout 20s.
- Retry: exponential backoff with jitter on 5xx and network errors. Honour 429 `retry_after`.
- A token bucket keeps usage under the per-entity budget and reads `rate_limit.remaining`.
- Fixtures with status FT are cached forever in `raw_payloads` and only re-fetched with `--refresh`.

## 7. Open questions
1. **Sportmonks plan:** Serie A isn't on the free plan. Will you take the Starter plan or the 14-day trial? A token is needed to record fixtures and for the DoD real-matchday run.
2. **Player rating availability:** confirm that rating (118) is included in the chosen plan. If not, voto falls back to s.v. for everyone, and we need a different VotoSource.
3. **Redistribution:** read the Sportmonks ToS (or ask them) before any public deployment of provider-rating-based voti.
4. **Classic defaults:** confirm the s.v. policy (voto d'ufficio 6 for a goal, 4 for a red card), the clean-sheet minimum minutes (proposed: any minutes, with 0 conceded while on pitch), and the penalty-scored value (proposed 300, same as goal).
5. **Licence:** proposed Apache-2.0 for code (patent grant, friendly to third-party protocol implementers).
6. **Target season** for the real-matchday demo (proposed: current Serie A 2026/27).

